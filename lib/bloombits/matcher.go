

package bloombits

import (
	"bytes"
	"context"
	"errors"
	"math"
	"sort"
	"sync/atomic"
	"github.com/aidoc/go-aidoc/lib/bitutil"
	"github.com/aidoc/go-aidoc/lib/crypto"
)

// bloomIndexes表示bloom过滤器内属于某个键的位索引。
type bloomIndexes [3]uint

// calcBloomIndexes 返回属于给定键的bloom过滤器位索引。
func calcBloomIndexes(b []byte) bloomIndexes {
	b = crypto.Keccak256(b)

	var idxs bloomIndexes
	for i := 0; i < len(idxs); i++ {
		idxs[i] = (uint(b[2*i])<<8)&2047 + uint(b[2*i+1])
	}
	return idxs
}

//具有非零向量的partialMatches表示某些子匹配器已找到潜在匹配的部分。 随后的子匹配器将
// 二进制AND与此向量匹配。 如果vector为nil，则表示由第一个子匹配器处理的部分。
type partialMatches struct {
	section uint64
	bitset  []byte
}

// Retrieval表示对具有给定数量的获取元素的给定位的检索任务分配的请求，或对此类请求
// 的响应。 它还可以将实际结果集用作交付数据结构。
//
//如果在管道的某个路径上发生错误，则轻型客户端将使用竞争和错误字段来提前终止匹配。
type Retrieval struct {
	Bit      uint
	Sections []uint64
	Bitsets  [][]byte

	Context context.Context
	Error   error
}

// Matcher是一个流水线调度器和逻辑匹配器系统，它对比特流执行二进制AND / OR运算，创建潜在块流以检查数据内容。
type Matcher struct {
	sectionSize uint64 // 要过滤的数据批次的大小

	filters    [][]bloomIndexes    // 过滤系统匹配
	schedulers map[uint]*scheduler // 用于加载bloom位的检索调度程序

	retrievers chan chan uint       // 检索器进程等待位分配
	counters   chan chan uint       // 检索器进程等待任务计数报告
	retrievals chan chan *Retrieval // 检索器进程等待任务分配
	deliveries chan *Retrieval      // 检索器进程等待任务响应交付

	running uint32 // 原子标记会话是否有效
}

// NewMatcher创建一个新的管道，用于检索bloom比特流并对它们进行地址和主题过滤。 允许
// 将过滤器组件设置为“nil”，这将导致跳过该过滤器规则（OR 0x11 ... 1）。
func NewMatcher(sectionSize uint64, filters [][][]byte) *Matcher {
	// Create the matcher instance
	m := &Matcher{
		sectionSize: sectionSize,
		schedulers:  make(map[uint]*scheduler),
		retrievers:  make(chan chan uint),
		counters:    make(chan chan uint),
		retrievals:  make(chan chan *Retrieval),
		deliveries:  make(chan *Retrieval),
	}
	//计算我们感兴趣的组的bloom位索引
	m.filters = nil

	for _, filter := range filters {
		//收集过滤规则的位索引，特殊大小为nil过滤器
		if len(filter) == 0 {
			continue
		}
		bloomBits := make([]bloomIndexes, len(filter))
		for i, clause := range filter {
			if clause == nil {
				bloomBits = nil
				break
			}
			bloomBits[i] = calcBloomIndexes(clause)
		}
		// 如果规则 不为 空 ，则累积过滤规则
		if bloomBits != nil {
			m.filters = append(m.filters, bloomBits)
		}
	}
	//
	for _, bloomIndexLists := range m.filters {
		for _, bloomIndexList := range bloomIndexLists {
			for _, bloomIndex := range bloomIndexList {
				m.addScheduler(bloomIndex)
			}
		}
	}
	return m
}

// addScheduler为给定的位索引添加了一个位流检索调度程序，如果它之前不存在的话。
// 如果已选择该位进行过滤，则可以使用现有的调度程序。
func (m *Matcher) addScheduler(idx uint) {
	if _, ok := m.schedulers[idx]; ok {
		return
	}
	m.schedulers[idx] = newScheduler(idx)
}

// Start开始匹配过程并返回给定范围的块中的bloom匹配流。
// 如果范围中没有更多匹配项，则结果通道将关闭。
func (m *Matcher) Start(ctx context.Context, begin, end uint64, results chan uint64) (*MatcherSession, error) {
	// 确保我们没有创建并发会话
	if atomic.SwapUint32(&m.running, 1) == 1 {
		return nil, errors.New("匹配器已经运行了")
	}
	defer atomic.StoreUint32(&m.running, 0)

	// 启动新的匹配轮次
	session := &MatcherSession{
		matcher: m,
		quit:    make(chan struct{}),
		kill:    make(chan struct{}),
		ctx:     ctx,
	}
	for _, scheduler := range m.schedulers {
		scheduler.reset()
	}
	sink := m.run(begin, end, cap(results), session)

	// 读取结果接收器的输出并传递给用户
	session.pend.Add(1)
	go func() {
		defer session.pend.Done()
		defer close(results)

		for {
			select {
			case <-session.quit:
				return

			case res, ok := <-sink:
				// 找到新的匹配结果
				if !ok {
					return
				}
				// 计算该部分的第一个和最后一个块
				sectionStart := res.section * m.sectionSize

				first := sectionStart
				if begin > first {
					first = begin
				}
				last := sectionStart + m.sectionSize - 1
				if end < last {
					last = end
				}
				// 迭代该部分中的所有块并返回匹配的块
				for i := first; i <= last; i++ {
					// 如果在内部找不到匹配项，则跳过整个字节（我们正在处理整个字节！）
					next := res.bitset[(i-sectionStart)/8]
					if next == 0 {
						if i%8 == 0 {
							i += 7
						}
						continue
					}
					// 有点设置，做实际的子匹配
					if bit := 7 - i%8; next&(1<<bit) != 0 {
						select {
						case <-session.quit:
							return
						case results <- i:
						}
					}
				}
			}
		}
	}()
	return session, nil
}

// run创建一个daisy-chain的子匹配器，一个用于地址集，一个用于每个主题集，每个子匹配器只接收一个部分，如果
// 之前的所有子匹配器都在其中一个块中找到了潜在的匹配。 部分，然后二进制和它自己的匹配并将结果转发到下一个。
//
//该方法开始将节索引提供给新goroutine上的第一个子匹配器，并返回接收结果的接收器通道。
func (m *Matcher) run(begin, end uint64, buffer int, session *MatcherSession) chan *partialMatches {
	// 创建源 aidoc 和 feed 部分索引
	source := make(chan *partialMatches, buffer)

	session.pend.Add(1)
	go func() {
		defer session.pend.Done()
		defer close(source)

		for i := begin / m.sectionSize; i <= end/m.sectionSize; i++ {
			select {
			case <-session.quit:
				return
			case source <- &partialMatches{i, bytes.Repeat([]byte{0xff}, int(m.sectionSize/8))}:
			}
		}
	}()
	// 组装daisy-chain式过滤管道
	next := source
	dist := make(chan *request, buffer)

	for _, bloom := range m.filters {
		next = m.subMatch(next, dist, bloom, session)
	}
	// 启动请求分发
	session.pend.Add(1)
	go m.distributor(dist, session)

	return next
}

// subMatch创建一个子匹配器，用于过滤一组地址或主题，二进制OR-s匹配，然后二进制AND-s结果到daisy-chain输入（源）并将
// 其转发到 daisy-chain 输出。
// 通过获取属于该地址/主题的三个bloom比特索引的给定部分，并将这些矢量二进制和，来计算每个地址/主题的匹配。
func (m *Matcher) subMatch(source chan *partialMatches, dist chan *request, bloom []bloomIndexes, session *MatcherSession) chan *partialMatches {
	//  为bloom过滤器所需的每个位启动并发调度程序
	sectionSources := make([][3]chan uint64, len(bloom))
	sectionSinks := make([][3]chan []byte, len(bloom))
	for i, bits := range bloom {
		for j, bit := range bits {
			sectionSources[i][j] = make(chan uint64, cap(source))
			sectionSinks[i][j] = make(chan []byte, cap(source))

			m.schedulers[bit].run(sectionSources[i][j], dist, sectionSinks[i][j], session.quit, &session.pend)
		}
	}

	process := make(chan *partialMatches, cap(source)) // 来自源的条目在启动提取后在此处转发
	results := make(chan *partialMatches, cap(source))

	session.pend.Add(2)
	go func() {
		// 撕下goroutine并终止所有源通道
		defer session.pend.Done()
		defer close(process)

		defer func() {
			for _, bloomSources := range sectionSources {
				for _, bitSource := range bloomSources {
					close(bitSource)
				}
			}
		}()
		//  读取源通道中的部分并复用到所有位调度程序中
		for {
			select {
			case <-session.quit:
				return

			case subres, ok := <-source:
				// 来自先前链接的新子结果
				if !ok {
					return
				}
				// 将段索引复用到所有位调度程序
				for _, bloomSources := range sectionSources {
					for _, bitSource := range bloomSources {
						select {
						case <-session.quit:
							return
						case bitSource <- subres.section:
						}
					}
				}
				// 通知处理器此部分将可用
				select {
				case <-session.quit:
					return
				case process <- subres:
				}
			}
		}
	}()

	go func() {
		// 撕下 goroutine 并终止最终的下沉通道
		defer session.pend.Done()
		defer close(results)

		// 阅读源通知并收集交付的结果
		for {
			select {
			case <-session.quit:
				return

			case subres, ok := <-process:
				//  通知被检索的部分
				if !ok {
					return
				}
				// 收集所有子结果并将它们合并在一起
				var orVector []byte
				for _, bloomSinks := range sectionSinks {
					var andVector []byte
					for _, bitSink := range bloomSinks {
						var data []byte
						select {
						case <-session.quit:
							return
						case data = <-bitSink:
						}
						if andVector == nil {
							andVector = make([]byte, int(m.sectionSize/8))
							copy(andVector, data)
						} else {
							bitutil.ANDBytes(andVector, andVector, data)
						}
					}
					if orVector == nil {
						orVector = andVector
					} else {
						bitutil.ORBytes(orVector, orVector, andVector)
					}
				}

				if orVector == nil {
					orVector = make([]byte, int(m.sectionSize/8))
				}
				if subres.bitset != nil {
					bitutil.ANDBytes(orVector, orVector, subres.bitset)
				}
				if bitutil.TestBytes(orVector) {
					select {
					case <-session.quit:
						return
					case results <- &partialMatches{subres.section, orVector}:
					}
				}
			}
		}
	}()
	return results
}

// distributor 从调度程序接收请求，并将它们排入一组待处理请求，这些请求被分配给想要实现它们的检索器。
func (m *Matcher) distributor(dist chan *request, session *MatcherSession) {
	defer session.pend.Done()

	var (
		requests   = make(map[uint][]uint64) // 每个部分请求列表，按部分编号排序
		unallocs   = make(map[uint]struct{}) // 具有待处理请求但未分配给任何检索器的位
		retrievers chan chan uint            // 等待检索器（如果unallocs为空，则切换为nil）
	)
	var (
		allocs   int            // 处理正常关闭请求的活动分配数
		shutdown = session.quit // 关机请求通道，将正常等待待处理的请求
	)

	// assign 是一种辅助方法，用于尝试将待处理位分配给主动侦听服务器，或者在一个到达时将其安排好。
	assign := func(bit uint) {
		select {
		case fetcher := <-m.retrievers:
			allocs++
			fetcher <- bit
		default:
			// 没有活动的检索器，开始侦听新的检索器
			retrievers = m.retrievers
			unallocs[bit] = struct{}{}
		}
	}

	for {
		select {
		case <-shutdown:
			// 请求正常关闭，等待所有待处理请求得到兑现
			if allocs == 0 {
				return
			}
			shutdown = nil

		case <-session.kill:
			// 待处理的请求未及时兑现，难以终止
			return

		case req := <-dist:
			// 新的检索请求到达以分发给某个提取程序进程
			queue := requests[req.bit]
			index := sort.Search(len(queue), func(i int) bool { return queue[i] >= req.section })
			requests[req.bit] = append(queue[:index], append([]uint64{req.section}, queue[index:]...)...)

			// 如果它是一个新的位并且我们有等待的提取器，则分配给它们
			if len(queue) == 0 {
				assign(req.bit)
			}

		case fetcher := <-retrievers:
			// 新的检索器到达，找到要分配的最低部分位
			bit, best := uint(0), uint64(math.MaxUint64)
			for idx := range unallocs {
				if requests[idx][0] < best {
					bit, best = idx, requests[idx][0]
				}
			}
			// 停止跟踪此位（如果没有更多工作可用则分配通知）
			delete(unallocs, bit)
			if len(unallocs) == 0 {
				retrievers = nil
			}
			allocs++
			fetcher <- bit

		case fetcher := <-m.counters:
			// 新任务计数请求到达，返回项目数
			fetcher <- uint(len(requests[<-fetcher]))

		case fetcher := <-m.retrievals:
			// 新的fetcher等待任务检索，分配
			task := <-fetcher
			if want := len(task.Sections); want >= len(requests[task.Bit]) {
				task.Sections = requests[task.Bit]
				delete(requests, task.Bit)
			} else {
				task.Sections = append(task.Sections[:0], requests[task.Bit][:want]...)
				requests[task.Bit] = append(requests[task.Bit][:0], requests[task.Bit][want:]...)
			}
			fetcher <- task

			// 如果有任何未分配的内容，请尝试分配给其他人
			if len(requests[task.Bit]) > 0 {
				assign(task.Bit)
			}

		case result := <-m.deliveries:
			// 来自 fetcher 的新检索任务响应，拆分缺失的部分并提供完整的部分
			var (
				sections = make([]uint64, 0, len(result.Sections))
				bitsets  = make([][]byte, 0, len(result.Bitsets))
				missing  = make([]uint64, 0, len(result.Sections))
			)
			for i, bitset := range result.Bitsets {
				if len(bitset) == 0 {
					missing = append(missing, result.Sections[i])
					continue
				}
				sections = append(sections, result.Sections[i])
				bitsets = append(bitsets, bitset)
			}
			m.schedulers[result.Bit].deliver(sections, bitsets)
			allocs--

			// 重新安排丢失的部分并在新的可用时分配位
			if len(missing) > 0 {
				queue := requests[result.Bit]
				for _, section := range missing {
					index := sort.Search(len(queue), func(i int) bool { return queue[i] >= section })
					queue = append(queue[:index], append([]uint64{section}, queue[index:]...)...)
				}
				requests[result.Bit] = queue

				if len(queue) == len(missing) {
					assign(result.Bit)
				}
			}
			// 重新安排丢失的部分并在新的可用时分配位
			if allocs == 0 && shutdown == nil {
				return
			}
		}
	}
}

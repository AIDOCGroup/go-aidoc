package bloombits

import (
	"sync"
)

// request表示一个bloom检索任务，用于优先处理和从本地数据库中提取或从网络远程提取。
type request struct {
	section uint64 // 用于从中检索位向量的节索引
	bit     uint   // 该部分内的位索引检索向量
}

// response 表示通过调度程序请求的位向量的状态。
type response struct {
	cached []byte        // 用于重复删除多个请求的缓存位
	done   chan struct{} // 允许等待完成的频道
}

// scheduler处理属于单个bloom位的整个section-batch的bloom-filter检索操作的调度。 除了调
// 度检索操作之外，此结构还对请求进行重复数据删除并缓存结果，以便在复杂的过滤方案中最小化网络/数据库开销。
type scheduler struct {
	bit       uint                 // 此调度程序负责的bloom过滤器中的位索引
	responses map[uint64]*response // 当前待处理的检索请求或已缓存的响应
	lock      sync.Mutex           // 锁定保护响应并发访问
}

// newScheduler 为特定的位索引创建一个新的 bloom-filter 检索调度程序。
func newScheduler(idx uint) *scheduler {
	return &scheduler{
		bit:       idx,
		responses: make(map[uint64]*response),
	}
}

// run创建一个检索管道，从部分接收部分索引，并通过完成通道以相同的顺序返回结果。
// 允许同时运行相同的调度程序，从而导致检索任务重复数据删除。
func (s *scheduler) run(sections chan uint64, dist chan *request, done chan []byte, quit chan struct{}, wg *sync.WaitGroup) {
	// 在请求和与分发通道大小相同的响应之间创建转发器通道（因为这将区块管道）。
	pend := make(chan uint64, cap(dist))

	// 启动管道调度用户之间转发 - >总代理 - >用户 （forward between user -> distributor -> user）
	wg.Add(2)
	go s.scheduleRequests(sections, dist, pend, quit, wg)
	go s.scheduleDeliveries(pend, done, quit, wg)
}

// reset清除以前运行的任何剩余物。
// 这在重新启动之前是必需的，以确保先前请求但从未传递的状态将导致锁定。
func (s *scheduler) reset() {
	s.lock.Lock()
	defer s.lock.Unlock()

	for section, res := range s.responses {
		if res.cached == nil {
			delete(s.responses, section)
		}
	}
}

// scheduleRequests从输入通道读取部分检索请求，对流进行重复数据删除，并将唯一检索任务推送到数据库或网络层的分发通道中。
func (s *scheduler) scheduleRequests(reqs chan uint64, dist chan *request, pend chan uint64, quit chan struct{}, wg *sync.WaitGroup) {
	// Clean up the goroutine and pipeline when done
	defer wg.Done()
	defer close(pend)

	// 继续阅读和安排部分请求
	for {
		select {
		case <-quit:
			return

		case section, ok := <-reqs:
			// 请求新的部分检索
			if !ok {
				return
			}
			// 重复数据删除检索请求
			unique := false

			s.lock.Lock()
			if s.responses[section] == nil {
				s.responses[section] = &response{
					done: make(chan struct{}),
				}
				unique = true
			}
			s.lock.Unlock()

			// 安排该部分进行检索，并通知交付者预期此部分
			if unique {
				select {
				case <-quit:
					return
				case dist <- &request{bit: s.bit, section: section}:
				}
			}
			select {
			case <-quit:
				return
			case pend <- section:
			}
		}
	}
}
// scheduleDeliveries读取部分接受通知并等待它们被传递，将它们推送到输出数据缓冲区。
func (s *scheduler) scheduleDeliveries(pend chan uint64, done chan []byte, quit chan struct{}, wg *sync.WaitGroup) {
	// Clean up the goroutine and pipeline when done
	defer wg.Done()
	defer close(done)

	// 继续阅读通知和安排交付
	for {
		select {
		case <-quit:
			return

		case idx, ok := <-pend:
			// 新部分检索待处理
			if !ok {
				return
			}
			// 等到请求得到兑现
			s.lock.Lock()
			res := s.responses[idx]
			s.lock.Unlock()

			select {
			case <-quit:
				return
			case <-res.done:
			}
			// 交付结果
			select {
			case <-quit:
				return
			case done <- res.cached:
			}
		}
	}
}

//当请求的回复到达时，请求分发者调用 deliver。
func (s *scheduler) deliver(sections []uint64, data [][]byte) {
	s.lock.Lock()
	defer s.lock.Unlock()

	for i, section := range sections {
		if res := s.responses[section]; res != nil && res.cached == nil { //  避免非请求和双重交付
			res.cached = data[i]
			close(res.done)
		}
	}
}

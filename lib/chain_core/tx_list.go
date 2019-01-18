package chain_core

import (
	"container/heap"
	"math"
	"math/big"
	"sort"

	"github.com/aidoc/go-aidoc/lib/chain_common"
	"github.com/aidoc/go-aidoc/lib/chain_core/types"
	"github.com/aidoc/go-aidoc/lib/logger"
)

// nonceHeap是一个heap.Interface实现，超过64位无符号整数，用于从可能有缺口的未来队列中检索已排序的交易。
type nonceHeap []uint64

func (h nonceHeap) Len() int           { return len(h) }
func (h nonceHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h nonceHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *nonceHeap) Push(x interface{}) {
	*h = append(*h, x.(uint64))
}

func (h *nonceHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// txSortedMap是一个nonce->交易哈希映射，带有基于堆的索引，允许以递增方式迭代内容。
type txSortedMap struct {
	items map[uint64]*types.Transaction // 存储交易数据的哈希映射
	index *nonceHeap                    // 所有存储交易的nonce堆（非严格模式）
	cache types.Transactions            // 已排序的交易缓存
}
// newTxSortedMap创建一个新的随机数排序的交易映射。
func newTxSortedMap() *txSortedMap {
	return &txSortedMap{
		items: make(map[uint64]*types.Transaction),
		index: new(nonceHeap),
	}
}
// 获取检索与给定nonce关联的当前交易。
func (m *txSortedMap) Get(nonce uint64) *types.Transaction {
	return m.items[nonce]
}
// 将一个新交易插入到地图中，同时更新地图的nonce索引。 如果已存在具有相同nonce的交易，则会覆盖该交易。
func (m *txSortedMap) Put(tx *types.Transaction) {
	nonce := tx.Nonce()
	if m.items[nonce] == nil {
		heap.Push(m.index, nonce)
	}
	m.items[nonce], m.cache = tx, nil
}
// Forward使用低于提供的阈值的nonce从地图中删除所有交易。 对于任何删除后维护，都会返回每个已删除的交易。
func (m *txSortedMap) Forward(threshold uint64) types.Transactions {
	var removed types.Transactions
	//弹出堆项目，直到达到阈值
	for m.index.Len() > 0 && (*m.index)[0] < threshold {
		nonce := heap.Pop(m.index).(uint64)
		removed = append(removed, m.items[nonce])
		delete(m.items, nonce)
	}
	//如果我们有一个缓存的订单，请转移前面
	if m.cache != nil {
		m.cache = m.cache[len(removed):]
	}
	return removed
}
//过滤迭代交易列表并删除指定函数计算结果为true的所有交易。
func (m *txSortedMap) Filter(filter func(*types.Transaction) bool) types.Transactions {
	var removed types.Transactions
	//收集所有要过滤掉的交易
	for nonce, tx := range m.items {
		if filter(tx) {
			removed = append(removed, tx)
			delete(m.items, nonce)
		}
	}
	//如果删除了交易，则会破坏堆和缓存
	if len(removed) > 0 {
		*m.index = make([]uint64, 0, len(m.items))
		for nonce := range m.items {
			*m.index = append(*m.index, nonce)
		}
		heap.Init(m.index)

		m.cache = nil
	}
	return removed
}

// Cap对项目数量设置了一个硬限制，返回超过该限制的所有交易。
func (m *txSortedMap) Cap(threshold int) types.Transactions {
	//如果项目数量低于限制，则短路
	if len(m.items) <= threshold {
		return nil
	}
	//否则收集并删除最高的nonce'd交易
	var drops types.Transactions

	sort.Sort(*m.index)
	for size := len(m.items); size > threshold; size-- {
		drops = append(drops, m.items[(*m.index)[size-1]])
		delete(m.items, (*m.index)[size-1])
	}
	*m.index = (*m.index)[:threshold]
	heap.Init(m.index)

	// 如果我们有一个缓存，请向后移动
	if m.cache != nil {
		m.cache = m.cache[:len(m.cache)-len(drops)]
	}
	return drops
}
// Remove 从维护的地图中删除一个交易，返回是否找到了该交易。
func (m *txSortedMap) Remove(nonce uint64) bool {
	// 如果没有交易，则短路
	_, ok := m.items[nonce]
	if !ok {
		return false
	}
	// 否则删除交易并修复堆索引
	for i := 0; i < m.index.Len(); i++ {
		if (*m.index)[i] == nonce {
			heap.Remove(m.index, i)
			break
		}
	}
	delete(m.items, nonce)
	m.cache = nil

	return true
}

// Ready准备从提供的已准备好处理的现时开始检索顺序增加的交易列表。 返回的交易将从列表中删除。
//
//注意，还会返回所有nonce低于start的交易，以防止进入和无效状态。 这不是应该发生的事情，但更好的是自我纠正而不是失败！
func (m *txSortedMap) Ready(start uint64) types.Transactions {
	// 如果没有可用的交易，则短路
	if m.index.Len() == 0 || (*m.index)[0] > start {
		return nil
	}
	// 否则开始累积增量交易
	var ready types.Transactions
	for next := (*m.index)[0]; m.index.Len() > 0 && (*m.index)[0] == next; next++ {
		ready = append(ready, m.items[next])
		delete(m.items, next)
		heap.Pop(m.index)
	}
	m.cache = nil

	return ready
}

// Len返回交易映射的长度。
func (m *txSortedMap) Len() int {
	return len(m.items)
}
// Flatten根据松散排序的内部表示创建一个随机数排序的交易片。 如果在对内容进行任何修改之前再次请求，则对缓存的结果进行缓存。
func (m *txSortedMap) Flatten() types.Transactions {
	// 如果尚未缓存排序，请创建并缓存它
	if m.cache == nil {
		m.cache = make(types.Transactions, 0, len(m.items))
		for _, tx := range m.items {
			m.cache = append(m.cache, tx)
		}
		sort.Sort(types.TxByNonce(m.cache))
	}
	// 复制缓存以防止意外修改
	txs := make(types.Transactions, len(m.cache))
	copy(txs, m.cache)
	return txs
}

// txList是属于账户的交易的“列表”，按账户nonce排序。 相同类型可用于存储可执行/挂起队列的连续交易;
// 并且用于存储不可执行/未来队列的间隙交易，并且具有微小的行为改变。
type txList struct {
	strict bool         // 无论是随机数是严格的连续或不连续
	txs    *txSortedMap // 堆索引已排序的交易哈希映射

	costcap *big.Int // 最高成本交易的价格（仅在超出余额时重置）
	gascap  uint64   // 最高支出交易的 gas 限制（仅在超过限额时重置）
}

// newTxList创建一个新的交易列表，用于维护可随意索引的快速，有缺口，可排序的交易列表。
func newTxList(strict bool) *txList {
	return &txList{
		strict:  strict,
		txs:     newTxSortedMap(),
		costcap: new(big.Int),
	}
}

// Overlaps 返回指定的交易是否与列表中已包含的交易具有相同的nonce。
func (l *txList) Overlaps(tx *types.Transaction) bool {
	return l.txs.Get(tx.Nonce()) != nil
}

//添加尝试将新交易插入列表，返回交易是否被接受，如果是，则替换之前的任何交易。
//
//如果新交易被接受到列表中，则列表的成本和gas阈值也可能会更新。
func (l *txList) Add(tx *types.Transaction, priceBump uint64) (bool, *types.Transaction) {
	// 如果有较旧的更好的交易，则中止
	old := l.txs.Get(tx.Nonce())
	logger.Info("tx_list.go Add()" , "old" , old)
	if old != nil {
		threshold := new(big.Int).Div(new(big.Int).Mul(old.GasPrice(), big.NewInt(100+int64(priceBump))), big.NewInt(100))
		// 必须确保新的 gas 价格高于旧 gas 价格
		// 价格以及检查百分比阈值以确保这一点
		// 这对于低（dose 级） gas 价格替代是准确的
		if old.GasPrice().Cmp(tx.GasPrice()) >= 0 || threshold.Cmp(tx.GasPrice()) > 0 {
			return false, nil
		}
	}
	// 否则用当前交易覆盖旧交易
	l.txs.Put(tx)
	if cost := tx.Cost(); l.costcap.Cmp(cost) < 0 {
		l.costcap = cost
	}
	if gas := tx.Gas(); l.gascap < gas {
		l.gascap = gas
	}
	return true, old
}
// Forward使用低于提供的阈值的nonce从列表中删除所有交易。 对于任何删除后维护，都会返回每个已删除的交易。
func (l *txList) Forward(threshold uint64) types.Transactions {
	return l.txs.Forward(threshold)
}
//过滤器从列表中删除所有交易，其成本或gas限制高于提供的阈值。 对于任何删除后维护，都会返回每个已删除的交易。 还返回严格模式的无效交易。
//
//此方法使用缓存的costcap和gascap快速确定是否有计算所有成本的余额，或者余额是否涵盖所有成本。 如果阈值低于成本限额上限，则在删除新的无效交易后，上限将重置为新的高。
func (l *txList) Filter(costLimit *big.Int, gasLimit uint64) (types.Transactions, types.Transactions) {
	// 如果所有交易都低于阈值，则短路
	if l.costcap.Cmp(costLimit) <= 0 && l.gascap <= gasLimit {
		return nil, nil
	}
	l.costcap = new(big.Int).Set(costLimit) // 将上限降低到阈值
	l.gascap = gasLimit
	//过滤掉账户资金以上的所有交易
	removed := l.txs.Filter(func(tx *types.Transaction) bool { return tx.Cost().Cmp(costLimit) > 0 || tx.Gas() > gasLimit })
	//如果列表是严格的，则过滤最低nonce之上的任何内容
	var invalids types.Transactions

	if l.strict && len(removed) > 0 {
		lowest := uint64(math.MaxUint64)
		for _, tx := range removed {
			if nonce := tx.Nonce(); lowest > nonce {
				lowest = nonce
			}
		}
		invalids = l.txs.Filter(func(tx *types.Transaction) bool { return tx.Nonce() > lowest })
	}
	return removed, invalids
}
// Cap对项目数量设置了一个硬限制，返回超过该限制的所有交易。
func (l *txList) Cap(threshold int) types.Transactions {
	return l.txs.Cap(threshold)
}

// Remove从维护列表中删除交易，返回是否找到交易，还返回因删除而无效的任何交易（仅限严格模式）。
func (l *txList) Remove(tx *types.Transaction) (bool, types.Transactions) {
	//  从集合中删除交易
	nonce := tx.Nonce()
	if removed := l.txs.Remove(nonce); !removed {
		return false, nil
	}
	// 在严格模式下，过滤掉不可执行的交易
	if l.strict {
		return true, l.txs.Filter(func(tx *types.Transaction) bool { return tx.Nonce() > nonce })
	}
	return true, nil
}

// Ready准备从提供的已准备好处理的现时开始检索顺序增加的交易列表。 返回的交易将从列表中删除。
//
//注意，还会返回所有nonce低于start的交易，以防止进入和无效状态。 这不是应该发生的事情，但更好的是自我纠正而不是失败！
func (l *txList) Ready(start uint64) types.Transactions {
	return l.txs.Ready(start)
}
// Len返回交易列表的长度。
func (l *txList) Len() int {
	return l.txs.Len()
}
// Empty返回交易列表是否为空。
func (l *txList) Empty() bool {
	return l.Len() == 0
}
// Flatten根据松散排序的内部表示创建一个随机数排序的交易片。 如果在对内容进行任何修改之前再次请求，则对缓存的结果进行缓存。
func (l *txList) Flatten() types.Transactions {
	return l.txs.Flatten()
}
// priceHeap是一个heap.Interface实现的交易，用于检索在池填满时丢弃的价格分类交易。
type priceHeap []*types.Transaction

func (h priceHeap) Len() int      { return len(h) }
func (h priceHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h priceHeap) Less(i, j int) bool {
	//主要按价格排序，返回更便宜的价格
	switch h[i].GasPrice().Cmp(h[j].GasPrice()) {
	case -1:
		return true
	case 1:
		return false
	}
	//如果价格匹配，则通过nonce稳定（高nonce更糟）
	return h[i].Nonce() > h[j].Nonce()
}

func (h *priceHeap) Push(x interface{}) {
	*h = append(*h, x.(*types.Transaction))
}

func (h *priceHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
// txPricedList是一个价格排序堆，允许以价格递增的方式对交易池内容进行操作。
type txPricedList struct {
	all    *txLookup  // 指向所有交易 map 的指针
	items  *priceHeap // 所有存储的交易的价格堆
	stales int        // 过期价格点数（重堆积触发器）
}
// newTxPricedList 创建一个新的按价格排序的交易堆。
func newTxPricedList(all *txLookup) *txPricedList {
	return &txPricedList {
		all:   all,
		items: new(priceHeap),
	}
}
//将新交易插入堆中。
func (l *txPricedList) Put(tx *types.Transaction) {
	heap.Push(l.items, tx)
}
// Removed通知价格交易列表旧的交易从池中删除。 该列表将保留一个陈旧对象的计数器，并在足够大的交易比例过时时更新堆。
func (l *txPricedList) Removed() {
	// 冲撞过时的计数器，但如果仍然太低（<25％）则退出
	l.stales++
	if l.stales <= len(*l.items)/4 {
		return
	}
	// 似乎我们已经达到了关键数量的陈旧交易，重新调整
	reheap := make(priceHeap, 0, l.all.Count())

	l.stales, l.items = 0, &reheap
	l.all.Range(func(hash chain_common.Hash, tx *types.Transaction) bool {
		*l.items = append(*l.items, tx)
		return true
	})
	heap.Init(l.items)
}

// Cap 找到低于给定价格阈值的所有交易，将它们从定价列表中删除，然后重新将它们从整个池中删除。
func (l *txPricedList) Cap(threshold *big.Int, local *accountSet) types.Transactions {
	drop := make(types.Transactions, 0, 128) // Remote underpriced transactions to drop
	save := make(types.Transactions, 0, 64)  // Local underpriced transactions to keep

	for len(*l.items) > 0 {
		//如果在清理过程中发现，则丢弃陈旧的交易
		tx := heap.Pop(l.items).(*types.Transaction)
		if l.all.Get(tx.Hash()) == nil {
			l.stales--
			continue
		}
		//如果达到阈值，请停止丢弃
		if tx.GasPrice().Cmp(threshold) >= 0 {
			save = append(save, tx)
			break
		}
		//找到非陈旧的交易，除非是本地的，否则丢弃
		if local.containsTx(tx) {
			save = append(save, tx)
		} else {
			drop = append(drop, tx)
		}
	}
	for _, tx := range save {
		heap.Push(l.items, tx)
	}
	return drop
}

// 低价检查交易是否比当前跟踪的最低价交易便宜（或便宜）。
func (l *txPricedList) Underpriced(tx *types.Transaction, local *accountSet) bool {
	//本地交易不能低估
	if local.containsTx(tx) {
		return false
	}
	//如果在堆开始时找到陈旧的价格点，则丢弃
	for len(*l.items) > 0 {
		head := []*types.Transaction(*l.items)[0]
		if l.all.Get(head.Hash()) == nil {
			l.stales--
			heap.Pop(l.items)
			continue
		}
		break
	}
	//检查交易是否定价过低
	if len(*l.items) == 0 {
		logger.Error("查询定价池为空") // 这不可能发生，打印以捕获编程错误
		return false
	}
	cheapest := []*types.Transaction(*l.items)[0]
	return cheapest.GasPrice().Cmp(tx.GasPrice()) >= 0
}
// Discard发现了许多价格最低的交易，将它们从定价列表中删除并返回它们以便从整个池中进一步删除。
func (l *txPricedList) Discard(count int, local *accountSet) types.Transactions {
	drop := make(types.Transactions, 0, count) // 远程低价交易下降
	save := make(types.Transactions, 0, 64)    // 本地低价交易保持不变

	for len(*l.items) > 0 && count > 0 {
		// 如果在清理过程中发现，则丢弃陈旧的交易
		tx := heap.Pop(l.items).(*types.Transaction)
		if l.all.Get(tx.Hash()) == nil {
			l.stales--
			continue
		}
		// 找到非陈旧的交易，除非是本地的，否则丢弃
		if local.containsTx(tx) {
			save = append(save, tx)
		} else {
			drop = append(drop, tx)
			count--
		}
	}
	for _, tx := range save {
		heap.Push(l.items, tx)
	}
	return drop
}

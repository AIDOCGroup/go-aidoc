package chain_core

import (
	crand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	mrand "math/rand"
	"sync/atomic"
	"time"

	"github.com/aidoc/go-aidoc/lib/chain_common"
	"github.com/aidoc/go-aidoc/lib/rawdb"
	"github.com/aidoc/go-aidoc/lib/chain_core/types"
	"github.com/aidoc/go-aidoc/service/db_model"
	"github.com/aidoc/go-aidoc/lib/logger"
	"github.com/aidoc/go-aidoc/configs"
	"github.com/hashicorp/golang-lru"
	"github.com/aidoc/go-aidoc/service/produce/consensus"
	"github.com/aidoc/go-aidoc/lib/i18"
)

const (
	headerCacheLimit = 512
	tdCacheLimit     = 1024
	numberCacheLimit = 2048
)
// HeaderChain实现了core.BlockChain和light.LightChain共享的基本块头链逻辑。 它本身不可用，
// 只作为任一结构的一部分。 它也不是线程安全的，封装链结构应该进行必要的互斥锁定/解锁。
type HeaderChain struct {
	config *configs.ChainConfig

	chainDb       db_model.Database
	genesisHeader *types.Header

	currentHeader     atomic.Value      // 标题链的当前头部（可能在块链上方！）
	currentHeaderHash chain_common.Hash // 标题链当前头部的哈希值（防止重新计算

	headerCache *lru.Cache // 缓存最新的块头
	tdCache     *lru.Cache // 缓存最近的块总难度
	numberCache *lru.Cache // 缓存最新的块编号

	procInterrupt func() bool

	rand   *mrand.Rand
	engine consensus.Engine
}
// NewHeaderChain 创建一个新的HeaderChain结构。 getValidator 应该返回父的验证器procInterrupt指向父级的中断信号量wg指向父级的关闭等待组
func NewHeaderChain(chainDb db_model.Database, config *configs.ChainConfig, engine consensus.Engine, procInterrupt func() bool) (*HeaderChain, error) {
	headerCache, _ := lru.New(headerCacheLimit)
	tdCache, _ := lru.New(tdCacheLimit)
	numberCache, _ := lru.New(numberCacheLimit)

	// 种子快速但加密的始发随机发生器
	seed, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return nil, err
	}

	hc := &HeaderChain{
		config:        config,
		chainDb:       chainDb,
		headerCache:   headerCache,
		tdCache:       tdCache,
		numberCache:   numberCache,
		procInterrupt: procInterrupt,
		rand:          mrand.New(mrand.NewSource(seed.Int64())),
		engine:        engine,
	}

	hc.genesisHeader = hc.GetHeaderByNumber(0)
	if hc.genesisHeader == nil {
		return nil, ErrNoGenesis
	}

	hc.currentHeader.Store(hc.genesisHeader)
	if head := rawdb.ReadHeadBlockHash(chainDb); head != (chain_common.Hash{}) {
		if chead := hc.GetHeaderByHash(head); chead != nil {
			hc.currentHeader.Store(chead)
		}
	}
	hc.currentHeaderHash = hc.CurrentHeader().Hash()

	return hc, nil
}
// GetBlockNumber从缓存或数据库中检索属于给定哈希的块编号
func (hc *HeaderChain) GetBlockNumber(hash chain_common.Hash) *uint64 {
	if cached, ok := hc.numberCache.Get(hash); ok {
		number := cached.(uint64)
		return &number
	}
	number := rawdb.ReadHeaderNumber(hc.chainDb, hash)
	if number != nil {
		hc.numberCache.Add(hash, *number)
	}
	return number
}
// WriteHeader将标头写入本地链，因为它的父节点已知。 如果新插入的报头的总难度变得大于当前已知的TD，则重新路由规范链。
//
// 注意：此方法在将链同时插入链中时不是并发安全的，因为重组导致的副作用无法在没有实际块的情况下进行模拟。 因此，
// 直接编写标题只能在两种情况下完成：纯标题操作模式（轻客户端）或正确分隔的标题/块阶段（非归档客户端）。
func (hc *HeaderChain) WriteHeader(header *types.Header) (status WriteStatus, err error) {
	// 缓存一些值以防止不断重新计算
	var (
		hash   = header.Hash()
		number = header.Number.Uint64()
	)
	// 计算标题的总难度
	ptd := hc.GetTd(header.ParentHash, number-1)
	if ptd == nil {
		return NonStatTy, consensus.ErrUnknownAncestor
	}
	localTd := hc.GetTd(hc.currentHeaderHash, hc.CurrentHeader().Number.Uint64())
	externTd := new(big.Int).Add(header.Difficulty, ptd)

	// 与规范状态无关，将 td 和 header 写入数据库
	if err := hc.WriteTd(hash, number, externTd); err != nil {
		logger.Crit("无法写入标题总难度",   err.Error())
	}
	rawdb.WriteHeader(hc.chainDb, header)
	// 如果总难度高于我们已知的，则将其添加到规范链中if语句中的第二个子句减少了自私挖掘的漏洞。
	// 请参阅http://www.cs.cornell.edu/~ie53/publications/btcProcFC.pdf
	if externTd.Cmp(localTd) > 0 || (externTd.Cmp(localTd) == 0 && mrand.Float64() < 0.5) {
		// 删除新头上方的任何规范数字分配
		for i := number + 1; ; i++ {
			hash := rawdb.ReadCanonicalHash(hc.chainDb, i)
			if hash == (chain_common.Hash{}) {
				break
			}
			rawdb.DeleteCanonicalHash(hc.chainDb, i)
		}
		// 覆盖任何过时的规范号码分配
		var (
			headHash   = header.ParentHash
			headNumber = header.Number.Uint64() - 1
			headHeader = hc.GetHeader(headHash, headNumber)
		)
		for rawdb.ReadCanonicalHash(hc.chainDb, headNumber) != headHash {
			rawdb.WriteCanonicalHash(hc.chainDb, headHash, headNumber)

			headHash = headHeader.ParentHash
			headNumber = headHeader.Number.Uint64() - 1
			headHeader = hc.GetHeader(headHash, headNumber)
		}
		// 使用新标头扩展规范链
		rawdb.WriteCanonicalHash(hc.chainDb, hash, number)
		rawdb.WriteHeadHeaderHash(hc.chainDb, hash)

		hc.currentHeaderHash = hash
		hc.currentHeader.Store(types.CopyHeader(header))

		status = CanonStatTy
	} else {
		status = SideStatTy
	}

	hc.headerCache.Add(hash, header)
	hc.numberCache.Add(hash, number)

	return
}
// WhCallback是一个用于插入单个标头的回调函数。 使用回调有两个原因：首先，在LightChain中，
// 应处理状态并发送轻链事件，而在BlockChain中这不是必需的，因为在插入块之后发送链事件。
// 其次，标题写入应该由父链互斥分别保护。
type WhCallback func(*types.Header) error

func (hc *HeaderChain) ValidateHeaderChain(chain []*types.Header, checkFreq int) (int, error) {
	// 进行健全性检查，确保所提供的链实际已订购和链接
	for i := 1; i < len(chain); i++ {
		if chain[i].Number.Uint64() != chain[i-1].Number.Uint64()+1 || chain[i].ParentHash != chain[i-1].Hash() {
			// 链打破了祖先，记录消息（编程错误）并跳过插入
			logger.Error("不连续的标头插入", "编号", chain[i].Number, "哈希", chain[i].Hash(),
				"parent", chain[i].ParentHash, "prevnumber", chain[i-1].Number, "prevhash", chain[i-1].Hash())

			return 0, fmt.Errorf( i18.I18_print.Sprintf("非连续插入：项目 %d 是 #%d [%x…], 项目 %d 是 #%d [%x…] (父哈希 [%x…])", i-1, chain[i-1].Number,
				chain[i-1].Hash().Bytes()[:4], i, chain[i].Number, chain[i].Hash().Bytes()[:4], chain[i].ParentHash[:4]))
		}
	}
	
	abort, results := hc.engine.VerifyHeaders(hc, chain)
	defer close(abort)
	// 迭代标题并确保它们全部结账
	for i, header := range chain {
		// 如果链正在终止，则停止处理块
		if hc.procInterrupt() {
			logger.Debug("标头验证过早中止")
			return 0, errors.New("中止")
		}
		// 如果标题是禁止标题，则直接中止
		if BadHashes[header.Hash()] {
			return i, ErrBlacklistedHash
		}
		// 否则等待标题检查并确保它们通过
		if err := <-results; err != nil {
			return i, err
		}
	}

	return 0, nil
}

// InsertHeaderChain尝试将给定的标题链插入到本地链中，可能会创建一个重组。 如果返回错误，它将返回失败标头的索引号以及描述错误的错误。
//
//验证参数可用于微调是否应该进行随机数验证。 可选检查背后的原因是因为某些标头检索机制已经需要验证nonce，以及因为nonce可以稀疏地验证，而不需要检查每个。
func (hc *HeaderChain) InsertHeaderChain(chain []*types.Header, writeHeader WhCallback, start time.Time) (int, error) {
	//收集一些导入统计信息以进行报告
	stats := struct{ processed, ignored int }{}
	//所有标头都通过验证，将它们导入数据库
	for i, header := range chain {
		//关闭时短路插入
		if hc.procInterrupt() {
			logger.Debug("标头导入期间过早中止")
			return i, errors.New("中止")
		}
		//如果标题已知，请跳过它，否则存储
		if hc.HasHeader(header.Hash(), header.Number.Uint64()) {
			stats.ignored++
			continue
		}
		if err := writeHeader(header); err != nil {
			return i, err
		}
		stats.processed++
	}
	//报告一些公共统计数据，以便用户知道发生了什么
	last := chain[len(chain)-1]
	logger.Info("导入的新块头", "count", stats.processed, "elapsed", chain_common.PrettyDuration(time.Since(start)),
		"number", last.Number, "hash", last.Hash(), "ignored", stats.ignored)

	return 0, nil
}
// GetBlockHashesFromHash检索从给定哈希开始的多个块哈希，从而获取创世块。
func (hc *HeaderChain) GetBlockHashesFromHash(hash chain_common.Hash, max uint64) []chain_common.Hash {
	//获取要从中获取的原始标头
	// Get the origin header from which to fetch
	header := hc.GetHeaderByHash(hash)
	if header == nil {
		return nil
	}
	//迭代标题，直到收集到足够数量或达到起源
	chain := make([]chain_common.Hash, 0, max)
	for i := uint64(0); i < max; i++ {
		next := header.ParentHash
		if header = hc.GetHeader(next, header.Number.Uint64()-1); header == nil {
			break
		}
		chain = append(chain, next)
		if header.Number.Sign() == 0 {
			break
		}
	}
	return chain
}
//  GetAncestor检索给定块的第N个祖先。 它假定给定的块或它的近祖先是规范的。
//  maxNonCanonical指向向下计数器，限制在我们到达规范链之前单独检查的块数。
//
// 注意：ancestor == 0返回相同的块，1返回其父节点，依此类推。
func (hc *HeaderChain) GetAncestor(hash chain_common.Hash, number, ancestor uint64, maxNonCanonical *uint64) (chain_common.Hash, uint64) {
	if ancestor > number {
		return chain_common.Hash{}, 0
	}
	if ancestor == 1 {
		// 在这种情况下，只读取标题更便宜
		if header := hc.GetHeader(hash, number); header != nil {
			return header.ParentHash, number - 1
		} else {
			return chain_common.Hash{}, 0
		}
	}
	for ancestor != 0 {
		if rawdb.ReadCanonicalHash(hc.chainDb, number) == hash {
			number -= ancestor
			return rawdb.ReadCanonicalHash(hc.chainDb, number), number
		}
		if *maxNonCanonical == 0 {
			return chain_common.Hash{}, 0
		}
		*maxNonCanonical--
		ancestor--
		header := hc.GetHeader(hash, number)
		if header == nil {
			return chain_common.Hash{}, 0
		}
		hash = header.ParentHash
		number--
	}
	return hash, number
}
// GetTd通过哈希和数字从数据库中检索规范链中的块总难度，如果找到则缓存它。
func (hc *HeaderChain) GetTd(hash chain_common.Hash, number uint64) *big.Int {
	//如果 td 已经在缓存中，则短路，否则检索
	if cached, ok := hc.tdCache.Get(hash); ok {
		return cached.(*big.Int)
	}
	td := rawdb.ReadTd(hc.chainDb, hash, number)
	if td == nil {
		return nil
	}
	//为下次缓存找到的主体并返回
	// 将下次找到的主体缓存并返回
	hc.tdCache.Add(hash, td)
	return td
}
// GetTdByHash通过哈希从数据库中检索规范链中的块总难度，如果找到则将其缓存。
func (hc *HeaderChain) GetTdByHash(hash chain_common.Hash) *big.Int {
	number := hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	return hc.GetTd(hash, *number)
}
// WriteTd将块的总难度存储到数据库中，同时也将其缓存。
func (hc *HeaderChain) WriteTd(hash chain_common.Hash, number uint64, td *big.Int) error {
	rawdb.WriteTd(hc.chainDb, hash, number, td)
	hc.tdCache.Add(hash, new(big.Int).Set(td))
	return nil
}
// GetHeader通过哈希和数字从数据库中检索块头，如果找到则缓存它。
func (hc *HeaderChain) GetHeader(hash chain_common.Hash, number uint64) *types.Header {
	// 如果标头已经在缓存中，则短路，否则检索
	if header, ok := hc.headerCache.Get(hash); ok {
		return header.(*types.Header)
	}
	header := rawdb.ReadHeader(hc.chainDb, hash, number)
	if header == nil {
		return nil
	}
	// 缓存下次找到的标题并返回
	hc.headerCache.Add(hash, header)
	return header
}
// GetHeaderByHash通过哈希从数据库中检索块头，如果找到则将其缓存。
func (hc *HeaderChain) GetHeaderByHash(hash chain_common.Hash) *types.Header {
	number := hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	return hc.GetHeader(hash, *number)
}

// HasHeader检查数据库中是否存在块头。
func (hc *HeaderChain) HasHeader(hash chain_common.Hash, number uint64) bool {
	if hc.numberCache.Contains(hash) || hc.headerCache.Contains(hash) {
		return true
	}
	return rawdb.HasHeader(hc.chainDb, hash, number)
}
// GetHeaderByNumber 按编号从数据库中检索块头，如果找到则将其缓存（与其哈希关联）。
func (hc *HeaderChain) GetHeaderByNumber(number uint64) *types.Header {
	hash := rawdb.ReadCanonicalHash(hc.chainDb, number)
	if hash == (chain_common.Hash{}) {
		return nil
	}
	return hc.GetHeader(hash, number)
}
// CurrentHeader 检索规范链的当前头标题。 从HeaderChain的内部缓存中检索标头。
func (hc *HeaderChain) CurrentHeader() *types.Header {
	return hc.currentHeader.Load().(*types.Header)
}
// SetCurrentHeader设置规范链的当前头部标题。
func (hc *HeaderChain) SetCurrentHeader(head *types.Header) {
	rawdb.WriteHeadHeaderHash(hc.chainDb, head.Hash())

	hc.currentHeader.Store(head)
	hc.currentHeaderHash = head.Hash()
}

// DeleteCallback是一个回调函数，在删除每个标题之前由SetHead调用。
type DeleteCallback func(chain_common.Hash, uint64)
// SetHead 将本地链回卷到新头。 新头部上方的所有内容都将被删除，新的头部将被删除。
func (hc *HeaderChain) SetHead(head uint64, delFn DeleteCallback) {

	height := uint64(0)

	if hdr := hc.CurrentHeader() ; hdr != nil {
		height = hdr.Number.Uint64()
	}

	for hdr := hc.CurrentHeader(); hdr != nil && hdr.Number.Uint64() > head; hdr = hc.CurrentHeader() {
		hash := hdr.Hash()
		num := hdr.Number.Uint64()
		if delFn != nil {
			delFn(hash, num)
		}
		rawdb.DeleteHeader(hc.chainDb, hash, num)
		rawdb.DeleteTd(hc.chainDb, hash, num)

		hc.currentHeader.Store(hc.GetHeader(hdr.ParentHash, hdr.Number.Uint64()-1))
	}
	// 回滚规范链编号
	for i := height; i > head; i-- {
		rawdb.DeleteCanonicalHash(hc.chainDb, i)
	}
	// 清除缓存中的任何陈旧内容
	hc.headerCache.Purge()
	hc.tdCache.Purge()
	hc.numberCache.Purge()

	if hc.CurrentHeader() == nil {
		hc.currentHeader.Store(hc.genesisHeader)
	}
	hc.currentHeaderHash = hc.CurrentHeader().Hash()

	rawdb.WriteHeadHeaderHash(hc.chainDb, hc.currentHeaderHash)
}

func (self *HeaderChain) VerifyUnconfirmBlock(block *types.Block) error{
	return nil
}

func (self *HeaderChain) VerifyConfirmedBlock(block *types.Block) error {
	return nil
}
// SetGenesis 为链设置一个新的genesis块头
func (hc *HeaderChain) SetGenesis(head *types.Header) {
	hc.genesisHeader = head
}
// Config 检索标题链的链配置。
func (hc *HeaderChain) Config() *configs.ChainConfig { return hc.config }

// 引擎检索标题链的共识引擎。
func (hc *HeaderChain) Engine() consensus.Engine { return hc.engine }
// GetBlock 实现了consensus.ChainReader，并为每个输入返回nil，因为标题链没有可用于检索的块。
func (hc *HeaderChain) GetBlock(hash chain_common.Hash, number uint64) *types.Block {
	return nil
}

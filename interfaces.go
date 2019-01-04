// 包 aidoc 定义了与 Aidoc 交互的接口。
package aidoc

import (
	"context"
	"errors"
	"math/big"

	"github.com/aidoc/go-aidoc/lib/chain_common"
	"github.com/aidoc/go-aidoc/lib/chain_core/types"
)

// 如果请求的项不存在，则API方法返回NotFound。
var NotFound = errors.New("未找到")

// TODO: 移动订阅包事件

//订阅表示事件订阅，其中事件在数据通道上传递。    【类型订阅】
type Subscription interface {
	// Unsubscribe取消向数据通道发送事件
	// 并关闭错误通道。
	Unsubscribe()

	// Err返回订阅错误频道。错误通道接收
	//如果订阅有问题（例如网络连接），则为值
	//交付活动已经关闭）。只会发送一个值。
	// Unsubscribe关闭了错误通道。

	// Err返回订阅错误频道。
	// 如果订阅存在问题（例如，已经关闭了传递事件的网络连接），则错误信道接收值。 只会发送一个值。
	// Unsubscribe将关闭错误通道。
	Err() <-chan error
}

// ChainReader提供对区块链的访问。此接口中的方法从规范链（通过块编号请求）或先前由节点下载和处理的任何区块链分支访问原始数据。
// 块编号参数可以为nil以选择最新的规范块。
// 只要有可能，读取块头应优先于整个块。
//
//如果请求的项不存在，则返回的错误为NotFound。
type ChainReader interface {
	BlockByHash(ctx context.Context, hash chain_common.Hash) (*types.Block, error)
	BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error)
	HeaderByHash(ctx context.Context, hash chain_common.Hash) (*types.Header, error)
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	TransactionCount(ctx context.Context, blockHash chain_common.Hash) (uint, error)
	TransactionInBlock(ctx context.Context, blockHash chain_common.Hash, index uint) (*types.Transaction, error)

	//此方法订阅有关头部块更改的通知
	//规范链。

	// This method subscribes to notifications about changes of the head block of
	// the canonical chain.
	SubscribeNewHead(ctx context.Context, ch chan<- *types.Header) (Subscription, error)
}

// TransactionReader提供对过去交易及其收据的访问。
// 实现可能会对可以检索的交易和收据施加任意限制。可能无法获得历史交易。
//
// 如果可能，请避免依赖此接口。合同日志（通过LogFilterer接口）更可靠，并且在链重组时通常更安全。
//
// 如果请求的项不存在，则返回的错误为NotFound。
type TransactionReader interface {
	// 除区块链外，TransactionByHash还会检查待处理事务池。
	// isPending返回值指示事务是否已被挖掘。
	// 请注意，即使交易未挂起，交易也可能不属于规范链。
	TransactionByHash(ctx context.Context, txHash chain_common.Hash) (tx *types.Transaction, isPending bool, err error)

	// TransactionReceipt返回已挖掘事务的收据。
	// 请注意，即使存在收据，交易也可能不包含在当前规范链中。
	TransactionReceipt(ctx context.Context, txHash chain_common.Hash) (*types.Receipt, error)
}

// ChainStateReader包含对规范区块链的状态trie的访问。
// 请注意，接口的实现可能无法返回旧块的状态值。
// 在许多情况下，使用CallContract可能比阅读原始合同存储更可取。
type ChainStateReader interface {
	BalanceAt(ctx context.Context, account chain_common.Address, blockNumber *big.Int) (*big.Int, error)
	StorageAt(ctx context.Context, account chain_common.Address, key chain_common.Hash, blockNumber *big.Int) ([]byte, error)
	CodeAt(ctx context.Context, account chain_common.Address, blockNumber *big.Int) ([]byte, error)
	NonceAt(ctx context.Context, account chain_common.Address, blockNumber *big.Int) (uint64, error)
}

// 当节点与Aidoc网络同步时，SyncProgress提供进度指示。
type SyncProgress struct {
	StartingBlock uint64 // 同步开始时的块号
	CurrentBlock  uint64 // 同步所在的当前块编号
	HighestBlock  uint64 // 链中最高的所谓块数
	PulledStates  uint64 // 已下载的状态trie条目数
	KnownStates   uint64 // 已知的状态trie条目总数
}

// ChainSyncReader 包装对节点当前同步状态的访问。
// 如果当前没有正在运行的同步，则返回nil。
type ChainSyncReader interface {
	SyncProgress(ctx context.Context) (*SyncProgress, error)
}

// CallMsg 包含合同调用的参数。
type CallMsg struct {
	From     chain_common.Address  // '交易'的发件人
	To       *chain_common.Address // 目的地合同（合同创建为零）
	Gas      uint64                // 如果为0，则调用近无限gas
	GasPrice *big.Int              // dose < - >gas交换率
	Value    *big.Int              // dose与调用
	Data     []byte                // 输入数据一起发送的数量,通常是ABI编码的合同方法调用
}

// ContractCaller 提供合同调用，实质上是由EVM执行但未开采到区块链中的交易。
// ContractCall 是执行此类调用的低级方法。
// 对于围绕特定合同构建的应用程序，本机工具提供了更好，正确类型的方式来执行调用。

type ContractCaller interface {
	CallContract(ctx context.Context, call CallMsg, blockNumber *big.Int) ([]byte, error)
}

// FilterQuery 包含合同日志过滤选项。
type FilterQuery struct {
	FromBlock *big.Int               // 查询范围的开头，nil表示创世块
	ToBlock   *big.Int               // 范围的结尾，nil表示最新的块
	Addresses []chain_common.Address // 限制匹配特定合同创建的事件

	// 主题列表限制与特定事件主题的匹配。 每个事件都有一个主题列表。 主题匹配该列表的前缀。
	// 空元素切片匹配任何主题。 非空元素表示与任何包含的主题匹配的替代方法。
	//
	// 例子：
	// {}或nil匹配任何主题列表
	// {{A}}在第一个位置匹配主题A.
	// {{}，{B}}匹配第一个位置的任何主题，B匹配第二个位置
	// {{A}，{B}}匹配第一个位置的主题A，第二个位置的B匹配
	// {{A，B}}，{C，D}}匹配第一个位置的主题（A OR B），（C OR D）第二个位置的
	Topics [][]chain_common.Hash
}

// LogFilterer使用一次性查询或连续事件订阅提供对合同日志事件的访问。
//
// 通过流式查询订阅接收的日志可能已将Removed设置为true，表示由于链重组而恢复了日志。
type LogFilterer interface {
	FilterLogs(ctx context.Context, q FilterQuery) ([]types.Log, error)
	SubscribeFilterLogs(ctx context.Context, q FilterQuery, ch chan<- types.Log) (Subscription, error)
}

// TransactionSender包装交易发送。SendTransaction方法将已签名的交易注入待处理的交易池以供执行。
// 如果交易是创建合同，则可以使用TransactionReceipt方法在交易挖掘后检索合同地址。
//
// 交易必须签署并包含有效的随机数。
// API的使用者可以使用包账户来维护本地私钥，并且需要使用PendingNonceAt检索下一个可用的nonce。
type TransactionSender interface {
	SendTransaction(ctx context.Context, tx *types.Transaction) error
}

// GasPricer包含天然气价格，它监测区块链以确定当前费用市场条件下的最佳天然气价格。
type GasPricer interface {
	SuggestGasPrice(ctx context.Context) (*big.Int, error)
}

// PendingStateReader提供对挂起状态的访问，这是所有已知的可执行事务的结果，这些事务尚未包含在区块链中。
// 它通常用于显示由用户发起的“未经证实的”动作（例如，钱包价值转移）的结果。
// PendingNonceAt操作是检索特定帐户的下一个可用事务随机数的好方法。
type PendingStateReader interface {
	PendingBalanceAt(ctx context.Context, account chain_common.Address) (*big.Int, error)
	PendingStorageAt(ctx context.Context, account chain_common.Address, key chain_common.Hash) ([]byte, error)
	PendingCodeAt(ctx context.Context, account chain_common.Address) ([]byte, error)
	PendingNonceAt(ctx context.Context, account chain_common.Address) (uint64, error)
	PendingTransactionCount(ctx context.Context) (uint, error)
}

// PendingContractCaller可以用来对待处理状态执行调用。
type PendingContractCaller interface {
	PendingCallContract(ctx context.Context, call CallMsg) ([]byte, error)
}

// GasEstimator包装EstimateGas，它试图根据待处理状态估算执行特定交易所需的气体。
// 无法保证这是真正的天然气限制要求，因为矿工可能会添加或删除其他交易，但它应该为设定合理的违约提供依据。
type GasEstimator interface {
	EstimateGas(ctx context.Context, call CallMsg) (uint64, error)
}

// PendingStateEventer 提供对有关挂起状态更改的实时通知的访问。
type PendingStateEventer interface {
	SubscribePendingTransactions(ctx context.Context, ch chan<- *types.Transaction) (Subscription, error)
}

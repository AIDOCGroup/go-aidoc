

package chain_core

import (
	"github.com/aidoc/go-aidoc/configs"
	"github.com/aidoc/go-aidoc/lib/chain_common"
	"github.com/aidoc/go-aidoc/lib/chain_core/types"
	"github.com/aidoc/go-aidoc/lib/chain_core/vm"
	"github.com/aidoc/go-aidoc/lib/crypto"
	"github.com/aidoc/go-aidoc/lib/state"
	"github.com/aidoc/go-aidoc/service/produce/consensus"
)

// StateProcessor是一个基本的处理器，它负责将状态从一个点转换到另一个点。
//
// StateProcessor实现Processor。
type StateProcessor struct {
	config *configs.ChainConfig // 链配置选项
	bc     *BlockChain          // 规范块链
	engine consensus.Engine     // 用于块奖励的共识引擎
}
// NewState Processor初始化一个新的状态处理器。
func NewStateProcessor(config *configs.ChainConfig, bc *BlockChain, engine consensus.Engine) *StateProcessor {
	return &StateProcessor{
		config: config,
		bc:     bc,
		engine: engine,
	}
}

// 进程通过使用 statedb 运行交易消息并将任何奖励应用于处理器（coinbase）和任何包含的叔区块，根据Aidoc规则处理状态更改。
//
// 流程返回流程中累积的收据和日志，并返回流程中使用的gas量。 如果任何交易因 gas 不足而未能执行，则会返回错误。
func (p *StateProcessor) Process(block *types.Block, statedb *state.StateDB, cfg vm.Config) (types.Receipts, []*types.Log, uint64, error) {
	var (
		receipts types.Receipts
		usedGas  = new(uint64)
		header   = block.Header()
		allLogs  []*types.Log
		gp       = new(GasPool).AddGas(block.GasLimit()) //在一个block处理过程中，GasPool的值可以获得还剩GAS可以使用
	)
	//// 根据任何硬叉规范改变块和状态
	//if p.config.DAOForkSupport && p.config.DAOForkBlock != nil && p.config.DAOForkBlock.Cmp(block.Number()) == 0 {
	//	misc.ApplyDAOHardFork(statedb)
	//}
	// 迭代并处理各个交易
	for i, tx := range block.Transactions() {
		statedb.Prepare(tx.Hash(), block.Hash(), i)
		receipt, _, err := ApplyTransaction(p.config, p.bc, nil, gp, statedb, header, tx, usedGas, cfg)
		if err != nil {
			return nil, nil, 0, err
		}
		receipts = append(receipts, receipt)
		allLogs = append(allLogs, receipt.Logs...)
	}
	// 最终确定块，应用任何共识引擎特定的额外内容（例如块奖励）
	p.engine.Finalize(p.bc, header, statedb, block.Transactions(), receipts)

	return receipts, allLogs, *usedGas, nil
}

// ApplyTransaction尝试将交易应用于给定的状态数据库，并将输入参数用于其环境。 它返回交易的
// 收据，使用的gas，如果交易失败则返回错误，表示块无效。
func ApplyTransaction(config *configs.ChainConfig, bc ChainContext, author *chain_common.Address, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *uint64, cfg vm.Config) (*types.Receipt, uint64, error) {
	msg, err := tx.AsMessage(types.MakeSigner(config, header.Number))
	if err != nil {
		return nil, 0, err
	}
	// 创建要在EVM环境中使用的新上下文
	context := NewEVMContext(msg, header, bc, author)

	// 创建一个新环境，其中包含有关transaction和调用机制的所有相关信息。
	vmenv := vm.NewEVM(context, statedb, config, cfg)

	// 将transaction应用于当前状态（包含在env中）
	_, gas, failed, err := ApplyMessage(vmenv, msg, gp)
	if err != nil {
		return nil, 0, err
	}

	// 使用挂起更改更新状态
	var root []byte

	//if config.IsByzantium(header.Number) {
	//	statedb.Finalise(true)
	//} else {
	//	root = statedb.IntermediateRoot(true/*config.IsEIP158(header.Number)*/).Bytes()
	//}
	statedb.Finalise(true)
	*usedGas += gas

	// 为交易创建一个新收据，根据eip阶段存储tx使用的中间根和gas，我们正在通过 root touch-delete账户。
	receipt := types.NewReceipt(root, failed, *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = gas

	// 如果交易创建了合同，则将创建地址存储在收据中。
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
	}

	// 设置收据日志并创建用于过滤的bloom
	receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	return receipt, gas, err
}

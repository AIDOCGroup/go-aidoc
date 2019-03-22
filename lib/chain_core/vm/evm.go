//执行器 对外提供一些外部接口

package vm

import (
	"math/big"
	"sync/atomic"
	"time"

	"github.com/aidoc/go-aidoc/lib/chain_common"
	"github.com/aidoc/go-aidoc/lib/crypto"
	"github.com/aidoc/go-aidoc/configs"
)

// createCodeHash 由 create 使用，以确保不允许部署合同地址（在账户提取后相关）。
var emptyCodeHash = crypto.Keccak256Hash(nil)

type (
	CanTransferFunc func(StateDB, chain_common.Address, *big.Int) bool
	TransferFunc    func(StateDB, chain_common.Address, chain_common.Address, *big.Int)
	// GetHashFunc 返回区块链中的第n个块哈希，并由 BLOCKHASH EVM 操作码使用。
	GetHashFunc func(uint64) chain_common.Hash
)

// run 运行给定的契约，并负责运行预编译，并回退到字节码解释器。
func run(evm *EVM, contract *Contract, input []byte) ([]byte, error) {
	if contract.CodeAddr != nil {
		//precompiles := PrecompiledContractsHomestead
		//if evm.ChainConfig().IsByzantium(evm.BlockNumber) {
		//	precompiles = PrecompiledContractsByzantium
		//}
		precompiles := PrecompiledContractsByzantium
		if p := precompiles[*contract.CodeAddr]; p != nil {
			return RunPrecompiledContract(p, input, contract)
		}
	}
	return evm.interpreter.Run(contract, input)
}

// Context 为EVM提供辅助信息。 提供后，不应修改。
type Context struct {
	// CanTransfer (可以转移) 返回是否含 有该账户
	// 足够的Aidoc 传递价值
	CanTransfer CanTransferFunc
	//  转账 将Aidoc 从一个账户 转账 到另一个账户
	Transfer TransferFunc
	// GetHash返回 与 n 对应的哈希值
	GetHash GetHashFunc

	// 消息信息
	Origin   chain_common.Address // 提供 ORIGIN(起源) 的信息
	GasPrice *big.Int             // 为GASPRICE (gas 价格)提供信息

	// Block information
	Coinbase    chain_common.Address //  Coinbase (币基础)  地址  提供ORIGIN(起源) 的信息
	GasLimit    uint64               //  GasLimit（gas限制）提供 GASLIMIT(gas限制) 的信息
	BlockNumber *big.Int             //  BlockNumber (区块数)    提供 NUMBER(数) 的信息
	Time        *big.Int             //  Time 时间               提供TIME(时间)的信息
	Difficulty  *big.Int             //  Difficulty 困难         提供有关DIFFICULTY（困难） 的信息
}

// EVM是Aidoc虚拟机基础对象，它提供了使用提供的上下文在给定状态下运行合同所需的工具。
// 应该注意的是，通过任何调用产生的任何错误都应该被视为恢复状态和消耗全部操作，不应该
// 执行特定错误的检查。 解释器确保生成的任何错误都被视为错误代码。
//
// 永远不应该重用EVM并且不是线程安全的。
type EVM struct {
	// Context 提供辅助区块链相关信息
	Context
	// StateDB 提供对底层状态的访问
	StateDB StateDB
	// 深度是当前的调用堆栈
	depth int

	// chainConfig 包含有关当前链的信息
	chainConfig *configs.ChainConfig
	// 链规则包含当前纪元的链规则
	chainRules configs.Rules

	// 用于初始化evm的虚拟机配置选项。
	vmConfig Config
	// 全局（对于此上下文）在整个tx执行过程中使用的Aidoc虚拟机。
	interpreter *Interpreter
	// abort用于中止EVM调用操作
	// 注意：必须以原子方式设置
	abort int32
	// callGasTemp 保存当前呼叫可用的gas。 这是必需的，因为可用gas是根据63/64
	// 规则在gasCall *中计算的，后来在opCall *中应用。
	callGasTemp uint64
}

// NewEVM 返回一个新的EVM。 返回的EVM不是线程安全的，应该只使用*一次*。
func NewEVM(ctx Context, statedb StateDB, chainConfig *configs.ChainConfig, vmConfig Config) *EVM {
	evm := &EVM{
		Context:     ctx,
		StateDB:     statedb,
		vmConfig:    vmConfig,
		chainConfig: chainConfig,
		chainRules:  chainConfig.Rules(ctx.BlockNumber),
	}

	evm.interpreter = NewInterpreter(evm, vmConfig)
	return evm
}

// 取消取消任何正在运行的EVM操作。 这可以同时调用，并且可以安全地多次调用。
func (evm *EVM) Cancel() {
	atomic.StoreInt32(&evm.abort, 1)
}

// Call 使用给定输入作为参数执行与addr关联的合约。 它还处理所需的任何必要的值传输，并采取
// 必要的步骤来创建账户并在执行错误或值传输失败时撤消状态。
func (evm *EVM) Call(caller ContractRef, addr chain_common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, err error) {
	if evm.vmConfig.NoRecursion && evm.depth > 0 {
		return nil, gas, nil
	}

	// 如果我们尝试在呼叫深度限制之上执行，则会失败
	if evm.depth > int(configs.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	// 如果我们尝试转移超过可用余额，则会失败
	if !evm.Context.CanTransfer(evm.StateDB, caller.Address(), value) {
		return nil, gas, ErrInsufficientBalance
	}

	var (
		to       = AccountRef(addr)
		snapshot = evm.StateDB.Snapshot()
	)
	if !evm.StateDB.Exist(addr) {
		//precompiles := PrecompiledContractsHomestead
		//if evm.ChainConfig().IsByzantium(evm.BlockNumber) {
		//	precompiles = PrecompiledContractsByzantium
		//}
		precompiles := PrecompiledContractsByzantium
		// TODO: 分析EIP158
		if precompiles[addr] == nil && /*evm.ChainConfig().IsEIP158(evm.BlockNumber) &&*/ value.Sign() == 0 {
			// 调用一个不存在的账户，不要做任何事情，但ping该跟踪器
			if evm.vmConfig.Debug && evm.depth == 0 {
				evm.vmConfig.Tracer.CaptureStart(caller.Address(), addr, false, input, gas, value)
				evm.vmConfig.Tracer.CaptureEnd(ret, 0, 0, nil)
			}
			return nil, gas, nil
		}
		evm.StateDB.CreateAccount(addr)
	}
	evm.Transfer(evm.StateDB, caller.Address(), to.Address(), value)

	// 初始化新合同并设置EVM要使用的代码。
	// 合同只是此执行上下文的作用域环境。
	contract := NewContract(caller, to, value, gas)
	contract.SetCallCode(&addr, evm.StateDB.GetCodeHash(addr), evm.StateDB.GetCode(addr))

	start := time.Now()
	// 在调试模式下捕获跟踪器开始/结束事件
	if evm.vmConfig.Debug && evm.depth == 0 {
		evm.vmConfig.Tracer.CaptureStart(caller.Address(), addr, false, input, gas, value)

		defer func() { // 懒惰的参数评估
			evm.vmConfig.Tracer.CaptureEnd(ret, gas-contract.Gas, time.Since(start), err)
		}()
	}
	ret, err = run(evm, contract, input)

	// 当EVM返回错误或设置上面的创建代码时，我们将恢复为快照并消耗剩余的gas。 此外，当我们在宅基地时，这也会导致代码存储 gas 错误。

	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != errExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, err
}

// CallCode使用给定输入作为参数执行与addr关联的契约。 它还处理所需的任何必要的值传输，
// 并采取必要的步骤来创建账户并在执行错误或值传输失败时撤消状态。
//
// CallCode与Call的不同之处在于它以调用者作为上下文执行给定地址的代码。
func (evm *EVM) CallCode(caller ContractRef, addr chain_common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, err error) {
	if evm.vmConfig.NoRecursion && evm.depth > 0 {
		return nil, gas, nil
	}

	// 如果我们尝试在呼叫深度限制之上执行，则会失败
	if evm.depth > int(configs.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	// 如果我们尝试转移超过可用余额，则会失败
	if !evm.CanTransfer(evm.StateDB, caller.Address(), value) {
		return nil, gas, ErrInsufficientBalance
	}

	var (
		snapshot = evm.StateDB.Snapshot()
		to       = AccountRef(caller.Address())
	)
	// 初始化新合同并设置EVM要使用的代码。 合同只是此执行上下文的范围环境。
	contract := NewContract(caller, to, value, gas)
	contract.SetCallCode(&addr, evm.StateDB.GetCodeHash(addr), evm.StateDB.GetCode(addr))

	ret, err = run(evm, contract, input)
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != errExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, err
}

// DelegateCall 使用给定输入作为参数执行与addr关联的合约。 它在执行错误的情况下反转状态。
//
// DelegateCall 与 CallCode 的不同之处在于它以调用者作为上下文执行给定地址的代码，并且调用者被设置为调用者的调用者。
func (evm *EVM) DelegateCall(caller ContractRef, addr chain_common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	if evm.vmConfig.NoRecursion && evm.depth > 0 {
		return nil, gas, nil
	}
	// 如果我们尝试在呼叫深度限制之上执行，则会失败
	if evm.depth > int(configs.CallCreateDepth) {
		return nil, gas, ErrDepth
	}

	var (
		snapshot = evm.StateDB.Snapshot()
		to       = AccountRef(caller.Address())
	)

	// 初始化新合同并初始化委托值
	contract := NewContract(caller, to, nil, gas).AsDelegate()
	contract.SetCallCode(&addr, evm.StateDB.GetCodeHash(addr), evm.StateDB.GetCode(addr))

	ret, err = run(evm, contract, input)
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != errExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, err
}

// StaticCall使用给定输入作为参数执行与addr关联的合同，同时禁止在调用期间对状态进行任何修改。
// 尝试执行此类修改的操作码将导致异常而不是执行修改。
func (evm *EVM) StaticCall(caller ContractRef, addr chain_common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	if evm.vmConfig.NoRecursion && evm.depth > 0 {
		return nil, gas, nil
	}
	// 如果我们尝试在呼叫深度限制之上执行，则会失败
	if evm.depth > int(configs.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	// 确保 readonly 仅在我们不是readonly时才设置，这确保了对于子调用不会删除readonly标志。
	if !evm.interpreter.readOnly {
		evm.interpreter.readOnly = true
		defer func() { evm.interpreter.readOnly = false }()
	}

	var (
		to       = AccountRef(addr)
		snapshot = evm.StateDB.Snapshot()
	)
	// 初始化新合同并设置EVM要使用的代码。 合同只是此执行上下文的范围环境。
	contract := NewContract(caller, to, new(big.Int), gas)
	contract.SetCallCode(&addr, evm.StateDB.GetCodeHash(addr), evm.StateDB.GetCode(addr))

	// 当EVM返回错误或设置上面的创建代码时，我们将恢复为快照并消耗剩余的gas。 此外，
	// 当我们在Homestead时，这也会导致代码存储gas错误。
	ret, err = run(evm, contract, input)
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != errExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, err
}
// 使用代码作为部署代码创建新合同。
func (evm *EVM) Create(caller ContractRef, code []byte, gas uint64, value *big.Int) (ret []byte, contractAddr chain_common.Address, leftOverGas uint64, err error) {

	// 深度检查执行。 如果我们尝试执行超出限制，则失败。
	if evm.depth > int(configs.CallCreateDepth) {
		return nil, chain_common.Address{}, gas, ErrDepth
	}
	if !evm.CanTransfer(evm.StateDB, caller.Address(), value) {
		return nil, chain_common.Address{}, gas, ErrInsufficientBalance
	}
	// 确保指定地址已经没有现有合同
	nonce := evm.StateDB.GetNonce(caller.Address())
	evm.StateDB.SetNonce(caller.Address(), nonce+1)

	//给定字节和随机数，创建一个Aidoc地址
	contractAddr = crypto.CreateAddress(caller.Address(), nonce)
	contractHash := evm.StateDB.GetCodeHash(contractAddr)
	if evm.StateDB.GetNonce(contractAddr) != 0 || (contractHash != (chain_common.Hash{}) && contractHash != emptyCodeHash) {
		return nil, chain_common.Address{}, 0, ErrContractAddressCollision
	}

	// 创建一个状态快照
	snapshot := evm.StateDB.Snapshot()
	evm.StateDB.CreateAccount(contractAddr)
	//if evm.ChainConfig().IsEIP158(evm.BlockNumber) {
	//	evm.StateDB.SetNonce(contractAddr, 1)
	//}
	evm.StateDB.SetNonce(contractAddr, 1)

	//转移从发件人中减去金额，并使用给定的Db向收件人添加金额
	evm.Transfer(evm.StateDB, caller.Address(), contractAddr, value)

	// 初始化新合同并设置EVM要使用的代码。 合同只是此执行上下文的范围环境。
	contract := NewContract(caller, AccountRef(contractAddr), value, gas)
	contract.SetCallCode(&contractAddr, crypto.Keccak256Hash(code), code)

	if evm.vmConfig.NoRecursion && evm.depth > 0 {
		return nil, contractAddr, gas, nil
	}

	if evm.vmConfig.Debug && evm.depth == 0 {
		evm.vmConfig.Tracer.CaptureStart(caller.Address(), contractAddr, true, code, gas, value)
	}
	start := time.Now()

	ret, err = run(evm, contract, nil)

	// 检查是否已超出最大代码大小
	maxCodeSizeExceeded := /*evm.ChainConfig().IsEIP158(evm.BlockNumber) &&*/ len(ret) > configs.MaxCodeSize

	// 如果合同创建成功运行且未返回任何错误，则计算存储代码所需的gas。 如果由于没有足够的
	// gas 设置错误而无法存储代码，请让它由下面的错误检查条件处理。

	if err == nil && !maxCodeSizeExceeded {
		//合约长度收费
		createDataGas := uint64(len(ret)) * configs.CreateDataGas
		if contract.UseGas(createDataGas) {
			evm.StateDB.SetCode(contractAddr, ret)
		} else {
			err = ErrCodeStoreOutOfGas
		}
	}
	// 当EVM返回错误或设置上面的创建代码时，我们将恢复为快照并消耗剩余的gas。 此外，
	// 当我们在宅基地时，这也会导致代码存储gas错误。
	if maxCodeSizeExceeded || (err != nil && (evm.ChainConfig().IsHomestead(evm.BlockNumber) || err != ErrCodeStoreOutOfGas)) {
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != errExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	//如果合同代码大小超过max而err仍然为空，则分配err。
	if maxCodeSizeExceeded && err == nil {
		err = errMaxCodeSizeExceeded
	}
	if evm.vmConfig.Debug && evm.depth == 0 {
		evm.vmConfig.Tracer.CaptureEnd(ret, gas-contract.Gas, time.Since(start), err)
	}
	return ret, contractAddr, contract.Gas, err
}
// ChainConfig 返回环境的链配置
func (evm *EVM) ChainConfig() *configs.ChainConfig { return evm.chainConfig }
// Interpreter 返回EVM解释器
func (evm *EVM) Interpreter() *Interpreter { return evm.interpreter }

package configs

import (
	"math/big"

	"github.com/aidoc/go-aidoc/lib/chain_common"
	"github.com/aidoc/go-aidoc/lib/i18"
)

// Genesis哈希强制执行以下配置。
var (
	MainnetGenesisHash = chain_common.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	TestnetGenesisHash = chain_common.HexToHash("0x41941023680923e0fe4d74a34bdac8141f2540e3ae90623718e47d66d1ca4a2d")
)

var (
	// MainnetChainConfig是在主网络上运行节点的链参数。
	MainnetChainConfig = &ChainConfig{
		ChainID:        big.NewInt(1),
		HomesteadBlock: big.NewInt(0),
		//DAOForkBlock:   nil,
		//DAOForkSupport: true,
		//EIP150Block:    big.NewInt(0),
		//EIP150Hash:     chain_common.HexToHash(""),
		//EIP155Block:    big.NewInt(10),
		//EIP158Block:    big.NewInt(10),

		//ByzantiumBlock:      big.NewInt(0),
		//ConstantinopleBlock: nil,
		Aidochash: new(AidochashConfig),
	}

	// TestnetChainConfig包含在Ropsten测试网络上运行节点的链参数。
	TestnetChainConfig = &ChainConfig{
		ChainID:        big.NewInt(3),
		HomesteadBlock: big.NewInt(0),
		//DAOForkBlock:   nil,
		//DAOForkSupport: true,
		//EIP150Block:    big.NewInt(0),
		//EIP150Hash:     chain_common.HexToHash("0x41941023680923e0fe4d74a34bdac8141f2540e3ae90623718e47d66d1ca4a2d"),
		//EIP155Block:    big.NewInt(10),
		//EIP158Block:    big.NewInt(10),

		//ByzantiumBlock:      big.NewInt(1700000),
		//ConstantinopleBlock: nil,
		Aidochash: new(AidochashConfig),
	}

	// RinkebyChainConfig包含在Rinkeby测试网络上运行节点的链参数。
	RinkebyChainConfig = &ChainConfig{
		ChainID:        big.NewInt(4),
		HomesteadBlock: big.NewInt(1),
		//DAOForkBlock:   nil,
		//DAOForkSupport: true,
		//EIP150Block:    big.NewInt(2),
		//EIP150Hash:     chain_common.HexToHash("0x9b095b36c15eaf13044373aef8ee0bd3a382a5abb92e402afa44b8249c3a90e9"),
		//EIP155Block:    big.NewInt(3),
		//EIP158Block:    big.NewInt(3),

		//ByzantiumBlock:      big.NewInt(1035301),
		//ConstantinopleBlock: nil,
		//Clique: &CliqueConfig{
		//	Period: 15,
		//	Epoch:  30000,
		//},
	}
	// AllAidochashProtocolChanges包含由Aidoc核心开发人员引入和接受的每个协议更改（EIP）到Aidochash共识。
	//
	//此配置有意不使用键控字段强制任何向配置添加标志的人也必须设置这些字段。

	AllAidochashProtocolChanges = &ChainConfig{
		big.NewInt(1337),
		big.NewInt(0),
		//nil,
		//false,
		//big.NewInt(0),
		//chain_common.Hash{},
		//big.NewInt(0),
		//big.NewInt(0),
		//big.NewInt(0) ,
		big.NewInt(0),
		//nil,
		new(AidochashConfig),
		//nil,
	}

	// AllCliqueProtocolChanges包含由Aidoc核心开发人员引入和接受的每个协议更改（EIP）到Clique共识中。
	//
	//此配置有意不使用键控字段强制任何向配置添加标志的人也必须设置这些字段。

	AllCliqueProtocolChanges = &ChainConfig{
		big.NewInt(1337),
		big.NewInt(0),
		//nil,
		//false,
		//big.NewInt(0),
		//chain_common.Hash{},
		//big.NewInt(0),
		//big.NewInt(0),
		//big.NewInt(0) ,
		big.NewInt(0),
		//nil,
		nil,
		//&CliqueConfig{Period: 0, Epoch: 30000}
	}

	TestChainConfig = &ChainConfig{
		big.NewInt(1),
		big.NewInt(0),
		//nil,
		//false,
		//big.NewInt(0),
		//chain_common.Hash{},
		//big.NewInt(0),
		//big.NewInt(0),
		//big.NewInt(0) ,
		big.NewInt(0),
		//nil,
		new(AidochashConfig),
		//nil
	}
	TestRules = TestChainConfig.Rules(new(big.Int))
)

// ChainConfig是确定区块链设置的核心配置。
//
// ChainConfig基于每个块存储在数据库中。 意即
// 由其创世块标识的任何网络都可以拥有自己的网络
// 一组配置选项。

type ChainConfig struct {
	ChainID *big.Int `json:"chainId"` //  chainId标识当前链并用于重放保护

	HomesteadBlock *big.Int `json:"homesteadBlock,omitempty"` //  Homestead 开关区（零=没有叉，0 =已经是宅基地）

	//DAOForkBlock   *big.Int `json:"daoForkBlock,omitempty"`   //   TheDAO硬叉开关块（nil =无叉）
	//DAOForkSupport bool     `json:"daoForkSupport,omitempty"` //   节点是支持还是反对DAO硬分叉

	// EIP150 实施  gas (gas)价格变动（https://github.com/aidoc/EIPs/issues/150）
	//EIP150Block *big.Int          `json:"eip150Block,omitempty"` // EIP150 HF block (nil = no fork) EIP150 HF块（零=无叉）
	//EIP150Hash  chain_common.Hash `json:"eip150Hash,omitempty"`  //   EIP150 HF哈希（由于只有 gas 价格发生变化，因此只有标题客户端需要）

	//EIP155Block *big.Int `json:"eip155Block,omitempty"` //  EIP155 HF块
	//EIP158Block *big.Int `json:"eip158Block,omitempty"` //  EIP158 HF块

	AiDocBlock *big.Int `json:"aiDocBlock,omitempty"` //aidoc HF block

	//ByzantiumBlock      *big.Int `json:"byzantiumBlock,omitempty"`      //  拜占庭开关块（nil =无叉，0 =已经在拜占庭）
	//ConstantinopleBlock *big.Int `json:"constantinopleBlock,omitempty"` // 君士坦丁堡开关块（nil =无叉，0 =已激活）

	//  各种共识引擎
	Aidochash *AidochashConfig `json:"aidochash,omitempty"`
	//Clique *CliqueConfig `json:"clique,omitempty"`
}

// AidochashConfig 是基于工作量证明的密封的共识发动机配置。
type AidochashConfig struct{}

// String实现 stringer 接口，返回共识引擎详细信息。
func (c *AidochashConfig) String() string {
	return "aidochash"
}

//
//// CliqueConfig 是基于权威证明的密封的共识引擎配置。
//type CliqueConfig struct {
//	Period uint64 `json:"period"` //    要强制执行的块之间的秒数
//	Epoch  uint64 `json:"epoch"`  //   大纪元长度重置投票和检查点
//}
//
//// String 实现 stringer 接口，返回共识引擎详细信息。
//func (c *CliqueConfig) String() string {
//	return "clique"
//}

// String 实现 i18.I18_print.Stringer 接口。
func (c *ChainConfig) String() string {
	var engine interface{}
	switch {
	case c.Aidochash != nil:
		engine = c.Aidochash
	//case c.Clique != nil:
	//	engine = c.Clique
	default:
		engine = "unknown"
	}
	//return i18.I18_print.Sprintf("{ChainID: %v Homestead: %v DAO: %v DAOSupport: %v EIP150: %v EIP155: %v EIP158: %v Byzantium: %v Constantinople: %v Engine: %v}",
	//	c.ChainID,
	//	c.HomesteadBlock,
	//	//c.DAOForkBlock,
	//	//c.DAOForkSupport,
	//	//c.EIP150Block,
	//	//c.EIP155Block,
	//	//c.EIP158Block,
	//	//c.ByzantiumBlock,
	//	//c.ConstantinopleBlock,
	//	engine,
	//)
	return i18.I18_print.Sprintf("{ChainID: %v Homestead: %v Engine: %v}",
		c.ChainID, c.HomesteadBlock, engine)
}

// IsHomestead 返回 num 是否等于 homestead 块或更大。
func (c *ChainConfig) IsHomestead(num *big.Int) bool {
	return isForked(c.HomesteadBlock, num)
}

//// IsDAOFork 返回 num 是否等于 DAO fork块（分叉块） 或更大。
//func (c *ChainConfig) IsDAOFork(num *big.Int) bool {
//	return isForked(c.DAOForkBlock, num)
//}

//// IsEIP150 返回 num 是否等于 EIP150 fork块 （分叉块） 或更大。
//func (c *ChainConfig) IsEIP150(num *big.Int) bool {
//	return isForked(c.EIP150Block, num)
//}

//// IsEIP155 返回 num 是否等于 EIP155 fork块（分叉块） 或更大。
//func (c *ChainConfig) IsEIP155(num *big.Int) bool {
//	return isForked(c.EIP155Block, num)
//}

//// IsEIP158 返回num是否等于 EIP158 fork块（分叉块）或更大。
//func (c *ChainConfig) IsEIP158(num *big.Int) bool {
//	return isForked(c.EIP158Block, num)
//}

//// IsByzantium 返回num是否等于Byzantium（拜占庭） fork块 （分叉块）或更大。
//func (c *ChainConfig) IsByzantium(num *big.Int) bool {
//	return isForked(c.ByzantiumBlock, num)
//}

func (c *ChainConfig) IsAiDoc(num *big.Int) bool {
	return isForked(c.AiDocBlock, num)
}

//// IsConstantinople 返回 num 是否等于 Constantinople fork 块或更大。
//func (c *ChainConfig) IsConstantinople(num *big.Int) bool {
//	return isForked(c.ConstantinopleBlock, num)
//}

// GasTable 返回与当前阶段（Homestead 或 Homestead 重新定价）相对应的gas表。
//
// 在任何情况下都不应更改返回的 GasTable 字段。
func (c *ChainConfig) GasTable(num *big.Int) GasTable {
	//if num == nil {
	//	return GasTableHomestead
	//}
	//switch {
	//case c.IsEIP158(num):
	//	return GasTableEIP158
	//case c.IsEIP150(num):
	//	return GasTableEIP150
	//default:
	//	return GasTableHomestead
	//}
	return GasTableHomestead
}

// CheckCompatible 检查是否使用不匹配的链配置导入了调度的fork转换。
func (c *ChainConfig) CheckCompatible(newcfg *ChainConfig, height uint64) *ConfigCompatError {
	bhead := new(big.Int).SetUint64(height)

	// 迭代 checkCompatible 找到最低冲突。
	var lasterr *ConfigCompatError
	for {
		err := c.checkCompatible(newcfg, bhead)
		if err == nil || (lasterr != nil && err.RewindTo == lasterr.RewindTo) {
			break
		}
		lasterr = err
		bhead.SetUint64(err.RewindTo)
	}
	return lasterr
}

func (c *ChainConfig) checkCompatible(newcfg *ChainConfig, head *big.Int) *ConfigCompatError {
	if isForkIncompatible(c.HomesteadBlock, newcfg.HomesteadBlock, head) {
		return newCompatError("HomesteadBlock", c.HomesteadBlock, newcfg.HomesteadBlock)
	}
	//if isForkIncompatible(c.DAOForkBlock, newcfg.DAOForkBlock, head) {
	//	return newCompatError("DAO叉块", c.DAOForkBlock, newcfg.DAOForkBlock)
	//}
	//if c.IsDAOFork(head) && c.DAOForkSupport != newcfg.DAOForkSupport {
	//	return newCompatError("DAO叉支持标志", c.DAOForkBlock, newcfg.DAOForkBlock)
	//}
	//if isForkIncompatible(c.EIP150Block, newcfg.EIP150Block, head) {
	//	return newCompatError("EIP150前叉块", c.EIP150Block, newcfg.EIP150Block)
	//}
	//if isForkIncompatible(c.EIP155Block, newcfg.EIP155Block, head) {
	//	return newCompatError("EIP155前叉块", c.EIP155Block, newcfg.EIP155Block)
	//}
	//if isForkIncompatible(c.EIP158Block, newcfg.EIP158Block, head) {
	//	return newCompatError("EIP158前叉块", c.EIP158Block, newcfg.EIP158Block)
	//}
	//if c.IsEIP158(head) && !configNumEqual(c.ChainID, newcfg.ChainID) {
	//	return newCompatError("EIP158链ID", c.EIP158Block, newcfg.EIP158Block)
	//}
	//if isForkIncompatible(c.ByzantiumBlock, newcfg.ByzantiumBlock, head) {
	//	return newCompatError("拜占庭叉块", c.ByzantiumBlock, newcfg.ByzantiumBlock)
	//}
	//if isForkIncompatible(c.ConstantinopleBlock, newcfg.ConstantinopleBlock, head) {
	//	return newCompatError("君士坦丁堡叉块", c.ConstantinopleBlock, newcfg.ConstantinopleBlock)
	//}
	return nil
}

//如果在s1调度的fork无法重新调度为阻塞s2，则 isForkIncompatible 返回 true，因为 head 已经超过了 fork。
func isForkIncompatible(s1, s2, head *big.Int) bool {
	return (isForked(s1, head) || isForked(s2, head)) && !configNumEqual(s1, s2)
}

// isForked 返回在块 s 处调度的 fork 是否在给定的 head 块处于活动状态。
func isForked(s, head *big.Int) bool {
	if s == nil || head == nil {
		return false
	}
	return s.Cmp(head) <= 0
}

func configNumEqual(x, y *big.Int) bool {
	if x == nil {
		return y == nil
	}
	if y == nil {
		return x == nil
	}
	return x.Cmp(y) == 0
}

// 如果使用会改变过去的ChainConfig初始化本地存储的区块链，则会引发ConfigCompatError。
type ConfigCompatError struct {
	What string
	// 块存储和新配置的编号
	StoredConfig, NewConfig *big.Int
	// 必须重绕本地链以更正错误的块编号
	RewindTo uint64
}

func newCompatError(what string, storedblock, newblock *big.Int) *ConfigCompatError {
	var rew *big.Int
	switch {
	case storedblock == nil:
		rew = newblock
	case newblock == nil || storedblock.Cmp(newblock) < 0:
		rew = storedblock
	default:
		rew = newblock
	}
	err := &ConfigCompatError{what, storedblock, newblock, 0}
	if rew != nil && rew.Sign() > 0 {
		err.RewindTo = rew.Uint64() - 1
	}
	return err
}

func (err *ConfigCompatError) Error() string {
	return i18.I18_print.Sprintf("数据库中 %s 不匹配（ %d，想要 %d，回放到 %d）", err.What, err.StoredConfig, err.NewConfig, err.RewindTo)
}

// 规则包装ChainConfig并且仅仅是合成糖，或者可以用于没有或需要有关块的信息的函数。
//
// 规则是一次性界面，意味着不应在过渡阶段之间使用它。
type Rules struct {
	ChainID     *big.Int
	IsHomestead bool
	//IsEIP150      bool
	//IsEIP155      bool
	//IsEIP158      bool
	//IsByzantium   bool
}

// 规则确保c的ChainID不是零。
func (c *ChainConfig) Rules(num *big.Int) Rules {
	chainID := c.ChainID
	if chainID == nil {
		chainID = new(big.Int)
	}
	return Rules{
		ChainID:     new(big.Int).Set(chainID),
		IsHomestead: c.IsHomestead(num),
		//IsEIP150: c.IsEIP150(num),
		//IsEIP155: c.IsEIP155(num),
		//IsEIP158: c.IsEIP158(num),
		//IsByzantium: c.IsByzantium(num),
	}
}

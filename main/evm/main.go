package main

import (
	"math/big"
	"os"
	"fmt"

	"github.com/aidoc/go-aidoc/main/utils"
	"gopkg.in/urfave/cli.v1"
)

var gitCommit = "" // Git SHA1提交发布的哈希值（通过链接器标志设置）

var (
	app = utils.NewApp(gitCommit, "evm 命令行界面")

	DebugFlag = cli.BoolFlag{
		Name:  "debug",
		Usage: "输出完整的跟踪日志",
	}
	MemProfileFlag = cli.StringFlag{
		Name:  "memprofile",
		Usage: "在给定路径创建内存配置文件",
	}
	CPUProfileFlag = cli.StringFlag{
		Name:  "cpuprofile",
		Usage: "在给定路径创建内存配置文件",
	}
	StatDumpFlag = cli.BoolFlag{
		Name:  "statdump",
		Usage: "显示堆栈和堆内存信息",
	}
	CodeFlag = cli.StringFlag{
		Name:  "code",
		Usage: "EVM 代码",
	}
	CodeFileFlag = cli.StringFlag{
		Name:  "codefile",
		Usage: "包含 EVM 代码的文件。 如果指定了' - '，则从 stdin 读取代码",
	}
	GasFlag = cli.Uint64Flag{
		Name:  "gas",
		Usage: "evm的 gas 限制",
		Value: 10000000000,
	}
	PriceFlag = utils.BigFlag{
		Name:  "price",
		Usage: "为 EVM 价格设定",
		Value: new(big.Int),
	}
	ValueFlag = utils.BigFlag{
		Name:  "value",
		Usage: "为evm设置的值",
		Value: new(big.Int),
	}
	DumpFlag = cli.BoolFlag{
		Name:  "dump",
		Usage: "运行后转储状态",
	}
	InputFlag = cli.StringFlag{
		Name:  "input",
		Usage: "EVM 的输入",
	}
	VerbosityFlag = cli.IntFlag{
		Name:  "verbosity",
		Usage: "设定详细程度",
	}
	CreateFlag = cli.BoolFlag{
		Name:  "create",
		Usage: "表示应该创建操作而不是调用",
	}
	GenesisFlag = cli.StringFlag{
		Name:  "prestate",
		Usage: "与预状态（prestate）配置JSON文件",
	}
	MachineFlag = cli.BoolFlag{
		Name:  "json",
		Usage: "以机器可读格式输出跟踪日志（json）",
	}
	SenderFlag = cli.StringFlag{
		Name:  "sender",
		Usage: "交易来源",
	}
	ReceiverFlag = cli.StringFlag{
		Name:  "receiver",
		Usage: "交易接收器（执行上下文）",
	}
	DisableMemoryFlag = cli.BoolFlag{
		Name:  "nomemory",
		Usage: "禁用内存输出",
	}
	DisableStackFlag = cli.BoolFlag{
		Name:  "nostack",
		Usage: "禁用堆栈输出",
	}
)

func init() {
	app.Flags = []cli.Flag{
		CreateFlag,
		DebugFlag,
		VerbosityFlag,
		CodeFlag,
		CodeFileFlag,
		GasFlag,
		PriceFlag,
		ValueFlag,
		DumpFlag,
		InputFlag,
		MemProfileFlag,
		CPUProfileFlag,
		StatDumpFlag,
		GenesisFlag,
		MachineFlag,
		SenderFlag,
		ReceiverFlag,
		DisableMemoryFlag,
		DisableStackFlag,
	}
	app.Commands = []cli.Command{
		compileCommand,
		disasmCommand,
		runCommand,
		stateTestCommand,
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// gad 是 AIDOC的官方命令行客户端。
package main

import (
	"math"
	"os"
	"runtime"
	godebug "runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aidoc/go-aidoc/service/debug"
	"github.com/aidoc/go-aidoc/main/utils"
	"github.com/aidoc/go-aidoc/service"
	"github.com/aidoc/go-aidoc/service/accounts"
	"github.com/aidoc/go-aidoc/service/accounts/keystore"
	"github.com/aidoc/go-aidoc/service/console"
	"github.com/aidoc/go-aidoc/service/adclient"
	"github.com/aidoc/go-aidoc/service/metrics"
	"github.com/aidoc/go-aidoc/service/node"
	"github.com/elastic/gosigar"
	"gopkg.in/urfave/cli.v1"
	"github.com/aidoc/go-aidoc/lib/logger"
	"github.com/aidoc/go-aidoc/lib/i18"
	"fmt"
)

const (
	clientIdentifier = "gad" //  通过网络通告的客户端标识符
)

var (
	// Git SHA1提交版本的哈希值（通过链接器标志设置）
	gitCommit = ""
	// 包含所有命令和标志的应用程序。
	app = utils.NewApp(gitCommit, "go-aidoc 命令行接口")
)

func init() {
	// 初始化CLI应用程序并启动GAD
	app.Action = gad
	app.HideVersion = true //  我们有一个命令打印版本
	app.Copyright = "Copyright 2018 The go-aidoc Authors"
	app.Commands = cliCommand
	sort.Sort(cli.CommandsByName(app.Commands))

	app.Flags = append(app.Flags, nodeFlags...)
	app.Flags = append(app.Flags, rpcFlags...)
	app.Flags = append(app.Flags, consoleFlags...)
	app.Flags = append(app.Flags, debug.Flags...)
	app.Flags = append(app.Flags, i18Flag)

	app.Before = func(ctx *cli.Context) error {
		runtime.GOMAXPROCS(runtime.NumCPU())

		//// 加载配置文件。
		if lang := ctx.GlobalString(i18Flag.Name); lang != "" {
			i18.I18_print.SwitchLang(lang)
		}
		if err := debug.Setup(ctx); err != nil {
			return err
		}
		// 限制缓存容量并调整垃圾收集器
		var mem gosigar.Mem
		if err := mem.Get(); err == nil {
			allowance := int(mem.Total / 1024 / 1024 / 3)
			if cache := ctx.GlobalInt(utils.CacheFlag.Name); cache > allowance {
				logger.Warn("清除 GO 缓存的 GC 限制", "provided", cache, "updated", allowance)
				ctx.GlobalSet(utils.CacheFlag.Name, strconv.Itoa(allowance))
			}
		}
		// 确保 Go 的 GC 忽略数据库缓存的触发百分比
		cache := ctx.GlobalInt(utils.CacheFlag.Name)
		gogc := math.Max(20, math.Min(100, 100/(float64(cache)/1024)))

		logger.Debug("配置 Go 的 GC 触发器间隔", "percent", int(gogc))
		godebug.SetGCPercent(int(gogc))

		// 启动系统运行时指标集合
		go metrics.CollectProcessMetrics(3 * time.Second)

		utils.SetupNetwork(ctx)
		return nil
	}

	app.After = func(ctx *cli.Context) error {
		debug.Exit()
		console.Stdin.Close() // 重置终端模式。
		return nil
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

// 如果没有运行特殊的子命令，gaidoc是进入系统的主要入口点。
// 它根据命令行参数创建一个默认节点，并以阻塞模式运行它，等待它关闭。
func gad(ctx *cli.Context) error {
	node := makeFullNode(ctx)
	startNode(ctx, node)
	node.Wait()
	return nil
}

// startNode启动系统节点和所有已注册的协议，然后解锁所有请求的账户，并启动RPC / IPC接口和矿工。
func startNode(ctx *cli.Context, stack *node.Node) {
	debug.Memsize.Add("node", stack)

	// 启动节点本身
	utils.StartNode(stack)

	// 解锁特别要求的任何账户
	ks := stack.AccountManager().Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)

	passwords := utils.MakePasswordList(ctx)
	unlocks := strings.Split(ctx.GlobalString(utils.UnlockedAccountFlag.Name), ",")
	for i, account := range unlocks {
		if trimmed := strings.TrimSpace(account); trimmed != "" {
			unlockAccount(ctx, ks, trimmed, i, passwords)
		}
	}
	// 注册钱包事件处理程序以打开和自动派生钱包
	events := make(chan accounts.WalletEvent, 16)
	stack.AccountManager().Subscribe(events)

	go func() {
		// 创建一个链状态读取器以进行自我推导
		rpcClient, err := stack.Attach()
		if err != nil {
			logger.CritF("无法附加到自己： %v", err.Error())
		}
		stateReader := adclient.NewClient(rpcClient)

		// 打开已经附加的钱包
		for _, wallet := range stack.AccountManager().Wallets() {
			if err := wallet.Open(""); err != nil {
				logger.Warn("无法打开钱包", "url", wallet.URL(),  err.Error())
			}
		}
		// 监听钱包事件，直至终止
		for event := range events {
			switch event.Kind {
			case accounts.WalletArrived:
				if err := event.Wallet.Open(""); err != nil {
					logger.Warn(" 新钱包出现了，未能打开 ", "url", event.Wallet.URL(),  err.Error())
				}
			case accounts.WalletOpened:
				status, _ := event.Wallet.Status()
				logger.Info("新钱包出现了", "url", event.Wallet.URL(), "状态", status)

				if event.Wallet.URL().Scheme == "ledger" {
					event.Wallet.SelfDerive(accounts.DefaultLedgerBaseDerivationPath, stateReader)
				} else {
					event.Wallet.SelfDerive(accounts.DefaultBaseDerivationPath, stateReader)
				}

			case accounts.WalletDropped:
				logger.Info("旧钱包掉了下来 ", "url", event.Wallet.URL())
				event.Wallet.Close()
			}
		}
	}()

	// 如果启用，则启动辅助服务
	if ctx.GlobalBool(utils.MiningEnabledFlag.Name) || ctx.GlobalBool(utils.DeveloperFlag.Name) {
		// 挖掘仅在完整的 AIDOC节点运行时才有意义
		if ctx.GlobalBool(utils.LightModeFlag.Name) || ctx.GlobalString(utils.SyncModeFlag.Name) == "light" {
			logger.CritF("轻客户端不支持采矿")
		}
		var aidoc *service.Aidoc
		if err := stack.Service(&aidoc); err != nil {
			logger.CritF("Aidoc 服务未运行 ：: %v", err.Error())
		}
		// 如果请求，请使用减少数量的线程
		if threads := ctx.GlobalInt(utils.MinerThreadsFlag.Name); threads > 0 {
			type threaded interface {
				SetThreads(threads int)
			}
			if th, ok := aidoc.Engine().(threaded); ok {
				th.SetThreads(threads)
			}
		}
		// 将gas价格设置为CLI的限制并开始挖掘
		aidoc.TxPool().SetGasPrice(utils.GlobalBig(ctx, utils.GasPriceFlag.Name))
		if err := aidoc.StartMining(true); err != nil {
			logger.CritF("无法开始挖掘： %v", err.Error())
		}
	}
}

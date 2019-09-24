


// puppeth 是组建和维护专用网络的命令。
package main

import (
	"math/rand"
	"os"
	"strings"
	"time"

	"gopkg.in/urfave/cli.v1"
	"github.com/aidoc/go-aidoc/lib/logger"
)
var log_puppeth = logger.New("puppeth")

// main 只是设置 CLI 应用程序的一个无聊的入口点。
func main() {
	app := cli.NewApp()
	app.Name = "puppeth"
	app.Usage = "assemble and maintain private aidoc networks"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "network",
			Usage: "name of the network to administer (no spaces or hyphens, please)",
		},
		cli.IntFlag{
			Name:  "loglevel",
			Value: 3,
			Usage: "log level to emit to the screen",
		},
	}
	app.Action = func(c *cli.Context) error {
		// 设置记录器以打印所有内容和随机生成器
		//log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(c.Int("loglevel")), log.StreamHandler(os.Stdout, log.TerminalFormat(true))))
		rand.Seed(time.Now().UnixNano())

		network := c.String("network")
		if strings.Contains(network, " ") || strings.Contains(network, "-") {
			log_puppeth.Crit("网络名称中不允许使用空格或连字符")
		}
		// 启动向导并放弃控制权
		makeWizard(c.String("network")).run()
		return nil
	}
	app.Run(os.Args)
}

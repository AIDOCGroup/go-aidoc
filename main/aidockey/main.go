package main

import (
	"os"
	"fmt"

	"github.com/aidoc/go-aidoc/main/utils"
	"gopkg.in/urfave/cli.v1"
)

const (
	defaultKeyfileName = "keyfile.json"
)

// Git SHA1提交发布的哈希值（通过链接器标志设置）
var gitCommit = ""

var app *cli.App

func init() {
	app = utils.NewApp(gitCommit, "Aidoc密钥管理员")
	app.Commands = []cli.Command{
		commandGenerate,
		commandInspect,
		commandChangePassphrase,
		commandSignMessage,
		commandVerifyMessage,
	}
}

//  常用的命令行标志。
var (
	passphraseFlag = cli.StringFlag{
		Name:  "passwordfile",
		Usage: "包含密钥文件密码的文件 ",
	}
	jsonFlag = cli.BoolFlag{
		Name:  "json",
		Usage: "输出JSON而不是人类可读的格式",
	}
)

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

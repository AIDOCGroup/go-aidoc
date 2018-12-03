// gad 是 AIDOC的官方命令行客户端。
package main

const (
	clientIdentifier = "gad" //  通过网络通告的客户端标识符
)

var (
	// Git SHA1提交版本的哈希值（通过链接器标志设置）
	gitCommit = ""
	// 包含所有命令和标志的应用程序。
	app = utils.NewApp(gitCommit, "go-aidoc 命令行接口")
)

func main() {

}

// 如果没有运行特殊的子命令，gaidoc是进入系统的主要入口点。
// 它根据命令行参数创建一个默认节点，并以阻塞模式运行它，等待它关闭。
func gad(ctx *cli.Context) error {
	node := makeFullNode(ctx)
	startNode(ctx, node)
	node.Wait()
	return nil
}
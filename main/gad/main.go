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

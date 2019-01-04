## Go Aidoc

官方golang实施Aidoc协议。

[![API Reference](
https://camo.githubusercontent.com/915b7be44ada53c290eb157634330494ebe3e30a/68747470733a2f2f676f646f632e6f72672f6769746875622e636f6d2f676f6c616e672f6764646f3f7374617475732e737667
)](https://godoc.org/github.com/aidoc/go-aidoc)
[![Go Report Card](https://goreportcard.com/badge/github.com/aidoc/go-aidoc)](https://goreportcard.com/report/github.com/aidoc/go-aidoc)
[![Travis](https://travis-ci.org/aidoc/go-aidoc.svg?branch=master)](https://travis-ci.org/aidoc/go-aidoc)
[![Gitter](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/aidoc/go-aidoc?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge)

自动构建可用于稳定版本和不稳定的主分支。
二进制存档发布在https://gaidoc.aidoc.org/downloads/。

## 构建源

有关先决条件和详细的构建说明，请阅读Wiki上的[安装说明]（https://github.com/aidoc/go-aidoc/wiki/Building-Aidoc）。

构建 gaidoc 需要Go（版本1.7或更高版本）和C编译器。
您可以使用自己喜欢的包管理器安装它们。
安装依赖项后，运行

 make gaidoc

或者，构建全套实用程序：

    make all

## 可执行文件

go-aidoc项目附带了几个在`cmd`目录中找到的包装器/可执行文件。

|  Command    | Description |
|:-----------:|-------------|
| **`gaidoc`**  | 我们的主要Aidoc CLI客户端。 它是进入Aidoc网络（主网络，测试网络或专用网络）的入口点，能够作为完整节点（默认）存档节点（保留所有历史状态）或轻型节点（实时检索数据）运行。 它可以被其他进程用作通过HTTP，WebSocket和/或IPC传输上公开的JSON RPC端点进入Aidoc网络的网关。 `gaidoc --help`和[CLI Wiki页面]（https://github.com/aidoc/go-aidoc/wiki/Command-Line-Options）用于命令行选项。|
| `abigen`    | 源代码生成器将Aidoc契约定义转换为易于使用的，编译时类型安全的Go包。 如果合同字节码也可用，它在普通[Aidoc contract ABIs]（https://github.com/aidoc/wiki/wiki/Aidoc-Contract-ABI）上运行，具有扩展功能。 但它也接受Solidity源文件，使开发更加简化。 有关详细信息，请参阅我们的[Native DApps]（https://github.com/aidoc/go-aidoc/wiki/Native-DApps:-Go-bindings-to-Aidoc-contracts）维基页面。|
| `bootnode`  | 剥离我们的Aidoc客户端实现版本，该版本仅参与网络节点发现协议，但不运行任何更高级别的应用程序协议。 它可以用作轻量级引导节点，以帮助在私有网络中查找对等点。|
| `evm`       | EVM（Aidoc虚拟机）的开发人员实用程序版本，能够在可配置环境和执行模式下运行字节码片段。 其目的是允许对EVM操作码进行隔离的，细粒度的调试（例如`evm --code 60ff60ff --debug`）。|
|`gaidocrpctest`| 开发人员实用工具，支持我们的[aidoc / service / rpc-test]（https://github.com/aidoc/service/rpc-tests）测试套件，该套件验证[Aidoc JSON RPC]的基线符合性（https：// github.com/aidoc/wiki/wiki/JSON-RPC）规范。 有关详细信息，请参阅[测试套件的自述文件]（https://github.com/aidoc/service/rpc-tests/blob/master/README.md）。|
| `rlpdump`   | 开发人员实用工具将二进制RLP（[递归长度前缀]（https://github.com/aidoc/wiki/wiki/RLP））转储（Aidoc协议使用的网络以及协商一致的数据编码）转换为用户 友好的分层表示（例如`rlpdump --hex CE0183FFFFFFC4C304050583616263`）。|
| `swarm`     | swarm守护进程和工具。 这是群网络的入口点。 `swarm --help`用于命令行选项和子命令。 有关swarm文档，请参阅https://swarm-guide.readthedocs.io。|
| `puppeth`   | 一个CLI向导，可帮助创建新的Aidoc网络。|

## 运行gaidoc

浏览所有可能的命令行标志超出了范围（请参阅我们的[CLI Wiki页面]
（https://github.com/aidoc/go-aidoc/wiki/Command-Line-Options）），
但我们' 我们枚举了一些常见的参数组合，以帮助您快速了解如何运行自己的Gaidoc实例。

### 主Aidoc网络上的完整节点

到目前为止，最常见的情况是人们想要简单地与Aidoc网络进行交互：
创建帐户; 转移资金; 部署并与合同互动。
 对于这个特定的用例，用户不关心多年的历史数据，因此我们可以快速快速同步到网络的当前状态。 为此：

```
$ gaidoc console
```

该命令将：

 *在快速同步模式下启动gaidoc（默认情况下，可以使用`--syncmode`标志进行更改），使其下载更多数据以换取避免处理Aidoc网络的整个历史记录，这非常占用CPU。
 *启动Gaidoc的内置交互式[JavaScript控制台]（https://github.com/aidoc/go-aidoc/wiki/JavaScript-Console），（通过尾随的`console`子命令），
  您可以通过它调用所有官方 [`web3`方法]（https://github.com/aidoc/wiki/wiki/JavaScript-API）
  以及Gaidoc自己的[管理API]（https://github.com/aidoc/go-aidoc/wiki/管理的API）。
  这也是可选的，如果你将其遗漏，你总是可以使用`gaidoc attach`附加到已经运行的Gaidoc实例。

### Aidoc测试网络上的完整节点

过渡到开发人员，如果你想玩创建Aidoc合同，你几乎肯定希望在没有任何真正的金钱的情况下这样做，直到你掌握了整个系统。
换句话说，您不想连接到主网络，而是希望将** test **网络加入您的节点，该节点完全等同于主网络，但仅限播放 AIDOC 。

```
$ gaidoc --testnet console
```

`console`子命令具有与上面完全相同的含义，它们在testnet上同样有用。 如果你跳到这里，请参阅上面的解释。

:但是，指定`--testnet`标志会重新配置你的Gaidoc实例

 *而不是使用默认数据目录（例如Linux上的`〜/ .aidoc`），Gaidoc将自己嵌套在`testnet`子文件夹（Linux上的`〜/ .aidoc / testnet`）中。
  注意，在OSX和Linux上，这也意味着连接到正在运行的testnet节点需要使用自定义端点，
  因为`gaidoc attach`将默认尝试连接到生产节点端点。 例如。
     `gaidoc attach <datadir> / testnet / gad.ipc`。 Windows用户不受此影响。
 *客户端将连接到测试网络，而不是连接主Aidoc网络，测试网络使用不同的P2P引导节点，不同的网络ID和创建状态。
   
*注意：虽然有一些内部保护措施可以防止交易在主网络和测试网络之间交叉，但您应该确保始终使用单独的账户来进行游戏币和真钱游戏。
 除非您手动移动帐户，否则Gaidoc默认情况下会正确分隔两个网络，并且不会在它们之间提供任何帐户。*

### Rinkeby测试网络上的完整节点

上述测试网络是基于aidochash工作证明一致性算法的跨客户端。 因此，它具有一定的额外开销，
并且由于网络的低难度/安全性而更容易受到重组攻击。
Go Aidoc还支持连接到基于证据的测试网络[* Rinkeby *]（https://www.rinkeby.io（由社区成员运营）。
这个网络更轻，更安全，但只有go-aidoc支持。


```
$ gaidoc --rinkeby console
```

### 表面配置

作为将众多标志传递给`gaidoc`二进制文件的替代方法，您还可以通过以下方式传递配置文件：

```
$ gaidoc --config /path/to/your_config.toml
```

要了解文件的外观，您可以使用`dumpconfig`子命令导出现有配置：

```
$ gaidoc --your-favourite-flags dumpconfig
```

*注意：这仅适用于gaidoc v1.6.0及更高版本。*

#### Docker快速启动

在您的机器上启动和运行 Aidoc 的最快方法之一是使用Docker：

```
docker run -d --name aidoc-node -v /Users/alice/aidoc:/root \
           -p 8545:8545 -p 30303:30303 \
           aidoc/client-go
```

这将在快速同步模式下启动gest，数据库内存容量为1GB，就像上面的命令一样。
它还将在您的主目录中创建一个持久卷，或者保存区块链以及映射默认端口。
还有一个“alpine”标签可用于图像的细长版本。

如果要从其他容器和/或主机访问 RPC，请不要忘记`--rpcaddr 0.0.0.0`。 默认情况下，`gaidoc`绑定到本地接口，并且无法从外部访问RPC端点。

### 以编程方式连接Gaidoc节点

作为开发人员，您很快就会想要通过自己的程序开始与Gaidoc和Aidoc网络进行交互，而不是通过控制台手动进行交互。
为了解决这个问题，Gaidoc内置了对基于JSON-RPC的API的支持
（[标准API]（https://github.com/aidoc/wiki/wiki/JSON-RPC）和[Gaidoc特定API]
（https：//github.com/aidoc/go-aidoc/wiki/Management-APIs））。
这些可以通过HTTP，WebSockets和IPC（基于unix的平台上的unix套接字，以及Windows上的命名管道）公开。

IPC接口默认启用并公开Gaidoc支持的所有API，而HTTP和WS接口需要手动启用，并且由于安全原因仅暴露API的子集。
这些可以打开/关闭并按照您的预期进行配置。

基于HTTP的JSON-RPC API选项：

   *`--rpc`启用HTTP-RPC服务器
   *`--rpcaddr` HTTP-RPC服务器侦听接口（默认值：“localhost”）
   *`--rpcport` HTTP-RPC服务器侦听端口（默认值：8545）
   *`--rpcapi` API通过HTTP-RPC接口提供（默认值：“aidoc，net，web3”）
   *`--rpccorsdomain`逗号分隔的域名列表，从中接受跨源请求（强制执行浏览器）
   *`--ws`启用 WS-RPC服务器
   *`--wsaddr` WS-RPC服务器监听接口（默认：“localhost”）
   *`--wsport` WS-RPC服务器侦听端口（默认值：8546）
   *`--wsapi` API通过WS-RPC接口提供（默认：“aidoc，net，web3”）
   *`--wsorigins`来自哪里接受websockets请求
   *`--ipcdisable`禁用IPC-RPC服务器
   *`--ipcapi` API通过IPC-RPC接口提供（默认：“admin，debug，aidoc，miner，net，personal，shh，txpool，web3”）
   *`--ipcpath` datadir中IPC套接字/管道的文件名（显式路径转义它）


您需要使用自己的编程环境功能（库，工具等）通过HTTP，WS或IPC连接到配置了上述标志的Gaidoc节点，
您需要说[JSON-RPC]（http ：//www.jsonrpc.org/specification）所有运输工具。
您可以为多个请求重用相同的连接！

**注意：请在执行此操作之前了解打开基于 HTTP/WS 的传输的安全隐患！
互联网上的黑客正在积极尝试用暴露的 API 破坏 Aidoc节点！
此外，所有浏览器选项卡都可以访问本地运行的Web服务器，因此恶意网页可能会试图破坏本地可用的API！**

### 运营专用网络

    维护您自己的专用网络更为复杂，因为需要手动设置官方网络中理所当然的许多配置。

#### 定义私人创世块状态

    首先，您需要创建网络的起源状态，所有节点都需要了解并达成一致意见。
    这包含一个小的JSON文件（例如称之为`genesis.json`）：

```json
{
  "config": {
        "chainId": 0,
        "homesteadBlock": 0,
        "eip155Block": 0,
        "eip158Block": 0
    },
  "alloc"      : {},
  "coinbase"   : "0x0000000000000000000000000000000000000000",
  "difficulty" : "0x20000",
  "extraData"  : "",
  "gasLimit"   : "0x2fefd8",
  "nonce"      : "0x0000000000000042",
  "mixhash"    : "0x0000000000000000000000000000000000000000000000000000000000000000",
  "parentHash" : "0x0000000000000000000000000000000000000000000000000000000000000000",
  "timestamp"  : "0x00"
}
```
    上述字段对于大多数用途应该没问题，尽管我们建议将`nonce`更改为某个随机值，以防止未知的远程节点能够连接到您。
    如果您想为某些帐户预先筹资以便于测试，可以使用帐户配置填充`alloc`字段：


```json
"alloc": {
  "0x0000000000000000000000000000000000000001": {"balance": "111111111"},
  "0x0000000000000000000000000000000000000002": {"balance": "222222222"}
}
```
使用上述JSON文件中定义的genesis状态，您需要在启动之前使用它初始化**每个** Gaidoc节点，以确保正确设置所有区块链参数：

```
$ gaidoc init path/to/genesis.json
```

#### 创建集合点

将要运行的所有节点初始化为所需的创建状态，您需要启动一个其他人可以用来在您的网络和/或互联网上查找彼此的引导节点。
干净的方法是配置和运行专用的bootnode：

```
$ bootnode --genkey=boot.key
$ bootnode --nodekey=boot.key
```
    在bootnode在线时，它将显示[`enode` URL]（https://github.com/aidoc/wiki/wiki/enode-url-format）
    其他节点可以用来连接它并交换对等信息。
    确保用外部可访问的IP替换显示的IP地址信息（最可能是`[::]`），以获得实际的`enode` URL。

*注意：您也可以使用完整的Gaidoc节点作为引导节点，但这是不太推荐的方式。*

#### 启动您的成员节点

    在bootnode可操作且可从外部访问的情况下（您可以尝试`telnet <ip> <port>`以确保它确实可以访问），
    通过`--bootnodes`标志启动指向引导节点的每个后续Gaidoc节点以进行对等体发现。
    可能还需要将您的专用网络的数据目录分开，因此还要指定自定义的`--datadir`标志。

```
$ gaidoc --datadir=path/to/custom/data/folder --bootnodes=<bootnode-enode-url-from-above>
```

*注意：由于您的网络将完全与主网络和测试网络隔离，因此您还需要配置一个矿工来处理事务并为您创建新块。*

#### 运行一个私有矿工

    在专用网络然而设置，单个CPU矿工实例是绰绰有余的实际用途，因为它可以产生在正确的时间间隔块的稳定流，
    而不需要重资源（考虑在单个线程中运行，无需多者要么）。
    要启动Gaidoc实例进行挖掘，请使用所有常用标志运行它，扩展为：

```
$ gaidoc <usual-flags> --mine --minerthreads=1 --aidocbase=0x0000000000000000000000000000000000000000
```
这将开始在单个CPU线程上挖掘块和事务，将所有程序记入
`--aidocbase`指定的帐户。 您可以通过更改默认气体来进一步调整采矿
限制块收敛到（`--targetgaslimit`），价格交易被接受（`--gasprice`）。


## Contribution
    感谢您考虑帮助解决源代码问题！ 我们欢迎任何人在互联网上的归属，并感谢即使是最小的修复！

    如果您想为go-aidoc做出贡献，请分叉，修复，提交并发送拉动请求，以便维护人员查看并合并到主代码库中。
    如果您希望提交更复杂的更改，请先在[我们的gitter频道]
    （https://gitter.im/aidoc/go-aidoc）上查看核心开发人员，以确保这些更改符合一般原则
    该项目和/或获得一些早期反馈，可以使您的工作更轻松，以及我们的审查和合并程序快速和简单。


请确保您的贡献符合我们的编码指南:

 * 代码必须遵守官方Go [格式化]（https://golang.org/doc/effective_go.html#formatting）指南（即使用[gofmt]（https://golang.org/cmd/gofmt/））。
 * 必须遵守官方的Go [评论]（https://golang.org/doc/effective_go.html#commentary）指南记录代码。
 * 拉取请求需要基于`master`分支并且打开。
 * 提交消息应以其修改的包为前缀。
 * 例如 “aidoc，rpc：make trace configs optional”

请参阅[开发人员指南]（https://github.com/aidoc/go-aidoc/wiki/Developers'-Guide）
有关配置环境，管理项目依赖性和测试过程的更多详细信息。

## 许可证

go-aidoc库（即`cmd`目录之外的所有代码）都是根据
[GNU较宽松通用公共许可证v3.0]（https://www.gnu.org/licenses/lgpl-3.0.en.html），
包含在我们的库中的'COPYING.LESSER`文件中。

go-aidoc二进制文件（即`cmd`目录中的所有代码）都是在
[GNU通用公共许可证v3.0]（https://www.gnu.org/licenses/gpl-3.0.en.html），也包括在内
在我们的'COPYING`文件的存储库中。

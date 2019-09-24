


package main

import (
	"bytes"
	"html/template"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"
	"github.com/aidoc/go-aidoc/lib/i18"
)

// walletDockerfile 是运行 Web 钱包所需的 Dockerfile。
var walletDockerfile = `
FROM puppeth/wallet:latest

ADD genesis.json /genesis.json

RUN \
  echo 'node server.js &'                     > wallet.sh && \
	echo 'gaidoc --cache 512 init /genesis.json' >> wallet.sh && \
	echo $'gaidoc --networkid {{.NetworkID}} --port {{.NodePort}} --bootnodes {{.Bootnodes}} --aidocstats \'{{.Aidocstats}}\' --cache=512 --rpc --rpcaddr=0.0.0.0 --rpccorsdomain "*" --rpcvhosts "*"' >> wallet.sh

RUN \
	sed -i 's/PuppethNetworkID/{{.NetworkID}}/g' dist/js/aidocwallet-master.js && \
	sed -i 's/PuppethNetwork/{{.Network}}/g'     dist/js/aidocwallet-master.js && \
	sed -i 's/PuppethDenom/{{.Denom}}/g'         dist/js/aidocwallet-master.js && \
	sed -i 's/PuppethHost/{{.Host}}/g'           dist/js/aidocwallet-master.js && \
	sed -i 's/PuppethRPCPort/{{.RPCPort}}/g'     dist/js/aidocwallet-master.js

ENTRYPOINT ["/bin/sh", "wallet.sh"]
`

// walletComposefile 是部署和维护Web钱包所需的 docker-compose.yml 文件。
var walletComposefile = `
version: '2'
services:
  wallet:
    build: .
    image: {{.Network}}/wallet
    ports:
      - "{{.NodePort}}:{{.NodePort}}"
      - "{{.NodePort}}:{{.NodePort}}/udp"
      - "{{.RPCPort}}:8545"{{if not .VHost}}
      - "{{.WebPort}}:80"{{end}}
    volumes:
      - {{.Datadir}}:/root/.aidoc
    environment:
      - NODE_PORT={{.NodePort}}/tcp
      - STATS={{.Aidocstats}}{{if .VHost}}
      - VIRTUAL_HOST={{.VHost}}
      - VIRTUAL_PORT=80{{end}}
    logging:
      driver: "json-file"
      options:
        max-size: "1m"
        max-file: "10"
    restart: always
`

// deployWallet 通过 SSH，docker 和 docker-compose 将新的 Web 钱包容器部署到远程计算机。
// 如果那里已经存在具有指定网络名称的实例，它将被覆盖！
func deployWallet(client *sshClient, network string, bootnodes []string, config *walletInfos, nocache bool) ([]byte, error) {
	// 生成要上载到服务器的内容
	workdir := i18.I18_print.Sprintf("%d", rand.Int63())
	files := make(map[string][]byte)

	dockerfile := new(bytes.Buffer)
	template.Must(template.New("").Parse(walletDockerfile)).Execute(dockerfile, map[string]interface{}{
		"Network":   strings.ToTitle(network),
		"Denom":     strings.ToUpper(network),
		"NetworkID": config.network,
		"NodePort":  config.nodePort,
		"RPCPort":   config.rpcPort,
		"Bootnodes": strings.Join(bootnodes, ","),
		"Aidocstats":  config.aidocstats,
		"Host":      client.address,
	})
	files[filepath.Join(workdir, "Dockerfile")] = dockerfile.Bytes()

	composefile := new(bytes.Buffer)
	template.Must(template.New("").Parse(walletComposefile)).Execute(composefile, map[string]interface{}{
		"Datadir":  config.datadir,
		"Network":  network,
		"NodePort": config.nodePort,
		"RPCPort":  config.rpcPort,
		"VHost":    config.webHost,
		"WebPort":  config.webPort,
		"Aidocstats": config.aidocstats[:strings.Index(config.aidocstats, ":")],
	})
	files[filepath.Join(workdir, "docker-compose.yaml")] = composefile.Bytes()

	files[filepath.Join(workdir, "genesis.json")] = config.genesis

	// 将部署文件上传到远程服务器（之后进行清理）
	if out, err := client.Upload(files); err != nil {
		return out, err
	}
	defer client.Run("rm -rf " + workdir)

	// 构建和部署启动或密封节点服务
	if nocache {
		return nil, client.Stream(i18.I18_print.Sprintf("cd %s && docker-compose -p %s build --pull --no-cache && docker-compose -p %s up -d --force-recreate", workdir, network, network))
	}
	return nil, client.Stream(i18.I18_print.Sprintf("cd %s && docker-compose -p %s up -d --build --force-recreate", workdir, network))
}

// walletInfos 从 Web 钱包状态检查返回，以允许报告各种配置参数。
type walletInfos struct {
	genesis  []byte
	network  int64
	datadir  string
	aidocstats string
	nodePort int
	rpcPort  int
	webHost  string
	webPort  int
}

// Report将类型化的结构转换为普通的字符串 - >字符串映射，其中包含用于向用户报告的大部分（但不是全部）字段。
func (info *walletInfos) Report() map[string]string {
	report := map[string]string{
		"Data directory":         info.datadir,
		"Aidocstats username":      info.aidocstats,
		"Node listener port ":    strconv.Itoa(info.nodePort),
		"RPC listener port ":     strconv.Itoa(info.rpcPort),
		"Website address ":       info.webHost,
		"Website listener port ": strconv.Itoa(info.webPort),
	}
	return report
}

// checkWallet 对 web 钱包服务器进行健康检查以验证它是否正在运行，如果是，则是否响应。
func checkWallet(client *sshClient, network string) (*walletInfos, error) {
	// 检查主机上可能的 Web 钱包容器
	infos, err := inspectContainer(client, i18.I18_print.Sprintf("%s_wallet_1", network))
	if err != nil {
		return nil, err
	}
	if !infos.running {
		return nil, ErrServiceOffline
	}
	// 从主机或反向代理解析端口
	webPort := infos.portmap["80/tcp"]
	if webPort == 0 {
		if proxy, _ := checkNginx(client, network); proxy != nil {
			webPort = proxy.port
		}
	}
	if webPort == 0 {
		return nil, ErrNotExposed
	}
	// 从反向代理和配置值中解析主机
	host := infos.envvars["VIRTUAL_HOST"]
	if host == "" {
		host = client.server
	}
	// 运行完整性检查以查看 devp2p 和 RPC 端口是否可访问
	nodePort := infos.portmap[infos.envvars["NODE_PORT"]]
	if err = checkPort(client.server, nodePort); err != nil {
		log_puppeth.Warn(i18.I18_print.Sprintf("钱包 devp2p 端口似乎无法访问"), "server", client.server, "port", nodePort,   err)
	}
	rpcPort := infos.portmap["8545/tcp"]
	if err = checkPort(client.server, rpcPort); err != nil {
		log_puppeth.Warn(i18.I18_print.Sprintf("钱包 RPC 端口似乎无法访问"), "server", client.server, "port", rpcPort,   err)
	}
	// 汇编并返回有用的信息
	stats := &walletInfos{
		datadir:  infos.volumes["/root/.aidoc"],
		nodePort: nodePort,
		rpcPort:  rpcPort,
		webHost:  host,
		webPort:  webPort,
		aidocstats: infos.envvars["STATS"],
	}
	return stats, nil
}


package main

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"github.com/aidoc/go-aidoc/lib/i18"
	"github.com/aidoc/go-aidoc/lib/chain_common"
	"fmt"
)

// nodeDockerfile 是运行 Aidoc 节点所需的 Dockerfile。
var nodeDockerfile = `
FROM aidoc/client-go:latest

ADD genesis.json /genesis.json
{{if .Unlock}}
	ADD signer.json /signer.json
	ADD signer.pass /signer.pass
{{end}}
RUN \
  echo 'gaidoc --cache 512 init /genesis.json' > gaidoc.sh && \{{if .Unlock}}
	echo 'mkdir -p /root/.aidoc/keystore/ && cp /signer.json /root/.aidoc/keystore/' >> gaidoc.sh && \{{end}}
	echo $'gaidoc --networkid {{.NetworkID}} --cache 512 --port {{.Port}} --maxpeers {{.Peers}} {{.LightFlag}} --aidocstats \'{{.Aidocstats}}\' {{if .Bootnodes}}--bootnodes {{.Bootnodes}}{{end}} {{if .Aidocbase}}--aidocbase {{.Aidocbase}} --mine --minerthreads 1{{end}} {{if .Unlock}}--unlock 0 --password /signer.pass --mine{{end}} --targetgaslimit {{.GasTarget}} --gasprice {{.GasPrice}}' >> gaidoc.sh

ENTRYPOINT ["/bin/sh", "gaidoc.sh"]
`

// nodeComposefile 是部署和维护Aidoc节点（现在为bootnode或miner）所需的 docker-compose.yml 文件。
var nodeComposefile = `
version: '2'
services:
  {{.Type}}:
    build: .
    image: {{.Network}}/{{.Type}}
    ports:
      - "{{.Port}}:{{.Port}}"
      - "{{.Port}}:{{.Port}}/udp"
    volumes:
      - {{.Datadir}}:/root/.aidoc{{if .Aidochashdir}}
      - {{.Aidochashdir}}:/root/.aidochash{{end}}
    environment:
      - PORT={{.Port}}/tcp
      - TOTAL_PEERS={{.TotalPeers}}
      - LIGHT_PEERS={{.LightPeers}}
      - STATS_NAME={{.Aidocstats}}
      - MINER_NAME={{.Aidocbase}}
      - GAS_TARGET={{.GasTarget}}
      - GAS_PRICE={{.GasPrice}}
    logging:
      driver: "json-file"
      options:
        max-size: "1m"
        max-file: "10"
    restart: always
`

// deployNode 通过 SSH，docker 和 docker-compose 将新的Aidoc点容器部署到远程计算机。
// 如果那里已经存在具有指定网络名称的实例，它将被覆盖！
func deployNode(client *sshClient, network string, bootnodes []string, config *nodeInfos, nocache bool) ([]byte, error) {
	kind := "sealnode"
	if config.keyJSON == "" && config.aidocbase == "" {
		kind = "bootnode"
		bootnodes = make([]string, 0)
	}
	// 生成要上载到服务器的内容
	workdir := i18.I18_print.Sprintf("%d", rand.Int63())
	files := make(map[string][]byte)

	lightFlag := ""
	if config.peersLight > 0 {
		lightFlag = i18.I18_print.Sprintf("--lightpeers=%d --lightserv=50", config.peersLight)
	}
	dockerfile := new(bytes.Buffer)
	template.Must(template.New("").Parse(nodeDockerfile)).Execute(dockerfile, map[string]interface{}{
		"NetworkID": config.network,
		"Port":      config.port,
		"Peers":     config.peersTotal,
		"LightFlag": lightFlag,
		"Bootnodes": strings.Join(bootnodes, ","),
		"Aidocstats":  config.aidocstats,
		"Aidocbase": config.aidocbase,
		"GasTarget": uint64(1000000 * config.gasTarget),
		"GasPrice":  uint64(1000000000 * config.gasPrice),
		"Unlock":    config.keyJSON != "",
	})
	files[filepath.Join(workdir, "Dockerfile")] = dockerfile.Bytes()

	composefile := new(bytes.Buffer)
	template.Must(template.New("").Parse(nodeComposefile)).Execute(composefile, map[string]interface{}{
		"Type":       kind,
		"Datadir":    config.datadir,
		"Aidochashdir":  config.aidochashdir,
		"Network":    network,
		"Port":       config.port,
		"TotalPeers": config.peersTotal,
		"Light":      config.peersLight > 0,
		"LightPeers": config.peersLight,
		"Aidocstats":   config.aidocstats[:strings.Index(config.aidocstats, ":")],
		"Aidocbase":  config.aidocbase,
		"GasTarget":  config.gasTarget,
		"GasPrice":   config.gasPrice,
	})
	files[filepath.Join(workdir, "docker-compose.yaml")] = composefile.Bytes()

	files[filepath.Join(workdir, "genesis.json")] = config.genesis
	if config.keyJSON != "" {
		files[filepath.Join(workdir, "signer.json")] = []byte(config.keyJSON)
		files[filepath.Join(workdir, "signer.pass")] = []byte(config.keyPass)
	}
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

// nodeInfos 从引导或密封节点状态检查返回，以允许报告各种配置参数。
type nodeInfos struct {
	genesis    []byte
	network    int64
	datadir    string
	aidochashdir  string
	aidocstats   string
	port       int
	enode      string
	peersTotal int
	peersLight int
	aidocbase  string
	keyJSON    string
	keyPass    string
	gasTarget  float64
	gasPrice   float64
}

// Report将类型化的结构转换为普通的字符串 - >字符串映射，其中包含用于向用户报告的大部分（但不是全部）字段。
func (info *nodeInfos) Report() map[string]string {
	report := map[string]string{
		"Data directory":           info.datadir,
		"Listener port":            strconv.Itoa(info.port),
		"Peer count (all total)":   strconv.Itoa(info.peersTotal),
		"Peer count (light nodes)": strconv.Itoa(info.peersLight),
		"Aidocstats username":        info.aidocstats,
	}
	if info.gasTarget > 0 {
		// 矿工或签名者节点
		report["Gas limit (baseline target)"] = i18.I18_print.Sprintf("%0.3f MGas", info.gasTarget)
		report["Gas price (minimum accepted)"] = i18.I18_print.Sprintf("%0.3f GDose", info.gasPrice)

		if info.aidocbase != "" {
			// Aidochash工作证明矿工
			report["Aidochash directory"] = info.aidochashdir
			report["Miner account"] = info.aidocbase
		}
		if info.keyJSON != "" {
			// Clique证明权威签名者
			var key struct {
				Address string `json:"address"`
			}
			if err := json.Unmarshal([]byte(info.keyJSON), &key); err == nil {
				report["Signer account"] = chain_common.HexToAddress(key.Address).Hex()
			} else {
				log_puppeth.Error("无法检索签名者地址",   err.Error())
			}
		}
	}
	return report
}

// checkNode 对引导或密封节点服务器进行运行状况检查，以验证它是否正在运行，如果是，则是否响应。
func checkNode(client *sshClient, network string, boot bool) (*nodeInfos, error) {
	kind := "bootnode"
	if !boot {
		kind = "sealnode"
	}
	// 检查主机上可能的 bootnode 容器
	infos, err := inspectContainer(client, i18.I18_print.Sprintf("%s_%s_1", network, kind))
	if err != nil {
		return nil, err
	}
	if !infos.running {
		return nil, ErrServiceOffline
	}
	// 从环境变量中解析几种类型
	totalPeers, _ := strconv.Atoi(infos.envvars["TOTAL_PEERS"])
	lightPeers, _ := strconv.Atoi(infos.envvars["LIGHT_PEERS"])
	gasTarget, _ := strconv.ParseFloat(infos.envvars["GAS_TARGET"], 64)
	gasPrice, _ := strconv.ParseFloat(infos.envvars["GAS_PRICE"], 64)

	// 容器可用，检索其节点ID及其创建 json
	var out []byte
	if out, err = client.Run(i18.I18_print.Sprintf("docker exec %s_%s_1 gaidoc --exec admin.nodeInfo.id attach", network, kind)); err != nil {
		return nil, ErrServiceUnreachable
	}
	id := bytes.Trim(bytes.TrimSpace(out), "\"")

	if out, err = client.Run(i18.I18_print.Sprintf("docker exec %s_%s_1 cat /genesis.json", network, kind)); err != nil {
		return nil, ErrServiceUnreachable
	}
	genesis := bytes.TrimSpace(out)

	keyJSON, keyPass := "", ""
	if out, err = client.Run(i18.I18_print.Sprintf("docker exec %s_%s_1 cat /signer.json", network, kind)); err == nil {
		keyJSON = string(bytes.TrimSpace(out))
	}
	if out, err = client.Run(i18.I18_print.Sprintf("docker exec %s_%s_1 cat /signer.pass", network, kind)); err == nil {
		keyPass = string(bytes.TrimSpace(out))
	}
	// 运行完整性检查以查看devp2p是否可访问
	port := infos.portmap[infos.envvars["PORT"]]
	if err = checkPort(client.server, port); err != nil {
		log_puppeth.Warn(i18.I18_print.Sprintf("%s devp2p port seems unreachable", strings.Title(kind)), "server", client.server, "port", port,   err.Error())
	}
	// 汇编并返回有用的信息
	stats := &nodeInfos{
		genesis:    genesis,
		datadir:    infos.volumes["/root/.aidoc"],
		aidochashdir:  infos.volumes["/root/.aidochash"],
		port:       port,
		peersTotal: totalPeers,
		peersLight: lightPeers,
		aidocstats:   infos.envvars["STATS_NAME"],
		aidocbase:  infos.envvars["MINER_NAME"],
		keyJSON:    keyJSON,
		keyPass:    keyPass,
		gasTarget:  gasTarget,
		gasPrice:   gasPrice,
	}
	stats.enode = fmt.Sprintf("enode://%s@%s:%d", id, client.address, stats.port)

	return stats, nil
}

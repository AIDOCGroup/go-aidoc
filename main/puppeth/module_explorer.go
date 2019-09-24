


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

// explorerDockerfile是运行块浏览器所需的Dockerfile。
var explorerDockerfile = `
FROM puppeth/explorer:latest

ADD aidocstats.json /aidocstats.json
ADD chain.json /chain.json

RUN \
  echo '(cd ../aidoc-net-intelligence-api && pm2 start /aidocstats.json)' >  explorer.sh && \
	echo '(cd ../aidocchain-light && npm start &)'                      >> explorer.sh && \
	echo '/parity/parity --chain=/chain.json --port={{.NodePort}} --tracing=on --fat-db=on --pruning=archive' >> explorer.sh

ENTRYPOINT ["/bin/sh", "explorer.sh"]
`

// explorerAidocstats是aidocstats javascript客户端的配置文件。
var explorerAidocstats = `[
  {
    "name"              : "node-app",
    "script"            : "app.js",
    "log_date_format"   : "YYYY-MM-DD HH:mm Z",
    "merge_logs"        : false,
    "watch"             : false,
    "max_restarts"      : 10,
    "exec_interpreter"  : "node",
    "exec_mode"         : "fork_mode",
    "env":
    {
      "NODE_ENV"        : "production",
      "RPC_HOST"        : "localhost",
      "RPC_PORT"        : "8545",
      "LISTENING_PORT"  : "{{.Port}}",
      "INSTANCE_NAME"   : "{{.Name}}",
      "CONTACT_DETAILS" : "",
      "WS_SERVER"       : "{{.Host}}",
      "WS_SECRET"       : "{{.Secret}}",
      "VERBOSITY"       : 2
    }
  }
]`

// explorerComposefile 是部署和维护块资源管理器所需的 docker-compose.yml 文件。
var explorerComposefile = `
version: '2'
services:
  explorer:
    build: .
    image: {{.Network}}/explorer
    ports:
      - "{{.NodePort}}:{{.NodePort}}"
      - "{{.NodePort}}:{{.NodePort}}/udp"{{if not .VHost}}
      - "{{.WebPort}}:3000"{{end}}
    volumes:
      - {{.Datadir}}:/root/.local/share/io.parity.aidoc
    environment:
      - NODE_PORT={{.NodePort}}/tcp
      - STATS={{.Aidocstats}}{{if .VHost}}
      - VIRTUAL_HOST={{.VHost}}
      - VIRTUAL_PORT=3000{{end}}
    logging:
      driver: "json-file"
      options:
        max-size: "1m"
        max-file: "10"
    restart: always
`

//  deployExplorer通过SSH，docker和docker-compose将新的块资源管理器容器部署到远程计算机。
//  如果那里已经存在具有指定网络名称的实例，它将被覆盖！
func deployExplorer(client *sshClient, network string, chainspec []byte, config *explorerInfos, nocache bool) ([]byte, error) {
	// 生成要上载到服务器的内容
	workdir := i18.I18_print.Sprintf("%d", rand.Int63())
	files := make(map[string][]byte)

	dockerfile := new(bytes.Buffer)
	template.Must(template.New("").Parse(explorerDockerfile)).Execute(dockerfile, map[string]interface{}{
		"NodePort": config.nodePort,
	})
	files[filepath.Join(workdir, "Dockerfile")] = dockerfile.Bytes()

	aidocstats := new(bytes.Buffer)
	template.Must(template.New("").Parse(explorerAidocstats)).Execute(aidocstats, map[string]interface{}{
		"Port":   config.nodePort,
		"Name":   config.aidocstats[:strings.Index(config.aidocstats, ":")],
		"Secret": config.aidocstats[strings.Index(config.aidocstats, ":")+1 : strings.Index(config.aidocstats, "@")],
		"Host":   config.aidocstats[strings.Index(config.aidocstats, "@")+1:],
	})
	files[filepath.Join(workdir, "aidocstats.json")] = aidocstats.Bytes()

	composefile := new(bytes.Buffer)
	template.Must(template.New("").Parse(explorerComposefile)).Execute(composefile, map[string]interface{}{
		"Datadir":  config.datadir,
		"Network":  network,
		"NodePort": config.nodePort,
		"VHost":    config.webHost,
		"WebPort":  config.webPort,
		"Aidocstats": config.aidocstats[:strings.Index(config.aidocstats, ":")],
	})
	files[filepath.Join(workdir, "docker-compose.yaml")] = composefile.Bytes()

	files[filepath.Join(workdir, "chain.json")] = chainspec

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

// explorerInfos 从块资源管理器状态检查返回，以允许报告各种配置参数。
type explorerInfos struct {
	datadir  string
	aidocstats string
	nodePort int
	webHost  string
	webPort  int
}

// Report将类型化的结构转换为普通的字符串 - >字符串映射，其中包含用于向用户报告的大部分（但不是全部）字段。
func (info *explorerInfos) Report() map[string]string {
	report := map[string]string{
		"Data directory":         info.datadir,
		"Node listener port ":    strconv.Itoa(info.nodePort),
		"Aidocstats username":      info.aidocstats,
		"Website address ":       info.webHost,
		"Website listener port ": strconv.Itoa(info.webPort),
	}
	return report
}

// checkExplorer对块浏览器服务器进行运行状况检查，以验证它是否正在运行，如果是，则是否响应。
func checkExplorer(client *sshClient, network string) (*explorerInfos, error) {
	// 检查主机上可能的块资源管理器容器
	infos, err := inspectContainer(client, i18.I18_print.Sprintf("%s_explorer_1", network))
	if err != nil {
		return nil, err
	}
	if !infos.running {
		return nil, ErrServiceOffline
	}
	// 从主机或反向代理解析端口
	webPort := infos.portmap["3000/tcp"]
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
	// 运行完整性检查以查看devp2p是否可访问
	nodePort := infos.portmap[infos.envvars["NODE_PORT"]]
	if err = checkPort(client.server, nodePort); err != nil {
		log_wizard.Warn(i18.I18_print.Sprintf("Explorer devp2p port seems unreachable"), "server", client.server, "port", nodePort,   err.Error())
	}
	// 汇编并返回有用的信息
	stats := &explorerInfos{
		datadir:  infos.volumes["/root/.local/share/io.parity.aidoc"],
		nodePort: nodePort,
		webHost:  host,
		webPort:  webPort,
		aidocstats: infos.envvars["STATS"],
	}
	return stats, nil
}

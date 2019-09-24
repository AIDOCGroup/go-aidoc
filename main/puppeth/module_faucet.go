


package main

import (
	"bytes"
	"encoding/json"
	"html/template"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"
	"github.com/aidoc/go-aidoc/lib/i18"
	"github.com/aidoc/go-aidoc/lib/chain_common"
)

// faucetDockerfile是构建 faucet 容器以根据GitHub身份验证授予加密令牌所需的Dockerfile。
var faucetDockerfile = `
FROM aidoc/client-go:alltools-latest

ADD genesis.json /genesis.json
ADD account.json /account.json
ADD account.pass /account.pass

EXPOSE 8080 30303 30303/udp

ENTRYPOINT [ \
	"faucet", "--genesis", "/genesis.json", "--network", "{{.NetworkID}}", "--bootnodes", "{{.Bootnodes}}", "--aidocstats", "{{.Aidocstats}}", "--ethport", "{{.AidocPort}}",     \
	"--faucet.name", "{{.FaucetName}}", "--faucet.amount", "{{.FaucetAmount}}", "--faucet.minutes", "{{.FaucetMinutes}}", "--faucet.tiers", "{{.FaucetTiers}}",             \
	"--account.json", "/account.json", "--account.pass", "/account.pass"                                                                                                    \
	{{if .CaptchaToken}}, "--captcha.token", "{{.CaptchaToken}}", "--captcha.secret", "{{.CaptchaSecret}}"{{end}}{{if .NoAuth}}, "--noauth"{{end}}                          \
]`

// faucetComposefile 是部署和维护加密 faucet 所需的docker-compose.yml文件。
var faucetComposefile = `
version: '2'
services:
  faucet:
    build: .
    image: {{.Network}}/faucet
    ports:
      - "{{.AidocPort}}:{{.AidocPort}}"{{if not .VHost}}
      - "{{.ApiPort}}:8080"{{end}}
    volumes:
      - {{.Datadir}}:/root/.faucet
    environment:
      - AIDOC_PORT={{.AidocPort}}
      - AIDOC_NAME={{.AidocName}}
      - FAUCET_AMOUNT={{.FaucetAmount}}
      - FAUCET_MINUTES={{.FaucetMinutes}}
      - FAUCET_TIERS={{.FaucetTiers}}
      - CAPTCHA_TOKEN={{.CaptchaToken}}
      - CAPTCHA_SECRET={{.CaptchaSecret}}
      - NO_AUTH={{.NoAuth}}{{if .VHost}}
      - VIRTUAL_HOST={{.VHost}}
      - VIRTUAL_PORT=8080{{end}}
    logging:
      driver: "json-file"
      options:
        max-size: "1m"
        max-file: "10"
    restart: always
`

// deployFaucet通过SSH，docker 和 docker-compose 将新的 Faucet 容器部署到远程机器。
// 如果那里已经存在具有指定网络名称的实例，它将被覆盖！
func deployFaucet(client *sshClient, network string, bootnodes []string, config *faucetInfos, nocache bool) ([]byte, error) {
	// 生成要上载到服务器的内容
	workdir := i18.I18_print.Sprintf("%d", rand.Int63())
	files := make(map[string][]byte)

	dockerfile := new(bytes.Buffer)
	template.Must(template.New("").Parse(faucetDockerfile)).Execute(dockerfile, map[string]interface{}{
		"NetworkID":     config.node.network,
		"Bootnodes":     strings.Join(bootnodes, ","),
		"Aidocstats":      config.node.aidocstats,
		"AidocPort":       config.node.port,
		"CaptchaToken":  config.captchaToken,
		"CaptchaSecret": config.captchaSecret,
		"FaucetName":    strings.Title(network),
		"FaucetAmount":  config.amount,
		"FaucetMinutes": config.minutes,
		"FaucetTiers":   config.tiers,
		"NoAuth":        config.noauth,
	})
	files[filepath.Join(workdir, "Dockerfile")] = dockerfile.Bytes()

	composefile := new(bytes.Buffer)
	template.Must(template.New("").Parse(faucetComposefile)).Execute(composefile, map[string]interface{}{
		"Network":       network,
		"Datadir":       config.node.datadir,
		"VHost":         config.host,
		"ApiPort":       config.port,
		"AidocPort":       config.node.port,
		"AidocName":       config.node.aidocstats[:strings.Index(config.node.aidocstats, ":")],
		"CaptchaToken":  config.captchaToken,
		"CaptchaSecret": config.captchaSecret,
		"FaucetAmount":  config.amount,
		"FaucetMinutes": config.minutes,
		"FaucetTiers":   config.tiers,
		"NoAuth":        config.noauth,
	})
	files[filepath.Join(workdir, "docker-compose.yaml")] = composefile.Bytes()

	files[filepath.Join(workdir, "genesis.json")] = config.node.genesis
	files[filepath.Join(workdir, "account.json")] = []byte(config.node.keyJSON)
	files[filepath.Join(workdir, "account.pass")] = []byte(config.node.keyPass)

	// 将部署文件上传到远程服务器（之后进行清理）
	if out, err := client.Upload(files); err != nil {
		return out, err
	}
	defer client.Run("rm -rf " + workdir)

	// 构建和部署 faucet 服务
	if nocache {
		return nil, client.Stream(i18.I18_print.Sprintf("cd %s && docker-compose -p %s build --pull --no-cache && docker-compose -p %s up -d --force-recreate", workdir, network, network))
	}
	return nil, client.Stream(i18.I18_print.Sprintf("cd %s && docker-compose -p %s up -d --build --force-recreate", workdir, network))
}

// faucetInfos 从 faucet 状态检查返回，以允许报告各种配置参数。
type faucetInfos struct {
	node          *nodeInfos
	host          string
	port          int
	amount        int
	minutes       int
	tiers         int
	noauth        bool
	captchaToken  string
	captchaSecret string
}

// Report将类型化的结构转换为普通的字符串 - >字符串映射，其中包含用于向用户报告的大部分（但不是全部）字段。
func (info *faucetInfos) Report() map[string]string {
	report := map[string]string{
		"Website address":              info.host,
		"Website listener port":        strconv.Itoa(info.port),
		"Aidoc listener port":       strconv.Itoa(info.node.port),
		"Funding amount (base tier)":   i18.I18_print.Sprintf("%d Aidocs", info.amount),
		"Funding cooldown (base tier)": i18.I18_print.Sprintf("%d mins", info.minutes),
		"Funding tiers":                strconv.Itoa(info.tiers),
		"Captha protection":            i18.I18_print.Sprintf("%v", info.captchaToken != ""),
		"Aidocstats username":            info.node.aidocstats,
	}
	if info.noauth {
		report["Debug mode (no auth)"] = "enabled"
	}
	if info.node.keyJSON != "" {
		var key struct {
			Address string `json:"address"`
		}
		if err := json.Unmarshal([]byte(info.node.keyJSON), &key); err == nil {
			report["Funding account"] = chain_common.HexToAddress(key.Address).Hex()
		} else {
			log_puppeth.Error("无法检索签名者地址",   err.Error())
		}
	}
	return report
}

// checkFaucet对 Faucet 服务器进行健康检查以验证它是否正在运行，如果是，则收集有关它的有用信息集合。...
func checkFaucet(client *sshClient, network string) (*faucetInfos, error) {
	// 检查主机上可能的 Faucet 容器
	infos, err := inspectContainer(client, i18.I18_print.Sprintf("%s_faucet_1", network))
	if err != nil {
		return nil, err
	}
	if !infos.running {
		return nil, ErrServiceOffline
	}
	// 从主机或反向代理解析端口
	port := infos.portmap["8080/tcp"]
	if port == 0 {
		if proxy, _ := checkNginx(client, network); proxy != nil {
			port = proxy.port
		}
	}
	if port == 0 {
		return nil, ErrNotExposed
	}
	// 从反向代理和配置值中解析主机
	host := infos.envvars["VIRTUAL_HOST"]
	if host == "" {
		host = client.server
	}
	amount, _ := strconv.Atoi(infos.envvars["FAUCET_AMOUNT"])
	minutes, _ := strconv.Atoi(infos.envvars["FAUCET_MINUTES"])
	tiers, _ := strconv.Atoi(infos.envvars["FAUCET_TIERS"])

	// 检索资金账户信息
	var out []byte
	keyJSON, keyPass := "", ""
	if out, err = client.Run(i18.I18_print.Sprintf("docker exec %s_faucet_1 cat /account.json", network)); err == nil {
		keyJSON = string(bytes.TrimSpace(out))
	}
	if out, err = client.Run(i18.I18_print.Sprintf("docker exec %s_faucet_1 cat /account.pass", network)); err == nil {
		keyPass = string(bytes.TrimSpace(out))
	}
	// 运行完整性检查以查看端口是否可访问
	if err = checkPort(host, port); err != nil {
		log_puppeth.Warn("Faucet service seems unreachable", "server", host, "port", port,   err)
	}
	// 容器可用，组装并返回有用的信息
	return &faucetInfos{
		node: &nodeInfos{
			datadir:  infos.volumes["/root/.faucet"],
			port:     infos.portmap[infos.envvars["AIDOC_PORT"]+"/tcp"],
			aidocstats: infos.envvars["AIDOC_NAME"],
			keyJSON:  keyJSON,
			keyPass:  keyPass,
		},
		host:          host,
		port:          port,
		amount:        amount,
		minutes:       minutes,
		tiers:         tiers,
		captchaToken:  infos.envvars["CAPTCHA_TOKEN"],
		captchaSecret: infos.envvars["CAPTCHA_SECRET"],
		noauth:        infos.envvars["NO_AUTH"] == "true",
	}, nil
}

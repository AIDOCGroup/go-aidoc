


package main

import (
	"bytes"
	"html/template"
	"math/rand"
	"path/filepath"
	"strconv"
	"github.com/aidoc/go-aidoc/lib/i18"
	"github.com/aidoc/go-aidoc/lib/logger"
)

// nginx Dockerfile是构建nginx反向代理所需的Dockerfile。
var nginxDockerfile = `FROM jwilder/nginx-proxy`

// nginxComposefile 是部署和维护 nginx 反向代理所需的 docker-compose.yml 文件。
// 代理负责公开在单个主机上运行的一个或多个HTTP服务。
var nginxComposefile = `
version: '2'
services:
  nginx:
    build: .
    image: {{.Network}}/nginx
    ports:
      - "{{.Port}}:80"
    volumes:
      - /var/run/docker.sock:/tmp/docker.sock:ro
    logging:
      driver: "json-file"
      options:
        max-size: "1m"
        max-file: "10"
    restart: always
`

// deployNginx部署新的nginx反向代理容器，以公开在单个主机上运行的一个或多个HTTP服务。
// 如果那里已经存在具有指定网络名称的实例，它将被覆盖！
func deployNginx(client *sshClient, network string, port int, nocache bool) ([]byte, error) {
	logger.Info("Deploying nginx reverse-proxy", "server", client.server, "port", port)

	// 生成要上载到服务器的内容
	workdir := i18.I18_print.Sprintf("%d", rand.Int63())
	files := make(map[string][]byte)

	dockerfile := new(bytes.Buffer)
	template.Must(template.New("").Parse(nginxDockerfile)).Execute(dockerfile, nil)
	files[filepath.Join(workdir, "Dockerfile")] = dockerfile.Bytes()

	composefile := new(bytes.Buffer)
	template.Must(template.New("").Parse(nginxComposefile)).Execute(composefile, map[string]interface{}{
		"Network": network,
		"Port":    port,
	})
	files[filepath.Join(workdir, "docker-compose.yaml")] = composefile.Bytes()

	// 将部署文件上传到远程服务器（之后进行清理）
	if out, err := client.Upload(files); err != nil {
		return out, err
	}
	defer client.Run("rm -rf " + workdir)

	// 构建和部署反向代理服务
	if nocache {
		return nil, client.Stream(i18.I18_print.Sprintf("cd %s && docker-compose -p %s build --pull --no-cache && docker-compose -p %s up -d --force-recreate", workdir, network, network))
	}
	return nil, client.Stream(i18.I18_print.Sprintf("cd %s && docker-compose -p %s up -d --build --force-recreate", workdir, network))
}

// nginxInfos从nginx反向代理状态检查返回，以允许报告各种配置参数。
type nginxInfos struct {
	port int
}

// Report将类型化的结构转换为普通的字符串 - >字符串映射，其中包含用于向用户报告的大部分（但不是全部）字段。
func (info *nginxInfos) Report() map[string]string {
	return map[string]string{
		"Shared listener port": strconv.Itoa(info.port),
	}
}

// checkNginx对nginx反向代理进行健康检查以验证它是否正在运行，如果是，则收集有关它的有用信息集合。
func checkNginx(client *sshClient, network string) (*nginxInfos, error) {
	// 检查主机上可能的nginx容器
	infos, err := inspectContainer(client, i18.I18_print.Sprintf("%s_nginx_1", network))
	if err != nil {
		return nil, err
	}
	if !infos.running {
		return nil, ErrServiceOffline
	}
	// 容器可用，组装并返回有用的信息
	return &nginxInfos{
		port: infos.portmap["80/tcp"],
	}, nil
}

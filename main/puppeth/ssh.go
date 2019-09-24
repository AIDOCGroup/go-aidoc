package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	logger2 "github.com/aidoc/go-aidoc/lib/logger"
	"github.com/aidoc/go-aidoc/lib/i18"
)


var log_ssh = logger2.New("puppeth-client")
// sshClient 是 Go 的 SSH 客户端的一个小包装器，在顶部实现了一些实用方法。
type sshClient struct {
	server  string // 没有端口号的服务器名称或IP
	address string // 远程服务器的IP地址
	pubkey  []byte // 用于验证服务器的RSA公钥
	client  *ssh.Client
}

// dial 使用当前用户和用户配置的专用 RSA 密钥建立与远程节点的 SSH 连接。
// 如果失败，密码验证将回落。
// 调用者可以通过user @ server：port 覆盖登录用户。
func dial(server string, pubkey []byte) (*sshClient, error) {
	// 找出服务器和记录器的标签
	label := server
	if strings.Contains(label, ":") {
		label = label[:strings.Index(label, ":")]
	}
	login := ""
	if strings.Contains(server, "@") {
		login = label[:strings.Index(label, "@")]
		label = label[strings.Index(label, "@")+1:]
		server = server[strings.Index(server, "@")+1:]
	}
	log_ssh.Debug(i18.I18_print.Sprintf("%s 试图建立 SSH 连接" , label))

	user, err := user.Current()
	if err != nil {
		return nil, err
	}
	if login == "" {
		login = user.Username
	}
	// 配置支持的身份验证方法（私钥和密码）
	var auths []ssh.AuthMethod

	path := filepath.Join(user.HomeDir, ".ssh", "id_rsa")
	if buf, err := ioutil.ReadFile(path); err != nil {
		log_ssh.Error("没有SSH密钥，回到密码", "path", path,   err)
	} else {
		key, err := ssh.ParsePrivateKey(buf)
		if err != nil {
			log_ssh.InfoF("%s 的解密密码是什么？ （不会回应）\n>", path)
			blob, err := terminal.ReadPassword(int(os.Stdin.Fd()))
			i18.I18_print.Println()
			if err != nil {
				log_ssh.Error("无法读取密码",  err)
			}
			key, err := ssh.ParsePrivateKeyWithPassphrase(buf, blob)
			if err != nil {
				log_ssh.Error("无法解密SSH密钥，回退到密码", "path", path,  err)
			} else {
				auths = append(auths, ssh.PublicKeys(key))
			}
		} else {
			auths = append(auths, ssh.PublicKeys(key))
		}
	}
	auths = append(auths, ssh.PasswordCallback(func() (string, error) {
		log_ssh.InfoF("什么是 %s 的 %s 的登录密码？ （不会回应）\n> ", login, server)
		blob, err := terminal.ReadPassword(int(os.Stdin.Fd()))

		i18.I18_print.Println()
		return string(blob), err
	}))
	// 解析远程服务器的IP地址
	addr, err := net.LookupHost(label)
	if err != nil {
		return nil, err
	}
	if len(addr) == 0 {
		return nil, errors.New("没有与域关联的IP")
	}
	// Try to dial in to the remote server
	log_ssh.Trace("拨打远程SSH服务器", "user", login)
	if !strings.Contains(server, ":") {
		server += ":22"
	}
	keycheck := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		// 如果没有SSH的公钥，请要求用户确认
		if pubkey == nil {
			i18.I18_print.Println()
			log_ssh.InfoF("无法建立主机'%s（%s）'的真实性。\n", hostname, remote)
			log_ssh.InfoF("SSH密钥指纹是%s [MD5] \n", ssh.FingerprintLegacyMD5(key))
			log_ssh.InfoF("您确定要继续连接（是/否）吗？")

			text, err := bufio.NewReader(os.Stdin).ReadString('\n')
			switch {
			case err != nil:
				return err
			case strings.TrimSpace(text) == "yes":
				pubkey = key.Marshal()
				return nil
			default:
				return fmt.Errorf(i18.I18_print.Sprintf("未知的身份验证选择：%v", text))
			}
		}
		//  如果此SSH服务器存在公钥，请检查它是否匹配
		if bytes.Equal(pubkey, key.Marshal()) {
			return nil
		}
		// 我们有不匹配，禁止连接
		return errors.New("ssh 密钥不匹配，读取机器更新")
	}
	client, err := ssh.Dial("tcp", server, &ssh.ClientConfig{User: login, Auth: auths, HostKeyCallback: keycheck})
	if err != nil {
		return nil, err
	}
	// 建立连接，返回我们的实用程序包装器
	c := &sshClient{
		server:  label,
		address: addr[0],
		pubkey:  pubkey,
		client:  client,
	}
	if err := c.init(); err != nil {
		client.Close()
		return nil, err
	}
	return c, nil
}

// init 在远程服务器上运行一些初始化命令，以确保它能够充当 puppeth 目标。
func (client *sshClient) init() error {
	log_ssh.Debug("验证docker是否可用")
	if out, err := client.Run("docker version"); err != nil {
		if len(out) == 0 {
			return err
		}
		return fmt.Errorf(i18.I18_print.Sprintf("docker 配置不正确: %s", out))
	}
	log_ssh.Debug("验证 docker-compose 是否可用")
	if out, err := client.Run("docker-compose version"); err != nil {
		if len(out) == 0 {
			return err
		}
		return fmt.Errorf(i18.I18_print.Sprintf("docker-compose 配置错误：%s", out))
	}
	return nil
}

// Close 将终止与 SSH 服务器的连接。
func (client *sshClient) Close() error {
	return client.client.Close()
}

// Run 在远程服务器上执行命令并返回组合输出以及任何错误状态。
func (client *sshClient) Run(cmd string) ([]byte, error) {
	// 建立单个命令会话
	session, err := client.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	// 执行命令并返回任何输出
	log_ssh.Trace("在远程服务器上运行命令", "cmd", cmd)
	return session.CombinedOutput(cmd)
}

//  Stream 在远程服务器上执行命令，并将所有输出流式传输到本地 stdout 和 stderr 流中。
func (client *sshClient) Stream(cmd string) error {
	// 建立单个命令会话
	session, err := client.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	// 执行命令并返回任何输出
	log_ssh.Trace("远程服务器上的 Streaming 命令", "cmd", cmd)
	return session.Run(cmd)
}

// 上传通过 SCP 将文件集复制到远程服务器，同时创建任何不存在的文件夹。
func (client *sshClient) Upload(files map[string][]byte) ([]byte, error) {
	// 建立单个命令会话
	session, err := client.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	//  创建一个流式传输 SCP 内容的 goroutine
	go func() {
		out, _ := session.StdinPipe()
		defer out.Close()

		for file, content := range files {
			log_ssh.Trace("将文件上传到服务器", "file", file, "bytes", len(content))

			fmt.Fprintln(out, "D0755", 0, filepath.Dir(file))             // 确保文件夹存在
			fmt.Fprintln(out, "C0644", len(content), filepath.Base(file)) // 创建实际文件
			out.Write(content)                                               // 流式传输数据内容
			fmt.Fprint(out, "\x00")                                       // 使用\ x00转移结束
			fmt.Fprintln(out, "E")                                        // 离开目录（更简单）
		}
	}()
	return session.CombinedOutput("/usr/bin/scp -v -tr ./")
}

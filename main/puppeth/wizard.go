package main

import (
	"bufio"
	"encoding/json"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"fmt"

	"github.com/aidoc/go-aidoc/lib/chain_common"
	"github.com/aidoc/go-aidoc/lib/chain_core"
	"golang.org/x/crypto/ssh/terminal"
	"github.com/aidoc/go-aidoc/lib/logger"
	"github.com/aidoc/go-aidoc/lib/i18"
)

var log_wizard = logger.New("wizard")
// config包含应在会话之间保存的puppeth所需的所有配置。
type config struct {
	path      string   // 包含配置值的文件
	bootnodes []string // Bootnodes始终由所有节点连接
	aidocstats  string   // 用于缓存节点部署的Aidocstats设置

	Genesis *chain_core.Genesis `json:"genesis,omitempty"` // Genesis 区块缓存节点部署
	Servers map[string][]byte   `json:"servers,omitempty"`
}

// 服务器检索按字母顺序排序的服务器列表。
func (c config) servers() []string {
	servers := make([]string, 0, len(c.Servers))
	for server := range c.Servers {
		servers = append(servers, server)
	}
	sort.Strings(servers)

	return servers
}

// flush 将config 的内容转储到磁盘。
func (c config) flush() {
	os.MkdirAll(filepath.Dir(c.path), 0755)

	out, _ := json.MarshalIndent(c, "", "  ")
	if err := ioutil.WriteFile(c.path, out, 0644); err != nil {
		log_wizard.Error("无法保存puppeth配置", "file", c.path,  err.Error())
	}
}

type wizard struct {
	network string // 要管理的网络名称
	conf    config // 以前运行的配置

	servers  map[string]*sshClient // SSH连接到要管理的服务器
	services map[string][]string   // 已知在服务器上运行的Aidoc服务

	in   *bufio.Reader // stdin 周围的包装器允许读取用户输入
	lock sync.Mutex    // 锁定以在并发服务发现期间保护配置
}

// read 从 stdin 读取一行，从空格中修剪。
func (w *wizard) read() string {
	fmt.Printf("> ")
	text, err := w.in.ReadString('\n')
	if err != nil {
		log_wizard.Crit("无法读取用户输入",  err.Error())
	}
	return strings.TrimSpace(text)
}

// readString 从 stdin 读取一行，从空格中修剪，强制执行非空。
func (w *wizard) readString() string {
	for {
		fmt.Printf("> ")
		text, err := w.in.ReadString('\n')
		if err != nil {
			log_wizard.Crit("无法读取用户输入",  err.Error())
		}
		if text = strings.TrimSpace(text); text != "" {
			return text
		}
	}
}

// readDefaultString 从 stdin 读取一行，从空格中修剪。
// 如果输入空行，则返回默认值。
func (w *wizard) readDefaultString(def string) string {
	fmt.Printf("> ")
	text, err := w.in.ReadString('\n')
	if err != nil {
		log_wizard.Crit("无法读取用户输入",   err.Error())
	}
	if text = strings.TrimSpace(text); text != "" {
		return text
	}
	return def
}

// readInt 从 stdin 读取一行，从空格中修剪，强制解析为整数。
func (w *wizard) readInt() int {
	for {
		fmt.Printf("> ")
		text, err := w.in.ReadString('\n')
		if err != nil {
			log_wizard.Crit("无法读取用户输入",   err.Error())
		}
		if text = strings.TrimSpace(text); text == "" {
			continue
		}
		val, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil {
			log_wizard.Error("输入无效，预期整数",   err.Error())
			continue
		}
		return val
	}
}

// readDefaultInt 从 stdin 中读取一行，从空格中修剪，强制解析为整数。 如果输入空行，则返回默认值。
func (w *wizard) readDefaultInt(def int) int {
	for {
		fmt.Printf("> ")
		text, err := w.in.ReadString('\n')
		if err != nil {
			log_wizard.Crit("无法读取用户输入",   err.Error())
		}
		if text = strings.TrimSpace(text); text == "" {
			return def
		}
		val, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil {
			log_wizard.Error("输入无效，预期整数",   err.Error())
			continue
		}
		return val
	}
}

// readDefaultBigInt 从 stdin 中读取一行，从空格中修剪，强制它解析成一个大整数。
// 如果输入空行，则返回默认值。
func (w *wizard) readDefaultBigInt(def *big.Int) *big.Int {
	for {
		fmt.Printf("> ")
		text, err := w.in.ReadString('\n')
		if err != nil {
			log_wizard.Crit("无法读取用户输入",   err.Error())
		}
		if text = strings.TrimSpace(text); text == "" {
			return def
		}
		val, ok := new(big.Int).SetString(text, 0)
		if !ok {
			log_wizard.Error("输入无效，预期大整数")
			continue
		}
		return val
	}
}

/*
// readFloat从stdin读取一行，从空格中修剪，强制解析为float。
func (w *wizard) readFloat() float64 {
	for {
		fmt.Printf("> ")
		text, err := w.in.ReadString('\n')
		if err != nil {
			log_wizard.Crit("无法读取用户输入",   err)
		}
		if text = strings.TrimSpace(text); text == "" {
			continue
		}
		val, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil {
			log_wizard.Error("输入无效，预期浮点数",   err)
			continue
		}
		return val
	}
}
*/

// readDefaultFloat从stdin中读取一行，从空格中修剪，强制它解析为float。
// 如果输入空行，则返回默认值。
func (w *wizard) readDefaultFloat(def float64) float64 {
	for {
		fmt.Printf("> ")
		text, err := w.in.ReadString('\n')
		if err != nil {
			log_wizard.Crit("无法读取用户输入",   err.Error())
		}
		if text = strings.TrimSpace(text); text == "" {
			return def
		}
		val, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil {
			log_wizard.Error("输入无效，预期浮点数",   err.Error())
			continue
		}
		return val
	}
}

// readPassword从stdin中读取一行，从尾随的新行中修剪它并返回它。
// 输入将不会回显。
func (w *wizard) readPassword() string {
	fmt.Printf("> ")
	text, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		log_wizard.Crit("无法读取密码",   err.Error())
	}
	i18.I18_print.Println()
	return string(text)
}

// readAddress从stdin中读取一行，从空格中修剪并将其转换为 AIDOC地址。
func (w *wizard) readAddress() *chain_common.Address {
	for {
		// 从用户那里读取地址
		fmt.Printf("> 0x")
		text, err := w.in.ReadString('\n')
		if err != nil {
			log_wizard.Crit("无法读取用户输入",   err.Error())
		}
		if text = strings.TrimSpace(text); text == "" {
			return nil
		}
		// 确保它看起来没问题并返回它
		if len(text) != 40 {
			log_wizard.Error("地址长度无效，请重试")
			continue
		}
		bigaddr, _ := new(big.Int).SetString(text, 16)
		address := chain_common.BigToAddress(bigaddr)
		return &address
	}
}

// readDefaultAddress 从 stdin 中读取一行，从空格中修剪并将其转换为 AIDOC地址。
// 如果输入空行，则返回默认值。
func (w *wizard) readDefaultAddress(def chain_common.Address) chain_common.Address {
	for {
		// 从用户那里读取地址
		fmt.Printf("> 0x")
		text, err := w.in.ReadString('\n')
		if err != nil {
			log_wizard.Crit("无法读取用户输入",   err.Error())
		}
		if text = strings.TrimSpace(text); text == "" {
			return def
		}
		// 确保它看起来没问题并返回它
		if len(text) != 40 {
			log_wizard.Error("地址长度无效，请重试")
			continue
		}
		bigaddr, _ := new(big.Int).SetString(text, 16)
		return chain_common.BigToAddress(bigaddr)
	}
}

// readJSON 读取原始 JSON 消息并返回它。
func (w *wizard) readJSON() string {
	var blob json.RawMessage

	for {
		fmt.Printf("> ")
		if err := json.NewDecoder(w.in).Decode(&blob); err != nil {
			log_wizard.Error("无效的JSON，请再试一次",   err)
			continue
		}
		return string(blob)
	}
}

// readIPAddress从stdin中读取一行，从空间中删除它，如果它可以转换为IP地址则返回它。
// 保持用户输入格式而不是返回Go net.IP 的原因
// 是为了匹配aidocstats使用的奇怪格式，它们以文本方式比较IP，而不是按值。
func (w *wizard) readIPAddress() string {
	for {
		// Read the IP address from the user
		fmt.Printf("> ")
		text, err := w.in.ReadString('\n')
		if err != nil {
			log_wizard.Crit("无法读取用户输入",   err)
		}
		if text = strings.TrimSpace(text); text == "" {
			return ""
		}
		// 确保它看起来没问题并返回它
		if ip := net.ParseIP(text); ip == nil {
			log_wizard.Error("无效的IP地址，请重试")
			continue
		}
		return text
	}
}

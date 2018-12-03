//  bootnode（启动节点）运行 AIDOC发现协议的引导节点。
package main

import (
	"crypto/ecdsa"
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/aidoc/go-aidoc/lib/crypto"
	"github.com/aidoc/go-aidoc/lib/logger"
	"github.com/aidoc/go-aidoc/service/p2p/discover"
	"github.com/aidoc/go-aidoc/service/p2p/nat"
	"github.com/aidoc/go-aidoc/service/p2p/netutil"
)

func main() {
	var (
		listenAddr  = flag.String("addr", ":30301", "监听地址")
		genKey      = flag.String("genkey", "", "生成节点密钥")
		writeAddr   = flag.Bool("writeaddress", false, "写出节点的pubkey哈希并退出")
		nodeKeyFile = flag.String("nodekey", "", "私钥文件名")
		nodeKeyHex  = flag.String("nodekeyhex", "", "私钥为十六进制（用于测试）")
		natdesc     = flag.String("nat", "none", "端口映射机制（any | none | upnp | pmp | extip：<IP>）")
		netrestrict = flag.String("netrestrict", "", "限制网络通信到给定的IP网络（CIDR掩码）")
		//verbosity   = flag.Int("verbosity", int(log.LvlInfo), "记录详细程度（0-9）")
		//vmodule     = flag.String("vmodule", "", " 记录详细程度模式 ")

		nodeKey *ecdsa.PrivateKey
		err     error
	)
	flag.Parse()

	natm, err := nat.Parse(*natdesc)
	if err != nil {
		logger.CritF("-nat: %v", err)
	}
	switch {
	case *genKey != "":
		nodeKey, err = crypto.GenerateKey()
		if err != nil {
			logger.CritF("无法生成密钥：%v", err)
		}
		if err = crypto.SaveECDSA(*genKey, nodeKey); err != nil {
			logger.CritF("%v", err)
		}
		return
	case *nodeKeyFile == "" && *nodeKeyHex == "":
		logger.CritF("使用 -nodekey 或 -nodekeyhex 指定私钥")
	case *nodeKeyFile != "" && *nodeKeyHex != "":
		logger.CritF("选项 -nodekey 和 -nodekeyhex 是互斥的")
	case *nodeKeyFile != "":
		if nodeKey, err = crypto.LoadECDSA(*nodeKeyFile); err != nil {
			logger.CritF("-nodekey: %v", err)
		}
	case *nodeKeyHex != "":
		if nodeKey, err = crypto.HexToECDSA(*nodeKeyHex); err != nil {
			logger.CritF("-nodekeyhex: %v", err)
		}
	}

	if *writeAddr {
		fmt.Printf("%v\n", discover.PubkeyID(&nodeKey.PublicKey))
		os.Exit(0)
	}

	var restrictList *netutil.Netlist
	if *netrestrict != "" {
		restrictList, err = netutil.ParseNetlist(*netrestrict)
		if err != nil {
			logger.CritF("-netrestrict: %v", err)
		}
	}

	addr, err := net.ResolveUDPAddr("udp", *listenAddr)
	if err != nil {
		logger.CritF("-ResolveUDPAddr: %v", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		logger.CritF("-ListenUDP: %v", err)
	}

	realaddr := conn.LocalAddr().(*net.UDPAddr)
	if natm != nil {
		if !realaddr.IP.IsLoopback() {
			go nat.Map(natm, nil, "udp", realaddr.Port, realaddr.Port, "Aidoc发现")
		}
		// TODO：随着时间的推移对外部IP变化做出反应。
		// TODO: react to external IP changes over time.
		if ext, err := natm.ExternalIP(); err == nil {
			realaddr = &net.UDPAddr{IP: ext, Port: realaddr.Port}
		}
	}

	cfg := discover.Config{
		PrivateKey:   nodeKey,
		AnnounceAddr: realaddr,
		NetRestrict:  restrictList,
	}
	if _, err := discover.ListenUDP(conn, cfg); err != nil {
		fmt.Printf("%v", err)
	}

	select {}
}

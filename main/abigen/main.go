package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/aidoc/go-aidoc/lib/compiler"
	"github.com/aidoc/go-aidoc/lib/logger"
	"github.com/aidoc/go-aidoc/service/accounts/abi/bind"
)

var (
	abiFlag = flag.String("abi", "", "Aidoc合约的路径ABI json要绑定， - 对于STDIN")
	binFlag = flag.String("bin", "", "Aidoc契约字节码的路径（生成部署方法）")
	typFlag = flag.String("type", "", "绑定的结构名称（默认=包名称）")

	solFlag  = flag.String("sol", "", "Aidoc契约的路径构建和绑定的Solidity源")
	solcFlag = flag.String("solc", "solc", "如果请求源构建，则使用Solidity编译器")
	excFlag  = flag.String("exc", "", "逗号分隔类型以从绑定中排除")

	pkgFlag  = flag.String("pkg", "", "包名称生成绑定到")
	outFlag  = flag.String("out", "", "生成的绑定的输出文件（默认= stdout）")
	langFlag = flag.String("lang", "go", "绑定的目标语言（go，java，objc）")
)

func main() {
	// 解析并确保指定所有需要的输入
	flag.Parse()

	if *abiFlag == "" && *solFlag == "" {
		logger.Info("没有合同ABI（--abi）或 Solidity source（--sol）指定 \n")
		os.Exit(-1)
	} else if (*abiFlag != "" || *binFlag != "" || *typFlag != "") && *solFlag != "" {
		logger.Info("合同ABI（--abi），字节码（--bin）和类型（ - type）标志与Solidity源（--sol）标志互斥 \n")
		os.Exit(-1)
	}
	if *pkgFlag == "" {
		logger.Info("没有指定目标包（--pkg）\n")
		os.Exit(-1)
	}
	var lang bind.Lang
	switch *langFlag {
	case "go":
		lang = bind.LangGo
	case "java":
		lang = bind.LangJava
	case "objc":
		lang = bind.LangObjC
	default:
		logger.InfoF("不支持的目标语言 \"%s\"（ - lang）\n", *langFlag)
		os.Exit(-1)
	}
	// 如果指定了整个可靠性代码，则基于此构建和绑定
	var (
		abis  []string
		bins  []string
		types []string
	)
	if *solFlag != "" || *abiFlag == "-" {
		// 生成要从绑定中排除的类型列表
		exclude := make(map[string]bool)
		for _, kind := range strings.Split(*excFlag, ",") {
			exclude[strings.ToLower(kind)] = true
		}

		var contracts map[string]*compiler.Contract
		var err error
		if *solFlag != "" {
			contracts, err = compiler.CompileSolidity(*solcFlag, *solFlag)
			if err != nil {
				logger.InfoF("无法建立Solidity合同：%v \n", err)
				os.Exit(-1)
			}
		} else {
			contracts, err = contractsFromStdin()
			if err != nil {
				logger.InfoF("无法从STDIN读取输入ABI：%v \n", err)
				os.Exit(-1)
			}
		}
		// 收集所有未排除的约束合同
		for name, contract := range contracts {
			if exclude[strings.ToLower(name)] {
				continue
			}
			abi, _ := json.Marshal(contract.Info.AbiDefinition) // 展平编译器解析
			abis = append(abis, string(abi))
			bins = append(bins, contract.Code)

			nameParts := strings.Split(name, ":")
			types = append(types, nameParts[len(nameParts)-1])
		}
	} else {
		// 否则从参数中加载ABI，可选字节码和类型名称
		abi, err := ioutil.ReadFile(*abiFlag)
		if err != nil {
			logger.InfoF("无法读取输入ABI：%v  \n", err)
			os.Exit(-1)
		}
		abis = append(abis, string(abi))

		bin := []byte{}
		if *binFlag != "" {
			if bin, err = ioutil.ReadFile(*binFlag); err != nil {
				logger.InfoF("无法读取输入字节码：%v \n", err)
				os.Exit(-1)
			}
		}
		bins = append(bins, string(bin))

		kind := *typFlag
		if kind == "" {
			kind = *pkgFlag
		}
		types = append(types, kind)
	}
	// 生成合同绑定
	code, err := bind.Bind(types, abis, bins, *pkgFlag, lang)
	if err != nil {
		logger.InfoF("无法生成ABI绑定：%v  \n", err)
		os.Exit(-1)
	}
	// 将其刷新到文件或显示在标准输出上
	if *outFlag == "" {
		fmt.Printf("%s\n", code)
		return
	}
	if err := ioutil.WriteFile(*outFlag, []byte(code), 0600); err != nil {
		logger.InfoF("无法编写ABI绑定：%v  \n", err)
		os.Exit(-1)
	}
}

func contractsFromStdin() (map[string]*compiler.Contract, error) {
	bytes, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return nil, err
	}

	return compiler.ParseCombinedJSON(bytes, "", "", "", "")
}

package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

var customGenesisTests = []struct {
	genesis string
	query   string
	result  string
}{
	// 简单的创世文件没有任何额外的
	{
		genesis: `{
			"alloc"      : {},
			"coinbase"   : "0x0000000000000000000000000000000000000000",
			"difficulty" : "0x20000",
			"extraData"  : "",
			"gasLimit"   : "0x2fefd8",
			"nonce"      : "0x0000000000000042",
			"mixhash"    : "0x0000000000000000000000000000000000000000000000000000000000000000",
			"parentHash" : "0x0000000000000000000000000000000000000000000000000000000000000000",
			"timestamp"  : "0x00"
		}`,
		query:  "aidoc.getBlock(0).nonce",
		result: "0x0000000000000042",
	},
	// 具有空链配置的Genesis文件（确保缺少字段工作）
	{
		genesis: `{
			"alloc"      : {},
			"coinbase"   : "0x0000000000000000000000000000000000000000",
			"difficulty" : "0x20000",
			"extraData"  : "",
			"gasLimit"   : "0x2fefd8",
			"nonce"      : "0x0000000000000042",
			"mixhash"    : "0x0000000000000000000000000000000000000000000000000000000000000000",
			"parentHash" : "0x0000000000000000000000000000000000000000000000000000000000000000",
			"timestamp"  : "0x00",
			"config"     : {}
		}`,
		query:  "aidoc.getBlock(0).nonce",
		result: "0x0000000000000042",
	},
	//  具有特定链配置的Genesis文件
	{
		genesis: `{
			"alloc"      : {},
			"coinbase"   : "0x0000000000000000000000000000000000000000",
			"difficulty" : "0x20000",
			"extraData"  : "",
			"gasLimit"   : "0x2fefd8",
			"nonce"      : "0x0000000000000042",
			"mixhash"    : "0x0000000000000000000000000000000000000000000000000000000000000000",
			"parentHash" : "0x0000000000000000000000000000000000000000000000000000000000000000",
			"timestamp"  : "0x00",
			"config"     : {
				"homesteadBlock" : 314,
				"daoForkBlock"   : 141,
				"daoForkSupport" : true
			},
		}`,
		query:  "aidoc.getBlock(0).nonce",
		result: "0x0000000000000042",
	},
}

// 使用自定义生成块和链定义初始化Gaidoc 的测试正常。
func TestCustomGenesis(t *testing.T) {
	for i, tt := range customGenesisTests {
		// 创建一个临时数据目录以供以后使用和检查
		datadir := tmpdir(t)
		defer os.RemoveAll(datadir)

		// 使用自定义生成块初始化数据目录
		json := filepath.Join(datadir, "genesis.json")
		if err := ioutil.WriteFile(json, []byte(tt.genesis), 0600); err != nil {
			t.Fatalf("测试  %d：写入genesis文件失败：%v", i, err)
		}
		runGaidoc(t, "--datadir", datadir, "init", json).WaitExit()

		// 查询自定义创世块
		gaidoc := runGaidoc(t,
			"--datadir", datadir, "--maxpeers", "0", "--port", "0",
			"--nodiscover", "--nat", "none", "--ipcdisable",
			"--exec", tt.query, "console")
		gaidoc.ExpectRegexp(tt.result)
		gaidoc.ExpectExit()
	}
}

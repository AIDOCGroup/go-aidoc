package configs

// MainnetBootnodes是主Aidoc网络上运行的P2P引导节点的enode URL。
// the main Aidoc network.
var MainnetBootnodes = []string{
	//Aidoc基金会Go Bootnodes
	// 线上引导节点
	//	"enode://5d4be0d872c2fd374f3fc0c409b66fa687dd1e37acf37535953cb8c76e49b1d5293a99fb36d4fb750659330909ae9b4c3a19c49e0a62ebd79ae4a595c06e8f42@119.3.28.32:30303",
	//	"enode://c1a5c828b5c1f9510b017fe818c0d686317e0a1df279357e039c017c646a5bf062ca2020693c087173da039338a7145533f9d8b3eefdda7f8f6bb08cb16203c5@119.3.28.32:30304",
	//"enode://aec875d2d3baa2a7fe5ee1814001f2bd0c7777be287ffd5f849c45cedf8b51d33f15079a4ae84ab4e70f7643894124e52b243313131ba48f73bb5c308ebdbaee@127.0.0.1:30000",
	// "enode://62b6a622c75985b5d6727ac01d467bbd327e2904df737cbea2a3026c2e8589cb814ce715f673626fadbed4c5540bad1f31b4587a2c4fffb2af6ce2c4ebe9c6be@127.0.0.1:40001", // liuxicai
	//"enode://b3b793bad372dd891ba8d4ef9e82454b03dd1ebfd99f57528dad8bac34289b047093c2533ee6b32b9ddb86693d00bd534231012f6530ab51bc8f630c71fb2058@127.0.0.1:30000", //houyi
	"enode://c60e84c2e5920819eb8bfb0bd4edf522e166edfa59e37730de4489d1f173a9698788b99e16f71f683912490cd80b71a4268faf1e401f6b11dfeae2da77a61ade@[127.0.0.1]:30603",
}

// TestnetBootnodes是在Ropsten测试网络上运行的P2P引导节点的enode URL。
var TestnetBootnodes = []string{}

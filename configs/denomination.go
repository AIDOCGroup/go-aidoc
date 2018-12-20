package configs

//这些是 Aidoc 面额的乘数。
//示例：要获取“LiShizhen”中金额的 Dose 值，请使用
//
// new（big.Int）.Mul（value，big.NewInt（params.LiShizhen））
const (
	Dose            = 1
	BianQue         = 1e3
	HuaTuo          = 1e6
	ZhangZhongjing  = 1e9
	HuangFumi       = 1e12
    Songci          = 1e15
	Aidoc           = 1e18
    SunSimiao       = 1e21
    LiShizhen       = 1e42
)

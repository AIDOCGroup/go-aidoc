package bloombits

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/aidoc/go-aidoc/lib/chain_core/types"
)

// 该成批开花比特测试正确从输入 Bloom 过滤器旋转。
func TestGenerator(t *testing.T) {
	// 生成输入和旋转输出
	var input, output [types.BloomBitLength][types.BloomByteLength]byte

	for i := 0; i < types.BloomBitLength; i++ {
		for j := 0; j < types.BloomBitLength; j++ {
			bit := byte(rand.Int() % 2)

			input[i][j/8] |= bit << byte(7-j%8)
			output[types.BloomBitLength-1-j][i/8] |= bit << byte(7-i%8)
		}
	}
	// 通过发生器压缩输入并验证结果
	gen, err := NewGenerator(types.BloomBitLength)
	if err != nil {
		t.Fatalf("无法创建bloombit生成器： %v", err)
	}
	for i, bloom := range input {
		if err := gen.AddBloom(uint(i), bloom); err != nil {
			t.Fatalf("bloom %d：无法添加：%v", i, err)
		}
	}
	for i, want := range output {
		have, err := gen.Bitset(uint(i))
		if err != nil {
			t.Fatalf("输出%d：无法检索位：%v", i, err)
		}
		if !bytes.Equal(have, want[:]) {
			t.Errorf("输出 %d：位向量不匹配有 %x，想要 %x", i, have, want)
		}
	}
}

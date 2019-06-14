// 版权所有2012 The Go Authors。 版权所有。
// 此源代码的使用受BSD样式许可证的约束，
// 该许可证可在LICENSE文件中找到。

package bn256

import (
	"crypto/rand"
)

func ExamplePair() {
	// 这实现了“A One”中的三方Diffie-Hellman算法
	// 三方Diffie-Hellman的圆形协议“，A。Joux。
	// http://www.springerlink.com/content/cddc57yyva0hburb/fulltext.pdf

	// 三方a，b和c中的每一方都生成私有值。
	a, _ := rand.Int(rand.Reader, Order)
	b, _ := rand.Int(rand.Reader, Order)
	c, _ := rand.Int(rand.Reader, Order)

	// 然后每一方计算其私有值的g₁ 和g₂ 倍。
	pa := new(G1).ScalarBaseMult(a)
	qa := new(G2).ScalarBaseMult(a)

	pb := new(G1).ScalarBaseMult(b)
	qb := new(G2).ScalarBaseMult(b)

	pc := new(G1).ScalarBaseMult(c)
	qc := new(G2).ScalarBaseMult(c)

	// 现在每一方都与其他两方交换其公共价值，所有各方都可以计算共享密钥。
	k1 := Pair(pb, qc)
	k1.ScalarMult(k1, a)

	k2 := Pair(pc, qa)
	k2.ScalarMult(k2, b)

	k3 := Pair(pa, qb)
	k3.ScalarMult(k3, c)

	// k1，k2和 k3 都是相等的。
}

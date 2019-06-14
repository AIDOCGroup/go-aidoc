// 版权所有2012 The Go Authors。 版权所有。
// 此源代码的使用受BSD样式许可证的约束，
// 该许可证可在LICENSE文件中找到。

package bn256

func lineFunctionAdd(r, p *twistPoint, q *curvePoint, r2 *gfP2, pool *bnPool) (a, b, c *gfP2, rOut *twistPoint) {
	// 参见“更快的计算”中的混合加法算法
	// Tate Pairing“，http：//arxiv.org/pdf/0904.0854v3.pdf

	B := newGFp2(pool).Mul(p.x, r.t, pool)

	D := newGFp2(pool).Add(p.y, r.z)
	D.Square(D, pool)
	D.Sub(D, r2)
	D.Sub(D, r.t)
	D.Mul(D, r.t, pool)

	H := newGFp2(pool).Sub(B, r.x)
	I := newGFp2(pool).Square(H, pool)

	E := newGFp2(pool).Add(I, I)
	E.Add(E, E)

	J := newGFp2(pool).Mul(H, E, pool)

	L1 := newGFp2(pool).Sub(D, r.y)
	L1.Sub(L1, r.y)

	V := newGFp2(pool).Mul(r.x, E, pool)

	rOut = newTwistPoint(pool)
	rOut.x.Square(L1, pool)
	rOut.x.Sub(rOut.x, J)
	rOut.x.Sub(rOut.x, V)
	rOut.x.Sub(rOut.x, V)

	rOut.z.Add(r.z, H)
	rOut.z.Square(rOut.z, pool)
	rOut.z.Sub(rOut.z, r.t)
	rOut.z.Sub(rOut.z, I)

	t := newGFp2(pool).Sub(V, rOut.x)
	t.Mul(t, L1, pool)
	t2 := newGFp2(pool).Mul(r.y, J, pool)
	t2.Add(t2, t2)
	rOut.y.Sub(t, t2)

	rOut.t.Square(rOut.z, pool)

	t.Add(p.y, rOut.z)
	t.Square(t, pool)
	t.Sub(t, r2)
	t.Sub(t, rOut.t)

	t2.Mul(L1, p.x, pool)
	t2.Add(t2, t2)
	a = newGFp2(pool)
	a.Sub(t2, t)

	c = newGFp2(pool)
	c.MulScalar(rOut.z, q.y)
	c.Add(c, c)

	b = newGFp2(pool)
	b.SetZero()
	b.Sub(b, L1)
	b.MulScalar(b, q.x)
	b.Add(b, b)

	B.Put(pool)
	D.Put(pool)
	H.Put(pool)
	I.Put(pool)
	E.Put(pool)
	J.Put(pool)
	L1.Put(pool)
	V.Put(pool)
	t.Put(pool)
	t2.Put(pool)

	return
}

func lineFunctionDouble(r *twistPoint, q *curvePoint, pool *bnPool) (a, b, c *gfP2, rOut *twistPoint) {
	// 从“更快的计算”中看出a = 0的加倍算法
	// Tate Pairing“，http：//arxiv.org/pdf/0904.0854v3.pdf

	A := newGFp2(pool).Square(r.x, pool)
	B := newGFp2(pool).Square(r.y, pool)
	C_ := newGFp2(pool).Square(B, pool)

	D := newGFp2(pool).Add(r.x, B)
	D.Square(D, pool)
	D.Sub(D, A)
	D.Sub(D, C_)
	D.Add(D, D)

	E := newGFp2(pool).Add(A, A)
	E.Add(E, A)

	G := newGFp2(pool).Square(E, pool)

	rOut = newTwistPoint(pool)
	rOut.x.Sub(G, D)
	rOut.x.Sub(rOut.x, D)

	rOut.z.Add(r.y, r.z)
	rOut.z.Square(rOut.z, pool)
	rOut.z.Sub(rOut.z, B)
	rOut.z.Sub(rOut.z, r.t)

	rOut.y.Sub(D, rOut.x)
	rOut.y.Mul(rOut.y, E, pool)
	t := newGFp2(pool).Add(C_, C_)
	t.Add(t, t)
	t.Add(t, t)
	rOut.y.Sub(rOut.y, t)

	rOut.t.Square(rOut.z, pool)

	t.Mul(E, r.t, pool)
	t.Add(t, t)
	b = newGFp2(pool)
	b.SetZero()
	b.Sub(b, t)
	b.MulScalar(b, q.x)

	a = newGFp2(pool)
	a.Add(r.x, E)
	a.Square(a, pool)
	a.Sub(a, A)
	a.Sub(a, G)
	t.Add(B, B)
	t.Add(t, t)
	a.Sub(a, t)

	c = newGFp2(pool)
	c.Mul(rOut.z, r.t, pool)
	c.Add(c, c)
	c.MulScalar(c, q.y)

	A.Put(pool)
	B.Put(pool)
	C_.Put(pool)
	D.Put(pool)
	E.Put(pool)
	G.Put(pool)
	t.Put(pool)

	return
}

func mulLine(ret *gfP12, a, b, c *gfP2, pool *bnPool) {
	a2 := newGFp6(pool)
	a2.x.SetZero()
	a2.y.Set(a)
	a2.z.Set(b)
	a2.Mul(a2, ret.x, pool)
	t3 := newGFp6(pool).MulScalar(ret.y, c, pool)

	t := newGFp2(pool)
	t.Add(b, c)
	t2 := newGFp6(pool)
	t2.x.SetZero()
	t2.y.Set(a)
	t2.z.Set(t)
	ret.x.Add(ret.x, ret.y)

	ret.y.Set(t3)

	ret.x.Mul(ret.x, t2, pool)
	ret.x.Sub(ret.x, a2)
	ret.x.Sub(ret.x, ret.y)
	a2.MulTau(a2, pool)
	ret.y.Add(ret.y, a2)

	a2.Put(pool)
	t3.Put(pool)
	t2.Put(pool)
	t.Put(pool)
}

// sixuPlus2NAF 是非相邻形式的 6u + 2。
var sixuPlus2NAF = []int8{0, 0, 0, 1, 0, 1, 0, -1, 0, 0, 1, -1, 0, 0, 1, 0,
	0, 1, 1, 0, -1, 0, 0, 1, 0, -1, 0, 0, 0, 0, 1, 1,
	1, 0, 0, -1, 0, 0, 1, 0, 0, 0, 0, 0, -1, 0, 0, 1,
	1, 0, 0, -1, 0, 0, 0, 1, 1, 0, -1, 0, 0, 1, 0, 1, 1}

// miller 实现了 Miller 循环来计算 Optimal Ate 配对。
// 参见http://cryptojedi.org/papers/dclxvi-20100714.pdf 中的算法1
func miller(q *twistPoint, p *curvePoint, pool *bnPool) *gfP12 {
	ret := newGFp12(pool)
	ret.SetOne()

	aAffine := newTwistPoint(pool)
	aAffine.Set(q)
	aAffine.MakeAffine(pool)

	bAffine := newCurvePoint(pool)
	bAffine.Set(p)
	bAffine.MakeAffine(pool)

	minusA := newTwistPoint(pool)
	minusA.Negative(aAffine, pool)

	r := newTwistPoint(pool)
	r.Set(aAffine)

	r2 := newGFp2(pool)
	r2.Square(aAffine.y, pool)

	for i := len(sixuPlus2NAF) - 1; i > 0; i-- {
		a, b, c, newR := lineFunctionDouble(r, bAffine, pool)
		if i != len(sixuPlus2NAF)-1 {
			ret.Square(ret, pool)
		}

		mulLine(ret, a, b, c, pool)
		a.Put(pool)
		b.Put(pool)
		c.Put(pool)
		r.Put(pool)
		r = newR

		switch sixuPlus2NAF[i-1] {
		case 1:
			a, b, c, newR = lineFunctionAdd(r, aAffine, bAffine, r2, pool)
		case -1:
			a, b, c, newR = lineFunctionAdd(r, minusA, bAffine, r2, pool)
		default:
			continue
		}

		mulLine(ret, a, b, c, pool)
		a.Put(pool)
		b.Put(pool)
		c.Put(pool)
		r.Put(pool)
		r = newR
	}

	// 为了计算Q1，我们必须将q从 sextic twist 转换为完整的GF（p^12）组，
	// 在那里应用Frobenius，然后转换回来。
	//
	// 扭曲同构是 (x', y') -> (xω², yω³)。 如果我们考虑的话
	//  x片刻，然后应用Frobenius后，我们有 x̄ω^(2p)
	// 其中 x ̄是 x 的共轭。 如果我们要应用逆
	// 同构我们需要一个具有单个系数 ω2 的值，所以我们
	// 将其重写为 x̄ω^(2p-2)ω². ξ⁶ = ω 和，由于构造
	//  p，2p-2是六的倍数。 因此我们可以重写为
	// x̄ξ^((p-1)/3)ω²  并应用逆同构消除了 ω²
	//
	//可以为y值创建类似的参数。

	q1 := newTwistPoint(pool)
	q1.x.Conjugate(aAffine.x)
	q1.x.Mul(q1.x, xiToPMinus1Over3, pool)
	q1.y.Conjugate(aAffine.y)
	q1.y.Mul(q1.y, xiToPMinus1Over2, pool)
	q1.z.SetOne()
	q1.t.SetOne()

	// 对于Q2，我们正在使用 p² Frobenius。 这两个结合被抵消了，我们只留下来自同构的因子。
	// 在x的情况下，我们最终得到一个纯数，这就是为什么 xiToPSquaredMinus1Over3 是 ∈ GF(p)。
	// 使用y，我们得到-1的因子。 我们忽略了这一点，最终得到了-Q2。

	minusQ2 := newTwistPoint(pool)
	minusQ2.x.MulScalar(aAffine.x, xiToPSquaredMinus1Over3)
	minusQ2.y.Set(aAffine.y)
	minusQ2.z.SetOne()
	minusQ2.t.SetOne()

	r2.Square(q1.y, pool)
	a, b, c, newR := lineFunctionAdd(r, q1, bAffine, r2, pool)
	mulLine(ret, a, b, c, pool)
	a.Put(pool)
	b.Put(pool)
	c.Put(pool)
	r.Put(pool)
	r = newR

	r2.Square(minusQ2.y, pool)
	a, b, c, newR = lineFunctionAdd(r, minusQ2, bAffine, r2, pool)
	mulLine(ret, a, b, c, pool)
	a.Put(pool)
	b.Put(pool)
	c.Put(pool)
	r.Put(pool)
	r = newR

	aAffine.Put(pool)
	bAffine.Put(pool)
	minusA.Put(pool)
	r.Put(pool)
	r2.Put(pool)

	return ret
}

// finalExponentiation计算元素的（p¹-1）/阶次幂
// GF（p¹²），以获得GT的要素（步骤从算法1的13-15
// http://cryptojedi.org/papers/dclxvi-20100714.pdf）
func finalExponentiation(in *gfP12, pool *bnPool) *gfP12 {
	t1 := newGFp12(pool)

	// 这是p^6-Frobenius
	t1.x.Negative(in.x)
	t1.y.Set(in.y)

	inv := newGFp12(pool)
	inv.Invert(in, pool)
	t1.Mul(t1, inv, pool)

	t2 := newGFp12(pool).FrobeniusP2(t1, pool)
	t1.Mul(t1, t2, pool)

	fp := newGFp12(pool).Frobenius(t1, pool)
	fp2 := newGFp12(pool).FrobeniusP2(t1, pool)
	fp3 := newGFp12(pool).Frobenius(fp2, pool)

	fu, fu2, fu3 := newGFp12(pool), newGFp12(pool), newGFp12(pool)
	fu.Exp(t1, u, pool)
	fu2.Exp(fu, u, pool)
	fu3.Exp(fu2, u, pool)

	y3 := newGFp12(pool).Frobenius(fu, pool)
	fu2p := newGFp12(pool).Frobenius(fu2, pool)
	fu3p := newGFp12(pool).Frobenius(fu3, pool)
	y2 := newGFp12(pool).FrobeniusP2(fu2, pool)

	y0 := newGFp12(pool)
	y0.Mul(fp, fp2, pool)
	y0.Mul(y0, fp3, pool)

	y1, y4, y5 := newGFp12(pool), newGFp12(pool), newGFp12(pool)
	y1.Conjugate(t1)
	y5.Conjugate(fu2)
	y3.Conjugate(y3)
	y4.Mul(fu, fu2p, pool)
	y4.Conjugate(y4)

	y6 := newGFp12(pool)
	y6.Mul(fu3, fu3p, pool)
	y6.Conjugate(y6)

	t0 := newGFp12(pool)
	t0.Square(y6, pool)
	t0.Mul(t0, y4, pool)
	t0.Mul(t0, y5, pool)
	t1.Mul(y3, y5, pool)
	t1.Mul(t1, t0, pool)
	t0.Mul(t0, y2, pool)
	t1.Square(t1, pool)
	t1.Mul(t1, t0, pool)
	t1.Square(t1, pool)
	t0.Mul(t1, y1, pool)
	t1.Mul(t1, y0, pool)
	t0.Square(t0, pool)
	t0.Mul(t0, t1, pool)

	inv.Put(pool)
	t1.Put(pool)
	t2.Put(pool)
	fp.Put(pool)
	fp2.Put(pool)
	fp3.Put(pool)
	fu.Put(pool)
	fu2.Put(pool)
	fu3.Put(pool)
	fu2p.Put(pool)
	fu3p.Put(pool)
	y0.Put(pool)
	y1.Put(pool)
	y2.Put(pool)
	y3.Put(pool)
	y4.Put(pool)
	y5.Put(pool)
	y6.Put(pool)

	return t0
}

func optimalAte(a *twistPoint, b *curvePoint, pool *bnPool) *gfP12 {
	e := miller(a, b, pool)
	ret := finalExponentiation(e, pool)
	e.Put(pool)

	if a.IsInfinity() || b.IsInfinity() {
		ret.SetOne()
	}
	return ret
}

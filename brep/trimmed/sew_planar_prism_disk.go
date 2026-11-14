package trimmed

import (
	"math/big"

	"plainsolid/geom"
)

type prismDisk struct {
	center, axis geom.Vec3
	radius       float64
}

type prismCircle struct {
	center geom.Vec2
	radius float64
}

func (d *prismDisk) inFrame(f geom.Frame) *prismCircle {
	if d == nil || !prismCardinal(f) || f.ZAxis() != d.axis && f.ZAxis() != d.axis.Scale(-1) {
		return nil
	}
	p, err := prismExactLocal(f, d.center)
	if err != nil {
		return nil
	}
	return &prismCircle{center: geom.Vec2{X: p.X, Y: p.Y}, radius: d.radius}
}

func prismDistanceSquared(a, b geom.Vec2) *big.Rat {
	x := new(big.Rat).Sub(ratInset(a.X), ratInset(b.X))
	y := new(big.Rat).Sub(ratInset(a.Y), ratInset(b.Y))
	return new(big.Rat).Add(x.Mul(x, x), y.Mul(y, y))
}

func (p prismCircle) inside(center geom.Vec2, radius *big.Rat, pad float64) bool {
	gap := new(big.Rat).Sub(radius, ratInset(p.radius))
	gap.Sub(gap, ratInset(pad))
	return gap.Sign() > 0 && prismDistanceSquared(p.center, center).Cmp(gap.Mul(gap, gap)) < 0
}

func (p prismCircle) outside(center geom.Vec2, radius *big.Rat, pad float64) bool {
	reach := new(big.Rat).Add(radius, ratInset(p.radius))
	reach.Add(reach, ratInset(pad))
	return prismDistanceSquared(p.center, center).Cmp(reach.Mul(reach, reach)) > 0
}

func (p prismCircle) leftOf(a, b geom.Vec2, pad float64) bool {
	x := new(big.Rat).Sub(ratInset(b.X), ratInset(a.X))
	y := new(big.Rat).Sub(ratInset(b.Y), ratInset(a.Y))
	u := new(big.Rat).Sub(ratInset(p.center.X), ratInset(a.X))
	v := new(big.Rat).Sub(ratInset(p.center.Y), ratInset(a.Y))
	cross := new(big.Rat).Sub(new(big.Rat).Mul(x, v), new(big.Rat).Mul(y, u))
	if cross.Sign() <= 0 {
		return false
	}
	reach := new(big.Rat).Add(ratInset(p.radius), ratInset(pad))
	reach.Mul(reach, reach)
	reach.Mul(reach, prismDistanceSquared(a, b))
	return cross.Mul(cross, cross).Cmp(reach) > 0
}

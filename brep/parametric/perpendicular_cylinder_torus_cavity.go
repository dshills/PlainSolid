package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// PerpendicularCylinderTorusCavitySet retains the original +Z cylinder and
// inward +X torus support, rather than a faceted/rounded orientation surrogate.
type PerpendicularCylinderTorusCavitySet struct {
	solid Solid
	query geom.PerpendicularCylinderTorusCavityMaterial
	valid bool
}

func (s PerpendicularCylinderTorusCavitySet) Solid() (Solid, error) {
	if !s.valid {
		return Solid{}, geom.ErrInvalid
	}
	return s.solid, nil
}
func (s PerpendicularCylinderTorusCavitySet) Query() geom.PerpendicularCylinderTorusCavityMaterial {
	return s.query
}
func (s PerpendicularCylinderTorusCavitySet) SourceFaces() ([4][]TorusCylinderNativeOwner, error) {
	if !s.valid {
		return [4][]TorusCylinderNativeOwner{}, geom.ErrInvalid
	}
	return [4][]TorusCylinderNativeOwner{{{0, 3, 3}}, {{0, 0, 0}}, {{0, 1, 1}}, {{0, 2, 2}}}, nil
}
func NewPerpendicularCylinderTorusCavitySet(ctx context.Context, R, r, c, lower, upper float64, center geom.Vec3, tolerance float64) (PerpendicularCylinderTorusCavitySet, error) {
	var zero PerpendicularCylinderTorusCavitySet
	if ctx == nil || !finite(tolerance) || tolerance <= 0 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	query, e := geom.NewPerpendicularCylinderTorusCavityMaterial(R, r, c, lower, upper, center)
	if e != nil {
		return zero, e
	}
	q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	pad := new(big.Rat).Mul(q(tolerance), big.NewRat(8, 1))
	extent := new(big.Rat).Add(q(R), q(r))
	gap := new(big.Rat).Sub(new(big.Rat).Sub(q(c), extent), pad)
	d2 := new(big.Rat).Add(new(big.Rat).Mul(q(center.X), q(center.X)), new(big.Rat).Mul(q(center.Y), q(center.Y)))
	if gap.Sign() <= 0 || new(big.Rat).Mul(gap, gap).Cmp(d2) <= 0 || new(big.Rat).Sub(new(big.Rat).Sub(q(center.Z), extent), q(lower)).Cmp(pad) <= 0 || new(big.Rat).Sub(q(upper), new(big.Rat).Add(q(center.Z), extent)).Cmp(pad) <= 0 {
		return zero, geom.ErrConvergence
	}
	for _, v := range []*big.Rat{extent, new(big.Rat).Add(q(center.Y), q(R)), new(big.Rat).Add(q(center.Y), extent)} {
		if _, e = geom.RepresentedExactScalar(v); e != nil {
			return zero, e
		}
	}
	h, e := geom.RepresentedExactScalar(new(big.Rat).Sub(q(upper), q(lower)))
	if e != nil {
		return zero, e
	}
	f, _ := geom.NewFrame(geom.Vec3{Z: lower}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	outer, e := Cylinder(f, c, h)
	if e != nil {
		return zero, e
	}
	tf, e := geom.NewFrame(center, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
	if e != nil {
		return zero, e
	}
	inner, e := Torus(tf, R, r)
	if e != nil {
		return zero, e
	}
	for i := range inner.faces {
		inner.faces[i].Reversed = !inner.faces[i].Reversed
	}
	solid, e := joinIndependent(outer, inner)
	if e != nil {
		return zero, e
	}
	solid, e = NewSolid(solid.Vertices(), solid.Edges(), solid.Faces(), tolerance)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return PerpendicularCylinderTorusCavitySet{solid, query, true}, nil
}

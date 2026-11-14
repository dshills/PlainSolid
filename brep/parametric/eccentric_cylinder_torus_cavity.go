package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// EccentricCylinderTorusCavitySet has an outward original cylinder and an
// inward translated original torus. Source face IDs remain wall0,caps1/2,torus3.
type EccentricCylinderTorusCavitySet struct {
	solid Solid
	query geom.EccentricCylinderTorusCavityMaterial
	valid bool
}

func (s EccentricCylinderTorusCavitySet) Solid() (Solid, error) {
	if !s.valid {
		return Solid{}, geom.ErrInvalid
	}
	return s.solid, nil
}
func (s EccentricCylinderTorusCavitySet) Query() geom.EccentricCylinderTorusCavityMaterial {
	return s.query
}
func (s EccentricCylinderTorusCavitySet) SourceFaces() ([4][]TorusCylinderNativeOwner, error) {
	if !s.valid {
		return [4][]TorusCylinderNativeOwner{}, geom.ErrInvalid
	}
	return [4][]TorusCylinderNativeOwner{{{0, 3, 3}}, {{0, 0, 0}}, {{0, 1, 1}}, {{0, 2, 2}}}, nil
}
func NewEccentricCylinderTorusCavitySet(ctx context.Context, R, r, c, lower, upper, dx, dy, tolerance float64) (EccentricCylinderTorusCavitySet, error) {
	var zero EccentricCylinderTorusCavitySet
	q, e := geom.NewEccentricCylinderTorusCavityMaterial(R, r, c, lower, upper, dx, dy)
	if e != nil {
		return zero, e
	}
	outer, e := NewEncompassingTorusCylinderSet(ctx, R, r, c, lower, upper, geom.TorusCylinderUnion, tolerance)
	if e != nil {
		return zero, e
	}
	rat := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	gap := new(big.Rat).Sub(rat(c), new(big.Rat).Add(rat(R), rat(r)))
	gap.Sub(gap, new(big.Rat).Mul(big.NewRat(8, 1), rat(tolerance)))
	d2 := new(big.Rat).Add(new(big.Rat).Mul(rat(dx), rat(dx)), new(big.Rat).Mul(rat(dy), rat(dy)))
	if gap.Sign() <= 0 || new(big.Rat).Mul(gap, gap).Cmp(d2) <= 0 {
		return zero, geom.ErrConvergence
	}
	// All authored axis-aligned seam coordinate sums must remain represented.
	for _, v := range []*big.Rat{new(big.Rat).Add(rat(R), rat(r)), new(big.Rat).Add(rat(dx), rat(R)), new(big.Rat).Add(rat(dx), new(big.Rat).Add(rat(R), rat(r)))} {
		if _, e = geom.RepresentedExactScalar(v); e != nil {
			return zero, e
		}
	}
	frame, e := geom.NewFrame(geom.Vec3{X: dx, Y: dy}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return zero, e
	}
	inner, e := Torus(frame, R, r)
	if e != nil {
		return zero, e
	}
	for i := range inner.faces {
		inner.faces[i].Reversed = !inner.faces[i].Reversed
	}
	a, _ := outer.Solid()
	body, e := joinIndependent(a, inner)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return EccentricCylinderTorusCavitySet{body, q, true}, nil
}

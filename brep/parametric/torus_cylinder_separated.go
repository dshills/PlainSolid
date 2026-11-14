package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// TorusCylinderNativeOwner identifies an original support in both its retained
// component and the optional combined native solid. IDs never imply welding.
type TorusCylinderNativeOwner struct{ Component, Face, CompoundFace int }

// SeparatedTorusCylinderMeasures represents exact ideal measures as
// VolumePi2*pi²+VolumePi*pi, AreaPi2*pi²+AreaPi*pi, and FirstZPi*pi.
type SeparatedTorusCylinderMeasures struct{ VolumePi2, VolumePi, AreaPi2, AreaPi, FirstZPi *big.Rat }

// SeparatedTorusCylinderSet preserves each independent closed native component.
// Common is expressly empty and has no valid native Solid boundary.
type SeparatedTorusCylinderSet struct {
	query      geom.SeparatedTorusCylinderMaterial
	components []Solid
	compound   Solid
	measures   SeparatedTorusCylinderMeasures
	owners     [4][]TorusCylinderNativeOwner
	valid      bool
}

func (s SeparatedTorusCylinderSet) Empty() (bool, error) {
	if !s.valid {
		return false, geom.ErrInvalid
	}
	return len(s.components) == 0, nil
}
func (s SeparatedTorusCylinderSet) Solids() []Solid { return append([]Solid(nil), s.components...) }
func (s SeparatedTorusCylinderSet) Compound() (Solid, error) {
	if !s.valid {
		return Solid{}, geom.ErrInvalid
	}
	if len(s.components) == 0 {
		return Solid{}, geom.ErrDomain
	}
	return s.compound, nil
}
func (s SeparatedTorusCylinderSet) Query() geom.SeparatedTorusCylinderMaterial { return s.query }
func (s SeparatedTorusCylinderSet) SourceFaces() ([4][]TorusCylinderNativeOwner, error) {
	if !s.valid {
		return [4][]TorusCylinderNativeOwner{}, geom.ErrInvalid
	}
	var out [4][]TorusCylinderNativeOwner
	for i := range out {
		out[i] = append([]TorusCylinderNativeOwner(nil), s.owners[i]...)
	}
	return out, nil
}
func (s SeparatedTorusCylinderSet) Measures() (SeparatedTorusCylinderMeasures, error) {
	if !s.valid {
		return SeparatedTorusCylinderMeasures{}, geom.ErrInvalid
	}
	m := s.measures
	return SeparatedTorusCylinderMeasures{new(big.Rat).Set(m.VolumePi2), new(big.Rat).Set(m.VolumePi), new(big.Rat).Set(m.AreaPi2), new(big.Rat).Set(m.AreaPi), new(big.Rat).Set(m.FirstZPi)}, nil
}

// NewSeparatedTorusCylinderSet proves exact positive radial clearance greater
// than eight native sewing tolerances. The tool spans beyond the full torus.
// Only original complete analytic supports are used; no seams are synthesized.
// Fuse retains two independently closed shells rather than welding them.
func NewSeparatedTorusCylinderSet(ctx context.Context, R, r, c, lower, upper float64, op geom.TorusCylinderOperation, tolerance float64) (SeparatedTorusCylinderSet, error) {
	var zero SeparatedTorusCylinderSet
	if ctx == nil || !finite(tolerance) || tolerance <= 0 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	query, e := geom.NewSeparatedTorusCylinderMaterial(R, r, c, lower, upper, op)
	if e != nil {
		return zero, e
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	gap, _ := query.RadialClearance()
	if gap.Cmp(new(big.Rat).Mul(q(tolerance), big.NewRat(8, 1))) <= 0 {
		return zero, geom.ErrConvergence
	}
	out := SeparatedTorusCylinderSet{query: query, valid: true, measures: SeparatedTorusCylinderMeasures{new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)}}
	if op == geom.TorusCylinderCommon {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		return out, nil
	}
	frame, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return zero, e
	}
	torus, e := Torus(frame, R, r)
	if e != nil {
		return zero, e
	}
	torus, e = NewSolid(torus.Vertices(), torus.Edges(), torus.Faces(), tolerance)
	if e != nil {
		return zero, e
	}
	out.components = []Solid{torus}
	out.compound = torus
	out.owners[0] = []TorusCylinderNativeOwner{{0, 0, 0}}
	out.measures.VolumePi2.Mul(big.NewRat(2, 1), new(big.Rat).Mul(q(R), new(big.Rat).Mul(q(r), q(r))))
	out.measures.AreaPi2.Mul(big.NewRat(4, 1), new(big.Rat).Mul(q(R), q(r)))
	if op == geom.TorusCylinderUnion {
		heightRat := new(big.Rat).Sub(q(upper), q(lower))
		height, e := geom.RepresentedExactScalar(heightRat)
		if e != nil {
			return zero, e
		}
		frame, e := geom.NewFrame(geom.Vec3{Z: lower}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if e != nil {
			return zero, e
		}
		cylinder, e := Cylinder(frame, c, height)
		if e != nil {
			return zero, e
		}
		cylinder, e = NewSolid(cylinder.Vertices(), cylinder.Edges(), cylinder.Faces(), tolerance)
		if e != nil {
			return zero, e
		}
		out.components = append(out.components, cylinder)
		out.compound, e = joinIndependent(torus, cylinder)
		if e != nil {
			return zero, e
		}
		for i := 1; i < 4; i++ {
			out.owners[i] = []TorusCylinderNativeOwner{{1, i - 1, i}}
		}
		c2 := new(big.Rat).Mul(q(c), q(c))
		out.measures.VolumePi.Mul(c2, heightRat)
		out.measures.AreaPi.Mul(big.NewRat(2, 1), new(big.Rat).Add(new(big.Rat).Mul(q(c), heightRat), c2))
		out.measures.FirstZPi.Mul(out.measures.VolumePi, new(big.Rat).Quo(new(big.Rat).Add(q(lower), q(upper)), big.NewRat(2, 1)))
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}

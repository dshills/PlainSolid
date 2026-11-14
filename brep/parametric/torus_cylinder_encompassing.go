package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// TorusCylinderExactMeasures is the common rational pi²/pi coefficient report.
type TorusCylinderExactMeasures = SeparatedTorusCylinderMeasures

// EncompassingTorusCylinderSet retains the complete original boundary that
// survives strict finite containment. Empty Difference has no native boundary.
type EncompassingTorusCylinderSet struct {
	query    geom.EncompassingTorusCylinderMaterial
	bodies   []Solid
	measures TorusCylinderExactMeasures
	owners   [4][]TorusCylinderNativeOwner
	valid    bool
}

func (s EncompassingTorusCylinderSet) Empty() (bool, error) {
	if !s.valid {
		return false, geom.ErrInvalid
	}
	return len(s.bodies) == 0, nil
}
func (s EncompassingTorusCylinderSet) Solids() []Solid { return append([]Solid(nil), s.bodies...) }
func (s EncompassingTorusCylinderSet) Solid() (Solid, error) {
	if !s.valid {
		return Solid{}, geom.ErrInvalid
	}
	if len(s.bodies) == 0 {
		return Solid{}, geom.ErrDomain
	}
	return s.bodies[0], nil
}
func (s EncompassingTorusCylinderSet) Query() geom.EncompassingTorusCylinderMaterial { return s.query }
func (s EncompassingTorusCylinderSet) Measures() (TorusCylinderExactMeasures, error) {
	if !s.valid {
		return TorusCylinderExactMeasures{}, geom.ErrInvalid
	}
	m := s.measures
	return TorusCylinderExactMeasures{new(big.Rat).Set(m.VolumePi2), new(big.Rat).Set(m.VolumePi), new(big.Rat).Set(m.AreaPi2), new(big.Rat).Set(m.AreaPi), new(big.Rat).Set(m.FirstZPi)}, nil
}
func (s EncompassingTorusCylinderSet) SourceFaces() ([4][]TorusCylinderNativeOwner, error) {
	if !s.valid {
		return [4][]TorusCylinderNativeOwner{}, geom.ErrInvalid
	}
	var out [4][]TorusCylinderNativeOwner
	for i := range out {
		out[i] = append([]TorusCylinderNativeOwner(nil), s.owners[i]...)
	}
	return out, nil
}
func NewEncompassingTorusCylinderSet(ctx context.Context, R, r, c, lower, upper float64, op geom.TorusCylinderOperation, tolerance float64) (EncompassingTorusCylinderSet, error) {
	var zero EncompassingTorusCylinderSet
	if ctx == nil || !finite(tolerance) || tolerance <= 0 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	query, e := geom.NewEncompassingTorusCylinderMaterial(R, r, c, lower, upper, op)
	if e != nil {
		return zero, e
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	pad := new(big.Rat).Mul(q(tolerance), big.NewRat(8, 1))
	gaps, _ := query.Clearances()
	for _, gap := range gaps {
		if gap.Cmp(pad) <= 0 {
			return zero, geom.ErrConvergence
		}
	}
	out := EncompassingTorusCylinderSet{query: query, valid: true, measures: TorusCylinderExactMeasures{new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)}}
	if op == geom.TorusCylinderDifference {
		if e = ctx.Err(); e != nil {
			return zero, e
		}
		return out, nil
	}
	frame, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return zero, e
	}
	var body Solid
	if op == geom.TorusCylinderCommon {
		body, e = Torus(frame, R, r)
		if e != nil {
			return zero, e
		}
		out.owners[0] = []TorusCylinderNativeOwner{{0, 0, 0}}
		out.measures.VolumePi2.Mul(big.NewRat(2, 1), new(big.Rat).Mul(q(R), new(big.Rat).Mul(q(r), q(r))))
		out.measures.AreaPi2.Mul(big.NewRat(4, 1), new(big.Rat).Mul(q(R), q(r)))
	} else {
		hRat := new(big.Rat).Sub(q(upper), q(lower))
		height, e := geom.RepresentedExactScalar(hRat)
		if e != nil {
			return zero, e
		}
		frame, e = geom.NewFrame(geom.Vec3{Z: lower}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if e != nil {
			return zero, e
		}
		body, e = Cylinder(frame, c, height)
		if e != nil {
			return zero, e
		}
		for i := 1; i < 4; i++ {
			out.owners[i] = []TorusCylinderNativeOwner{{0, i - 1, i - 1}}
		}
		c2 := new(big.Rat).Mul(q(c), q(c))
		out.measures.VolumePi.Mul(c2, hRat)
		out.measures.AreaPi.Mul(big.NewRat(2, 1), new(big.Rat).Add(new(big.Rat).Mul(q(c), hRat), c2))
		out.measures.FirstZPi.Mul(out.measures.VolumePi, new(big.Rat).Quo(new(big.Rat).Add(q(lower), q(upper)), big.NewRat(2, 1)))
	}
	body, e = NewSolid(body.Vertices(), body.Edges(), body.Faces(), tolerance)
	if e != nil {
		return zero, e
	}
	out.bodies = []Solid{body}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}

package parametric

import (
	"context"
	"plainsolid/geom"
)

// CylinderTorusCavitySet has one material component and two disjoint boundary
// shells: an outward cylinder and an inward genus-one toroidal cavity.
// Strict radial and axial clearances prove embedding, independent of display.
type CylinderTorusCavitySet struct {
	base  EncompassingTorusCylinderSet
	query geom.CylinderTorusCavityMaterial
}

func (s CylinderTorusCavitySet) Solid() (Solid, error)                   { return s.base.Solid() }
func (s CylinderTorusCavitySet) Query() geom.CylinderTorusCavityMaterial { return s.query }
func (s CylinderTorusCavitySet) Measures() (TorusCylinderExactMeasures, error) {
	return s.base.Measures()
}
func (s CylinderTorusCavitySet) SourceFaces() ([4][]TorusCylinderNativeOwner, error) {
	return s.base.SourceFaces()
}
func NewCylinderTorusCavitySet(ctx context.Context, R, r, c, lower, upper, tolerance float64) (CylinderTorusCavitySet, error) {
	var zero CylinderTorusCavitySet
	outer, e := NewEncompassingTorusCylinderSet(ctx, R, r, c, lower, upper, geom.TorusCylinderUnion, tolerance)
	if e != nil {
		return zero, e
	}
	inner, e := NewEncompassingTorusCylinderSet(ctx, R, r, c, lower, upper, geom.TorusCylinderCommon, tolerance)
	if e != nil {
		return zero, e
	}
	q, e := geom.NewCylinderTorusCavityMaterial(R, r, c, lower, upper)
	if e != nil {
		return zero, e
	}
	a, _ := outer.Solid()
	b, _ := inner.Solid()
	for i := range b.faces {
		b.faces[i].Reversed = !b.faces[i].Reversed
	}
	body, e := joinIndependent(a, b)
	if e != nil {
		return zero, e
	}
	outer.bodies = []Solid{body}
	outer.owners[0] = []TorusCylinderNativeOwner{{0, 3, 3}}
	outer.measures.VolumePi2.Neg(inner.measures.VolumePi2)
	outer.measures.AreaPi2.Set(inner.measures.AreaPi2)
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return CylinderTorusCavitySet{outer, q}, nil
}

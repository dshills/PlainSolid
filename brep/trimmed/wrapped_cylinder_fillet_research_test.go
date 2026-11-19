//go:build research

package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestWrappedCylinderRimFilletHorizontalSplit(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 3, 5, o)
	if e != nil {
		t.Fatal(e)
	}
	stock, _, e := NewWrappedCylinder(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	s, history, e := FilletWrappedCylinderRims(ctx, stock, .5, 1, o)
	if e != nil {
		t.Fatal(e)
	}
	section, e := s.SectionWrappedCylinderRimFillet(ctx, .25, o)
	if e != nil || section.Major.Cmp(big.NewRat(5, 2)) != 0 || section.Radical.Cmp(big.NewRat(3, 16)) != 0 ||
		section.Curve != nil || len(section.Faces) != 1 || section.Faces[0] != history.Blends[0] {
		t.Fatal(section, e)
	}
	section, e = s.SectionWrappedCylinderRimFillet(ctx, .5, o)
	if e != nil || section.Curve == nil || len(section.Faces) != 2 || section.Radical.Sign() != 0 ||
		section.Major.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal(section, e)
	}
	section, e = s.SectionWrappedCylinderRimFillet(ctx, 0, o)
	if e != nil || section.Curve == nil || len(section.Faces) != 2 || section.Major.Cmp(big.NewRat(5, 2)) != 0 {
		t.Fatal(section, e)
	}
	lo, hi, h, e := s.SplitWrappedCylinderRimFilletAtZ(ctx, 2, o)
	if e != nil || h.LowerSource[h.LowerCut] != -1 || h.UpperSource[h.UpperCut] != -1 {
		t.Fatal(h, e)
	}
	for i, q := range []Solid{lo, hi} {
		p, e := q.ValidateWrappedCylinderRimFillet(ctx, o)
		if e != nil || p.Blends[i] < 0 || p.Blends[1-i] >= 0 ||
			[2][]int{h.LowerSource, h.UpperSource}[i][p.Blends[i]] != history.Blends[i] {
			t.Fatal(p, h, e)
		}
		m, e := q.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		r, z := [2]float64{.5, 1}[i], [2]float64{1.5, 2}[i]
		major := 3 - r
		want := math.Pi * (9*z + major*major*r + major*r*r*math.Pi/2 + 2*r*r*r/3)
		if e != nil || math.Abs(m.Volume-want) > 1e-8 {
			t.Fatal(m, want, e)
		}
	}
	if lo, hi, _, e := s.SplitWrappedCylinderRimFilletAtZ(ctx, .25, o); !errors.Is(e, geom.ErrDomain) || len(lo.faces)+len(hi.faces) != 0 {
		t.Fatal("cut enters a toroidal band", e)
	}
}

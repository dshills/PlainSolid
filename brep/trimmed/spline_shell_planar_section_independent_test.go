//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
	"plainsolid/section"
)

func TestIndependentShearedSplineSectionAncestry(t *testing.T) {
	ctx := context.Background()
	s, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	s, e := s.Transformed(geom.Affine{{1, .25, .5, 0}, {0, 1, .25, 0}, {0, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	plane, _ := geom.NewPlane(geom.Vec3{Z: .5}, geom.Vec3{Z: 1})
	r, e := SectionAffineSplineShell(ctx, s, plane, AffineSplineShellBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Loops) != 1 || len(r.Loops[0].Points) != 4 || r.Loops[0].SignedArea != 1 {
		t.Fatal("independent parallelogram area", r.Loops)
	}
	want := map[geom.Vec3]bool{{X: .25, Y: .125, Z: .5}: true, {X: 1.25, Y: .125, Z: .5}: true, {X: 1.5, Y: 1.125, Z: .5}: true, {X: .5, Y: 1.125, Z: .5}: true}
	for _, p := range r.Loops[0].Points {
		if !want[p] {
			t.Fatal("section vertex", p)
		}
		delete(want, p)
	}
	if len(want) != 0 {
		t.Fatal("missing section corners")
	}
	sourceCorners := 0
	for _, c := range r.Corners {
		if c.Point[2].Cmp(big.NewRat(1, 2)) != 0 {
			t.Fatal("nonexact section height")
		}
		if c.Edge < 0 {
			continue
		}
		sourceCorners++
		if c.Edge >= len(s.edges) || c.EdgeParameter == nil {
			t.Fatal("source edge", c.Edge)
		}
		edge := s.edges[c.Edge]
		a, b := s.vertices[edge.Start], s.vertices[edge.End]
		av, bv := []float64{a.X, a.Y, a.Z}, []float64{b.X, b.Y, b.Z}
		for k := 0; k < 3; k++ {
			ar, br := new(big.Rat).SetFloat64(av[k]), new(big.Rat).SetFloat64(bv[k])
			p := new(big.Rat).Add(ar, new(big.Rat).Mul(c.EdgeParameter, new(big.Rat).Sub(br, ar)))
			if p.Cmp(c.Point[k]) != 0 {
				t.Fatal("source edge identity or parameter changed", c.Edge, k)
			}
		}
	}
	if sourceCorners != 4 {
		t.Fatal("source corner count", sourceCorners)
	}
	faces := map[int]bool{}
	for _, seg := range r.Segments {
		if seg.Face < 0 || seg.Face >= len(s.faces) {
			t.Fatal("source face", seg.Face)
		}
		faces[seg.Face] = true
	}
	if len(faces) != 4 {
		t.Fatal("side face ancestry", faces)
	}
	tangent, _ := geom.NewPlane(geom.Vec3{}, geom.Vec3{Z: 1})
	bad, e := SectionAffineSplineShell(ctx, s, tangent, AffineSplineShellBooleanOptions{})
	if !errors.Is(e, section.ErrContact) || len(bad.Segments) != 0 {
		t.Fatal("tangent gate", e)
	}
}

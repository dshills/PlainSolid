//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestRationalSplineMaterialLocationOriginalCavity(t *testing.T) {
	ctx := context.Background()
	outer, _ := makeOpenSplineBox(t, [3]float64{-.125, -.125, -2}, [3]float64{1.125, 1.125, 0}, 0)
	inner := quarticRoofSolid(t, 1.0/16, 1.0/16)
	s, _, _, e := CutInternallyTangentSplineShellEllipseCavity(ctx, outer, inner, SplineShellRepeatedEllipseOptions{Faces: [2]int{5, 1}})
	if e != nil {
		t.Fatal(e)
	}
	x, y := big.NewRat(1, 3), big.NewRat(2, 5)
	face, e := exactFaceSupportPatch(s.faces[7], geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	boundary, e := face.EvaluateRational(ctx, x, y, geom.SurfacePatchEvaluationOptions{})
	if e != nil || boundary.Point[0].Cmp(x) != 0 || boundary.Point[1].Cmp(y) != 0 {
		t.Fatal("original curved boundary point", boundary, e)
	}
	for _, q := range []struct {
		z    *big.Rat
		want parametric.Location
	}{
		{big.NewRat(-1, 3), parametric.Outside},
		{big.NewRat(-4, 3), parametric.Inside},
		{boundary.Point[2], parametric.Boundary},
		{big.NewRat(1, 3), parametric.Outside},
	} {
		point := [3]*big.Rat{x, y, q.z}
		loc, e := s.LocateRationalSplineMaterialPoint(ctx, point, SplineShellBooleanOptions{})
		if e != nil || loc != q.want {
			t.Fatal("exact cavity material query", point, loc, q.want, e)
		}
	}
	origin := [3]*big.Rat{x, y, big.NewRat(-4, 3)}
	direction := [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(2, 3)}
	o := SplineRayOptions{Line: geom.LineSplineSurfaceOptions{MaxParameter: 4}}
	r, e := s.SplineRayRational(ctx, origin, direction, o)
	if e != nil || !r.Complete || len(r.Hits) != 3 || r.Hits[0].Direction != 1 || r.Hits[1].Direction != -1 ||
		r.Hits[2].Direction != 1 || r.Hits[2].ParameterMin > 2 || r.Hits[2].ParameterMax < 2 {
		t.Fatal("original cavity walls and stock cap at exact nonnormalized line scale", r, e)
	}
	limited := SplineShellBooleanOptions{Ray: SplineRayOptions{MaxFaces: 1}}
	if _, e := s.LocateRationalSplineMaterialPoint(ctx, origin, limited); !errors.Is(e, parametric.ErrBudget) {
		t.Fatal("face allowance", e)
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, e := s.LocateRationalSplineMaterialPoint(canceled, origin, SplineShellBooleanOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal("canceled classification", e)
	}
}

func TestRationalSplineMaterialLocationDisplayVertex(t *testing.T) {
	ctx := context.Background()
	shift := big.NewRat(1, 3)
	var parts []SewFace
	for axis := range 3 {
		for side := range 2 {
			reverse := (side == 0) != (axis == 1)
			parts = append(parts, exactSewTestFace(t, axis, big.NewRat(int64(side), 1), shift, reverse))
		}
	}
	s, _, e := SewRationalQuadratic(ctx, parts, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	point := [3]*big.Rat{shift, new(big.Rat), new(big.Rat)}
	o := SplineShellBooleanOptions{}
	if loc, e := s.LocateRationalSplineMaterialPoint(ctx, point, o); e != nil || loc != parametric.Boundary {
		t.Fatal("exact nonrepresentable shared vertex", loc, e)
	}
	x, _ := shift.Float64()
	rounded := geom.Vec3{X: x}
	if ratInset(x).Cmp(shift) >= 0 {
		t.Fatal("the fixture requires its stored vertex to round outside the stock")
	}
	if loc, e := s.LocateRationalSplineMaterialPoint(ctx, affineMaterialXYZ(rounded), o); e != nil || loc != parametric.Outside {
		t.Fatal("rounded vertex is only a display position", loc, e)
	}
	if loc, e := s.LocateSplineMaterialPoint(ctx, rounded, o); e != nil || loc != parametric.Outside {
		t.Fatal("represented query must also respect the exact original boundary", loc, e)
	}
}

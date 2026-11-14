package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestExactBezierTrimmedQueries(t *testing.T) {
	ctx := context.Background()
	// The independent graph is z = 1 − (u−1/2)² − (v−1/2)². The
	// cubic U representation has nonbinary X and Z control coefficients.
	uHeight := []*big.Rat{big.NewRat(-1, 4), big.NewRat(1, 12), big.NewRat(1, 12), big.NewRat(-1, 4)}
	vHeight := []*big.Rat{big.NewRat(-1, 4), big.NewRat(1, 4), big.NewRat(-1, 4)}
	var controls [][4]string
	for u, a := range uHeight {
		for v, b := range vHeight {
			z := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Add(a, b))
			controls = append(controls, [4]string{big.NewRat(int64(u), 3).RatString(), big.NewRat(int64(v), 2).RatString(), z.RatString(), "1"})
		}
	}
	d := geom.ParameterRectangle{UMax: 1, VMax: 1}
	p, e := geom.ImportExactBezierSurfaceRecipe(ctx, geom.ExactBezierSurfaceRecipe{Version: 1, Domain: d, DegreeU: 3, DegreeV: 2, Controls: controls}, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	exact, e := geom.NewExactBezierSurface(p)
	if e != nil {
		t.Fatal(e)
	}
	part, e := ExactSplineFaceRectangle(ctx, exact, d, false, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// These face queries use the genuine native face and its exact coedges.
	s := Solid{faces: []Face{part.Face}, vertices: part.Vertices}
	u, v := big.NewRat(2, 5), big.NewRat(1, 3)
	q := [3]*big.Rat{u, v, big.NewRat(433, 450)}
	loc, e := s.LocateRationalOnSplineFace(ctx, 0, q, SplineFacePointOptions{})
	if e != nil || loc.Location != SplineFacePointInterior || loc.UV[0].Cmp(u) != 0 || loc.UV[1].Cmp(v) != 0 {
		t.Fatal("nonbinary surface contact", loc, e)
	}
	q[2] = new(big.Rat).Add(q[2], big.NewRat(1, 1<<60))
	loc, e = s.LocateRationalOnSplineFace(ctx, 0, q, SplineFacePointOptions{UV: [][2]*big.Rat{{u, v}}})
	if e != nil || loc.Location != SplineFacePointOutside {
		t.Fatal("small exact model offset must separate the point", loc, e)
	}
	line, e := geom.NewLine(geom.Vec3{X: .375, Y: .5625, Z: 2}, geom.Vec3{Z: -1})
	if e != nil {
		t.Fatal(e)
	}
	hit, e := s.IntersectSplineFaceLine(ctx, 0, line, SplineFaceLineOptions{Support: geom.LineSplineSurfaceOptions{MinParameter: 0, MaxParameter: 2}})
	const tExact = 1 + 1.0/64 + 1.0/256
	if e != nil || !hit.Complete || len(hit.Hits) != 1 || hit.Hits[0].ParameterMin > tExact || hit.Hits[0].ParameterMax < tExact {
		t.Fatal("independent vertical contact", hit, e)
	}
	distance, e := s.BoundSplineBoundaryDistance(ctx, geom.Vec3{X: .5, Y: .5, Z: 2}, SplineBoundaryDistanceOptions{SquaredTolerance: 1e-4})
	if e != nil || !distance.Complete || !distance.HasWitness || distance.SquaredMin > 1 || distance.SquaredMax < 1 || distance.SquaredMax-distance.SquaredMin > 1e-4 {
		t.Fatal("independent peak distance", distance, e)
	}
	bounds, e := s.Bounds()
	if e != nil || bounds.Min.Z > .5 || bounds.Max.Z < 1 || bounds.Max.Z >= 1.5 {
		t.Fatal("native curved support has no conservative bounds", bounds, e)
	}
	if limited, e := s.LocateRationalOnSplineFace(ctx, 0, q, SplineFacePointOptions{Patch: geom.SurfacePatchOptions{MaxPoles: 11}}); !errors.Is(e, geom.ErrConvergence) || limited.Location != SplineFacePointUnknown {
		t.Fatal("exact support must respect the query patch limit", limited, e)
	}
	placement := geom.Identity()
	placement[0][3], placement[1][3], placement[2][3] = 1, -2, 3
	s.faces[0].Surface = parametric.MapSurface(exact, placement)
	regularity, e := s.ValidateSplineRegularity(ctx, geom.SurfacePatchOptions{}, geom.SurfaceRegularityOptions{})
	if e != nil || len(regularity) != 1 || !regularity[0].Supported || !regularity[0].Proof.Complete {
		t.Fatal("original placed chart regularity", regularity, e)
	}
	line, e = geom.NewLine(placement.Apply(line.Origin()), line.Direction())
	if e != nil {
		t.Fatal(e)
	}
	placedHit, e := s.IntersectSplineFaceLine(ctx, 0, line, SplineFaceLineOptions{Support: geom.LineSplineSurfaceOptions{MinParameter: 0, MaxParameter: 2}})
	if e != nil || !placedHit.Complete || len(placedHit.Hits) != 1 || placedHit.Hits[0].UV != hit.Hits[0].UV || placedHit.Hits[0].ParameterMin > tExact || placedHit.Hits[0].ParameterMax < tExact {
		t.Fatal("placed contact lost original chart coordinates", placedHit, e)
	}
	span := parametric.Interval{Min: .25, Max: .5}
	bounds, e = parametric.SurfaceBounds(s.faces[0].Surface, span, span)
	if e != nil || bounds.Min != (geom.Vec3{X: 1.25, Y: -1.75, Z: 3.875}) || bounds.Max != (geom.Vec3{X: 1.5, Y: -1.5, Z: 4}) {
		t.Fatal("exact placed subpatch hull", bounds, e)
	}
}

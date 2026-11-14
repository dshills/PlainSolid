package trimmed

import (
	"context"
	"math/big"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func TestPlacedWeightedFrustumGenerator(t *testing.T) {
	ctx := context.Background()
	patch := geom.SurfacePatchOptions{}
	w, _, e := geom.DraftWrappedFrustum(ctx, makeCanonicalWrappedCylinder(t, false, 0), 0, 1, patch)
	if e != nil {
		t.Fatal(e)
	}
	s, proof, e := NewWrappedFrustum(ctx, w, 1e-9, patch)
	if e != nil || proof.Circular {
		t.Fatal(proof, e)
	}
	faces := s.Faces()
	slices.Reverse(faces)
	s, e = NewSolidContext(ctx, s.Vertices(), s.Edges(), faces, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	q := big.NewRat
	z := q(0, 1)
	pose, e := placement.NewRationalRigid([3][4]*big.Rat{{q(3, 5), z, q(4, 5), q(1, 7)}, {z, q(1, 1), z, q(-2, 7)}, {q(-4, 5), z, q(3, 5), q(1, 13)}}, 0)
	if e != nil {
		t.Fatal(e)
	}
	// The profile's first quadrant at local U=1/2 is (5/7,5/7)
	// times its local radius. Radius grows 2→3 at Z=-2→3; this
	// line tracks that full generator with local Z=-2+2t.
	origin := [3]*big.Rat{q(-3, 5), q(8, 7), q(-1031, 455)}
	direction := [3]*big.Rat{q(62, 35), q(2, 7), q(34, 35)}
	r, e := s.IntersectPlacedWrappedFrustumLineRational(ctx, pose, origin, direction, q(-1, 7), q(3, 1), geom.ToroidLineOptions{ParameterTolerance: 1e-11})
	if e != nil || len(r.Hits) != 2 || len(r.Overlaps) != 1 {
		t.Fatal(r, e)
	}
	for i, h := range r.Hits {
		want := [2]*big.Rat{z, q(5, 2)}[i]
		owners := [2][]int{{1, 2}, {0, 2}}[i]
		if h.Parameter.Min.Cmp(want) != 0 || h.Parameter.Max.Cmp(want) != 0 || !slices.Equal(h.Faces, owners) {
			t.Fatal("reindexed exact generator and cap owners", i, h, want)
		}
	}
	span := r.Overlaps[0]
	if span.Face != 2 || span.Start.Min.Sign() != 0 || span.End.Max.Cmp(q(5, 2)) != 0 {
		t.Fatal("missing original curved face overlap", span)
	}
	material, e := s.ClipPlacedWrappedFrustumLineRational(ctx, pose, origin, direction, q(-1, 7), q(3, 1), geom.ToroidLineOptions{ParameterTolerance: 1e-11})
	if e != nil || len(material.Spans) != 1 || len(material.Boundary.Hits) != 2 || len(material.Boundary.Overlaps) != 1 || material.Spans[0][0].Min.Sign() != 0 || material.Spans[0][1].Max.Cmp(q(5, 2)) != 0 {
		t.Fatal("full closed generator is one material span", material, e)
	}
	// Exact world copy of the local axial line (0,0,-3)+(0,0,2/3)t.
	axisOrigin := [3]*big.Rat{q(-79, 35), q(-2, 7), q(-112, 65)}
	axisDir := [3]*big.Rat{q(8, 15), z, q(2, 5)}
	material, e = s.ClipPlacedWrappedFrustumLineRational(ctx, pose, axisOrigin, axisDir, q(1, 9), q(10, 1), geom.ToroidLineOptions{ParameterTolerance: 1e-11})
	if e != nil || len(material.Spans) != 1 || len(material.Boundary.Hits) != 2 || material.Spans[0][0].Min.Cmp(q(3, 2)) != 0 || material.Spans[0][1].Max.Cmp(q(9, 1)) != 0 {
		t.Fatal("solid axial clip between exact original caps", material, e)
	}
	clipped, e := s.ClipPlacedWrappedFrustumLineRational(ctx, pose, axisOrigin, axisDir, q(2, 1), q(3, 1), geom.ToroidLineOptions{ParameterTolerance: 1e-11})
	if e != nil || len(clipped.Spans) != 1 || len(clipped.Boundary.Hits) != 0 || clipped.Spans[0][0].Min.Cmp(q(2, 1)) != 0 || clipped.Spans[0][1].Max.Cmp(q(3, 1)) != 0 {
		t.Fatal("interior finite clip must not create boundary hits", clipped, e)
	}

	// At nonbinary local Z=1/3, the midpoint-profile boundary has
	// x=y=(5/7)(37/15)=37/21.
	// Pull back two distinct exact world points a sub-ULP radial step apart.
	epsilon := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	for _, sign := range []int64{-1, 0, 1} {
		x := new(big.Rat).Add(q(37, 21), new(big.Rat).Mul(q(sign, 1), epsilon))
		y := new(big.Rat).Set(x)
		world := [3]*big.Rat{new(big.Rat).Add(q(43, 105), new(big.Rat).Mul(q(3, 5), x)), new(big.Rat).Sub(y, q(2, 7)), new(big.Rat).Sub(q(18, 65), new(big.Rat).Mul(q(4, 5), x))}
		loc, e := s.LocatePlacedWrappedFrustumRational(ctx, pose, world, patch)
		want := [3]parametric.Location{parametric.Inside, parametric.Boundary, parametric.Outside}[sign+1]
		if e != nil || loc != want {
			t.Fatal("exact noncircular meridian classification", sign, loc, e)
		}
	}

	// Exact upper cap projection at local (1/3,1/7,3+1/9).
	query := [3]*big.Rat{q(892, 315), q(-1, 7), q(327, 195)}
	closest, e := s.BoundPointPlacedWrappedFrustumDistanceRational(ctx, pose, query, geom.SurfaceDistanceOptions{SquaredTolerance: 1e-14})
	if e != nil || closest.Lower.Cmp(q(1, 81)) != 0 || closest.Upper.Cmp(q(1, 81)) != 0 || closest.Face != 0 {
		t.Fatal("global exact noncircular cap projection", closest, e)
	}
	for i, want := range [3]*big.Rat{q(96, 35), q(-1, 7), q(314, 195)} {
		if closest.Witness[i].Cmp(want) != 0 {
			t.Fatal("exact world cap witness", i, closest.Witness, want)
		}
	}

}

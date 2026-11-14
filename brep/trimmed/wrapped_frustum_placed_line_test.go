package trimmed

import (
	"context"
	"math/big"
	"slices"
	"testing"

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
}

//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestMappedSplineChainNativeMaterialQueries(t *testing.T) {
	ctx := context.Background()
	boundary, closing, planar, seams := mappedChainClosedFixture(t)
	shell, err := CertifyMappedSplineChainShell(ctx, boundary, closing, [2]bool{false, true}, planar, seams, MappedChainShellProofOptions{})
	if err != nil {
		t.Fatal(err)
	}
	out, err := shell.BuildRectangularSolid(ctx, 1e-9, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var wire bytes.Buffer
	if err := WriteJSON(&wire, out.Solid); err != nil {
		t.Fatal(err)
	}
	solid, err := ReadJSON(&wire, JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	zero := new(big.Rat)
	for _, test := range []struct {
		name string
		p    [3]*big.Rat
		want parametric.Location
	}{
		{"north interior", [3]*big.Rat{big.NewRat(1, 3), big.NewRat(1, 2), zero}, parametric.Inside},
		{"south interior", [3]*big.Rat{big.NewRat(1, 4), big.NewRat(1, 32), big.NewRat(-1, 2)}, parametric.Inside},
		{"roof at nonbinary coordinates", [3]*big.Rat{big.NewRat(1, 3), big.NewRat(1, 2), big.NewRat(1, 6)}, parametric.Boundary},
		{"east wall", [3]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 3), big.NewRat(-1, 2)}, parametric.Boundary},
		{"above roof", [3]*big.Rat{big.NewRat(1, 3), big.NewRat(1, 2), big.NewRat(1, 2)}, parametric.Outside},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := solid.LocateRationalSplineMaterialPoint(ctx, test.p, SplineShellBooleanOptions{})
			if err != nil || got != test.want {
				t.Fatalf("rational material location got %v, want %v, err %v", got, test.want, err)
			}
		})
	}
	line, err := solid.SplineRayRational(ctx, [3]*big.Rat{big.NewRat(1, 3), big.NewRat(1, 2), zero},
		[3]*big.Rat{zero, zero, big.NewRat(2, 1)}, SplineRayOptions{
			Line: geom.LineSplineSurfaceOptions{MinParameter: -1, MaxParameter: 1},
		})
	if err != nil || !line.Complete || len(line.Issues) != 0 || len(line.Hits) != 2 {
		t.Fatal("complete exact rational line through both solid boundaries", line, err)
	}
	if line.Hits[0].Face != 6 || line.Hits[0].Direction != -1 || line.Hits[1].Face != 0 || line.Hits[1].Direction != 1 {
		t.Fatal("original floor and roof crossing order", line.Hits)
	}
	for i, want := range [2]*big.Rat{big.NewRat(-1, 2), big.NewRat(1, 12)} {
		h := line.Hits[i]
		if want.Cmp(ratInset(h.ParameterMin)) < 0 || want.Cmp(ratInset(h.ParameterMax)) > 0 {
			t.Fatal("line parameter enclosure excludes analytic contact", i, want, h)
		}
	}

	raw, err := boundary.sources[0].AuthoredParameter(big.NewRat(1, 8))
	if err != nil {
		t.Fatal(err)
	}
	uv := authoredMixedSplinePoint(t, boundary.sources[0].parts[0].source, raw)
	point := [3]*big.Rat{uv[0], uv[1], new(big.Rat).Mul(uv[0], uv[1])}
	t.Run("both original roof owners at a non-knot seam point", func(t *testing.T) {
		for owner := range 2 {
			r, err := solid.LocateRationalOnSplineFace(ctx, owner, point, SplineFacePointOptions{})
			if err != nil || r.Location != SplineFacePointOnTrim {
				t.Fatalf("owner %d location %v, err %v", owner, r.Location, err)
			}
		}
	})
	origin, err := geom.RepresentedExactPoint([3]*big.Rat{point[0], point[1], zero})
	if err != nil {
		t.Fatal(err)
	}
	candidates := []ExactSplineLineCandidate{
		{Face: 0, UV: uv},
		{Face: 1, UV: [2]*big.Rat{uv[1], uv[0]}},
	}
	contact, err := solid.VerifySplineLineCandidates(ctx, origin, geom.Vec3{Z: 2}, 0, 1, candidates, SplineFacePointOptions{})
	if err != nil || len(contact.Points) != 1 || len(contact.Rejected) != 0 || len(contact.Unresolved) != 0 {
		t.Fatal("exact original shared seam line witness", contact, err)
	}
	p := contact.Points[0]
	wantT := new(big.Rat).Quo(point[2], big.NewRat(2, 1))
	if p.Parameter.Cmp(wantT) != 0 || len(p.Faces) != 2 || p.Faces[0] != 0 || p.Faces[1] != 1 || !p.OnTrim[0] || !p.OnTrim[1] {
		t.Fatal("original two-owner incidence at one exact line point", p)
	}

}

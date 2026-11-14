package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestRationalStationBoundaryDistance(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	body, _, e := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{rationalStationBand(t, 0, 1, 3, 1, .75), rationalStationBand(t, 1, 3, 1, 2, .75)}, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	faces := body.Faces()
	slices.Reverse(faces)
	body, e = NewSolidContext(ctx, body.Vertices(), body.Edges(), faces, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	var buf bytes.Buffer
	if e = WriteJSON(&buf, body); e != nil {
		t.Fatal(e)
	}
	body, e = ReadJSONContext(ctx, &buf, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	proof, e := body.ValidateWrappedRationalStationLoft(ctx, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	point := func(x, y, z *big.Rat) [3]*big.Rat {
		return [3]*big.Rat{new(big.Rat).Add(r(1, 4), x), new(big.Rat).Add(r(-1, 2), y), z}
	}
	o := WrappedRationalStationDistanceOptions{Search: geom.SurfaceDistanceOptions{SquaredTolerance: .02}}
	// Every normalized profile radius is >=1: the weighted conic lies outside
	// the unit circle: its homogeneous radial residual is
	// (4*(3/4)^2-2)*t^2*(1-t)^2 >=0, with cardinal equality. On axis the exact
	// nearest squared distance is therefore the minimum over two linear radius
	// bands and the two terminal disks. No circular substitution is used by API.
	for _, tc := range []struct {
		name   string
		p      [3]*big.Rat
		want   *big.Rat
		owners []int
	}{
		// Original lower wall lies in x+2z<=3 (center-relative x).
		// Projection of (5/2,0,1/2) onto its cardinal generator is
		// (12/5,0,3/10), attaining squared distance1/20. All other
		// original faces have an axial gap of at least1/2.
		{"outside wall", point(r(5, 2), r(0, 1), r(1, 2)), r(1, 20), []int{proof.History.WallFaces[0]}},
		{"outside rim", point(r(4, 1), r(0, 1), r(0, 1)), r(1, 1), nil},
		{"outside cap", point(r(0, 1), r(0, 1), r(-1, 1)), r(1, 1), []int{proof.History.CapFaces[0]}},
		{"inside cap", point(r(0, 1), r(0, 1), r(1, 2)), r(1, 4), []int{proof.History.CapFaces[0]}},
		// Artificial lower cap of band1 would give1/25, but original boundary
		// minimum is the waist rim at26/25. This is the crucial omitted-cap case.
		{"inside near internal disk", point(r(0, 1), r(0, 1), r(6, 5)), r(26, 25), nil},
		{"exact noncircular crease", point(r(5, 7), r(5, 7), r(1, 1)), r(0, 1), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q, e := body.BoundWrappedRationalStationBoundaryDistanceRational(ctx, tc.p, o)
			if e != nil {
				t.Fatal(e)
			}
			if q.Lower.Cmp(tc.want) > 0 || q.Upper.Cmp(tc.want) < 0 || new(big.Rat).Sub(q.Upper, q.Lower).Cmp(r(1, 50)) > 0 || len(q.PerFace) != 4 {
				t.Fatal("bound", q.Lower, q.Upper, tc.want)
			}
			if tc.owners != nil && !reflect.DeepEqual(q.Faces, tc.owners) {
				t.Fatal("owners", q.Faces)
			}
			if tc.name == "inside near internal disk" && q.Lower.Cmp(r(1, 1)) <= 0 {
				t.Fatal("artificial cap selected", q.Lower)
			}
			if tc.name == "exact noncircular crease" {
				want := append([]int(nil), proof.History.WallFaces...)
				slices.Sort(want)
				if !reflect.DeepEqual(q.Faces, want) {
					t.Fatal("crease owners", q.Faces, want)
				}
			}
			sq := new(big.Rat)
			for i := range tc.p {
				d := new(big.Rat).Sub(q.Witness[i], tc.p[i])
				sq.Add(sq, new(big.Rat).Mul(d, d))
			}
			if sq.Cmp(q.Upper) != 0 {
				t.Fatal("unattained upper")
			}
			for _, f := range q.PerFace {
				loc, e := body.LocateWrappedRationalStationLoftRational(ctx, f.Witness, WrappedRationalStationPointOptions{})
				if e != nil || loc.Location != parametric.Boundary || !slices.Contains(loc.Faces, f.Face) {
					t.Fatal("face witness", f.Face, loc, e)
				}
			}
			saved := new(big.Rat).Set(q.Upper)
			q.PerFace[0].Upper.SetInt64(-9)
			if q.Upper.Cmp(saved) != 0 {
				t.Fatal("alias")
			}
		})
	}
}

func TestRationalStationBoundaryDistanceGuards(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	body, _, e := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{rationalStationBand(t, 0, 1, 3, 1, .75), rationalStationBand(t, 1, 3, 1, 2, .75)}, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p := [3]*big.Rat{r(1, 4), r(-1, 2), r(6, 5)}
	for _, o := range []WrappedRationalStationDistanceOptions{
		{Search: geom.SurfaceDistanceOptions{SquaredTolerance: .02, MaxCells: 1}},
		{Search: geom.SurfaceDistanceOptions{MaxOperations: 1}},
		{MaxPointOperations: 3},
		{Source: geom.SurfacePatchOptions{MaxOperations: 1}},
	} {
		q, e := body.BoundWrappedRationalStationBoundaryDistanceRational(ctx, p, o)
		if e == nil || q.Lower != nil || len(q.PerFace) != 0 {
			t.Fatal("non-atomic budget", q, e)
		}
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := body.BoundWrappedRationalStationBoundaryDistanceRational(cancelCtx, p, WrappedRationalStationDistanceOptions{}); e == nil || q.Lower != nil {
		t.Fatal("cancel")
	}
}

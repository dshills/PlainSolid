package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestRationalStationPointExactOwners(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	bands := []*geom.WrappedSurfacePatch{rationalStationBand(t, 0, 1, 3, 1, .75), rationalStationBand(t, 1, 3, 1, 2, .75)}
	body, _, e := NewWrappedRationalStationLoft(ctx, bands, 1e-8, geom.SurfacePatchOptions{})
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
	p, e := body.ValidateWrappedRationalStationLoft(ctx, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	h := p.History
	point := func(u, v, z, scale *big.Rat) [3]*big.Rat {
		return [3]*big.Rat{new(big.Rat).Add(r(1, 4), new(big.Rat).Mul(u, scale)), new(big.Rat).Add(r(-1, 2), new(big.Rat).Mul(v, scale)), new(big.Rat).Set(z)}
	}
	cases := []struct {
		name   string
		q      [3]*big.Rat
		where  parametric.Location
		owners []int
	}{
		{"internal disk is interior", point(r(0, 1), r(0, 1), r(1, 1), r(1, 1)), parametric.Inside, nil},
		{"crease rim", point(r(5, 7), r(5, 7), r(1, 1), r(1, 1)), parametric.Boundary, []int{h.WallFaces[0], h.WallFaces[1]}},
		{"lower cap interior", point(r(0, 1), r(0, 1), r(0, 1), r(3, 1)), parametric.Boundary, []int{h.CapFaces[0]}},
		{"upper cap rim", point(r(5, 7), r(5, 7), r(3, 1), r(2, 1)), parametric.Boundary, []int{h.CapFaces[1], h.WallFaces[1]}},
		{"nonbinary band height", point(r(5, 7), r(5, 7), r(1, 3), r(7, 3)), parametric.Boundary, []int{h.WallFaces[0]}},
		{"circle surrogate would reject", point(r(71, 100), r(71, 100), r(1, 1), r(1, 1)), parametric.Inside, nil},
		{"outside profile on station", point(r(3, 4), r(3, 4), r(1, 1), r(1, 1)), parametric.Outside, nil},
		{"below original source", point(r(0, 1), r(0, 1), r(-1, 3), r(1, 1)), parametric.Outside, nil},
		{"above original source", point(r(0, 1), r(0, 1), r(10, 3), r(1, 1)), parametric.Outside, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, e := body.LocateWrappedRationalStationLoftRational(ctx, tc.q, WrappedRationalStationPointOptions{})
			if e != nil {
				t.Fatal(e)
			}
			slices.Sort(tc.owners)
			if got.Location != tc.where || !reflect.DeepEqual(got.Faces, tc.owners) {
				t.Fatal(got, tc.where, tc.owners)
			}
		})
	}
	epsilon := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	for _, sign := range []int64{-1, 1} {
		u := new(big.Rat).Add(r(5, 7), new(big.Rat).Mul(r(sign, 1), epsilon))
		got, e := body.LocateWrappedRationalStationLoftRational(ctx, point(u, u, r(1, 1), r(1, 1)), WrappedRationalStationPointOptions{})
		want := parametric.Inside
		if sign > 0 {
			want = parametric.Outside
		}
		if e != nil || got.Location != want || len(got.Faces) != 0 {
			t.Fatal("sub-float exact side", sign, got, e)
		}
	}
	// All four original rational quadrant guards, including signed coordinates.
	for _, uv := range [][2]*big.Rat{{r(5, 7), r(5, 7)}, {r(-5, 7), r(5, 7)}, {r(-5, 7), r(-5, 7)}, {r(5, 7), r(-5, 7)}, {r(1, 1), r(0, 1)}} {
		got, e := body.LocateWrappedRationalStationLoftRational(ctx, point(uv[0], uv[1], r(2, 1), r(3, 2)), WrappedRationalStationPointOptions{})
		if e != nil || got.Location != parametric.Boundary || !reflect.DeepEqual(got.Faces, []int{h.WallFaces[1]}) {
			t.Fatal("quadrant owner", got, e)
		}
	}
	q := point(r(5, 7), r(5, 7), r(1, 1), r(1, 1))
	for _, o := range []WrappedRationalStationPointOptions{{MaxPointOperations: 1}, {Source: geom.SurfacePatchOptions{MaxOperations: 1}}} {
		got, e := body.LocateWrappedRationalStationLoftRational(ctx, q, o)
		if e == nil || len(got.Faces) != 0 || got.Location != parametric.Outside {
			t.Fatal("atomic quota", got, e)
		}
	}
	q[0].Add(q[0], epsilon)
	if got, e := body.LocateWrappedRationalStationLoftRational(ctx, q, WrappedRationalStationPointOptions{MaxCoefficientBits: 64}); e == nil || len(got.Faces) != 0 {
		t.Fatal("query coefficient bound", got, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := body.LocateWrappedRationalStationLoftRational(cancelled, q, WrappedRationalStationPointOptions{}); e == nil || len(got.Faces) != 0 {
		t.Fatal("cancel")
	}
	bad := body
	bad.faces = body.Faces()
	bad.faces[h.WallFaces[1]].Surface = rationalStationBand(t, 1, 3, 1, 2, .5)
	if got, e := bad.LocateWrappedRationalStationLoftRational(ctx, q, WrappedRationalStationPointOptions{}); e == nil || len(got.Faces) != 0 {
		t.Fatal("changed original profile accepted")
	}
}

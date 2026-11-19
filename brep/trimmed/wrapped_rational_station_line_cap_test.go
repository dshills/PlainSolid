//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"testing"

	"plainsolid/geom"
)

func TestRationalStationCoincidentCapClipping(t *testing.T) {
	slowtest.Skip(t)
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
	h := proof.History
	point := func(x, y, z *big.Rat) [3]*big.Rat {
		return [3]*big.Rat{new(big.Rat).Add(r(1, 4), x), new(big.Rat).Add(r(-1, 2), y), new(big.Rat).Set(z)}
	}
	d := [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}
	o := WrappedRationalStationLineOptions{Geometry: geom.ToroidLineOptions{ParameterTolerance: 1e-11}}
	rim := []int{h.CapFaces[0], h.WallFaces[0]}
	slices.Sort(rim)
	contains := func(root geom.ToroidLineRoot, x *big.Rat) bool { return root.Min.Cmp(x) <= 0 && root.Max.Cmp(x) >= 0 }
	full, e := body.ClipWrappedRationalStationLoftCapLineRational(ctx, point(r(0, 1), r(3, 2), r(0, 1)), d, r(-4, 1), r(4, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	if len(full.Hits) != 2 || len(full.Boundary) != 1 {
		t.Fatal(full)
	}
	span := full.Boundary[0]
	if !contains(span.Start, r(-21, 8)) || !contains(span.End, r(21, 8)) || !reflect.DeepEqual(span.Faces, []int{h.CapFaces[0]}) || !reflect.DeepEqual(span.StartFaces, rim) || !reflect.DeepEqual(span.EndFaces, rim) {
		t.Fatal("original noncircular cap/rim", span)
	}
	for _, hit := range full.Hits {
		if !reflect.DeepEqual(hit.Faces, rim) {
			t.Fatal("endpoint original owners")
		}
	}
	// Changing a returned event cannot mutate the independently owned span.
	saved := new(big.Rat).Set(span.Start.Min)
	full.Hits[0].Parameter.Min.SetInt64(-999)
	if span.Start.Min.Cmp(saved) != 0 {
		t.Fatal("aliased root result")
	}
	partial, e := body.ClipWrappedRationalStationLoftCapLineRational(ctx, point(r(0, 1), r(3, 2), r(0, 1)), d, r(0, 1), r(4, 1), o)
	if e != nil || len(partial.Hits) != 1 || len(partial.Boundary) != 1 {
		t.Fatal(partial, e)
	}
	if !reflect.DeepEqual(partial.Boundary[0].StartFaces, []int{h.CapFaces[0]}) || !reflect.DeepEqual(partial.Boundary[0].EndFaces, rim) {
		t.Fatal("finite endpoint inside cap")
	}
	inside, e := body.ClipWrappedRationalStationLoftCapLineRational(ctx, point(r(0, 1), r(3, 2), r(0, 1)), d, r(-1, 3), r(2, 3), o)
	if e != nil || len(inside.Hits) != 0 || len(inside.Boundary) != 1 || !contains(inside.Boundary[0].Start, r(-1, 3)) || !contains(inside.Boundary[0].End, r(2, 3)) {
		t.Fatal("whole clip remains boundary", inside, e)
	}
	// At upper radius2 and y=.5 the original cap equation is8x²+3x−36=0.
	upper, e := body.ClipWrappedRationalStationLoftCapLineRational(ctx, point(r(0, 1), r(1, 2), r(3, 1)), d, r(-4, 1), r(4, 1), o)
	if e != nil || len(upper.Boundary) != 1 || len(upper.Hits) != 2 {
		t.Fatal(upper, e)
	}
	q := upper.Hits[1].Parameter
	poly := func(x *big.Rat) *big.Rat {
		return new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Mul(r(8, 1), new(big.Rat).Mul(x, x)), new(big.Rat).Mul(r(3, 1), x)), r(36, 1))
	}
	upperRim := []int{h.CapFaces[1], h.WallFaces[1]}
	slices.Sort(upperRim)
	if q.Min.Cmp(q.Max) >= 0 || poly(q.Min).Sign() > 0 || poly(q.Max).Sign() < 0 || !reflect.DeepEqual(upper.Boundary[0].EndFaces, upperRim) {
		t.Fatal("irrational upper cap root", upper)
	}
	reversed, e := body.ClipWrappedRationalStationLoftCapLineRational(ctx, point(r(0, 1), r(3, 2), r(0, 1)), [3]*big.Rat{r(-2, 1), r(0, 1), r(0, 1)}, r(-2, 1), r(2, 1), o)
	if e != nil || len(reversed.Boundary) != 1 || !contains(reversed.Boundary[0].Start, r(-21, 16)) || !contains(reversed.Boundary[0].End, r(21, 16)) {
		t.Fatal("raw reversed t", reversed, e)
	}
	tangent, e := body.ClipWrappedRationalStationLoftCapLineRational(ctx, point(r(0, 1), r(3, 1), r(0, 1)), d, r(-4, 1), r(4, 1), o)
	if e != nil || len(tangent.Hits) != 1 || tangent.Hits[0].Parameter.Multiplicity != 2 || len(tangent.Boundary) != 0 || !reflect.DeepEqual(tangent.Hits[0].Faces, rim) {
		t.Fatal("cap tangent not a span", tangent, e)
	}
	for _, tc := range []struct {
		x      *big.Rat
		owners []int
	}{{r(0, 1), []int{h.CapFaces[0]}}, {r(3, 1), rim}, {r(4, 1), nil}} {
		singleton, e := body.ClipWrappedRationalStationLoftCapLineRational(ctx, point(tc.x, r(0, 1), r(0, 1)), d, r(0, 1), r(0, 1), o)
		if e != nil || len(singleton.Boundary) != 0 {
			t.Fatal("singleton", singleton, e)
		}
		if tc.owners == nil {
			if len(singleton.Hits) != 0 {
				t.Fatal("outside singleton")
			}
		} else if len(singleton.Hits) != 1 || !reflect.DeepEqual(singleton.Hits[0].Faces, tc.owners) {
			t.Fatal("singleton owners", singleton)
		}
	}
	if got, e := body.ClipWrappedRationalStationLoftCapLineRational(ctx, point(r(0, 1), r(0, 1), r(1, 1)), d, r(-2, 1), r(2, 1), o); e == nil || len(got.Boundary) != 0 || len(got.Hits) != 0 {
		t.Fatal("internal artificial cap accepted")
	}
	if got, e := body.ClipWrappedRationalStationLoftLineRational(ctx, point(r(0, 1), r(3, 2), r(0, 1)), d, r(-4, 1), r(4, 1), o); e == nil || len(got.Interior) != 0 {
		t.Fatal("legacy strict contract widened")
	}
	limited := o
	limited.MaxEvents = 1
	if got, e := body.ClipWrappedRationalStationLoftCapLineRational(ctx, point(r(0, 1), r(3, 2), r(0, 1)), d, r(-4, 1), r(4, 1), limited); e == nil || len(got.Hits) != 0 || len(got.Boundary) != 0 {
		t.Fatal("atomic event quota")
	}
	limited = o
	limited.Geometry.Surface.MaxOperations = 1
	if got, e := body.ClipWrappedRationalStationLoftCapLineRational(ctx, point(r(0, 1), r(3, 2), r(0, 1)), d, r(-4, 1), r(4, 1), limited); e == nil || len(got.Hits) != 0 || len(got.Boundary) != 0 {
		t.Fatal("atomic proof quota")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := body.ClipWrappedRationalStationLoftCapLineRational(cancelled, point(r(0, 1), r(0, 1), r(0, 1)), d, r(-4, 1), r(4, 1), o); e == nil || len(got.Hits) != 0 {
		t.Fatal("cancel")
	}
}

//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"testing"
)

func TestRationalStationGeneratorSpans(t *testing.T) {
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
		return [3]*big.Rat{new(big.Rat).Add(r(1, 4), x), new(big.Rat).Add(r(-1, 2), y), z}
	}
	o := WrappedRationalStationLineOptions{Geometry: geom.ToroidLineOptions{ParameterTolerance: 1e-11}}
	sorted := func(a ...int) []int { slices.Sort(a); return a }
	cases := []struct {
		name         string
		band         int
		p, d         [3]*big.Rat
		lo, hi, a, b *big.Rat
		first, last  []int
	}{
		{"lower cardinal", 0, point(r(3, 1), r(0, 1), r(0, 1)), [3]*big.Rat{r(-2, 1), r(0, 1), r(1, 1)}, r(-2, 1), r(4, 1), r(0, 1), r(1, 1), sorted(h.CapFaces[0], h.WallFaces[0]), sorted(h.WallFaces[0], h.WallFaces[1])},
		// The true weighted-profile diagonal is (5/7,5/7), not a unit circle.
		{"noncircular diagonal", 0, point(r(15, 7), r(15, 7), r(0, 1)), [3]*big.Rat{r(-10, 7), r(-10, 7), r(1, 1)}, r(-2, 1), r(4, 1), r(0, 1), r(1, 1), sorted(h.CapFaces[0], h.WallFaces[0]), sorted(h.WallFaces[0], h.WallFaces[1])},
		{"upper", 1, point(r(1, 1), r(0, 1), r(1, 1)), [3]*big.Rat{r(1, 2), r(0, 1), r(1, 1)}, r(-3, 1), r(5, 1), r(0, 1), r(2, 1), sorted(h.WallFaces[0], h.WallFaces[1]), sorted(h.WallFaces[1], h.CapFaces[1])},
		{"finite clip", 0, point(r(3, 1), r(0, 1), r(0, 1)), [3]*big.Rat{r(-2, 1), r(0, 1), r(1, 1)}, r(1, 3), r(2, 3), r(1, 3), r(2, 3), []int{h.WallFaces[0]}, []int{h.WallFaces[0]}},
		{"reverse", 0, point(r(1, 1), r(0, 1), r(1, 1)), [3]*big.Rat{r(4, 1), r(0, 1), r(-2, 1)}, r(-1, 1), r(2, 1), r(0, 1), r(1, 2), sorted(h.WallFaces[0], h.WallFaces[1]), sorted(h.CapFaces[0], h.WallFaces[0])},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q, e := body.ClipWrappedRationalStationLoftGeneratorRational(ctx, h.WallFaces[c.band], c.p, c.d, c.lo, c.hi, o)
			if e != nil {
				t.Fatal(e)
			}
			if q.Start.Min.Cmp(c.a) != 0 || q.Start.Max.Cmp(c.a) != 0 || q.End.Min.Cmp(c.b) != 0 || q.End.Max.Cmp(c.b) != 0 || !reflect.DeepEqual(q.Faces, []int{h.WallFaces[c.band]}) || !reflect.DeepEqual(q.StartFaces, c.first) || !reflect.DeepEqual(q.EndFaces, c.last) {
				t.Fatal(q)
			}
		})
	}
	c := cases[0]
	for _, face := range []int{h.CapFaces[0], h.WallFaces[1], -1} {
		if _, e := body.ClipWrappedRationalStationLoftGeneratorRational(ctx, face, c.p, c.d, c.lo, c.hi, o); e == nil {
			t.Fatal("wrong support accepted", face)
		}
	}
	d := [3]*big.Rat{r(0, 1), r(0, 1), r(1, 1)}
	if _, e := body.ClipWrappedRationalStationLoftGeneratorRational(ctx, h.WallFaces[0], c.p, d, c.lo, c.hi, o); e == nil {
		t.Fatal("secant accepted")
	}
	for _, n := range []int{2, 8} {
		bad := o
		bad.MaxPointOperations = n
		q, e := body.ClipWrappedRationalStationLoftGeneratorRational(ctx, h.WallFaces[0], c.p, c.d, c.lo, c.hi, bad)
		if e == nil || q.Start.Min != nil {
			t.Fatal("non-atomic point budget", e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := body.ClipWrappedRationalStationLoftGeneratorRational(cancelled, h.WallFaces[0], c.p, c.d, c.lo, c.hi, o); e == nil {
		t.Fatal("cancellation")
	}
}

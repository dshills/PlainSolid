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

func TestRationalStationLineClipping(t *testing.T) {
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
	vector := func(x, y, z int64) [3]*big.Rat { return [3]*big.Rat{r(x, 1), r(y, 1), r(z, 1)} }
	options := WrappedRationalStationLineOptions{Geometry: geom.ToroidLineOptions{ParameterTolerance: 1e-10}}
	cases := []struct {
		name   string
		p, d   [3]*big.Rat
		lo, hi *big.Rat
		roots  []*big.Rat
		owners [][]int
		spans  [][2]int
	}{
		{"axis internal caps removed", point(r(0, 1), r(0, 1), r(-1, 1)), vector(0, 0, 1), r(0, 1), r(5, 1), []*big.Rat{r(1, 1), r(4, 1)}, [][]int{{h.CapFaces[0]}, {h.CapFaces[1]}}, [][2]int{{0, 1}}},
		{"waist separates material", point(r(1, 1), r(1, 1), r(-1, 1)), vector(0, 0, 1), r(0, 1), r(5, 1), []*big.Rat{r(1, 1), r(9, 5), r(14, 5), r(4, 1)}, [][]int{{h.CapFaces[0]}, {h.WallFaces[0]}, {h.WallFaces[1]}, {h.CapFaces[1]}}, [][2]int{{0, 1}, {2, 3}}},
		{"horizontal original crease", point(r(0, 1), r(1, 2), r(1, 1)), vector(1, 0, 0), r(-2, 1), r(2, 1), []*big.Rat{r(-7, 8), r(7, 8)}, [][]int{{h.WallFaces[0], h.WallFaces[1]}, {h.WallFaces[0], h.WallFaces[1]}}, [][2]int{{0, 1}}},
		{"oblique weighted profile", point(r(0, 1), r(0, 1), r(0, 1)), vector(1, 1, 1), r(-1, 1), r(4, 1), []*big.Rat{r(0, 1), r(15, 17)}, [][]int{{h.CapFaces[0]}, {h.WallFaces[0]}}, [][2]int{{0, 1}}},
		{"nonhorizontal crease tie", point(r(5, 7), r(5, 7), r(0, 1)), vector(0, 0, 1), r(-1, 1), r(4, 1), []*big.Rat{r(0, 1), r(1, 1), r(3, 1)}, [][]int{{h.CapFaces[0]}, {h.WallFaces[0], h.WallFaces[1]}, {h.CapFaces[1]}}, [][2]int{{0, 1}, {1, 2}}},
		{"reversed unnormalized parameter", point(r(0, 1), r(0, 1), r(4, 1)), vector(0, 0, -2), r(0, 1), r(3, 1), []*big.Rat{r(1, 2), r(2, 1)}, [][]int{{h.CapFaces[1]}, {h.CapFaces[0]}}, [][2]int{{0, 1}}},
	}
	contains := func(root geom.ToroidLineRoot, q *big.Rat) bool { return root.Min.Cmp(q) <= 0 && root.Max.Cmp(q) >= 0 }
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, e := body.ClipWrappedRationalStationLoftLineRational(ctx, tc.p, tc.d, tc.lo, tc.hi, options)
			if e != nil {
				t.Fatal(e)
			}
			if len(got.Hits) != len(tc.roots) || len(got.Interior) != len(tc.spans) {
				t.Fatal("count", got)
			}
			for i, q := range tc.roots {
				slices.Sort(tc.owners[i])
				if !contains(got.Hits[i].Parameter, q) || !reflect.DeepEqual(got.Hits[i].Faces, tc.owners[i]) {
					t.Fatal("root/owner", i, got.Hits[i], q, tc.owners[i])
				}
			}
			for i, pair := range tc.spans {
				if !contains(got.Interior[i].Start, tc.roots[pair[0]]) || !contains(got.Interior[i].End, tc.roots[pair[1]]) {
					t.Fatal("material clip", got.Interior)
				}
			}
		})
	}
	// The independent positive quadrant equation at y=1/4 is 16x²+3x−18.
	irrational, e := body.ClipWrappedRationalStationLoftLineRational(ctx, point(r(0, 1), r(1, 4), r(1, 1)), vector(1, 0, 0), r(-2, 1), r(2, 1), options)
	if e != nil || len(irrational.Hits) != 2 || len(irrational.Interior) != 1 {
		t.Fatal("irrational roots", irrational, e)
	}
	polynomial := func(x *big.Rat) *big.Rat {
		return new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Mul(r(16, 1), new(big.Rat).Mul(x, x)), new(big.Rat).Mul(r(3, 1), x)), r(18, 1))
	}
	q := irrational.Hits[1].Parameter
	if q.Min.Cmp(q.Max) >= 0 || polynomial(q.Min).Sign() > 0 || polynomial(q.Max).Sign() < 0 {
		t.Fatal("algebraic bracket", q)
	}
	tangent, e := body.ClipWrappedRationalStationLoftLineRational(ctx, point(r(0, 1), r(1, 1), r(1, 1)), vector(1, 0, 0), r(-2, 1), r(2, 1), options)
	if e != nil || len(tangent.Hits) != 1 || tangent.Hits[0].Parameter.Multiplicity != 2 || len(tangent.Interior) != 0 {
		t.Fatal("isolated tangent", tangent, e)
	}
	// Interior clip endpoints are retained exactly without inventing contacts.
	clipped, e := body.ClipWrappedRationalStationLoftLineRational(ctx, point(r(0, 1), r(0, 1), r(0, 1)), vector(0, 0, 1), r(1, 3), r(2, 3), options)
	if e != nil || len(clipped.Hits) != 0 || len(clipped.Interior) != 1 || !contains(clipped.Interior[0].Start, r(1, 3)) || !contains(clipped.Interior[0].End, r(2, 3)) {
		t.Fatal("finite interior clip", clipped, e)
	}
	for _, bad := range []struct{ p, d [3]*big.Rat }{{point(r(0, 1), r(0, 1), r(0, 1)), vector(1, 0, 0)}, {point(r(3, 1), r(0, 1), r(0, 1)), vector(-2, 0, 1)}} {
		got, e := body.ClipWrappedRationalStationLoftLineRational(ctx, bad.p, bad.d, r(-1, 1), r(4, 1), options)
		if e == nil || len(got.Hits) != 0 || len(got.Interior) != 0 {
			t.Fatal("coincident interval must fail atomically", got, e)
		}
	}
	for _, limited := range []WrappedRationalStationLineOptions{{Geometry: options.Geometry, MaxEvents: 1}, {Geometry: options.Geometry, MaxPointOperations: 1}, {Geometry: geom.ToroidLineOptions{ParameterTolerance: 1e-10, Surface: geom.SurfacePatchOptions{MaxOperations: 1}}}} {
		got, e := body.ClipWrappedRationalStationLoftLineRational(ctx, point(r(0, 1), r(0, 1), r(-1, 1)), vector(0, 0, 1), r(0, 1), r(5, 1), limited)
		if e == nil || len(got.Hits) != 0 || len(got.Interior) != 0 {
			t.Fatal("atomic budget", got, e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := body.ClipWrappedRationalStationLoftLineRational(cancelled, point(r(0, 1), r(0, 1), r(0, 1)), vector(1, 0, 0), r(-2, 1), r(2, 1), options); e == nil || len(got.Hits) != 0 {
		t.Fatal("cancel")
	}
}

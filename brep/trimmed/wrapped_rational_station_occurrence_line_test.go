package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func TestRationalStationOccurrenceLines(t *testing.T) {
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
	pose, e := placement.NewRationalRigid([3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 3)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 7)}, {r(-4, 5), r(0, 1), r(3, 5), r(5, 11)}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	occurrence, e := body.PlaceWrappedRationalStation(ctx, pose, WrappedRationalStationOccurrenceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// Independently author the world line; do not use the adapter's transform.
	rotate := func(p [3]*big.Rat) [3]*big.Rat {
		return [3]*big.Rat{new(big.Rat).Add(new(big.Rat).Mul(r(3, 5), p[0]), new(big.Rat).Mul(r(4, 5), p[2])), new(big.Rat).Set(p[1]), new(big.Rat).Add(new(big.Rat).Mul(r(-4, 5), p[0]), new(big.Rat).Mul(r(3, 5), p[2]))}
	}
	world := func(p [3]*big.Rat) [3]*big.Rat {
		v := rotate(p)
		for i, shift := range []*big.Rat{r(1, 3), r(-2, 7), r(5, 11)} {
			v[i].Add(v[i], shift)
		}
		return v
	}
	point := func(x, y, z *big.Rat) [3]*big.Rat {
		return [3]*big.Rat{new(big.Rat).Add(r(1, 4), x), new(big.Rat).Add(r(-1, 2), y), z}
	}
	o := WrappedRationalStationLineOptions{Geometry: geom.ToroidLineOptions{ParameterTolerance: 1e-11}}
	for _, tc := range []struct {
		name   string
		p, d   [3]*big.Rat
		lo, hi *big.Rat
	}{
		{"two material spans", point(r(1, 1), r(1, 1), r(-1, 1)), [3]*big.Rat{r(0, 1), r(0, 1), r(1, 1)}, r(0, 1), r(5, 1)},
		{"irrational reversed crease", point(r(0, 1), r(1, 4), r(1, 1)), [3]*big.Rat{r(-2, 1), r(0, 1), r(0, 1)}, r(-2, 1), r(2, 1)},
		{"oblique", point(r(0, 1), r(0, 1), r(0, 1)), [3]*big.Rat{r(1, 1), r(1, 1), r(1, 1)}, r(-1, 1), r(4, 1)},
		{"tangent", point(r(0, 1), r(1, 1), r(1, 1)), [3]*big.Rat{r(3, 1), r(0, 1), r(0, 1)}, r(-1, 1), r(1, 1)},
		{"finite interior clip", point(r(0, 1), r(0, 1), r(4, 1)), [3]*big.Rat{r(0, 1), r(0, 1), r(-2, 1)}, r(3, 4), r(1, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want, e := body.ClipWrappedRationalStationLoftLineRational(ctx, tc.p, tc.d, tc.lo, tc.hi, o)
			if e != nil {
				t.Fatal(e)
			}
			got, e := occurrence.ClipLineRational(ctx, world(tc.p), rotate(tc.d), tc.lo, tc.hi, o)
			if e != nil || !reflect.DeepEqual(got, want) {
				t.Fatal("changed original t/bracket/owner", got, want, e)
			}
			if tc.name == "two material spans" && len(got.Interior) != 2 {
				t.Fatal(got)
			}
			if tc.name == "irrational reversed crease" {
				if len(got.Hits) != 2 || len(got.Interior) != 1 {
					t.Fatal(got)
				}
				root := got.Hits[1].Parameter
				poly := func(t *big.Rat) *big.Rat {
					return new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Mul(r(32, 1), new(big.Rat).Mul(t, t)), new(big.Rat).Mul(r(3, 1), t)), r(9, 1))
				}
				if root.Min.Cmp(root.Max) >= 0 || poly(root.Min).Sign() > 0 || poly(root.Max).Sign() < 0 {
					t.Fatal("raw reversed t:32t²+3t-9", root)
				}
				owners := append([]int(nil), proof.History.WallFaces...)
				slices.Sort(owners)
				for _, hit := range got.Hits {
					if !reflect.DeepEqual(hit.Faces, owners) {
						t.Fatal("crease", hit.Faces)
					}
				}
			}
			if tc.name == "tangent" && (len(got.Interior) != 0 || len(got.Hits) != 1 || got.Hits[0].Parameter.Multiplicity != 2) {
				t.Fatal("tangent interior", got)
			}
		})
	}
	p, d := point(r(0, 1), r(3, 2), r(0, 1)), [3]*big.Rat{r(2, 1), r(0, 1), r(0, 1)}
	cap, e := occurrence.ClipCapLineRational(ctx, world(p), rotate(d), r(-2, 1), r(2, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	original, e := body.ClipWrappedRationalStationLoftCapLineRational(ctx, p, d, r(-2, 1), r(2, 1), o)
	if e != nil || !reflect.DeepEqual(cap, original) || len(cap.Boundary) != 1 || cap.Boundary[0].Start.Min.Cmp(r(-21, 16)) != 0 || cap.Boundary[0].End.Min.Cmp(r(21, 16)) != 0 {
		t.Fatal("oblique world cap", cap, e)
	}
	if _, e := occurrence.ClipLineRational(ctx, world(p), rotate(d), r(-2, 1), r(2, 1), o); e == nil {
		t.Fatal("strict API accepted cap coincidence")
	}
	p, d = point(r(1, 1), r(0, 1), r(1, 1)), [3]*big.Rat{r(1, 2), r(0, 1), r(1, 1)}
	generator, e := occurrence.ClipGeneratorRational(ctx, proof.History.WallFaces[1], world(p), rotate(d), r(-1, 1), r(4, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	want, e := body.ClipWrappedRationalStationLoftGeneratorRational(ctx, proof.History.WallFaces[1], p, d, r(-1, 1), r(4, 1), o)
	if e != nil || !reflect.DeepEqual(generator, want) || generator.Start.Min.Sign() != 0 || generator.End.Min.Cmp(r(2, 1)) != 0 {
		t.Fatal("world selected generator", generator, e)
	}
	if _, e := occurrence.ClipLineRational(ctx, world(p), rotate(d), r(-1, 1), r(4, 1), o); e == nil {
		t.Fatal("strict API accepted generator")
	}
	if _, e := occurrence.ClipGeneratorRational(ctx, proof.History.CapFaces[1], world(p), rotate(d), r(-1, 1), r(4, 1), o); e == nil {
		t.Fatal("wrong original face")
	}
}

func TestRationalStationOccurrenceLineGuards(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	body, _, e := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{rationalStationBand(t, 0, 1, 3, 1, .75)}, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pose, e := placement.NewRationalRigid([3][4]*big.Rat{{r(1, 1), r(0, 1), r(0, 1), r(0, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(0, 1), r(0, 1), r(1, 1), r(0, 1)}}, 64)
	if e != nil {
		t.Fatal(e)
	}
	q, e := body.PlaceWrappedRationalStation(ctx, pose, WrappedRationalStationOccurrenceOptions{MaxTransformBits: 64})
	if e != nil {
		t.Fatal(e)
	}
	p, d := [3]*big.Rat{r(1, 4), r(-1, 2), r(-1, 1)}, [3]*big.Rat{r(0, 1), r(0, 1), r(1, 1)}
	o := WrappedRationalStationLineOptions{Geometry: geom.ToroidLineOptions{ParameterTolerance: 1e-11}, MaxEvents: 1}
	if got, e := q.ClipLineRational(ctx, p, d, r(0, 1), r(3, 1), o); e == nil || len(got.Hits) != 0 || len(got.Interior) != 0 {
		t.Fatal("original budget", got, e)
	}
	d[2] = new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	if got, e := q.ClipLineRational(ctx, p, d, r(0, 1), r(3, 1), o); e == nil || len(got.Hits) != 0 {
		t.Fatal("direction transform budget", got, e)
	}
	if got, e := q.ClipCapLineRational(ctx, p, d, r(0, 1), r(3, 1), o); e == nil || len(got.Boundary) != 0 {
		t.Fatal("cap atomic transform", got, e)
	}
	if got, e := q.ClipGeneratorRational(ctx, 0, p, d, r(0, 1), r(3, 1), o); e == nil || got.Start.Min != nil {
		t.Fatal("generator atomic transform", got, e)
	}
	if _, e := (WrappedRationalStationOccurrence{}).ClipLineRational(ctx, p, d, r(0, 1), r(3, 1), o); e == nil {
		t.Fatal("zero occurrence")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := q.ClipLineRational(cancelled, p, d, r(0, 1), r(3, 1), o); e == nil {
		t.Fatal("cancel")
	}
}

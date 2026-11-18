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
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func TestRationalStationOccurrenceDistance(t *testing.T) {
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
	pose, e := placement.NewRationalRigid([3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 3)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 7)}, {r(-4, 5), r(0, 1), r(3, 5), r(5, 11)}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	q, e := body.PlaceWrappedRationalStation(ctx, pose, WrappedRationalStationOccurrenceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	world := func(p [3]*big.Rat) [3]*big.Rat {
		return [3]*big.Rat{new(big.Rat).Add(r(1, 3), new(big.Rat).Add(new(big.Rat).Mul(r(3, 5), p[0]), new(big.Rat).Mul(r(4, 5), p[2]))), new(big.Rat).Add(r(-2, 7), p[1]), new(big.Rat).Add(r(5, 11), new(big.Rat).Add(new(big.Rat).Mul(r(-4, 5), p[0]), new(big.Rat).Mul(r(3, 5), p[2])))}
	}
	square := func(a, b [3]*big.Rat) *big.Rat {
		v := new(big.Rat)
		for i := range a {
			d := new(big.Rat).Sub(a[i], b[i])
			v.Add(v, new(big.Rat).Mul(d, d))
		}
		return v
	}
	o := WrappedRationalStationDistanceOptions{Search: geom.SurfaceDistanceOptions{SquaredTolerance: .02}}
	for _, tc := range []struct {
		name  string
		p     [3]*big.Rat
		truth *big.Rat
	}{
		{"wall", [3]*big.Rat{r(11, 4), r(-1, 2), r(1, 2)}, r(1, 20)},
		{"internal station not cap", [3]*big.Rat{r(1, 4), r(-1, 2), r(6, 5)}, r(26, 25)},
		{"exposed lower cap", [3]*big.Rat{r(1, 4), r(-1, 2), r(-1, 1)}, r(1, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := world(tc.p)
			got, e := q.BoundBoundaryDistanceRational(ctx, p, o)
			if e != nil {
				t.Fatal(e)
			}
			if got.World.Lower.Cmp(tc.truth) > 0 || got.World.Upper.Cmp(tc.truth) < 0 || got.World.Lower.Cmp(got.Original.Lower) != 0 || got.World.Upper.Cmp(got.Original.Upper) != 0 || !reflect.DeepEqual(got.World.Faces, got.Original.Faces) || got.World.VisitedCells != got.Original.VisitedCells || got.World.ExactOperations != got.Original.ExactOperations {
				t.Fatal("invariant interval/owners/work", got)
			}
			if square(p, got.World.Witness).Cmp(got.World.Upper) != 0 {
				t.Fatal("world attained upper")
			}
			expected := world(got.Original.Witness)
			for i := range expected {
				if expected[i].Cmp(got.World.Witness[i]) != 0 {
					t.Fatal("independent world formula")
				}
			}
			loc, e := q.LocateRational(ctx, got.World.Witness, WrappedRationalStationPointOptions{})
			if e != nil || loc.Location != parametric.Boundary || !reflect.DeepEqual(loc.Faces, got.World.Faces) {
				t.Fatal("authenticated world witness owners", loc, e)
			}
			for i, f := range got.World.PerFace {
				old := got.Original.PerFace[i]
				if f.Face != old.Face || f.Lower.Cmp(old.Lower) != 0 || f.Upper.Cmp(old.Upper) != 0 || square(p, f.Witness).Cmp(f.Upper) != 0 {
					t.Fatal("per-face metric", f, old)
				}
				expected := world(old.Witness)
				for k := range expected {
					if expected[k].Cmp(f.Witness[k]) != 0 {
						t.Fatal("per-face transform")
					}
				}
				loc, e := q.LocateRational(ctx, f.Witness, WrappedRationalStationPointOptions{})
				if e != nil || loc.Location != parametric.Boundary || !slices.Contains(loc.Faces, f.Face) {
					t.Fatal("per-face retained witness", f, loc, e)
				}
			}
			if tc.name == "wall" {
				original, e := body.BoundWrappedRationalStationBoundaryDistanceRational(ctx, tc.p, o)
				if e != nil || !reflect.DeepEqual(original, got.Original) {
					t.Fatal("single unchanged original query", e)
				}
			}
			saved := new(big.Rat).Set(got.World.Upper)
			got.Original.Upper.SetInt64(-9)
			got.Original.Faces[0] = -99
			got.Original.PerFace[0].Witness[0].SetInt64(-99)
			if got.World.Upper.Cmp(saved) != 0 || got.World.Faces[0] < 0 || got.World.PerFace[0].Witness[0].Cmp(r(-99, 1)) == 0 {
				t.Fatal("aliased world evidence")
			}
		})
	}
}

func TestRationalStationOccurrenceDistanceAtomic(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	body, _, e := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{rationalStationBand(t, 0, 1, 3, 1, .6)}, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// Exact Pythagorean rotation has29-bit ratios; it fits the64-bit pose
	// limit but transforms nonbinary original wall samples beyond that limit.
	c, s := r(536838144, 536838145), r(32767, 536838145)
	pose, e := placement.NewRationalRigid([3][4]*big.Rat{{c, r(0, 1), s, r(0, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {new(big.Rat).Neg(s), r(0, 1), c, r(0, 1)}}, 64)
	if e != nil {
		t.Fatal(e)
	}
	q, e := body.PlaceWrappedRationalStation(ctx, pose, WrappedRationalStationOccurrenceOptions{MaxTransformBits: 64})
	if e != nil {
		t.Fatal(e)
	}
	p := [3]*big.Rat{r(0, 1), r(0, 1), r(1, 1)}
	local, e := pose.TransformPoint(ctx, p, true, 64)
	if e != nil {
		t.Fatal(e)
	}
	o := WrappedRationalStationDistanceOptions{Search: geom.SurfaceDistanceOptions{SquaredTolerance: .05}}
	original, e := body.BoundWrappedRationalStationBoundaryDistanceRational(ctx, local, o)
	if e != nil {
		t.Fatal(e)
	}
	failed := false
	for _, f := range original.PerFace {
		if _, e := pose.TransformPoint(ctx, f.Witness, false, 64); e != nil {
			failed = true
		}
	}
	if !failed {
		t.Fatal("fixture lacks post-query witness overflow")
	}
	if got, e := q.BoundBoundaryDistanceRational(ctx, p, o); e == nil || got.Original.Lower != nil || got.World.Lower != nil || len(got.World.PerFace) != 0 {
		t.Fatal("partial report after transport failure", got, e)
	}
	o.Search.MaxOperations = 1
	if got, e := q.BoundBoundaryDistanceRational(ctx, p, o); e == nil || got.Original.Lower != nil || got.World.Lower != nil {
		t.Fatal("source query quota", got, e)
	}
	if _, e := (WrappedRationalStationOccurrence{}).BoundBoundaryDistanceRational(ctx, p, o); e == nil {
		t.Fatal("zero occurrence")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := q.BoundBoundaryDistanceRational(cancelled, p, o); e == nil {
		t.Fatal("cancel")
	}
}

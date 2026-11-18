package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestNativeObliqueCavityPartition(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	frame := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(-4, 5), r(0, 1), r(3, 5), r(0, 1)}}
	body, _, e := ObliqueTorusCavity(ctx, 3, 1, 8, -6, 6, frame, 1e-9, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	prepared, e := PrepareObliqueTorusCavity(ctx, body, 3, 1, 8, -6, 6, frame, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	o := NativeObliqueCavityLineOptions{Line: geom.ToroidLineOptions{ParameterTolerance: 1e-9}}
	origin := [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}
	dir := [3]*big.Rat{r(3, 5), r(0, 1), r(-4, 5)}
	section, e := prepared.PartitionRationalLine(ctx, origin, dir, r(-8, 1), r(8, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	want := []*big.Rat{r(-8, 1), r(-15, 2), r(-4, 1), r(-2, 1), r(2, 1), r(4, 1), r(15, 2), r(8, 1)}
	if len(section.Stations) != len(want) || len(section.Spans) != 7 {
		t.Fatal("complete stations", len(section.Stations), len(section.Spans))
	}
	for i, station := range section.Stations {
		sign, e := station.Parameter.SignPolynomial(ctx, []*big.Rat{new(big.Rat).Neg(want[i]), r(1, 1)}, geom.ExactCurvePlaneEventOptions{})
		if e != nil || sign.Sign != 0 {
			t.Fatal("exact station", i, e)
		}
		if i == 0 || i == 7 {
			if station.Location != geom.BoreOutside || station.Multiplicity != 0 || len(section.StationFaces[i]) != 0 {
				t.Fatal("clip endpoint")
			}
		} else if station.Location != geom.BoreBoundary {
			t.Fatal("boundary station")
		}
	}
	for i, span := range section.Spans {
		loc := geom.BoreOutside
		if i%2 == 1 {
			loc = geom.BoreInside
		}
		if span.Location != loc || span.Start != i || span.End != i+1 || len(section.SpanFaces[i]) != 0 {
			t.Fatal("material gap", i, span)
		}
	}
	pose, e := placement.NewRationalRigid([3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 10)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 5)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 8)}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := pose.TransformPoint(ctx, origin, false, 4096)
	d, _ := pose.TransformVector(ctx, dir, false, 4096)
	world, e := prepared.PartitionPlacedRationalLine(ctx, pose, a, d, r(-8, 1), r(8, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(section.Spans, world.Spans) || !reflect.DeepEqual(section.StationFaces, world.StationFaces) || section.OwnershipOperations != world.OwnershipOperations {
		t.Fatal("world partition")
	}
	for i, station := range world.Stations {
		cmp, e := station.Parameter.Compare(ctx, section.Stations[i].Parameter, geom.ExactCurvePlaneEventOptions{})
		if e != nil || !cmp.Proven || cmp.Order != 0 {
			t.Fatal("world t", e)
		}
	}
	for _, tc := range []struct {
		name   string
		p, d   [3]*big.Rat
		lo, hi *big.Rat
		states []geom.BorePointLocation
	}{
		{"void tangent", [3]*big.Rat{r(17, 5), r(0, 1), r(-16, 5)}, [3]*big.Rat{r(0, 1), r(1, 1), r(0, 1)}, r(-1, 1), r(1, 1), []geom.BorePointLocation{geom.BoreInside, geom.BoreInside}},
		{"exterior tangent", [3]*big.Rat{r(8, 1), r(0, 1), r(0, 1)}, [3]*big.Rat{r(0, 1), r(1, 1), r(0, 1)}, r(-1, 1), r(1, 1), []geom.BorePointLocation{geom.BoreOutside, geom.BoreOutside}},
		{"cap contained", [3]*big.Rat{r(0, 1), r(0, 1), r(6, 1)}, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, r(-1, 1), r(1, 1), []geom.BorePointLocation{geom.BoreBoundary}},
		{"generator", [3]*big.Rat{r(8, 1), r(0, 1), r(0, 1)}, [3]*big.Rat{r(0, 1), r(0, 1), r(1, 1)}, r(-7, 1), r(7, 1), []geom.BorePointLocation{geom.BoreOutside, geom.BoreBoundary, geom.BoreOutside}},
		{"void clip", origin, dir, r(2, 1), r(3, 1), []geom.BorePointLocation{geom.BoreOutside}},
		{"point clip", origin, dir, r(2, 1), r(2, 1), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q, e := prepared.PartitionRationalLine(ctx, tc.p, tc.d, tc.lo, tc.hi, o)
			if e != nil {
				t.Fatal(e)
			}
			if len(q.Spans) != len(tc.states) {
				t.Fatal("span count", q.Spans)
			}
			for i, s := range q.Spans {
				if s.Location != tc.states[i] {
					t.Fatal("location", s)
				}
				if s.Location == geom.BoreBoundary && len(q.SpanFaces[i]) == 0 {
					t.Fatal("missing coincident owner")
				}
			}
			if tc.name == "generator" {
				if !reflect.DeepEqual(q.StationFaces[1], []int{0, 1}) || !reflect.DeepEqual(q.StationFaces[2], []int{0, 2}) {
					t.Fatal("generator cap incidence", q.StationFaces)
				}
			}
			if tc.name == "exterior tangent" || tc.name == "void tangent" {
				if q.Stations[1].Multiplicity != 2 || q.Stations[1].Location != geom.BoreBoundary {
					t.Fatal("tangent event")
				}
			}
		})
	}
	// A sub-float endpoint immediately inside the void remains outside material.
	epsilon := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	q, e := prepared.PartitionRationalLine(ctx, origin, dir, new(big.Rat).Add(r(2, 1), epsilon), r(3, 1), o)
	if e != nil || len(q.Stations) != 2 || q.Stations[0].Location != geom.BoreOutside {
		t.Fatal("subfloat clip", e)
	}
	o.Ownership.MaxOperations = 129
	bad, e := prepared.PartitionRationalLine(ctx, origin, dir, r(-8, 1), r(8, 1), o)
	if e == nil || len(bad.Stations) != 0 {
		t.Fatal("atomic ownership quota")
	}
	o.Ownership = geom.ExactCurvePlaneEventOptions{}
	o.Line.Surface.MaxOperations = 1
	bad, e = prepared.PartitionRationalLine(ctx, origin, dir, r(-8, 1), r(8, 1), o)
	if e == nil || len(bad.Stations) != 0 {
		t.Fatal("atomic source quota")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	o.Line.Surface = geom.SurfacePatchOptions{}
	bad, e = prepared.PartitionRationalLine(canceled, origin, dir, r(-8, 1), r(8, 1), o)
	if e == nil || len(bad.Stations) != 0 {
		t.Fatal("atomic cancellation")
	}
}

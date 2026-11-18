package document

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/csg/poly"
	"plainsolid/geom"
)

func TestPlanarFeatureEdgeHistoryRegeneration(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	state := State{LengthUnit: .001, Features: map[string]Feature{
		"stock":      {Kind: BoxFeature, Max: geom.Vec3{X: 4, Y: 4, Z: 4}},
		"a":          {Kind: BoxFeature, Min: geom.Vec3{X: 1, Y: -1, Z: 3}, Max: geom.Vec3{X: 2, Y: 1, Z: 5}},
		"b":          {Kind: BoxFeature, Min: geom.Vec3{X: -1, Y: -1, Z: 3}, Max: geom.Vec3{X: .5, Y: 1, Z: 5}},
		"cut":        {Kind: BooleanFeature, Input: "stock", Tool: "a", Operation: poly.Difference},
		"second-cut": {Kind: BooleanFeature, Input: "cut", Tool: "b", Operation: poly.Difference},
		"moved":      {Kind: TransformFeature, Input: "second-cut", Transform: geom.Affine{{-2, 0, 0, 10}, {0, 1, 0, 0}, {0, 0, 1, 0}}},
	}}
	ref := PlanarEdgeHistory{Source: FeatureEdge{Feature: "stock", Roles: [2]string{"y-min", "z-max"}}, Target: "moved"}
	d, err := New(state, 2)
	if err != nil {
		t.Fatal(err)
	}
	check := func(s State, width *big.Rat, endpoints map[float64]bool) ResolvedPlanarEdgeHistory {
		t.Helper()
		got, err := s.ResolvePlanarEdgeHistory(ctx, ref)
		if err != nil {
			t.Fatal(err)
		}
		g, err := s.EvaluateFeatures(ctx)
		if err != nil {
			t.Fatal(err)
		}
		shape := g.Planar[ref.Target]
		edges, vertices := shape.Edges(), shape.Vertices()
		total := new(big.Rat)
		seen := map[float64]bool{}
		for _, part := range got.Intervals {
			e := edges[part.Index]
			p, q := vertices[e.Start], vertices[e.End]
			sourceIndex, err := s.sourceBoxEdge(ref.Source)
			if err != nil {
				t.Fatal(err)
			}
			src := g.Planar["stock"]
			orig := src.Edges()[sourceIndex]
			originalVertices := src.Vertices()
			a, b := originalVertices[orig.Start], originalVertices[orig.End]
			for i, station := range []*big.Rat{part.OutputFrom, part.OutputTo} {
				f, _ := station.Float64()
				v := p.Add(q.Sub(p).Scale(f))
				if v.Y != 0 || v.Z != 4 {
					t.Fatal("history jumped to a different source edge", v)
				}
				h, _ := [2]*big.Rat{part.From, part.To}[i].Float64()
				if expected := s.Features["moved"].Transform.Apply(a.Add(b.Sub(a).Scale(h))); v != expected {
					t.Fatal("original source endpoint mapping or orientation changed", v, expected)
				}
				seen[v.X] = true
			}
			total.Add(total, new(big.Rat).Abs(new(big.Rat).Sub(part.To, part.From)))
		}
		if total.Cmp(width) != 0 {
			t.Fatal("original rational interval length", total, width, got)
		}
		for v := range endpoints {
			if !seen[v] {
				t.Fatal("missing endpoint of retained source interval", v, seen)
			}
		}
		return got
	}
	before, _ := d.Snapshot()
	initial := check(before, big.NewRat(5, 8), map[float64]bool{9: true, 8: true, 6: true, 2: true})
	swapped := ref
	swapped.Source.Roles[0], swapped.Source.Roles[1] = swapped.Source.Roles[1], swapped.Source.Roles[0]
	if same, err := before.ResolvePlanarEdgeHistory(ctx, swapped); err != nil || !reflect.DeepEqual(same, initial) {
		t.Fatal("the pair of incident roles is unordered", same, err)
	}
	bad := ref
	bad.Source.Roles = [2]string{"y-min", "y-max"}
	if _, err := before.ResolvePlanarEdgeHistory(ctx, bad); !errors.Is(err, geom.ErrInvalid) {
		t.Fatal("parallel roles cannot name a source edge", err)
	}
	tx := d.Begin()
	feature := tx.State.Features["b"]
	feature.Min.Y, feature.Max.Y = 5, 6
	tx.State.Features["b"] = feature
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	edited, _ := d.Snapshot()
	check(edited, big.NewRat(3, 4), map[float64]bool{10: true, 8: true, 6: true, 2: true})
	if err := d.Undo(); err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := d.Write(&data); err != nil {
		t.Fatal(err)
	}
	reopened, err := Read(bytes.NewReader(data.Bytes()), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	cur, _ := reopened.Snapshot()
	if got, err := cur.ResolvePlanarEdgeHistory(ctx, ref); err != nil || !reflect.DeepEqual(got, initial) {
		t.Fatal("reloaded recipe lost exact edge interval lineage", got, err)
	}
}

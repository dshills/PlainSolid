package trimmed

import (
	"context"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestWrappedFaceRegionTopology(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	refined, _, e := w.InsertKnot(ctx, 0, .125, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a := wrappedOverlapFace(t, w, geom.ParameterRectangle{UMin: 4.125, UMax: 4.375, VMin: -1, VMax: 1})
	b := wrappedOverlapFace(t, refined, geom.ParameterRectangle{UMin: .25, UMax: .5, VMin: 0, VMax: 2})
	b.Face.Reversed = true
	opts := WrappedRectangleFaceOverlapOptions{ProjectionAxes: [2]int{1, 2}}
	for _, selection := range []WrappedFaceRegionSelection{WrappedFaceRegionUnion, WrappedFaceRegionIntersection} {
		r, err := NewWrappedFaceRegionTopology(ctx, a, b, selection, opts)
		if err != nil {
			t.Fatal(err)
		}
		nv, ne, nf, nb := 14, 20, 7, 12
		if selection == WrappedFaceRegionIntersection {
			nv, ne, nf, nb = 4, 4, 1, 4
		}
		if len(r.Vertices()) != nv || len(r.Edges()) != ne || len(r.Faces()) != nf || len(r.Exterior()) != nb || r.SourceOrientations() != [2]bool{false, true} {
			t.Fatal("wrong indexed open region")
		}
		edges, vertices, faces := r.Edges(), r.Vertices(), r.Faces()
		incidence := make([][]bool, ne)
		for _, f := range faces {
			for _, u := range f.Outer {
				incidence[u.Edge] = append(incidence[u.Edge], u.Reversed)
				edge := edges[u.Edge]
				start, end := edge.Start, edge.End
				if u.Reversed {
					start, end = end, start
				}
				for i, uv := range [2]geom.Vec2{u.UVStart, u.UVEnd} {
					jet, err := w.EvaluateRational(ctx, ratInset(uv.X), ratInset(uv.Y), geom.SurfacePatchEvaluationOptions{})
					if err != nil {
						t.Fatal(err)
					}
					point, err := geom.RepresentedExactPoint(jet.Point)
					v := start
					if i == 1 {
						v = end
					}
					if err != nil || point != vertices[v] {
						t.Fatal("use/model incidence")
					}
				}
			}
		}
		for _, uses := range incidence {
			if len(uses) != 1 && (len(uses) != 2 || uses[0] == uses[1]) {
				t.Fatal("nonmanifold edge", uses)
			}
		}
		area := new(big.Rat)
		for _, u := range r.Exterior() {
			area.Add(area, new(big.Rat).Sub(new(big.Rat).Mul(ratInset(u.UVStart.X), ratInset(u.UVEnd.Y)), new(big.Rat).Mul(ratInset(u.UVStart.Y), ratInset(u.UVEnd.X))))
		}
		area.Quo(area, big.NewRat(2, 1))
		want := big.NewRat(7, 8)
		if selection == WrappedFaceRegionIntersection {
			want = big.NewRat(1, 8)
		}
		if area.Cmp(want) != 0 {
			t.Fatal("exterior orientation/area", area)
		}
		again, err := NewWrappedFaceRegionTopology(ctx, a, b, selection, opts)
		if err != nil || !reflect.DeepEqual(r.Vertices(), again.Vertices()) || !reflect.DeepEqual(r.Edges(), again.Edges()) || !reflect.DeepEqual(r.Exterior(), again.Exterior()) || !reflect.DeepEqual(r.EdgeAncestry(), again.EdgeAncestry()) {
			t.Fatal("nondeterministic export", err)
		}
		reversedA := a
		reversedA.Face.Reversed = true
		reverse, err := NewWrappedFaceRegionTopology(ctx, reversedA, b, selection, opts)
		if err != nil {
			t.Fatal(err)
		}
		boundary := map[int]Use{}
		for _, u := range r.Exterior() {
			boundary[u.Edge] = u
		}
		for _, u := range reverse.Exterior() {
			q, ok := boundary[u.Edge]
			if !ok || q.Reversed == u.Reversed || q.UVStart != u.UVEnd || q.UVEnd != u.UVStart {
				t.Fatal("reversed exterior")
			}
		}
		for _, f := range reverse.Faces() {
			if !f.Reversed {
				t.Fatal("lost face orientation")
			}
		}
		// All exported mutable containers and exact owner values are snapshots.
		vertices[0].X += 10
		edges[0].Start = 999
		faces[0].Outer[0].Edge = 999
		exterior := r.Exterior()
		exterior[0].Edge = 999
		cells := r.CellAncestry()
		for owner := 0; owner < 2; owner++ {
			if cells[0].Owners&(1<<owner) != 0 {
				cells[0].OwnerUV[owner][0][0].SetInt64(999)
			}
		}
		ancestry := r.EdgeAncestry()
		for _, list := range ancestry {
			if len(list) > 0 {
				list[0].Parameters[0].SetInt64(999)
				list[0].UV[0][0].SetInt64(999)
				break
			}
		}
		if !reflect.DeepEqual(r.Vertices(), again.Vertices()) || !reflect.DeepEqual(r.Edges(), again.Edges()) || !reflect.DeepEqual(r.Faces(), again.Faces()) || !reflect.DeepEqual(r.Exterior(), again.Exterior()) || !reflect.DeepEqual(r.CellAncestry(), again.CellAncestry()) || !reflect.DeepEqual(r.EdgeAncestry(), again.EdgeAncestry()) {
			t.Fatal("mutable proof storage exposed")
		}
		bad := opts
		bad.MaxOperations = r.ExactOperations() - 1
		if out, err := NewWrappedFaceRegionTopology(ctx, a, b, selection, bad); err == nil || out != nil {
			t.Fatal("partial graph on quota", err)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if out, err := NewWrappedFaceRegionTopology(canceled, a, b, WrappedFaceRegionUnion, opts); err == nil || out != nil {
		t.Fatal("canceled graph", err)
	}
	if out, err := NewWrappedFaceRegionTopology(ctx, a, b, 0, opts); err == nil || out != nil {
		t.Fatal("unknown selection", err)
	}
}

func TestWrappedOpenRegionIncidenceRejectsPinchesAndExtraUses(t *testing.T) {
	ctx := context.Background()
	// Two quadrilateral disks meet at only one vertex. Euler characteristic is
	// still one; an edge-only/Euler check would incorrectly accept this pinch.
	edges := []parametric.Edge{{Start: 0, End: 1}, {Start: 1, End: 2}, {Start: 2, End: 3}, {Start: 3, End: 0}, {Start: 0, End: 4}, {Start: 4, End: 5}, {Start: 5, End: 6}, {Start: 6, End: 0}}
	faces := []Face{{Outer: []Use{{Edge: 0}, {Edge: 1}, {Edge: 2}, {Edge: 3}}}, {Outer: []Use{{Edge: 4}, {Edge: 5}, {Edge: 6}, {Edge: 7}}}}
	if _, e := validateWrappedOpenRegionIncidence(ctx, 7, edges, faces); e == nil {
		t.Fatal("pinched fan accepted")
	}
	if _, e := validateWrappedOpenRegionIncidence(ctx, 4, edges[:4], []Face{faces[0]}); e != nil {
		t.Fatal("single disk rejected", e)
	}
	if _, e := validateWrappedOpenRegionIncidence(ctx, 4, edges[:4], []Face{faces[0], faces[0], faces[0]}); e == nil {
		t.Fatal("three face uses accepted")
	}
	broken := cloneFace(faces[0])
	broken.Outer[1].Reversed = true
	if _, e := validateWrappedOpenRegionIncidence(ctx, 4, edges[:4], []Face{broken}); e == nil {
		t.Fatal("broken directed loop accepted")
	}
}

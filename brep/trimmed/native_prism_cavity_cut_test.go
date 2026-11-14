package trimmed_test

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func TestContainedNativePrismCavityOriginalShells(t *testing.T) {
	stock, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 4})
	tool, _ := brep.Box(geom.Vec3{X: .25, Y: .5, Z: 1}, geom.Vec3{X: 1.25, Y: 1.5, Z: 3})
	q, e := trimmed.CutContainedNativePrismCavity(context.Background(), stock, tool, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if q.ExactVolume().Cmp(big.NewRat(14, 1)) != 0 {
		t.Fatal(q.ExactVolume())
	}
	s := q.Solid()
	if e = s.Validate(); e != nil {
		t.Fatal(e)
	}
	mass, e := s.MassProperties()
	if e != nil || math.Abs(mass.Volume-14) > 1e-12 || math.Abs(mass.Centroid.X-29./28) > 1e-12 || math.Abs(mass.Centroid.Y-1) > 1e-12 || math.Abs(mass.Centroid.Z-2) > 1e-12 {
		t.Fatal(mass, e)
	}
	owners := [2]brep.Solid{stock, tool}
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	if len(vertices) != 16 || len(edges) != 24 || len(faces) != 12 {
		t.Fatal("shell graph cardinality")
	}
	for _, u := range q.Vertices() {
		if vertices[u.Index] != owners[u.Operand].Vertices()[u.Original] || u.Reversed {
			t.Fatal(u)
		}
	}
	for _, u := range q.Edges() {
		e := edges[u.Index]
		orig := owners[u.Operand].Edges()[u.Original]
		source := owners[u.Operand].Vertices()
		if vertices[e.Start] != source[orig.Start] || vertices[e.End] != source[orig.End] || u.Reversed {
			t.Fatal(u)
		}
	}
	for _, u := range q.Faces() {
		orig := owners[u.Operand].Faces()[u.Original].Boundary
		got := faces[u.Index].Boundary
		if len(got) != len(orig) || u.Reversed != (u.Operand == 1) {
			t.Fatal(u)
		}
		for j, v := range got {
			oi := j
			if u.Operand == 1 {
				oi = len(orig) - 1 - j
			}
			ref := orig[oi]
			expected := ref.Reversed
			if u.Operand == 1 {
				expected = !expected
			}
			history := q.Edges()[v.Edge]
			if history.Operand != u.Operand || history.Original != ref.Edge || v.Reversed != expected {
				t.Fatal("original face use", u, j)
			}
		}
	}
	// The two exact graphs remain disconnected. Their signed integrations have
	// opposite signs; strict profile/height containment independently proves nesting.
	toolFaces := tool.Faces()
	for i := range toolFaces {
		loop := toolFaces[i].Boundary
		for j := range loop {
			loop[j].Reversed = !loop[j].Reversed
		}
		for l, r := 0, len(loop)-1; l < r; l, r = l+1, r-1 {
			loop[l], loop[r] = loop[r], loop[l]
		}
	}
	inner, e := brep.NewSolid(tool.Vertices(), tool.Edges(), toolFaces)
	if e != nil {
		t.Fatal(e)
	}
	im, e := inner.MassProperties()
	if e != nil || im.Volume != -2 {
		t.Fatal(im, e)
	}
	rebuilt, e := brep.NewSolid(vertices, edges, faces)
	if e != nil {
		t.Fatal(e)
	}
	rm, _ := rebuilt.MassProperties()
	if rm.Volume != mass.Volume {
		t.Fatal(rm)
	}
	q.ExactVolume().SetInt64(0)
	copyFaces := q.Faces()
	copyFaces[0].Original = 999
	if q.ExactVolume().Sign() <= 0 || q.Faces()[0].Original == 999 {
		t.Fatal("mutable publication")
	}
	for _, limit := range []int{q.NativeOperations(), q.NativeOperations() - 1} {
		out, e := trimmed.CutContainedNativePrismCavity(context.Background(), stock, tool, trimmed.NativePrismBooleanOptions{Native: trimmed.AffineArrangementPrismOptions{MaxOperations: limit}})
		if limit == q.NativeOperations() {
			if e != nil || out == nil {
				t.Fatal(e)
			}
		} else if e == nil || out != nil {
			t.Fatal("partial cavity")
		}
	}
}

func TestContainedNativePrismCavityNonconvexAndReject(t *testing.T) {
	stock, e := brep.Prism([]geom.Vec3{{}, {X: 3}, {X: 3, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 3}, {Y: 3}}, geom.Vec3{Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	tool, e := brep.Prism([]geom.Vec3{{X: .125, Y: .125, Z: 1}, {X: .875, Y: .125, Z: 1}, {X: .125, Y: .875, Z: 1}}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	pose := geom.Affine{{0, 0, -1, 5}, {1, .5, 0, -2}, {0, 1, 0, 1}}
	stock, e = stock.Transformed(pose)
	if e != nil {
		t.Fatal(e)
	}
	tool, e = tool.Transformed(pose)
	if e != nil {
		t.Fatal(e)
	}
	q, e := trimmed.CutContainedNativePrismCavity(context.Background(), stock, tool, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if q.ExactVolume().Cmp(big.NewRat(471, 32)) != 0 {
		t.Fatal(q.ExactVolume())
	}
	m, e := q.Solid().MassProperties()
	if e != nil || math.Abs(m.Volume-471./32) > 1e-11 {
		t.Fatal(m, e)
	}
	box, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 4})
	for _, ends := range [][2]geom.Vec3{
		{{X: .5, Y: .5, Z: 0}, {X: 1.5, Y: 1.5, Z: 3}},
		{{X: .5, Y: .5, Z: 1}, {X: 1.5, Y: 1.5, Z: 4}},
		{{X: 0, Y: .5, Z: 1}, {X: 1, Y: 1.5, Z: 3}},
		{{X: -.5, Y: .5, Z: 1}, {X: 1, Y: 1.5, Z: 3}},
		{{X: .5, Y: .5, Z: -1}, {X: 1.5, Y: 1.5, Z: 5}},
	} {
		bad, _ := brep.Box(ends[0], ends[1])
		if out, e := trimmed.CutContainedNativePrismCavity(context.Background(), box, bad, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
			t.Fatal("non-strict cavity", ends)
		}
	}
	if out, e := trimmed.CutContainedNativePrismCavity(context.Background(), stock, tool, trimmed.NativePrismBooleanOptions{Native: trimmed.AffineArrangementPrismOptions{MaxVertices: 6}}); e == nil || out != nil {
		t.Fatal("limit")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, e := trimmed.CutContainedNativePrismCavity(ctx, stock, tool, trimmed.NativePrismBooleanOptions{}); e != context.Canceled || out != nil {
		t.Fatal(out, e)
	}
}

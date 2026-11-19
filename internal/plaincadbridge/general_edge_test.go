package plaincadbridge

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"plainsolid/brep/blend"
	"plainsolid/brep/boolean"
	"plainsolid/geom"
)

// general publishes, on a default bridge, the result of the general
// Boolean for two stored shapes, without asking a legacy path. PlainCAD
// reaches such a body where a legacy path declines a request.
func general(t *testing.T, b *Bridge, op string, a, c Shape) Shape {
	t.Helper()
	s, err := b.generalBoolean(context.Background(), b.shapes[a.Handle], b.shapes[c.Handle], op)
	if err != nil {
		t.Fatal(err)
	}
	if body := b.shapes[s.Handle].analytic; body == nil || !b.generalResult(body) {
		t.Fatalf("the %s did not publish a general result", op)
	}
	return s
}

// slottedBox is the body of Q1: the box (0,0,0)–(20,10,10) less the box
// (12,−1,6)–(16,11,11), as a general result.
func slottedBox(t *testing.T, b *Bridge) Shape {
	t.Helper()
	s := general(t, b, "cut", box(t, b, vector{}, vector{20, 10, 10}), box(t, b, vector{12, -1, 6}, vector{16, 11, 11}))
	near(t, s.GeometryAssertions.Volume, 1840)
	near(t, s.GeometryAssertions.SurfaceArea, 1048)
	return s
}

func available(t *testing.T, b *Bridge, handle string, start, end vector) bool {
	t.Helper()
	var reply struct{ Available bool }
	if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "edgeAvailable", "handle": handle, "edgeStart": start, "edgeEnd": end}), &reply); err != nil {
		t.Fatal(err)
	}
	return reply.Available
}

// TestGeneralEdgeAvailable asks a general result which references resolve
// (R11).
func TestGeneralEdgeAvailable(t *testing.T) {
	b := New()
	s := slottedBox(t, b)
	for _, c := range []struct {
		name       string
		start, end vector
		want       bool
	}{
		{"a bottom edge", vector{0, 0, 0}, vector{20, 0, 0}, true},
		{"a top edge up to the slot", vector{0, 0, 10}, vector{12, 0, 10}, true},
		{"a top edge across the slot", vector{0, 0, 10}, vector{20, 0, 10}, false},
		{"a line on a face", vector{0, 2, 0}, vector{20, 2, 0}, false},
		{"one point", vector{0, 0, 0}, vector{0, 0, 0}, false},
	} {
		if got := available(t, b, s.Handle, c.start, c.end); got != c.want {
			t.Errorf("%s: available %v, want %v", c.name, got, c.want)
		}
	}
	// A bore that leaves through the front face meets it along straight
	// edges with a cylinder on one side, which take no treatment.
	bored := general(t, b, "cut", box(t, b, vector{}, vector{20, 10, 10}), circle(t, b, vector{5, 0, -1}, vector{0, 0, 1}, 2, 12))
	if available(t, b, bored.Handle, vector{3, 0, 0}, vector{3, 0, 10}) {
		t.Error("an edge between a plane and a cylinder is available")
	}
	if !available(t, b, bored.Handle, vector{0, 10, 0}, vector{20, 10, 0}) {
		t.Error("an untouched edge of the bored box is not available")
	}
}

// lifted publishes a stored shape again as a general result of the same
// shape: its union with the empty solid.
func lifted(t *testing.T, b *Bridge, s Shape) Shape {
	t.Helper()
	r, err := generalCombine(context.Background(), b.shapes[s.Handle], stored{}, boolean.Union)
	if err != nil {
		t.Fatal(err)
	}
	out, err := b.putGeneral(r)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// refused requires that a request fails with the code and the message,
// and that it publishes nothing (D6).
func refused(t *testing.T, b *Bridge, q map[string]any, code, message string) {
	t.Helper()
	count, next := len(b.shapes), b.next
	a := ask(t, b, q)
	if a.OK || a.Error == nil || a.Error.Code != code || a.Error.Message != message {
		t.Fatalf("got ok %v, error %+v; want %s %q", a.OK, a.Error, code, message)
	}
	if len(b.shapes) != count || b.next != next {
		t.Fatal("the refused request published a shape")
	}
}

// The messages of section 9 of the specification.
const (
	lostEdge   = " failed: edge reference was lost or is ambiguous; no complete sharp edge resolves"
	curvedEdge = " failed: this stock takes a treatment only on a straight edge between two planar faces"
	badNormal  = " failed: extrusion normal does not match the selected edge"
	badAxis    = " failed: direction does not match the selected edge"
	sharedEnd  = " failed: grouped edges must not share an end point"
	doesNotFit = " failed: the treatment does not fit; a void, another feature or the far side of the stock lies within its size"
	doesNotEnd = " failed: the treatment does not end on faces of the stock, or reaches under a neighbouring feature"
)

type q = map[string]any

// treat is a fillet or chamfer request for one edge.
func treat(op, handle string, start, end, normal vector, size float64) q {
	out := q{"op": op, "handle": handle, "edgeStart": start, "edgeEnd": end, "normal": normal, "distance": size}
	if op == "fillet" {
		delete(out, "distance")
		out["radius"] = size
	}
	return out
}

// requireGeneral requires that a shape is a general result with the
// volume and the area, in one solid.
func requireGeneral(t *testing.T, b *Bridge, s Shape, volume, area float64) {
	t.Helper()
	if body := b.shapes[s.Handle].analytic; body == nil || !b.generalResult(body) {
		t.Fatal("the treated body is not a general result")
	}
	if !s.GeometryAssertions.Valid || s.GeometryAssertions.SolidCount != 1 {
		t.Fatalf("assertions %+v", s.GeometryAssertions)
	}
	near(t, s.GeometryAssertions.Volume, volume)
	near(t, s.GeometryAssertions.SurfaceArea, area)
}

// holedBox is the body of Q4: the box (0,0,0)–(20,10,10) with through
// holes of radius 2 and 2.5 and a counterbore of radius 3 and depth 1,
// all entering the face z = 0.
func holedBox(t *testing.T, b *Bridge) Shape {
	t.Helper()
	s := box(t, b, vector{}, vector{20, 10, 10})
	s = general(t, b, "cut", s, circle(t, b, vector{5, 5, 0}, vector{0, 0, 1}, 2, 10))
	s = general(t, b, "cut", s, circle(t, b, vector{15, 5, 0}, vector{0, 0, 1}, 2.5, 10))
	s = general(t, b, "cut", s, circle(t, b, vector{5, 5, 0}, vector{0, 0, 1}, 3, 1))
	near(t, s.GeometryAssertions.Volume, 2000-107.5*math.Pi)
	near(t, s.GeometryAssertions.SurfaceArea, 1000+71.5*math.Pi)
	return s
}

// pocketedBox is the body of Q5: the box (0,0,0)–(40,30,10) with a blind
// pocket (15,10,8)–(25,20,10) in its top.
func pocketedBox(t *testing.T, b *Bridge) Shape {
	t.Helper()
	s := general(t, b, "cut", box(t, b, vector{}, vector{40, 30, 10}), box(t, b, vector{15, 10, 8}, vector{25, 20, 11}))
	near(t, s.GeometryAssertions.Volume, 11800)
	near(t, s.GeometryAssertions.SurfaceArea, 3880)
	return s
}

// TestGeneralChamfer chamfers convex edges of general results and
// compares volume and area with the closed forms of section 11.1: an
// edge of length L loses L·d²/2 of volume, 2·L·d of its two faces and
// d²/2 of each end face, and gains L·d·√2.
func TestGeneralChamfer(t *testing.T) {
	b := New()
	chamfered := func(v, a, length, d float64) (float64, float64) {
		return v - length*d*d/2, a + length*d*(math.Sqrt2-2) - d*d
	}
	t.Run("Q1, a slotted box", func(t *testing.T) {
		s := slottedBox(t, b)
		out := shape(t, b, treat("chamfer", s.Handle, vector{0, 0, 0}, vector{20, 0, 0}, vector{0, 0, 1}, 0.5))
		v, a := chamfered(1840, 1048, 20, 0.5)
		requireGeneral(t, b, out, v, a)
		if available(t, b, out.Handle, vector{0, 0, 0}, vector{20, 0, 0}) || !available(t, b, out.Handle, vector{0, 10, 0}, vector{20, 10, 0}) {
			t.Fatal("the treated edge is available, or an untouched one is not")
		}
		// X1: the edge is consumed.
		refused(t, b, treat("chamfer", out.Handle, vector{0, 0, 0}, vector{20, 0, 0}, vector{0, 0, 1}, 0.5), "UNSUPPORTED", "chamfer"+lostEdge)
	})
	t.Run("Q4, holes and a counterbore", func(t *testing.T) {
		s := holedBox(t, b)
		out := shape(t, b, q{"op": "chamfer", "handle": s.Handle, "normal": vector{0, 0, 1}, "distance": 0.5,
			"edges": []any{edgeRef(vector{0, 0, 0}, vector{20, 0, 0})}})
		v, a := chamfered(2000-107.5*math.Pi, 1000+71.5*math.Pi, 20, 0.5)
		requireGeneral(t, b, out, v, a)
	})
	t.Run("Q5, each top edge of a pocketed box", func(t *testing.T) {
		s := pocketedBox(t, b)
		for _, e := range [][2]vector{{{0, 0, 10}, {40, 0, 10}}, {{40, 0, 10}, {40, 30, 10}}, {{40, 30, 10}, {0, 30, 10}}, {{0, 30, 10}, {0, 0, 10}}} {
			out := shape(t, b, treat("chamfer", s.Handle, e[0], e[1], vector{0, 0, 1}, 1))
			length := math.Abs(float64(e[1][0]-e[0][0])) + math.Abs(float64(e[1][1]-e[0][1]))
			v, a := chamfered(11800, 3880, length, 1)
			requireGeneral(t, b, out, v, a)
		}
	})
	t.Run("a box bored by a legacy path and slotted by the fallback", func(t *testing.T) {
		stock := box(t, b, vector{}, vector{20, 10, 10})
		bored := shape(t, b, q{"op": "cut", "a": stock.Handle, "b": circle(t, b, vector{5, 5, 0}, vector{0, 0, 1}, 2, 10).Handle})
		slotted := declined(t, b, "", "cut", bored.Handle, box(t, b, vector{12, -1, 6}, vector{16, 11, 11}).Handle)
		out := shape(t, b, treat("chamfer", slotted.Handle, vector{0, 0, 0}, vector{20, 0, 0}, vector{0, 0, 1}, 0.5))
		v, a := chamfered(slotted.GeometryAssertions.Volume, slotted.GeometryAssertions.SurfaceArea, 20, 0.5)
		requireGeneral(t, b, out, v, a)
	})
	t.Run("a duplicate and a reversed reference are one edge", func(t *testing.T) {
		s := slottedBox(t, b)
		out := shape(t, b, q{"op": "chamfer", "handle": s.Handle, "normal": vector{0, 0, 1}, "distance": 0.5, "edges": []any{
			edgeRef(vector{0, 0, 0}, vector{20, 0, 0}), edgeRef(vector{20, 0, 0}, vector{0, 0, 0}), edgeRef(vector{0, 0, 0}, vector{20, 0, 0})}})
		v, a := chamfered(1840, 1048, 20, 0.5)
		requireGeneral(t, b, out, v, a)
	})
	t.Run("two edges that do not meet", func(t *testing.T) {
		s := slottedBox(t, b)
		out := shape(t, b, q{"op": "chamfer", "handle": s.Handle, "normal": vector{0, 0, 1}, "distance": 0.5, "edges": []any{
			edgeRef(vector{0, 0, 0}, vector{20, 0, 0}), edgeRef(vector{0, 10, 0}, vector{20, 10, 0})}})
		v, a := chamfered(1840, 1048, 40, 0.5)
		requireGeneral(t, b, out, v, a-0.25) // four end triangles, not two
	})
	t.Run("holes that miss the chamfer by a millionth", func(t *testing.T) {
		s := box(t, b, vector{}, vector{20, 10, 10})
		for _, x := range []float64{5, 15} {
			s = general(t, b, "cut", s, circle(t, b, vector{x, 2.000001, 0}, vector{0, 0, 1}, 1, 3))
		}
		out := shape(t, b, treat("chamfer", s.Handle, vector{0, 0, 0}, vector{20, 0, 0}, vector{0, 0, 1}, 1))
		v, a := chamfered(s.GeometryAssertions.Volume, s.GeometryAssertions.SurfaceArea, 20, 1)
		requireGeneral(t, b, out, v, a)
	})
}

// TestGeneralChamferRefused gives a general result the inputs that are
// out of scope (section 9), each with its code and message, and requires
// that nothing is published.
func TestGeneralChamferRefused(t *testing.T) {
	b := New()
	slotted := slottedBox(t, b)
	bottom := func(s Shape, d float64) q {
		return treat("chamfer", s.Handle, vector{0, 0, 0}, vector{20, 0, 0}, vector{0, 0, 1}, d)
	}
	// holes cuts two blind holes of radius 1 whose axes are y from the
	// edge (0,0,0)–(20,0,0).
	holes := func(y float64) Shape {
		s := box(t, b, vector{}, vector{20, 10, 10})
		for _, x := range []float64{5, 15} {
			s = general(t, b, "cut", s, circle(t, b, vector{x, y, 0}, vector{0, 0, 1}, 1, 3))
		}
		return s
	}
	plain := func() Shape { return box(t, b, vector{}, vector{20, 10, 10}) }
	t.Run("X1, half an edge", func(t *testing.T) {
		refused(t, b, treat("chamfer", slotted.Handle, vector{0, 0, 0}, vector{10, 0, 0}, vector{0, 0, 1}, 0.5), "UNSUPPORTED", "chamfer"+lostEdge)
	})
	t.Run("X2, a plane against a cylinder", func(t *testing.T) {
		bored := general(t, b, "cut", plain(), circle(t, b, vector{5, 0, -1}, vector{0, 0, 1}, 2, 12))
		refused(t, b, treat("chamfer", bored.Handle, vector{3, 0, 0}, vector{3, 0, 10}, vector{0, 1, 0}, 0.5), "UNSUPPORTED", "chamfer"+curvedEdge)
	})
	// A cylinder that stands on the top face and touches the top front
	// edge: the Boolean divides the top face, which leaves an edge
	// between coplanar faces and makes the front edge a chain.
	touched := general(t, b, "fuse", plain(), circle(t, b, vector{10, 2, 10}, vector{0, 0, 1}, 2, 3))
	t.Run("X3, between coplanar faces", func(t *testing.T) {
		solid := b.shapes[touched.Handle].analytic.solid
		vertices := solid.Vertices()
		var ends [][2]geom.Vec3
		for _, e := range solid.Edges() {
			if _, err := blend.Locate(solid, vertices[e.Start], vertices[e.End], generalEdgeTolerance); errors.Is(err, blend.ErrParallel) {
				ends = append(ends, [2]geom.Vec3{vertices[e.Start], vertices[e.End]})
			}
		}
		if len(ends) != 1 {
			t.Fatalf("%d edges between coplanar faces", len(ends))
		}
		refused(t, b, treat("chamfer", touched.Handle, coordinates(ends[0][0]), coordinates(ends[0][1]), vector{0, 0, 1}, 0.5), "UNSUPPORTED", "chamfer"+lostEdge)
	})
	t.Run("X9, a chain that a cylinder stands on", func(t *testing.T) {
		if !available(t, b, touched.Handle, vector{0, 0, 10}, vector{20, 0, 10}) {
			t.Fatal("the chain does not resolve")
		}
		refused(t, b, treat("chamfer", touched.Handle, vector{0, 0, 10}, vector{20, 0, 10}, vector{0, 0, 1}, 0.5), "UNSUPPORTED", "chamfer"+doesNotEnd)
	})
	t.Run("X4, a normal of neither face", func(t *testing.T) {
		refused(t, b, treat("chamfer", slotted.Handle, vector{0, 0, 0}, vector{20, 0, 0}, vector{0, 1, 1}, 0.5), "UNSUPPORTED", "chamfer"+badNormal)
	})
	t.Run("X5, a direction across the edge", func(t *testing.T) {
		request := bottom(slotted, 0.5)
		request["direction"] = vector{0, 1, 0}
		refused(t, b, request, "UNSUPPORTED", "chamfer"+badAxis)
	})
	t.Run("X6, two edges that share a vertex", func(t *testing.T) {
		refused(t, b, q{"op": "chamfer", "handle": slotted.Handle, "normal": vector{0, 0, 1}, "distance": 0.5, "edges": []any{
			edgeRef(vector{0, 0, 0}, vector{20, 0, 0}), edgeRef(vector{0, 0, 0}, vector{0, 10, 0})}}, "UNSUPPORTED", "chamfer"+sharedEnd)
	})
	t.Run("X8 by K1, a chamfer larger than the stock", func(t *testing.T) {
		refused(t, b, bottom(slotted, 100), "UNSUPPORTED", "chamfer"+doesNotFit)
	})
	t.Run("X8 by K3, holes that touch the chamfer", func(t *testing.T) {
		refused(t, b, bottom(holes(2), 1), "UNSUPPORTED", "chamfer"+doesNotFit)
	})
	t.Run("X8 by K3, holes that enter the chamfer by a thousandth", func(t *testing.T) {
		refused(t, b, bottom(holes(1.999), 1), "UNSUPPORTED", "chamfer"+doesNotFit)
	})
	t.Run("X8 by K3, a pocket wall inside the chamfer", func(t *testing.T) {
		pocketed := general(t, b, "cut", box(t, b, vector{}, vector{40, 30, 10}), box(t, b, vector{15, 0.5, 8}, vector{25, 10.5, 11}))
		refused(t, b, treat("chamfer", pocketed.Handle, vector{0, 0, 10}, vector{40, 0, 10}, vector{0, 0, 1}, 1), "UNSUPPORTED", "chamfer"+doesNotFit)
	})
	t.Run("X8 by K3, a wall thinner than the chamfer", func(t *testing.T) {
		wall := general(t, b, "cut", plain(), box(t, b, vector{-1, 0.3, -1}, vector{21, 11, 11}))
		refused(t, b, bottom(wall, 1), "UNSUPPORTED", "chamfer"+doesNotFit)
	})
	t.Run("X8 by K4, a closed void inside the chamfer", func(t *testing.T) {
		hollow := general(t, b, "cut", plain(), box(t, b, vector{5, 0.1, 0.1}, vector{6, 0.3, 0.3}))
		refused(t, b, bottom(hollow, 1), "UNSUPPORTED", "chamfer"+doesNotFit)
	})
	t.Run("X9, an edge with oblique ends", func(t *testing.T) {
		var profile []vector
		for k := range 6 {
			a := float64(k) * math.Pi / 3
			profile = append(profile, vector{5 * math.Cos(a), 5 * math.Sin(a), 0})
		}
		hex := lifted(t, b, shape(t, b, q{"op": "prism", "outer": profile, "offset": vector{0, 0, 8}}))
		start, end := profile[0], profile[1]
		start[2], end[2] = 8, 8
		refused(t, b, treat("chamfer", hex.Handle, start, end, vector{0, 0, 1}, 0.5), "UNSUPPORTED", "chamfer"+doesNotEnd)
	})
	t.Run("X9, an edge that ends at a taller block", func(t *testing.T) {
		stepped := general(t, b, "fuse", plain(), box(t, b, vector{20, -5, -5}, vector{25, 15, 15}))
		refused(t, b, bottom(stepped, 0.5), "UNSUPPORTED", "chamfer"+doesNotEnd)
	})
	t.Run("X9, a boss on the flank", func(t *testing.T) {
		bossed := general(t, b, "fuse", plain(), box(t, b, vector{5, 0.2, -3}, vector{8, 2, 0}))
		refused(t, b, bottom(bossed, 1), "UNSUPPORTED", "chamfer"+doesNotEnd)
	})
	t.Run("order: a lost reference before a wrong normal", func(t *testing.T) {
		refused(t, b, treat("chamfer", slotted.Handle, vector{0, 0, 0}, vector{10, 0, 0}, vector{0, 1, 1}, 0.5), "UNSUPPORTED", "chamfer"+lostEdge)
	})
	t.Run("order: a wrong normal before a wrong direction", func(t *testing.T) {
		request := treat("chamfer", slotted.Handle, vector{0, 0, 0}, vector{20, 0, 0}, vector{0, 1, 1}, 0.5)
		request["direction"] = vector{0, 1, 0}
		refused(t, b, request, "UNSUPPORTED", "chamfer"+badNormal)
	})
}

// TestGeneralCapChamfer chamfers the four edges of a cap of a general
// result (C1 to C5): the box of Q6 with a through hole.
func TestGeneralCapChamfer(t *testing.T) {
	b := New()
	s := general(t, b, "cut", box(t, b, vector{}, vector{20, 20, 10}), circle(t, b, vector{10, 10, 0}, vector{0, 0, 1}, 2, 10))
	corners := []vector{{0, 0, 10}, {20, 0, 10}, {20, 20, 10}, {0, 20, 10}}
	perimeter := func(op string, normal vector, corners []vector) q {
		var edges []any
		for i, c := range corners {
			edges = append(edges, edgeRef(c, corners[(i+1)%len(corners)]))
		}
		out := q{"op": op, "handle": s.Handle, "normal": normal, "edges": edges, "distance": 1}
		if op == "capFillet" {
			delete(out, "distance")
			out["radius"] = 1
		}
		return out
	}
	t.Run("Q6", func(t *testing.T) {
		out := shape(t, b, perimeter("capChamfer", vector{0, 0, 1}, corners))
		requireGeneral(t, b, out, 4000-40*math.Pi-(40-4.0/3), 1444+76*math.Sqrt2+32*math.Pi)
		if available(t, b, out.Handle, corners[0], corners[1]) {
			t.Fatal("a chamfered edge is available")
		}
	})
	const lost = "original cap perimeter was lost or is ambiguous"
	t.Run("a fillet around the cap", func(t *testing.T) {
		// Each fillet of radius 1 along 20 removes 20(1 − π/4), and two
		// that meet remove 5/3 − π/2 in common at their corner. A fillet
		// surface is a quarter cylinder cut at both ends by the plane
		// that bisects the corner: its length at the angle φ from the
		// cap is 18 + 2·sin φ, and its area 9π + 2.
		out := shape(t, b, perimeter("capFillet", vector{0, 0, 1}, corners))
		requireGeneral(t, b, out, 4000-40*math.Pi-80*(1-math.Pi/4)+4*(5./3-math.Pi/2), 1452+68*math.Pi)
		if available(t, b, out.Handle, corners[0], corners[1]) {
			t.Fatal("a filleted edge is available")
		}
		// The filleted part meshes, is written to STEP and read back,
		// and takes a bore clear of the fillets and the other cap.
		m, err := b.shapes[out.Handle].tessellate(request{})
		if err != nil {
			t.Fatal(err)
		}
		if err := m.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
		sameShape(t, shape(t, b, q{"op": "importStep", "source": generalSTEPExported(t, b, out.Handle)}), out)
		bored := shape(t, b, q{"op": "cut", "a": out.Handle, "b": circle(t, b, vector{5, 5, 0}, vector{0, 0, 1}, 1, 10).Handle})
		near(t, bored.GeometryAssertions.Volume, out.GeometryAssertions.Volume-10*math.Pi)
		// The other cap of the filleted part takes the same fillet.
		second := perimeter("capFillet", vector{0, 0, -1}, []vector{{0, 0, 0}, {20, 0, 0}, {20, 20, 0}, {0, 20, 0}})
		second["handle"] = out.Handle
		both := shape(t, b, second)
		near(t, both.GeometryAssertions.Volume, out.GeometryAssertions.Volume-80*(1-math.Pi/4)+4*(5./3-math.Pi/2))
	})
	t.Run("X13, a reference that is lost", func(t *testing.T) {
		refused(t, b, perimeter("capChamfer", vector{0, 0, 1}, []vector{{0, 0, 10}, {20, 0, 10}, {20, 20, 10}, {0, 19, 10}}), "UNSUPPORTED", lost)
	})
	t.Run("X13, four edges that do not close", func(t *testing.T) {
		request := perimeter("capChamfer", vector{0, 0, 1}, corners)
		request["edges"] = []any{edgeRef(corners[0], corners[1]), edgeRef(corners[1], corners[2]), edgeRef(corners[2], corners[3]),
			edgeRef(vector{0, 0, 0}, vector{0, 0, 10})}
		refused(t, b, request, "UNSUPPORTED", lost)
	})
	t.Run("X13, the normal of the other cap", func(t *testing.T) {
		refused(t, b, perimeter("capChamfer", vector{0, 0, -1}, corners), "UNSUPPORTED", lost)
	})
	t.Run("X13, one edge twice", func(t *testing.T) {
		request := perimeter("capChamfer", vector{0, 0, 1}, corners)
		request["edges"] = []any{edgeRef(corners[0], corners[1]), edgeRef(corners[1], corners[0]), edgeRef(corners[1], corners[2]), edgeRef(corners[2], corners[3])}
		refused(t, b, request, "UNSUPPORTED", lost)
	})
}

// TestGeneralChamferConcave chamfers a concave edge of a general result,
// and shows that a request of two calls is atomic (D6): when the union
// of the second call is refused, the cut of the first is not published.
func TestGeneralChamferConcave(t *testing.T) {
	b := New()
	// An L: the concave axial edge is at (10,10).
	l := general(t, b, "cut", box(t, b, vector{}, vector{20, 20, 10}), box(t, b, vector{10, 10, 0}, vector{20, 20, 10}))
	near(t, l.GeometryAssertions.Volume, 3000)
	near(t, l.GeometryAssertions.SurfaceArea, 1400)
	axial := func(s Shape, x, y float64) q {
		request := treat("chamfer", s.Handle, vector{x, y, 0}, vector{x, y, 10}, vector{0, 0, 1}, 1)
		request["direction"] = vector{0, 0, 1}
		return request
	}
	out := shape(t, b, axial(l, 10, 10))
	requireGeneral(t, b, out, 3005, 1400-20+10*math.Sqrt2+1)
	// Without a direction the edge is concave all the same (N3).
	out = shape(t, b, treat("chamfer", l.Handle, vector{10, 10, 0}, vector{10, 10, 10}, vector{1, 0, 0}, 1))
	requireGeneral(t, b, out, 3005, 1400-20+10*math.Sqrt2+1)

	// A block that stands on one face of the concave edge, inside the
	// reach of the chamfer, and leaves the edge in place.
	blocked := general(t, b, "fuse", l, box(t, b, vector{10.2, 10, 0}, vector{10.6, 10.6, 10}))
	convex := shape(t, b, axial(blocked, 20, 0))
	near(t, convex.GeometryAssertions.Volume, blocked.GeometryAssertions.Volume-5)
	alone := ask(t, b, axial(blocked, 10, 10))
	if alone.OK || alone.Error == nil || alone.Error.Code != "UNSUPPORTED" {
		t.Fatalf("the blocked concave edge: ok %v, error %+v", alone.OK, alone.Error)
	}
	both := axial(blocked, 20, 0)
	delete(both, "edgeStart")
	delete(both, "edgeEnd")
	both["edges"] = []any{edgeRef(vector{20, 0, 0}, vector{20, 0, 10}), edgeRef(vector{10, 10, 0}, vector{10, 10, 10})}
	refused(t, b, both, "UNSUPPORTED", alone.Error.Message)
	t.Logf("the blocked concave edge is refused with: %s", alone.Error.Message)
}

// TestGeneralFillet fillets edges of general results and compares volume
// and area with the closed forms of section 11.1: at a right angle an
// edge of length L and radius r loses L·r²(1 − π/4) of volume, 2·L·r of
// its two faces and r²(1 − π/4) of each end face, and gains L·r·π/2.
func TestGeneralFillet(t *testing.T) {
	b := New()
	filleted := func(v, a, length, r float64, edges int) (float64, float64) {
		section := r * r * (1 - math.Pi/4)
		return v - length*section, a + length*r*(math.Pi/2-2) - 2*float64(edges)*section
	}
	t.Run("Q2, a slotted box", func(t *testing.T) {
		s := slottedBox(t, b)
		out := shape(t, b, treat("fillet", s.Handle, vector{0, 0, 0}, vector{20, 0, 0}, vector{0, 0, 1}, 2))
		v, a := filleted(1840, 1048, 20, 2, 1)
		requireGeneral(t, b, out, v, a)
		// F4: the fillet meets its two faces along smooth edges, which
		// take no treatment; the treated edge is gone.
		for _, e := range [][2]vector{{{0, 0, 0}, {20, 0, 0}}, {{0, 2, 0}, {20, 2, 0}}, {{0, 0, 2}, {20, 0, 2}}} {
			if available(t, b, out.Handle, e[0], e[1]) {
				t.Fatalf("the edge %v is available on the filleted body", e)
			}
		}
		if !available(t, b, out.Handle, vector{0, 10, 0}, vector{20, 10, 0}) {
			t.Fatal("an untouched edge is not available")
		}
	})
	t.Run("Q3, two parallel edges", func(t *testing.T) {
		s := slottedBox(t, b)
		out := shape(t, b, q{"op": "fillet", "handle": s.Handle, "normal": vector{0, 0, 1}, "radius": 2, "edges": []any{
			edgeRef(vector{0, 0, 0}, vector{20, 0, 0}), edgeRef(vector{0, 10, 0}, vector{20, 10, 0})}})
		v, a := filleted(1840, 1048, 40, 2, 2)
		requireGeneral(t, b, out, v, a)
	})
	t.Run("Q5, each top edge of a pocketed box", func(t *testing.T) {
		s := pocketedBox(t, b)
		for _, e := range [][2]vector{{{0, 0, 10}, {40, 0, 10}}, {{40, 0, 10}, {40, 30, 10}}, {{40, 30, 10}, {0, 30, 10}}, {{0, 30, 10}, {0, 0, 10}}} {
			out := shape(t, b, treat("fillet", s.Handle, e[0], e[1], vector{0, 0, 1}, 1))
			length := math.Abs(float64(e[1][0]-e[0][0])) + math.Abs(float64(e[1][1]-e[0][1]))
			v, a := filleted(11800, 3880, length, 1, 1)
			requireGeneral(t, b, out, v, a)
		}
	})
	t.Run("a box bored by a legacy path and slotted by the fallback", func(t *testing.T) {
		stock := box(t, b, vector{}, vector{20, 10, 10})
		bored := shape(t, b, q{"op": "cut", "a": stock.Handle, "b": circle(t, b, vector{5, 5, 0}, vector{0, 0, 1}, 2, 10).Handle})
		slotted := declined(t, b, "", "cut", bored.Handle, box(t, b, vector{12, -1, 6}, vector{16, 11, 11}).Handle)
		out := shape(t, b, treat("fillet", slotted.Handle, vector{0, 0, 0}, vector{20, 0, 0}, vector{0, 0, 1}, 1))
		v, a := filleted(slotted.GeometryAssertions.Volume, slotted.GeometryAssertions.SurfaceArea, 20, 1, 1)
		requireGeneral(t, b, out, v, a)
	})
	t.Run("Q10, a body that is not axis-aligned", func(t *testing.T) {
		// A turn of 0.7 about the direction (1,2,3), and a shift.
		x, y, z := 1/math.Sqrt(14), 2/math.Sqrt(14), 3/math.Sqrt(14)
		c, s := math.Cos(0.7), math.Sin(0.7)
		m := [][]float64{
			{c + x*x*(1-c), x*y*(1-c) - z*s, x*z*(1-c) + y*s, 3},
			{y*x*(1-c) + z*s, c + y*y*(1-c), y*z*(1-c) - x*s, -2},
			{z*x*(1-c) - y*s, z*y*(1-c) + x*s, c + z*z*(1-c), 5},
		}
		moved := func(p vector) vector {
			var out vector
			for i, row := range m {
				out[i] = row[0]*p[0] + row[1]*p[1] + row[2]*p[2] + row[3]
			}
			return out
		}
		turned := shape(t, b, q{"op": "transform", "handle": slottedBox(t, b).Handle, "matrix": m})
		normal := moved(vector{0, 0, 1})
		for i := range normal {
			normal[i] -= m[i][3]
		}
		out := shape(t, b, treat("fillet", turned.Handle, moved(vector{0, 0, 0}), moved(vector{20, 0, 0}), normal, 2))
		v, a := filleted(1840, 1048, 20, 2, 1)
		requireGeneral(t, b, out, v, a)
	})
	t.Run("eight edges in one request", func(t *testing.T) {
		// The four edges of the pocketed box along x, and the four
		// concave edges at the corners of its pocket.
		s := pocketedBox(t, b)
		var edges []any
		for _, p := range []vector{{0, 0, 0}, {0, 30, 0}, {0, 0, 10}, {0, 30, 10}} {
			edges = append(edges, edgeRef(p, vector{40, p[1], p[2]}))
		}
		for _, p := range []vector{{15, 10, 8}, {25, 10, 8}, {25, 20, 8}, {15, 20, 8}} {
			edges = append(edges, edgeRef(p, vector{p[0], p[1], 10}))
		}
		start := time.Now()
		out := shape(t, b, q{"op": "fillet", "handle": s.Handle, "normal": vector{0, 0, 1}, "radius": 0.5, "edges": edges})
		t.Logf("eight fillets took %v", time.Since(start))
		section := 0.25 * (1 - math.Pi/4)
		near(t, out.GeometryAssertions.Volume, 11800-4*40*section+4*2*section)
		if out.GeometryAssertions.SolidCount != 1 {
			t.Fatalf("%d solids", out.GeometryAssertions.SolidCount)
		}
	})
}

// TestGeneralFilletRefused gives a general result the fillets that are
// out of scope.
func TestGeneralFilletRefused(t *testing.T) {
	b := New()
	slotted := slottedBox(t, b)
	bottom := func(s Shape, r float64) q {
		return treat("fillet", s.Handle, vector{0, 0, 0}, vector{20, 0, 0}, vector{0, 0, 1}, r)
	}
	t.Run("X7, a radius below the working tolerance", func(t *testing.T) {
		refused(t, b, bottom(slotted, 5e-8), "UNSUPPORTED", "fillet failed: the treatment size does not fit the selected edge")
	})
	t.Run("X8 by K3, a fillet as wide as its face", func(t *testing.T) {
		refused(t, b, bottom(box10(t, b), 10), "UNSUPPORTED", "fillet"+doesNotFit)
	})
	t.Run("X1, the same fillet twice", func(t *testing.T) {
		out := shape(t, b, bottom(slotted, 1))
		refused(t, b, bottom(out, 1), "UNSUPPORTED", "fillet"+lostEdge)
	})
	t.Run("X11, a bore across the fillet", func(t *testing.T) {
		// The bore opens in the bottom face inside the reach of the
		// fillet and leaves the edge whole. Its wall and the fillet are
		// cylinders with crossing axes.
		bored := general(t, b, "cut", box(t, b, vector{}, vector{20, 10, 10}), circle(t, b, vector{9, 1, 0}, vector{0, 0, 1}, 0.5, 10))
		count, next := len(b.shapes), b.next
		a := ask(t, b, bottom(bored, 2))
		if a.OK || a.Error == nil || a.Error.Code != "UNSUPPORTED" || !strings.HasPrefix(a.Error.Message, "fillet failed: boolean: unsupported input") {
			t.Fatalf("ok %v, error %+v", a.OK, a.Error)
		}
		if len(b.shapes) != count || b.next != next {
			t.Fatal("the refused request published a shape")
		}
	})
}

// box10 is the box (0,0,0)–(20,10,10) as a general result.
func box10(t *testing.T, b *Bridge) Shape {
	t.Helper()
	return lifted(t, b, box(t, b, vector{}, vector{20, 10, 10}))
}

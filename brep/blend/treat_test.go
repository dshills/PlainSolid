package blend

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/boolean"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

var treatOptions = Options{Boolean: testOptions, Tolerance: testTolerance}

func near(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-8*math.Max(1, math.Abs(want)) {
		t.Fatalf("got %.15g, want %.15g", got, want)
	}
}

func located(t *testing.T, s trimmed.Solid, start, end geom.Vec3) Edge {
	t.Helper()
	e, err := Locate(s, start, end, testTolerance)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// measured is a solid with its volume and area, which the Boolean gives
// for the union of the solid with nothing.
func measured(t *testing.T, s trimmed.Solid) (Body, float64, float64) {
	t.Helper()
	r := combined(t, s, trimmed.Solid{}, boolean.Union)
	return Body{Solid: s, Solids: r.Solids}, r.Properties.Volume, r.Properties.Area
}

// form gives the closed form of section 11.1 for one edge: the area of
// the section of its tool, the width of the new face, and the setback
// along each old face.
func form(e Edge, kind Kind, size float64) (section, width, setback float64) {
	if kind == Fillet {
		setback = size / math.Tan(e.angle/2)
		return size*setback - size*size*(math.Pi-e.angle)/2, size * (math.Pi - e.angle), setback
	}
	return size * size * math.Sin(e.angle) / 2, 2 * size * math.Sin(e.angle/2), size
}

// requireForm treats edges that do not meet and requires the volume and
// the area of section 11.1.
func requireForm(t *testing.T, s trimmed.Solid, kind Kind, size float64, edges ...Edge) boolean.Result {
	t.Helper()
	body, volume, area := measured(t, s)
	r, err := Treat(context.Background(), body, edges, kind, size, treatOptions)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range edges {
		section, width, setback := form(e, kind, size)
		length := e.end.Sub(e.start).Length()
		sign := 1.0
		if !e.convex {
			sign = -1
		}
		volume -= sign * length * section
		area += length*(width-2*setback) - sign*2*section
	}
	near(t, r.Properties.Volume, volume)
	near(t, r.Properties.Area, area)
	if r.Solids != 1 {
		t.Fatalf("%d solids", r.Solids)
	}
	return r
}

// splitBox is the box (0,0,0)–(20,10,10) with the edge (0,0,0)–(20,0,0)
// stored as two edges that meet at (8,0,0).
func splitBox(t *testing.T) trimmed.Solid {
	t.Helper()
	whole, err := brep.Box(vec(0, 0, 0), vec(20, 10, 10))
	if err != nil {
		t.Fatal(err)
	}
	vertices, edges, faces := whole.Vertices(), whole.Edges(), whole.Faces()
	target := -1
	for i, e := range edges {
		a, b := vertices[e.Start], vertices[e.End]
		if a.Y == 0 && a.Z == 0 && b.Y == 0 && b.Z == 0 {
			target = i
		}
	}
	if target < 0 {
		t.Fatal("the box has no such edge")
	}
	middle, second := len(vertices), len(edges)
	vertices = append(vertices, vec(8, 0, 0))
	if vertices[edges[target].Start].X != 0 {
		vertices[middle] = vec(12, 0, 0)
	}
	edges = append(edges, brep.Edge{Start: middle, End: edges[target].End})
	edges[target].End = middle
	for f, face := range faces {
		var loop []brep.EdgeUse
		for _, u := range face.Boundary {
			switch {
			case u.Edge != target:
				loop = append(loop, u)
			case u.Reversed:
				loop = append(loop, brep.EdgeUse{Edge: second, Reversed: true}, u)
			default:
				loop = append(loop, u, brep.EdgeUse{Edge: second})
			}
		}
		faces[f].Boundary = loop
	}
	s, err := brep.NewSolid(vertices, edges, faces)
	return lifted(t, s, err)
}

// TestChamferClosedForms chamfers edges below the bridge and compares
// with the closed forms.
func TestChamferClosedForms(t *testing.T) {
	t.Run("Q7, an edge of 120 degrees", func(t *testing.T) {
		s := hexPrism(t)
		requireForm(t, s, Chamfer, 1, located(t, s, vec(5, 0, 0), vec(5, 0, 8)))
	})
	// The values the legacy path's test states for Q8: the two corner
	// cuts remove triangles of area 15/512 and 25/512.
	q8 := func(t *testing.T, s trimmed.Solid, acute, right [2]geom.Vec3) {
		t.Helper()
		r := requireForm(t, s, Chamfer, 5.0/16, located(t, s, acute[0], acute[1]), located(t, s, right[0], right[1]))
		area := 6 - 40.0/512
		perimeter := 12 - 20.0/16 + (math.Sqrt(10)+5*math.Sqrt2)/16
		near(t, r.Properties.Volume, 5*area)
		near(t, r.Properties.Area, 2*area+5*perimeter)
	}
	t.Run("Q8, two edges of a triangular prism", func(t *testing.T) {
		s := trianglePrism(t)
		q8(t, s, [2]geom.Vec3{vec(4, 0, 0), vec(4, 0, 5)}, [2]geom.Vec3{vec(0, 0, 0), vec(0, 0, 5)})
	})
	t.Run("Q8 in the pose of the legacy test along x", func(t *testing.T) {
		// The profile point (x, y) at height z is (z + 2, x − 3, y + 4).
		tri, err := brep.Prism([]geom.Vec3{vec(2, -3, 4), vec(2, 1, 4), vec(2, -3, 7)}, vec(5, 0, 0))
		s := lifted(t, tri, err)
		q8(t, s, [2]geom.Vec3{vec(2, 1, 4), vec(7, 1, 4)}, [2]geom.Vec3{vec(2, -3, 4), vec(7, -3, 4)})
	})
	t.Run("a chain of two edges", func(t *testing.T) {
		s := splitBox(t)
		e := located(t, s, vec(0, 0, 0), vec(20, 0, 0))
		if len(e.chain) != 2 {
			t.Fatalf("a chain of %d edges", len(e.chain))
		}
		r := requireForm(t, s, Chamfer, 0.5, e)
		near(t, r.Properties.Volume, 1997.5)
	})
	t.Run("a result of the Boolean", func(t *testing.T) {
		s := slottedBox(t)
		requireForm(t, s, Chamfer, 0.5, located(t, s, vec(0, 0, 0), vec(20, 0, 0)))
	})
}

// TestTreatGuards exercises the checks that the requests of the bridge
// do not reach alone: K1, K5, K6 and K7, and the order of K2 and K3.
func TestTreatGuards(t *testing.T) {
	ctx := context.Background()
	s := slottedBox(t)
	e := located(t, s, vec(0, 0, 0), vec(20, 0, 0))
	wedge, err := e.tool(ctx, 0, Chamfer, 0.5, treatOptions)
	if err != nil {
		t.Fatal(err)
	}
	good := combined(t, s, wedge.solid, boolean.Cut)
	edges := []Edge{e}
	if err := check(good, 1, wedge, edges, []int{0}, false, treatOptions); err != nil {
		t.Fatalf("a sound chamfer fails a check: %v", err)
	}
	t.Run("K1, nothing is left", func(t *testing.T) {
		empty := good
		empty.Solids = 0
		if err := check(empty, 1, wedge, edges, []int{0}, false, treatOptions); !errors.Is(err, ErrFit) {
			t.Fatal(err)
		}
	})
	t.Run("K5, the solids are not as many", func(t *testing.T) {
		if err := check(good, 2, wedge, edges, []int{0}, false, treatOptions); !errors.Is(err, ErrDivides) {
			t.Fatal(err)
		}
	})
	t.Run("K6, an edge left no face", func(t *testing.T) {
		if err := check(good, 1, wedge, []Edge{e, e}, []int{0, 1}, false, treatOptions); !errors.Is(err, ErrDivides) {
			t.Fatal(err)
		}
	})
	t.Run("K7, an edge is still there", func(t *testing.T) {
		if err := gone(good.Solid, edges, testTolerance); err != nil {
			t.Fatalf("the treated edge still resolves: %v", err)
		}
		other := located(t, s, vec(0, 10, 0), vec(20, 10, 0))
		if err := gone(good.Solid, []Edge{other}, testTolerance); !errors.Is(err, ErrDivides) {
			t.Fatal(err)
		}
	})
	t.Run("K2 before K3", func(t *testing.T) {
		// Half of an edge, beside a hole that the chamfer touches: the
		// tool ends in the material, and a foreign face touches the
		// new face. The first is the answer.
		holed := combined(t, boxSolid(t, vec(0, 0, 0), vec(20, 10, 10)), cylinderSolid(t, vec(5, 2, 0), vec(0, 0, 1), 1, 3), boolean.Cut).Solid
		half := located(t, holed, vec(0, 0, 0), vec(20, 0, 0))
		half.end = vec(10, 0, 0)
		if _, err := Treat(ctx, Body{Solid: holed, Solids: 1}, []Edge{half}, Chamfer, 1, treatOptions); !errors.Is(err, ErrEnds) {
			t.Fatal(err)
		}
	})
	t.Run("a size below the point tolerance", func(t *testing.T) {
		if _, err := Treat(ctx, Body{Solid: s, Solids: 1}, edges, Chamfer, 5e-8, treatOptions); !errors.Is(err, ErrSize) {
			t.Fatal(err)
		}
	})
}

// TestChamferConcave chamfers the concave axial edge of the L prism of
// Q9, alone and with a convex edge in one request: a concave edge gains
// what a convex edge loses.
func TestChamferConcave(t *testing.T) {
	s := lPrism(t)
	concave := located(t, s, vec(10, 10, 0), vec(10, 10, 10))
	convex := located(t, s, vec(20, 10, 0), vec(20, 10, 10))
	t.Run("the concave edge alone", func(t *testing.T) {
		r := requireForm(t, s, Chamfer, 1, concave)
		near(t, r.Properties.Volume, 3005)
	})
	t.Run("half of the concave edge", func(t *testing.T) {
		// The tool ends in mid air, where no end face takes its cap
		// (K2 in a union).
		half := concave
		half.end = vec(10, 10, 5)
		if _, err := Treat(context.Background(), Body{Solid: s, Solids: 1}, []Edge{half}, Chamfer, 1, treatOptions); !errors.Is(err, ErrEnds) {
			t.Fatal(err)
		}
	})
	t.Run("a concave and a convex edge", func(t *testing.T) {
		// The values the legacy path's test states for this request.
		for _, edges := range [][]Edge{{convex, concave}, {concave, convex}} {
			r := requireForm(t, s, Chamfer, 1, edges...)
			near(t, r.Properties.Volume, 3000)
			near(t, r.Properties.Area, 1400-20*(2-math.Sqrt2))
		}
	})
}

// TestFilletClosedForms fillets edges below the bridge and compares with
// the closed forms.
func TestFilletClosedForms(t *testing.T) {
	t.Run("Q7, an edge of 120 degrees", func(t *testing.T) {
		s := hexPrism(t)
		requireForm(t, s, Fillet, 1.5, located(t, s, vec(5, 0, 0), vec(5, 0, 8)))
	})
	t.Run("Q8, the acute edge of a triangular prism", func(t *testing.T) {
		s := trianglePrism(t)
		r := requireForm(t, s, Fillet, 5.0/16, located(t, s, vec(4, 0, 0), vec(4, 0, 5)))
		// The values the legacy path's test states for this request.
		radius, w := 5.0/16, math.Atan2(3, -4)
		area := 6 - radius*radius*(3-w/2)
		perimeter := 12 - 6*radius + radius*w
		near(t, r.Properties.Volume, 5*area)
		near(t, r.Properties.Area, 2*area+5*perimeter)
	})
	t.Run("Q9, the concave edge of an L prism", func(t *testing.T) {
		s := lPrism(t)
		concave := located(t, s, vec(10, 10, 0), vec(10, 10, 10))
		convex := located(t, s, vec(20, 10, 0), vec(20, 10, 10))
		r := requireForm(t, s, Fillet, 1, concave)
		near(t, r.Properties.Volume, 3000+10*(1-math.Pi/4))
		// The values the legacy path's test states for both edges.
		r = requireForm(t, s, Fillet, 1, convex, concave)
		near(t, r.Properties.Volume, 3000)
		near(t, r.Properties.Area, 1400-20*(2-math.Pi/2))
	})
	t.Run("a result of the Boolean", func(t *testing.T) {
		s := slottedBox(t)
		requireForm(t, s, Fillet, 2, located(t, s, vec(0, 0, 0), vec(20, 0, 0)))
	})
	t.Run("three edges, in at most seven calls (L1)", func(t *testing.T) {
		s := boxSolid(t, vec(0, 0, 0), vec(20, 10, 10))
		edges := []Edge{located(t, s, vec(0, 0, 0), vec(20, 0, 0)), located(t, s, vec(0, 10, 0), vec(20, 10, 0)), located(t, s, vec(0, 10, 10), vec(20, 10, 10))}
		calls := 0
		counted := treatOptions
		counted.calls = &calls
		r, err := Treat(context.Background(), Body{Solid: s, Solids: 1}, edges, Fillet, 1, counted)
		if err != nil {
			t.Fatal(err)
		}
		near(t, r.Properties.Volume, 2000-60*(1-math.Pi/4))
		if calls > 2*len(edges)+1 {
			t.Fatalf("%d calls of boolean.Combine for %d edges", calls, len(edges))
		}
	})
	t.Run("two fillets that meet at a corner", func(t *testing.T) {
		// Two cylinders of one radius with crossing axes, which the
		// Boolean takes since specs/general-boolean-crossing. Each fillet
		// of radius r along a length l removes (1 − π/4)·r²·l, and what
		// both remove at the corner is (5/3 − π/2)·r³.
		s := boxSolid(t, vec(0, 0, 0), vec(20, 10, 10))
		edges := []Edge{located(t, s, vec(0, 0, 0), vec(20, 0, 0)), located(t, s, vec(0, 0, 0), vec(0, 10, 0))}
		r, err := Treat(context.Background(), Body{Solid: s, Solids: 1}, edges, Fillet, 1, treatOptions)
		if err != nil {
			t.Fatal(err)
		}
		near(t, r.Properties.Volume, 2000-30*(1-math.Pi/4)+(5./3-math.Pi/2))
	})
}

// TestExtremeAngles treats an edge of 5 degrees and an edge of 175
// degrees. Each tool fits its prism with room to spare, so each must
// meet its closed form.
func TestExtremeAngles(t *testing.T) {
	rise := 10 * math.Tan(5*math.Pi/180)
	sharp := prismSolid(t, 10, [2]float64{0, 0}, [2]float64{100, 0}, [2]float64{100, 10 * rise})
	flat := prismSolid(t, 10, [2]float64{0, 0}, [2]float64{10, 0}, [2]float64{20, rise}, [2]float64{20, 10}, [2]float64{0, 10})
	for _, c := range []struct {
		name  string
		solid trimmed.Solid
		x     float64
		angle float64
	}{{"5 degrees", sharp, 0, 5}, {"175 degrees", flat, 10, 175}} {
		e := located(t, c.solid, vec(c.x, 0, 0), vec(c.x, 0, 10))
		if math.Abs(e.angle-c.angle*math.Pi/180) > 1e-12 || !e.convex {
			t.Fatalf("%s: angle %v, convex %v", c.name, e.angle*180/math.Pi, e.convex)
		}
		t.Run(c.name+", chamfer", func(t *testing.T) { requireForm(t, c.solid, Chamfer, 1, e) })
		t.Run(c.name+", fillet", func(t *testing.T) { requireForm(t, c.solid, Fillet, 0.5, e) })
	}
}

package boolean

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// matrixOptions are the options every leaf of the matrix runs with. The
// integration tolerance is the bridge's (L4). Its sample budget is raised
// above the default of four million, which the integration of an oblique
// section of a cylinder exhausts at that tolerance.
var matrixOptions = Options{Linear: 1e-7, Angular: 1e-9, Volume: 1e-9, Integration: parametric.IntegrationOptions{RelativeTolerance: 1e-9, MaxSamples: 64000000}}

var operationNames = [...]string{Union: "union", Common: "common", Cut: "cut"}

// outcome is what section 11 states for one operation of a row.
type outcome struct {
	open    bool    // section 11 states no outcome: the leaf checks nothing
	fails   error   // the error Combine must return
	maybe   error   // an error Combine may return in place of a validated result (T8)
	volume  float64 // analytic volume; zero for the empty solid
	solids  int
	voids   int
	faces   int                            // zero when section 11 states no count
	sources func(r Result, c config) error // a fixed face source list, when the row has one
}

// config is one pair of operands of a row with its three outcomes.
type config struct {
	a, b   trimmed.Solid
	planar [2]brep.Solid // the same operands for csg/poly
	want   [3]outcome
}

func solidOf(volume float64, solids, voids, faces int) outcome {
	return outcome{volume: volume, solids: solids, voids: voids, faces: faces}
}

var (
	nothing     = outcome{}
	nonManifold = outcome{fails: ErrNonManifold}
)

// matrixEps is the working tolerance of the matrix, for the rows that
// are stated in terms of it.
const matrixEps = 1e-7

// matrix lists the rows of section 11 that are implemented, by variant.
// In every planar row A is the box [0,2]³ unless the row says otherwise.
var matrix = []struct {
	row, variant string
	build        func(t *testing.T) []config
}{
	{"C1", "planar", func(t *testing.T) []config {
		return pair(t, box(t, 5, 0, 0, 6, 1, 1), solidOf(9, 2, 0, 12), nothing, solidOf(8, 1, 0, 6))
	}},
	{"C2", "planar", func(t *testing.T) []config {
		return pair(t, box(t, 0, 0, 0, 2, 2, 2), solidOf(8, 1, 0, 6), solidOf(8, 1, 0, 6), nothing)
	}},
	{"C3", "planar", func(t *testing.T) []config {
		return pair(t, box(t, 0.5, 0.5, 0.5, 1.5, 1.5, 1.5), solidOf(8, 1, 0, 6), solidOf(1, 1, 0, 6), solidOf(7, 1, 1, 12))
	}},
	{"C4", "planar", func(t *testing.T) []config {
		union := solidOf(12, 1, 0, 6)
		union.sources = func(r Result, c config) error {
			// Bottom from A, top from B, and four sides from one face of
			// each (merged by M1).
			return sourceShapes(r, map[string]int{"A at z=0": 1, "B at z=3": 1, "AB": 4})
		}
		return pair(t, box(t, 0, 0, 2, 2, 2, 3), union, nothing, solidOf(8, 1, 0, 6))
	}},
	{"C5", "planar", func(t *testing.T) []config {
		return pair(t, box(t, 1, 0, 2, 3, 2, 3), solidOf(12, 1, 0, 0), nothing, solidOf(8, 1, 0, 6))
	}},
	{"C6", "planar", func(t *testing.T) []config {
		common := solidOf(1, 1, 0, 6)
		common.sources = func(r Result, c config) error {
			// The three faces on the planes shared with A come from a
			// face of each; the other three from B alone.
			return sourceShapes(r, map[string]int{"AB": 3, "B": 3})
		}
		return pair(t, box(t, 0, 0, 0, 1, 1, 1), solidOf(8, 1, 0, 6), common, solidOf(7, 1, 0, 9))
	}},
	{"C7", "planar", func(t *testing.T) []config {
		return pair(t, box(t, 2, 0, 2, 3, 2, 3), nonManifold, nothing, solidOf(8, 1, 0, 6))
	}},
	{"C8", "planar", func(t *testing.T) []config {
		return pair(t, box(t, 2, 2, 2, 3, 3, 3), nonManifold, nothing, solidOf(8, 1, 0, 6))
	}},
	{"C9", "planar", func(t *testing.T) []config {
		// A unit box turned 45° about the y axis, resting on its edge
		// x = 1, z = 2 inside the top face of A.
		turn, err := geom.Rotation(geom.Vec3{Y: 1}, -math.Pi/4)
		if err != nil {
			t.Fatal(err)
		}
		return pair(t, moved(t, box(t, 0, 0.5, 0, 1, 1.5, 1), geom.Translation(geom.Vec3{X: 1, Z: 2}).Mul(turn)), nonManifold, nothing, solidOf(8, 1, 0, 6))
	}},
	{"C10", "planar", func(t *testing.T) []config {
		// A box of side 0.75 standing on one corner at (1,1,2), inside
		// the top face of A. Its three edges there rise along
		// (2,2,1)/4, (-2,1,2)/4 and (1,-2,2)/4, so that every corner has
		// coordinates csg/poly can represent exactly.
		tilt := geom.Affine{{0.5, -0.5, 0.25, 1}, {0.5, 0.25, -0.5, 1}, {0.25, 0.5, 0.5, 2}}
		return pair(t, moved(t, box(t, 0, 0, 0, 1, 1, 1), tilt), nonManifold, nothing, solidOf(8, 1, 0, 6))
	}},
	{"C24", "planar", func(t *testing.T) []config {
		// C4 with B moved away from A by ε/2: as C4 within β.
		d := matrixEps / 2
		return pair(t, box(t, 0, 0, 2+d, 2, 2, 3+d), solidOf(12, 1, 0, 6), nothing, solidOf(8, 1, 0, 6))
	}},
	{"C25", "planar", func(t *testing.T) []config {
		// C4 with B moved into A by 100ε: a real overlap.
		d := 100 * matrixEps
		return pair(t, box(t, 0, 0, 2-d, 2, 2, 3-d), solidOf(12-4*d, 1, 0, 6), solidOf(4*d, 1, 0, 6), solidOf(8-4*d, 1, 0, 6))
	}},
	{"C26", "planar", func(t *testing.T) []config {
		// C4 with B moved into A by 2ε: tolerance-scale. ErrTolerance
		// or a result that validated; a result is as C4 within β.
		d := 2 * matrixEps
		either := func(o outcome) outcome { o.maybe = ErrTolerance; return o }
		return pair(t, box(t, 0, 0, 2-d, 2, 2, 3-d), either(solidOf(12, 1, 0, 0)), either(nothing), either(solidOf(8, 1, 0, 0)))
	}},
	{"C1", "cyl", func(t *testing.T) []config {
		return curved(tube(t, 0, 0, 0, 1, 2), tube(t, 5, 0, 0, 0.5, 1), solidOf(2.25*math.Pi, 2, 0, 6), nothing, solidOf(2*math.Pi, 1, 0, 3))
	}},
	{"C2", "cyl", func(t *testing.T) []config {
		whole := solidOf(2*math.Pi, 1, 0, 3)
		return curved(tube(t, 0, 0, 0, 1, 2), tube(t, 0, 0, 0, 1, 2), whole, whole, nothing)
	}},
	{"C3", "cyl", func(t *testing.T) []config {
		return curved(tube(t, 0, 0, 0, 1, 2), tube(t, 0.2, 0, 0.5, 0.5, 1), solidOf(2*math.Pi, 1, 0, 3), solidOf(0.25*math.Pi, 1, 0, 3), solidOf(1.75*math.Pi, 1, 1, 6))
	}},
	{"C11", "cyl", func(t *testing.T) []config {
		// A cylinder outside A, touching the face x = 2 along a line.
		return curved(cube(t), tube(t, 2.5, 1, 0.5, 0.5, 1), nonManifold, nothing, solidOf(8, 1, 0, 6))
	}},
	{"C12", "cyl", func(t *testing.T) []config {
		// A cylinder inside A, touching the face x = 0 along a line.
		return curved(cube(t), tube(t, 0.5, 1, 0.5, 0.5, 1), solidOf(8, 1, 0, 6), solidOf(0.25*math.Pi, 1, 0, 3), nonManifold)
	}},
	{"C13", "cyl", func(t *testing.T) []config {
		return curved(tube(t, 0, 0, 0, 1, 2), tube(t, 2, 0, 0, 1, 2), nonManifold, nothing, solidOf(2*math.Pi, 1, 0, 3))
	}},
	{"C14", "cyl", func(t *testing.T) []config {
		return curved(tube(t, 0, 0, 0, 1, 2), tube(t, 0.5, 0, 0, 0.5, 2), solidOf(2*math.Pi, 1, 0, 3), solidOf(0.5*math.Pi, 1, 0, 3), nonManifold)
	}},
	{"C16", "cyl", func(t *testing.T) []config {
		return curved(tube(t, 0, 0, 0, 1, 2), tube(t, 0, 0, 2, 1, 2), solidOf(4*math.Pi, 1, 0, 3), nothing, solidOf(2*math.Pi, 1, 0, 3))
	}},
	{"C17", "cyl", func(t *testing.T) []config {
		// A of radius 1 and height [0,2]; B of radius 0.5 and height [-1,3].
		return curved(tube(t, 0, 0, 0, 1, 2), tube(t, 0, 0, -1, 0.5, 4), solidOf(2.5*math.Pi, 1, 0, 0), solidOf(0.5*math.Pi, 1, 0, 0), solidOf(1.5*math.Pi, 1, 0, 4))
	}},
	{"C18", "cyl", func(t *testing.T) []config {
		// Two cylinders of radius 1 and height 2 with axes 1 apart, so
		// that 4ε <= d <= 2r - 4ε.
		lens := 2*math.Acos(0.5) - 0.5*math.Sqrt(3)
		return curved(tube(t, 0, 0, 0, 1, 2), tube(t, 1, 0, 0, 1, 2), solidOf(2*(2*math.Pi-lens), 1, 0, 0), solidOf(2*lens, 1, 0, 0), solidOf(2*(math.Pi-lens), 1, 0, 0))
	}},
	{"C19", "cyl", func(t *testing.T) []config {
		union := solidOf(8+0.25*math.Pi, 1, 0, 8)
		union.sources = func(r Result, c config) error {
			// The top face of A has one hole.
			holed := 0
			for _, f := range r.Solid.Faces() {
				holed += len(f.Holes)
			}
			if holed != 1 {
				return fmt.Errorf("%d holes in the faces of the result, want 1", holed)
			}
			return nil
		}
		return curved(cube(t), tube(t, 1, 1, 2, 0.5, 1), union, nothing, solidOf(8, 1, 0, 6))
	}},
	{"C20", "cyl", func(t *testing.T) []config {
		// As C19, but the base circle touches the edge x = 2 of the top.
		// The faces around the point of contact form one cycle, so the
		// union is a solid whose top face touches itself there; M6
		// divides it in two, between a corner of the face and a vertex
		// of the quarter-turn cut of the circle. Nine faces: five of
		// the cube, the two parts of its top, the wall and the cap.
		// The first cylinder has its vertex at the point of contact, the
		// second a quarter turn from it.
		want := [3]outcome{Union: solidOf(8+math.Pi/4, 1, 0, 9), Common: nothing, Cut: solidOf(8, 1, 0, 6)}
		return []config{{a: cube(t), b: tube(t, 1.5, 1, 2, 0.5, 1), want: want}, {a: cube(t), b: tube(t, 1, 0.5, 2, 0.5, 1), want: want}}
	}},
	{"C22", "cyl", func(t *testing.T) []config {
		// A cylinder entering the top face of A and ending inside it.
		return curved(cube(t), tube(t, 1, 1, 1, 0.5, 2), solidOf(8+0.25*math.Pi, 1, 0, 0), solidOf(0.25*math.Pi, 1, 0, 0), solidOf(8-0.25*math.Pi, 1, 0, 0))
	}},
	{"C23", "cyl", func(t *testing.T) []config {
		// A slab 2 thick and a cylinder of radius 0.5 and length 6
		// through it, its axis 30° from the vertical. Section 11 states
		// no union.
		inside := 0.25 * math.Pi * 2 / math.Cos(math.Pi/6)
		turn, err := geom.Rotation(geom.Vec3{Y: 1}, math.Pi/6)
		if err != nil {
			t.Fatal(err)
		}
		b, err := tube(t, 0, 0, -3, 0.5, 6).Transformed(geom.Translation(geom.Vec3{X: 2, Y: 2, Z: 1}).Mul(turn))
		if err != nil {
			t.Fatal(err)
		}
		return curved(lifted(t, box(t, 0, 0, 0, 4, 4, 2)), b, outcome{open: true}, solidOf(inside, 1, 0, 0), solidOf(32-inside, 1, 0, 0))
	}},
	{"C28", "planar", func(t *testing.T) []config {
		// The smallest input the planar stress test found for a face that
		// touches itself: a corner of B lies on an edge of A, and B
		// passes through the faces x = 5 and y = 8 of A.
		a := box(t, 0, 0, 2, 5, 8, 12)
		b := polyhedron(t, []geom.Vec3{{X: 3, Y: 6, Z: 2}, {X: 5, Y: 8, Z: 3}, {X: 3, Y: 9, Z: 5}, {X: 1, Y: 7, Z: 4}, {X: 5, Y: 2, Z: 6}, {X: 7, Y: 4, Z: 7}, {X: 5, Y: 5, Z: 9}, {X: 3, Y: 3, Z: 8}},
			[][]int{{0, 3, 2, 1}, {4, 5, 6, 7}, {0, 1, 5, 4}, {1, 2, 6, 5}, {2, 3, 7, 6}, {3, 0, 4, 7}})
		union := solidOf(410.125, 1, 0, 14)
		union.sources = func(r Result, c config) error {
			// Two faces of A touch themselves at (5,8,3) and are divided
			// (M6): each of the four parts has the face it was divided
			// from as its one source, and the dividers run from (5,2,6)
			// to (5,0,2) and from (2,8,4.5) to (0,8,2).
			for _, want := range [2][2]geom.Vec3{{{X: 5, Y: 2, Z: 6}, {X: 5, Z: 2}}, {{X: 2, Y: 8, Z: 4.5}, {Y: 8, Z: 2}}} {
				found := false
				vertices := r.Solid.Vertices()
				for _, e := range r.Solid.Edges() {
					p, q := vertices[e.Start], vertices[e.End]
					found = found || p.Distance(want[0]) < 1e-9 && q.Distance(want[1]) < 1e-9 || p.Distance(want[1]) < 1e-9 && q.Distance(want[0]) < 1e-9
				}
				if !found {
					return fmt.Errorf("no divider from %v to %v", want[0], want[1])
				}
			}
			for _, on := range [2]geom.Vec3{{X: 5}, {Y: 8}} {
				var parts [][2]int
				for i, f := range r.Solid.Faces() {
					level := true
					for _, u := range f.Outer {
						p, _ := f.Surface.Eval(u.UVStart.X, u.UVStart.Y)
						level = level && math.Abs(p.Sub(on).Dot(on)) < 1e-9*on.Length()
					}
					if level {
						if len(r.Sources[i]) != 1 || r.Sources[i][0][0] != 0 {
							return fmt.Errorf("a face on the plane through %v has sources %v", on, r.Sources[i])
						}
						parts = append(parts, r.Sources[i][0])
					}
				}
				if len(parts) != 2 || parts[0] != parts[1] {
					return fmt.Errorf("the faces on the plane through %v come from %v, want two from one face of A", on, parts)
				}
			}
			return nil
		}
		return []config{planarConfig(t, a, b, [3]outcome{Union: union, Common: solidOf(43.875, 1, 0, 8), Cut: nonManifold})}
	}},
	{"C27", "cyl", func(t *testing.T) []config {
		a, none := tube(t, 0, 0, 0, 1, 2), trimmed.Solid{}
		whole := solidOf(2*math.Pi, 1, 0, 3)
		return []config{
			{a: a, b: none, want: [3]outcome{Union: whole, Common: nothing, Cut: whole}},
			{a: none, b: a, want: [3]outcome{Union: whole, Common: nothing, Cut: nothing}},
		}
	}},
	{"C27", "planar", func(t *testing.T) []config {
		a, none := box(t, 0, 0, 0, 2, 2, 2), brep.Solid{}
		whole := solidOf(8, 1, 0, 6)
		return []config{
			planarConfig(t, a, none, [3]outcome{Union: whole, Common: nothing, Cut: whole}),
			planarConfig(t, none, a, [3]outcome{Union: whole, Common: nothing, Cut: nothing}),
			planarConfig(t, none, none, [3]outcome{}),
		}
	}},
}

// tube is the cylinder of radius r about the vertical axis through
// (x, y), from height z0 up by h, as a parametric cylinder lifted with
// trimmed.FromParametric (the stored form SF2).
func tube(t testing.TB, x, y, z0, r, h float64) trimmed.Solid {
	t.Helper()
	frame, err := geom.NewFrame(geom.Vec3{X: x, Y: y, Z: z0}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	c, err := parametric.Cylinder(frame, r, h)
	if err != nil {
		t.Fatal(err)
	}
	s, err := trimmed.FromParametric(c)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// curved is a row of the cylinder variant: two operands, at least one
// with a cylinder face, and the three outcomes.
func curved(a, b trimmed.Solid, union, common, cut outcome) []config {
	return []config{{a: a, b: b, want: [3]outcome{Union: union, Common: common, Cut: cut}}}
}

// cube is the box [0,2]³ as a trimmed solid.
func cube(t testing.TB) trimmed.Solid { return lifted(t, box(t, 0, 0, 0, 2, 2, 2)) }

// storedForms are the variants of the rows that section 11 runs in every
// stored form of a cylinder: SF1 to SF3, and each under a rigid placement
// that is not axis-aligned (SF4).
var storedForms = []string{"sf1", "sf2", "sf3", "sf1p", "sf2p", "sf3p"}

// formed is the cylinder of tube in one of the stored forms. sf1 is the
// form a result has (B6): line and ellipse edges; it is taken from a
// union with the empty solid. sf2 is tube itself. sf3 is a disc built by
// trimmed.ExtrudeDiskWithHoles.
func formed(t testing.TB, form string, x, y, z0, r, h float64) trimmed.Solid {
	t.Helper()
	switch form[:3] {
	case "sf1":
		res, err := Combine(context.Background(), tube(t, x, y, z0, r, h), trimmed.Solid{}, Union, matrixOptions)
		if err != nil {
			t.Fatal(err)
		}
		return res.Solid
	case "sf3":
		frame, err := geom.NewFrame(geom.Vec3{X: x, Y: y, Z: z0}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if err != nil {
			t.Fatal(err)
		}
		s, err := trimmed.ExtrudeDiskWithHoles(context.Background(), frame, trimmed.ExtrusionCircle{Radius: r}, nil, nil, h)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	return tube(t, x, y, z0, r, h)
}

// placedAs moves both operands of a row by the placement of SF4 when the
// form asks for it.
func placedAs(t testing.TB, form string, solids ...trimmed.Solid) []trimmed.Solid {
	t.Helper()
	if !strings.HasSuffix(form, "p") {
		return solids
	}
	turn, err := geom.Rotation(geom.Vec3{X: 1, Y: 2, Z: 3}, 0.7)
	if err != nil {
		t.Fatal(err)
	}
	m := geom.Translation(geom.Vec3{X: 0.3, Y: -0.2, Z: 0.1}).Mul(turn)
	out := make([]trimmed.Solid, len(solids))
	for i, s := range solids {
		if out[i], err = s.Transformed(m); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func init() {
	for _, form := range storedForms {
		matrix = append(matrix, struct {
			row, variant string
			build        func(t *testing.T) []config
		}{"C15", form, func(t *testing.T) []config {
			// Coaxial cylinders of radius 1 and heights [0,2] and [1,3].
			union := solidOf(3*math.Pi, 1, 0, 3)
			union.sources = func(r Result, c config) error {
				// The lateral face from the lateral faces of both; one
				// cap from A alone and the other from B alone.
				return sourceShapes(r, map[string]int{"AB": 1, "A": 1, "B": 1})
			}
			s := placedAs(t, form, formed(t, form, 0, 0, 0, 1, 2), formed(t, form, 0, 0, 1, 1, 2))
			return curved(s[0], s[1], union, solidOf(math.Pi, 1, 0, 3), solidOf(math.Pi, 1, 0, 3))
		}}, struct {
			row, variant string
			build        func(t *testing.T) []config
		}{"C21", form, func(t *testing.T) []config {
			// A cylinder through A along z with both caps flush.
			cut := solidOf(8-0.5*math.Pi, 1, 0, 7)
			cut.sources = func(r Result, c config) error {
				// Six faces from A; the bore wall from B.
				return sourceShapes(r, map[string]int{"A": 6, "B": 1})
			}
			s := placedAs(t, form, cube(t), formed(t, form, 1, 1, 0, 0.5, 2))
			return curved(s[0], s[1], solidOf(8, 1, 0, 6), solidOf(0.5*math.Pi, 1, 0, 3), cut)
		}})
	}
}

// pair is the usual row: A is the box [0,2]³ and b is the other operand.
func pair(t *testing.T, b brep.Solid, union, common, cut outcome) []config {
	t.Helper()
	return []config{planarConfig(t, box(t, 0, 0, 0, 2, 2, 2), b, [3]outcome{Union: union, Common: common, Cut: cut})}
}

func moved(t testing.TB, s brep.Solid, m geom.Affine) brep.Solid {
	t.Helper()
	out, err := s.Transformed(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// sourceShapes counts the faces of a result by the operands in their
// source lists ("A", "B" or "AB") and compares with want. A key such as
// "A at z=0" also requires every corner of those faces at that height.
func sourceShapes(r Result, want map[string]int) error {
	got := map[string]int{}
	for i, f := range r.Solid.Faces() {
		key := ""
		for _, s := range r.Sources[i] {
			if name := operandName(s[0]); !strings.HasSuffix(key, name) {
				key += name
			}
		}
		for full := range want {
			var z float64
			if n, _ := fmt.Sscanf(strings.TrimPrefix(full, key+" at z="), "%g", &z); n != 1 || !strings.HasPrefix(full, key+" at z=") {
				continue
			}
			level := true
			for _, u := range f.Outer {
				p, _ := f.Surface.Eval(u.UVStart.X, u.UVStart.Y)
				level = level && math.Abs(p.Z-z) < 1e-6
			}
			if level {
				key = full
			}
		}
		got[key]++
	}
	if len(got) != len(want) {
		return fmt.Errorf("face sources by operand: got %v, want %v", got, want)
	}
	for k, n := range want {
		if got[k] != n {
			return fmt.Errorf("face sources by operand: got %v, want %v", got, want)
		}
	}
	return nil
}

func box(t testing.TB, x0, y0, z0, x1, y1, z1 float64) brep.Solid {
	t.Helper()
	s, err := brep.Box(geom.Vec3{X: x0, Y: y0, Z: z0}, geom.Vec3{X: x1, Y: y1, Z: z1})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// lifted promotes a planar solid with line edges; the empty solid stays
// the zero value.
func lifted(t testing.TB, s brep.Solid) trimmed.Solid {
	t.Helper()
	out, err := trimmed.FromPlanar(s, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func planarConfig(t testing.TB, a, b brep.Solid, want [3]outcome) config {
	t.Helper()
	return config{a: lifted(t, a), b: lifted(t, b), planar: [2]brep.Solid{a, b}, want: want}
}

// pendingLeaves reads testdata/pending.txt: one leaf name on a line.
func pendingLeaves(t *testing.T) map[string]bool {
	t.Helper()
	data, err := os.ReadFile("testdata/pending.txt")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, line := range strings.Fields(string(data)) {
		out[line] = true
	}
	return out
}

// TestMatrix runs the rows of section 11 as TestMatrix/<row>/<variant>/<operation>.
// A leaf named in testdata/pending.txt is expected to fail: its failure
// is logged, and its passing is a test failure.
func TestMatrix(t *testing.T) {
	pending := pendingLeaves(t)
	var rows []string
	seen := map[string]bool{}
	for _, m := range matrix {
		if !seen[m.row] {
			seen[m.row] = true
			rows = append(rows, m.row)
		}
	}
	for _, row := range rows {
		t.Run(row, func(t *testing.T) {
			t.Parallel()
			for _, m := range matrix {
				if m.row != row {
					continue
				}
				t.Run(m.variant, func(t *testing.T) {
					t.Parallel()
					for op := Union; op <= Cut; op++ {
						t.Run(operationNames[op], func(t *testing.T) {
							// Leaves share nothing: each builds its own operands.
							t.Parallel()
							var err error
							for i, c := range m.build(t) {
								if e := checkLeaf(c, op); e != nil && err == nil {
									err = fmt.Errorf("operands %d: %w", i, e)
								}
							}
							switch {
							case pending[t.Name()] && err == nil:
								t.Fatal("the leaf passes but is listed in testdata/pending.txt")
							case pending[t.Name()]:
								t.Logf("pending (%s): %v", errorName(err), err)
							case err != nil:
								t.Fatal(err)
							}
						})
					}
				})
			}
		})
	}
}

// errorName is the identifier of the error of section 9 that err wraps.
func errorName(err error) string {
	for name, kind := range map[string]error{"ErrInvalid": ErrInvalid, "ErrUnsupported": ErrUnsupported, "ErrNonManifold": ErrNonManifold, "ErrTolerance": ErrTolerance, "ErrInconsistent": ErrInconsistent, "ErrValidation": ErrValidation, "ErrBudget": ErrBudget} {
		if errors.Is(err, kind) {
			return name
		}
	}
	return "a mismatch"
}

// checkLeaf runs one operation on one pair of operands and compares the
// result with the outcome section 11 states.
func checkLeaf(c config, op Operation) error {
	want := c.want[op]
	if want.open {
		return nil
	}
	r, err := Combine(context.Background(), c.a, c.b, op, matrixOptions)
	switch {
	case want.fails != nil:
		if !errors.Is(err, want.fails) {
			if err == nil {
				return fmt.Errorf("got a result, want %v", want.fails)
			}
			return fmt.Errorf("got %w, want %v", err, want.fails)
		}
		return nil
	case err != nil && want.maybe != nil && errors.Is(err, want.maybe):
		return nil
	case err != nil:
		return err
	}
	topology := r.Solid.Topology()
	voids := 0
	for _, v := range r.Voids {
		if v {
			voids++
		}
	}
	if r.Solids != want.solids || voids != want.voids || len(r.Shells) != len(topology.Shells) {
		return fmt.Errorf("got %d solids, %d voids and %d shells, want %d solids and %d voids", r.Solids, voids, len(r.Shells), want.solids, want.voids)
	}
	if want.faces != 0 && topology.Faces != want.faces {
		return fmt.Errorf("got %d faces, want %d", topology.Faces, want.faces)
	}
	if beta := leafBound(c); !(math.Abs(r.Volume-want.volume) <= beta) {
		return fmt.Errorf("got volume %.12g, want %.12g within %g", r.Volume, want.volume, beta)
	}
	if err := checkSources(r, c); err != nil {
		return err
	}
	if want.sources != nil {
		return want.sources(r, c)
	}
	return nil
}

// leafBound is the bound β of V5 for a pair of operands.
func leafBound(c config) float64 {
	bound, largest := 0., 0.
	for _, s := range [2]trimmed.Solid{c.a, c.b} {
		if s.Topology().Faces == 0 {
			continue
		}
		p, err := s.MassProperties(context.Background(), matrixOptions.Integration)
		if err != nil {
			return math.NaN()
		}
		bound += matrixOptions.Linear * p.Area
		largest = math.Max(largest, p.Volume)
	}
	return bound + matrixOptions.Volume*largest
}

// checkSources applies the face source checks of section 11 to a result:
// every face has a source, the list is ordered A before B and then by
// index without duplicates, each index names a face of its operand, and
// each source face lies on the surface of the result face.
func checkSources(r Result, c config) error {
	faces := r.Solid.Faces()
	if len(r.Sources) != len(faces) {
		return fmt.Errorf("%d source lists for %d faces", len(r.Sources), len(faces))
	}
	inputs := [2][]trimmed.Face{c.a.Faces(), c.b.Faces()}
	tau := math.Max(r.Tolerance, r.Solid.Tolerance())
	for i, list := range r.Sources {
		if len(list) == 0 {
			return fmt.Errorf("face %d has no source", i)
		}
		for k, s := range list {
			if s[0] < 0 || s[0] > 1 || s[1] < 0 || s[1] >= len(inputs[s[0]]) {
				return fmt.Errorf("face %d names source %v, which is no input face", i, s)
			}
			if k > 0 && (list[k-1][0] > s[0] || list[k-1][0] == s[0] && list[k-1][1] >= s[1]) {
				return fmt.Errorf("face %d has sources %v out of order", i, list)
			}
			if d := surfaceGap(faces[i], inputs[s[0]][s[1]]); !(d <= tau) {
				return fmt.Errorf("face %d is %g from its source %v", i, d, s)
			}
		}
	}
	return nil
}

// surfaceGap is the largest distance from a corner of a result face to
// the surface of a source face: a plane, a polar plane chart or a
// cylinder, under its placement.
func surfaceGap(result, source trimmed.Face) float64 {
	base, m := parametric.SurfaceMapping(source.Surface)
	back, err := m.Inverse()
	if err != nil {
		return math.Inf(1)
	}
	distance := func(p geom.Vec3) float64 { return math.Inf(1) }
	if frame, ok := parametric.PolarPlaneFrame(base); ok {
		distance = func(p geom.Vec3) float64 { return math.Abs(frame.ToLocal(p).Z) }
	}
	switch b := base.(type) {
	case geom.PlaneSurface:
		distance = func(p geom.Vec3) float64 { return math.Abs(b.Frame().ToLocal(p).Z) }
	case geom.CylinderSurface:
		frame, radius := b.Parameters()
		distance = func(p geom.Vec3) float64 {
			q := frame.ToLocal(p)
			return math.Abs(math.Hypot(q.X, q.Y) - radius)
		}
	}
	worst := 0.
	for _, loop := range append([][]trimmed.Use{result.Outer}, result.Holes...) {
		for _, u := range loop {
			p, _ := result.Surface.Eval(u.UVStart.X, u.UVStart.Y)
			worst = math.Max(worst, distance(back.Apply(p)))
		}
	}
	return worst
}

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

// matrixOptions are the options every leaf of the matrix runs with.
var matrixOptions = Options{Linear: 1e-7, Angular: 1e-9, Volume: 1e-9, Integration: parametric.IntegrationOptions{RelativeTolerance: 1e-10}}

var operationNames = [...]string{Union: "union", Common: "common", Cut: "cut"}

// outcome is what section 11 states for one operation of a row.
type outcome struct {
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
			for _, m := range matrix {
				if m.row != row {
					continue
				}
				t.Run(m.variant, func(t *testing.T) {
					for op := Union; op <= Cut; op++ {
						t.Run(operationNames[op], func(t *testing.T) {
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
								t.Logf("pending: %v", err)
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

// checkLeaf runs one operation on one pair of operands and compares the
// result with the outcome section 11 states.
func checkLeaf(c config, op Operation) error {
	want := c.want[op]
	r, err := Combine(context.Background(), c.a, c.b, op, matrixOptions)
	switch {
	case want.fails != nil:
		if !errors.Is(err, want.fails) {
			return fmt.Errorf("got %v, want %v", err, want.fails)
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

// surfaceGap is the largest distance from a vertex of a result face to
// the plane of a source face, or infinity when the planes are not
// parallel.
func surfaceGap(result, source trimmed.Face) float64 {
	normal := func(f trimmed.Face) (geom.Vec3, geom.Vec3) {
		origin, _ := f.Surface.Eval(0, 0)
		du, dv, _ := f.Surface.Partials(0, 0)
		n, _ := du.Cross(dv).Unit()
		return origin, n
	}
	_, n := normal(result)
	origin, m := normal(source)
	if n.Cross(m).Length() > 1e-9 {
		return math.Inf(1)
	}
	worst := 0.
	for _, loop := range append([][]trimmed.Use{result.Outer}, result.Holes...) {
		for _, u := range loop {
			p, _ := result.Surface.Eval(u.UVStart.X, u.UVStart.Y)
			worst = math.Max(worst, math.Abs(p.Sub(origin).Dot(m)))
		}
	}
	return worst
}

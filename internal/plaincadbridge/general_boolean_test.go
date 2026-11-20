package plaincadbridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"testing"

	"plainsolid/brep/boolean"
)

// newGeneral is the constructor that selects the general Boolean. Only
// tests use it; New is unchanged.
func newGeneral() *Bridge {
	b := New()
	b.general = true
	return b
}

// answer is a response as a test reads it.
type answer struct {
	OK     bool     `json:"ok"`
	Result Shape    `json:"result"`
	Error  *failure `json:"error"`
}

func ask(t *testing.T, b *Bridge, q any) answer {
	t.Helper()
	data, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	var a answer
	if err := json.Unmarshal([]byte(b.Call(string(data))), &a); err != nil {
		t.Fatal(err)
	}
	return a
}

// sameShape requires the assertions of A3 between a shape of the general
// bridge and the shape the default bridge gives for the same requests.
func sameShape(t *testing.T, have, want Shape) {
	t.Helper()
	g, w := have.GeometryAssertions, want.GeometryAssertions
	if !g.Valid || g.SolidCount != w.SolidCount || generalBooleanError(g.Volume, w.Volume) > 1e-7 || generalBooleanError(g.SurfaceArea, w.SurfaceArea) > 1e-7 {
		t.Fatalf("got %+v, want %+v", g, w)
	}
	for i := range want.Bounds.Min {
		if math.Abs(float64(have.Bounds.Min[i]-want.Bounds.Min[i])) > 1e-7 || math.Abs(float64(have.Bounds.Max[i]-want.Bounds.Max[i])) > 1e-7 {
			t.Fatalf("got bounds %+v, want %+v", have.Bounds, want.Bounds)
		}
	}
}

// TestGeneralBooleanAdapter runs requests on a general bridge and on a
// default bridge and requires the same shapes: the three Booleans on
// boxes and on a box with a cylinder, and the compositions of section 12.
func TestGeneralBooleanAdapter(t *testing.T) {
	type q = map[string]any
	block := q{"op": "box", "min": vector{}, "max": vector{4, 4, 4}}
	for _, c := range []struct {
		name  string
		build []q // shapes ps:1, ps:2, …
		last  q   // the request compared
	}{
		{"cut", []q{block, {"op": "box", "min": vector{2, 1, 1}, "max": vector{6, 3, 3}}}, q{"op": "cut", "a": "ps:1", "b": "ps:2"}},
		{"common", []q{block, {"op": "box", "min": vector{2, 1, 1}, "max": vector{6, 3, 3}}}, q{"op": "common", "a": "ps:1", "b": "ps:2"}},
		{"fuse", []q{block, {"op": "box", "min": vector{2, 1, 1}, "max": vector{6, 3, 3}}}, q{"op": "fuse", "a": "ps:1", "b": "ps:2"}},
		{"fuse to two solids", []q{block, {"op": "box", "min": vector{7, 0, 0}, "max": vector{8, 1, 1}}}, q{"op": "fuse", "a": "ps:1", "b": "ps:2"}},
		{"common that is empty", []q{block, {"op": "box", "min": vector{7, 0, 0}, "max": vector{8, 1, 1}}}, q{"op": "common", "a": "ps:1", "b": "ps:2"}},
		{"cut by a cylinder", []q{block, {"op": "circle", "origin": vector{2, 2, -1}, "normal": vector{0, 0, 1}, "radius": 1, "height": 6}}, q{"op": "cut", "a": "ps:1", "b": "ps:2"}},
		{"cutMany", []q{block, {"op": "box", "min": vector{1, 1, 3}, "max": vector{2, 2, 5}}, {"op": "box", "min": vector{3, 3, -1}, "max": vector{5, 5, 1}}}, q{"op": "cutMany", "a": "ps:1", "tools": []string{"ps:2", "ps:3"}}},
		{"joinAll", []q{block, {"op": "box", "min": vector{6, 0, 0}, "max": vector{8, 4, 4}}, {"op": "box", "min": vector{3, 1, 1}, "max": vector{7, 2, 2}}}, q{"op": "joinAll", "handles": []string{"ps:1", "ps:2"}, "b": "ps:3"}},
		{"prismToFace", []q{{"op": "box", "min": vector{-5, -5, 10}, "max": vector{5, 5, 12}}}, q{"op": "prismToFace", "b": "ps:1", "origin": vector{0, 0, 10}, "normal": vector{0, 0, -1}, "direction": vector{0, 0, 1}, "outer": []vector{{-1, -1, 0}, {1, -1, 0}, {1, 1, 0}, {-1, 1, 0}}}},
		{"circleToFace", []q{{"op": "box", "min": vector{-5, -5, 10}, "max": vector{5, 5, 12}}}, q{"op": "circleToFace", "b": "ps:1", "origin": vector{0, 0, 10}, "normal": vector{0, 0, -1}, "direction": vector{0, 0, 1}, "center": vector{1, 0, 0}, "radius": 2}},
	} {
		t.Run(c.name, func(t *testing.T) {
			var got [2]answer
			for i, b := range [2]*Bridge{New(), newGeneral()} {
				for _, request := range c.build {
					shape(t, b, request)
				}
				got[i] = ask(t, b, c.last)
			}
			if !got[0].OK || !got[1].OK {
				t.Fatalf("legacy %+v, general %+v", got[0].Error, got[1].Error)
			}
			sameShape(t, got[1].Result, got[0].Result)
		})
	}
	t.Run("overlapVolume", func(t *testing.T) {
		b := newGeneral()
		shape(t, b, block)
		shape(t, b, q{"op": "circle", "origin": vector{2, 2, 3}, "normal": vector{0, 0, 1}, "radius": 1, "height": 6})
		var got struct {
			Volume float64 `json:"volume"`
		}
		if err := json.Unmarshal(invoke(t, b, q{"op": "overlapVolume", "a": "ps:1", "b": "ps:2"}), &got); err != nil || generalBooleanError(got.Volume, math.Pi) > 1e-7 {
			t.Fatalf("overlap volume %v (%v), want π", got.Volume, err)
		}
	})
}

// TestGeneralMirroredStock cuts a general result that the transform
// request has reflected (S4): a box with a side notch off its middle,
// mirrored and moved, and then a blind bore. On the default bridge no
// legacy path takes a general result as stock, so the fallback answers.
func TestGeneralMirroredStock(t *testing.T) {
	notched := 3200 - 6*(9*math.Pi-9*math.Acos(1./3)+math.Sqrt(8))
	for name, open := range map[string]func() *Bridge{"general": newGeneral, "default": New} {
		t.Run(name, func(t *testing.T) {
			b := open()
			stock := box(t, b, vector{}, vector{20, 20, 8})
			notch := circle(t, b, vector{6, 1, 0}, vector{0, 0, 1}, 3, 6)
			cut := general(t, b, "cut", stock, notch)
			near(t, cut.GeometryAssertions.Volume, notched)
			moved := shape(t, b, map[string]any{"op": "transform", "handle": cut.Handle, "matrix": [][]float64{{-1, 0, 0, 30}, {0, 1, 0, 2}, {0, 0, 1, 0}}})
			near(t, moved.GeometryAssertions.Volume, notched)
			near(t, moved.GeometryAssertions.SurfaceArea, cut.GeometryAssertions.SurfaceArea)
			tool := circle(t, b, vector{25, 12, 0}, vector{0, 0, 1}, 2, 3)
			var bored Shape
			if name == "default" {
				bored = declined(t, b, "", "cut", moved.Handle, tool.Handle)
			} else {
				bored = shape(t, b, map[string]any{"op": "cut", "a": moved.Handle, "b": tool.Handle})
			}
			near(t, bored.GeometryAssertions.Volume, notched-12*math.Pi)
			near(t, bored.GeometryAssertions.SurfaceArea, moved.GeometryAssertions.SurfaceArea+12*math.Pi)
			if bored.Bounds != (Bounds{Min: vector{10, 2, 0}, Max: vector{30, 22, 8}}) || bored.GeometryAssertions.SolidCount != 1 {
				t.Fatal(bored)
			}
			// The notch is where the mirror put it, at x = 24: a tool in
			// it removes nothing. At x = 16, where the move alone would
			// have put it, a tool removes the part of its disk that lies
			// in the stock, through its depth.
			near(t, overlapMeasure(t, b, bored.Handle, circle(t, b, vector{24, 3, 0}, vector{0, 0, 1}, 2, 6).Handle), 0)
			near(t, overlapMeasure(t, b, bored.Handle, circle(t, b, vector{16, 3, 0}, vector{0, 0, 1}, 2, 6).Handle), 16*math.Pi+6*math.Sqrt(3))
		})
	}
}

// TestGeneralCutRemovesMaterial holds the cuts of the general bridge to
// the contract that a cut removes material, where the volumes alone
// cannot tell. A bore whose floor is within tolerance of the far face is
// cut through, and its wall ends where the tool ended, 5e-8 short; the
// same tool again finds 1e-7 of volume more than the first result
// reported and removes none. A tool a hundredth across removes less than
// the working tolerance times the area of the stock, and removes it.
func TestGeneralCutRemovesMaterial(t *testing.T) {
	b := newGeneral()
	base := box(t, b, vector{-4, -4, 0}, vector{4, 4, 8})
	tool := circle(t, b, vector{0, 0, 0}, vector{0, 0, 1}, 1, 8-5e-8)
	bored := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": tool.Handle})
	near(t, bored.GeometryAssertions.Volume, 512-8*math.Pi)
	inside := circle(t, b, vector{0, 0, 2}, vector{0, 0, 1}, .5, 3)
	small := circle(t, b, vector{3, 3, 8}, vector{0, 0, -1}, .01, .01)
	for _, c := range []struct {
		name    string
		request map[string]any
		removed float64 // zero where the request is refused
	}{
		{"the same tool again", map[string]any{"op": "cut", "a": bored.Handle, "b": tool.Handle}, 0},
		{"a tool inside the bore", map[string]any{"op": "cut", "a": bored.Handle, "b": inside.Handle}, 0},
		{"both as a group", map[string]any{"op": "cutMany", "a": bored.Handle, "tools": []string{inside.Handle, tool.Handle}}, 0},
		{"a small tool", map[string]any{"op": "cut", "a": bored.Handle, "b": small.Handle}, math.Pi * 1e-6},
		{"a small tool in a group with one that removes nothing", map[string]any{"op": "cutMany", "a": bored.Handle, "tools": []string{tool.Handle, small.Handle}}, math.Pi * 1e-6},
	} {
		t.Run(c.name, func(t *testing.T) {
			before := len(b.shapes)
			reply := ask(t, b, c.request)
			if c.removed == 0 {
				if reply.OK || reply.Error.Code != "UNSUPPORTED" || len(b.shapes) != before {
					t.Fatalf("a cut that removes nothing was answered: %+v %+v", reply.Result, reply.Error)
				}
				return
			}
			if !reply.OK {
				t.Fatal(reply.Error)
			}
			if got := bored.GeometryAssertions.Volume - reply.Result.GeometryAssertions.Volume; math.Abs(got-c.removed) > 2e-7 {
				t.Fatalf("removed %.6g, want %.6g", got, c.removed)
			}
		})
	}
}

// TestGeneralJoinAddsVolume holds the joins of the general bridge to the
// contract that the tool adds material, where the volumes alone cannot
// tell. A boss whose base is 5e-8 above the face it stands on is joined
// to it, and the union is measured as built; a tool inside the two then
// finds 1e-7 of volume more and adds none. A block a hundredth across on
// the face adds less than the working tolerance times the area, and adds
// it.
func TestGeneralJoinAddsVolume(t *testing.T) {
	b := newGeneral()
	base := box(t, b, vector{-4, -4, 0}, vector{4, 4, 8})
	boss := circle(t, b, vector{0, 0, 8 + 5e-8}, vector{0, 0, 1}, 1, 3)
	join := func(tool Shape) answer {
		return ask(t, b, map[string]any{"op": "joinAll", "handles": []string{base.Handle, boss.Handle}, "b": tool.Handle})
	}
	inside := box(t, b, vector{-.5, -.5, 6}, vector{.5, .5, 10})
	before := len(b.shapes)
	if reply := join(inside); reply.OK || len(b.shapes) != before {
		t.Fatalf("a join whose tool is inside its targets was answered: %+v", reply.Result)
	}
	reply := join(box(t, b, vector{3, 3, 8}, vector{3.01, 3.01, 8.01}))
	if !reply.OK {
		t.Fatal(reply.Error)
	}
	if got := reply.Result.GeometryAssertions.Volume - (512 + 3*math.Pi); math.Abs(got-1e-6) > 2e-7 {
		t.Fatalf("the join added %.6g, want 1e-06", got)
	}
}

// TestGeneralBooleanTransform covers the one change to the transform
// request (RL-2): a result with a sealed void is moved and published
// again, with its measures, its solid count and the bounds of its
// geometry.
func TestGeneralBooleanTransform(t *testing.T) {
	b := newGeneral()
	box(t, b, vector{}, vector{10, 10, 10})
	box(t, b, vector{2, 2, 2}, vector{8, 8, 8})
	hollow := shape(t, b, map[string]any{"op": "cut", "a": "ps:1", "b": "ps:2"})
	moved := shape(t, b, map[string]any{"op": "transform", "handle": hollow.Handle, "matrix": [][]float64{{0, 0, 1, 7}, {1, 0, 0, 8}, {0, 1, 0, 9}}})
	want := hollow
	want.Bounds = Bounds{vector{7, 8, 9}, vector{17, 18, 19}}
	sameShape(t, moved, want)
	if g := moved.GeometryAssertions; g.SolidCount != 1 || generalBooleanError(g.Volume, 1000-216) > 1e-7 {
		t.Fatalf("got %+v", g)
	}
	if n := len(b.shapes[moved.Handle].analytic.solid.Topology().Shells); n != 2 {
		t.Fatalf("%d shells, want 2", n)
	}
}

// TestGeneralBooleanBounds requires the bounds of the geometry, which the
// adapter takes from the edges of the result: a quarter wall, whose
// extent is at its vertices; a cylinder that leans, whose rims turn
// inside their arcs; and an oblique cut, whose wall trimmed.Solid.Bounds
// boxes half a unit past the stock at either end (acceptance row T1-035).
func TestGeneralBooleanBounds(t *testing.T) {
	type q = map[string]any
	for _, c := range []struct {
		name     string
		op       string
		build    []q
		min, max vector
	}{
		{"a quarter wall", "common", []q{
			{"op": "box", "min": vector{0, 0, 0}, "max": vector{20, 20, 8}},
			{"op": "circle", "origin": vector{0, 0, -1}, "normal": vector{0, 0, 1}, "radius": 3, "height": 10},
		}, vector{0, 0, 0}, vector{3, 3, 8}},
		{"a leaning cylinder", "common", []q{
			{"op": "box", "min": vector{-20, -20, -20}, "max": vector{20, 20, 20}},
			{"op": "circle", "origin": vector{0, 0, 0}, "normal": vector{1, 2, 2}, "radius": 1, "height": 6},
		}, vector{-math.Sqrt(8) / 3, -math.Sqrt(5) / 3, -math.Sqrt(5) / 3}, vector{2 + math.Sqrt(8)/3, 4 + math.Sqrt(5)/3, 4 + math.Sqrt(5)/3}},
		{"an oblique cut", "cut", []q{
			{"op": "box", "min": vector{0, 0, 0}, "max": vector{20, 20, 20}},
			{"op": "circle", "origin": vector{1, 4, -4}, "normal": vector{1, 0, 4}, "radius": 1, "height": 30},
		}, vector{0, 0, 0}, vector{20, 20, 20}},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := newGeneral()
			for _, request := range c.build {
				shape(t, b, request)
			}
			got := shape(t, b, q{"op": c.op, "a": "ps:1", "b": "ps:2"}).Bounds
			for i := range 3 {
				if math.Abs(float64(got.Min[i]-c.min[i])) > 1e-9 || math.Abs(float64(got.Max[i]-c.max[i])) > 1e-9 {
					t.Fatalf("bounds %v, want %v to %v", got, c.min, c.max)
				}
			}
		})
	}
}

// TestGeneralBooleanToFace covers the three rules of CO3 that fail a
// to-face request: the face is gone, the profile is not before the
// plane, and the target does not cover the end of the tool, which is
// decided from the sources of the faces of two more Booleans.
func TestGeneralBooleanToFace(t *testing.T) {
	type q = map[string]any
	plate := q{"op": "box", "min": vector{-5, -5, 10}, "max": vector{5, 5, 12}}
	under := func(change q) q {
		request := q{"op": "circleToFace", "b": "ps:1", "origin": vector{0, 0, 10}, "normal": vector{0, 0, -1}, "direction": vector{0, 0, 1}, "center": vector{0, 0, 0}, "radius": 1}
		for k, v := range change {
			request[k] = v
		}
		return request
	}
	for _, c := range []struct {
		name    string
		target  q
		request q
		ok      bool
	}{
		{"covered", plate, under(nil), true},
		{"the end hangs over the edge of the face", plate, under(q{"center": vector{4.5, 0, 0}}), false},
		{"the end lies over a hole of the face", q{"op": "prism", "outer": []vector{{-5, -5, 10}, {5, -5, 10}, {5, 5, 10}, {-5, 5, 10}}, "holes": [][]vector{{{-2, -2, 10}, {-2, 2, 10}, {2, 2, 10}, {2, -2, 10}}}, "offset": vector{0, 0, 2}}, under(nil), false},
		{"the face looks away from the tool", plate, under(q{"origin": vector{0, 0, 12}, "normal": vector{0, 0, 1}}), false},
		{"no face on the plane", plate, under(q{"origin": vector{0, 0, 11}}), false},
		{"the profile is past the plane", plate, under(q{"center": vector{0, 0, 10.5}}), false},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := newGeneral()
			shape(t, b, c.target)
			if a := ask(t, b, c.request); a.OK != c.ok || b.next != 1+map[bool]uint64{true: 1}[c.ok] {
				t.Fatalf("ok %v with %d shapes, want %v (%+v)", a.OK, b.next, c.ok, a.Error)
			}
		})
	}
}

// TestGeneralBooleanFailures fixes the bridge failure of each error of
// the general Boolean, and of a stored shape the adapter does not take.
func TestGeneralBooleanFailures(t *testing.T) {
	var f *failureError
	for _, c := range []struct {
		err  error
		code string
	}{
		{boolean.ErrInvalid, "INVALID"},
		{boolean.ErrUnsupported, "UNSUPPORTED"},
		{boolean.ErrNonManifold, "UNSUPPORTED"},
		{boolean.ErrTolerance, "UNSUPPORTED"},
		{boolean.ErrBudget, "LIMIT"},
		{boolean.ErrInconsistent, "INTERNAL"},
		{boolean.ErrValidation, "INTERNAL"},
	} {
		if err := generalFailure(fmt.Errorf("stage: %w", c.err)); !errors.As(err, &f) || f.code != c.code {
			t.Fatalf("%v maps to %v, want %s", c.err, err, c.code)
		}
	}
	if _, err := generalOperand(stored{tangent: &tangentBlindBore{}}); !errors.As(err, &f) || f.code != "UNSUPPORTED" {
		t.Fatalf("a stored shape with no trimmed solid gives %v", err)
	}
	// A cut has to remove material and leave some, as on the legacy
	// paths; cutMany is atomic, so nothing is published.
	b := newGeneral()
	x := box(t, b, vector{}, vector{2, 2, 2})
	y := shape(t, b, map[string]any{"op": "circle", "origin": vector{5, 5, 0}, "normal": vector{0, 0, 1}, "radius": 1, "height": 2})
	z := box(t, b, vector{-1, -1, -1}, vector{3, 3, 3})
	for _, request := range []map[string]any{
		{"op": "cut", "a": x.Handle, "b": y.Handle},
		{"op": "cut", "a": x.Handle, "b": z.Handle},
		{"op": "cutMany", "a": x.Handle, "tools": []string{y.Handle, z.Handle}},
	} {
		if a := ask(t, b, request); a.OK || a.Error.Code != "UNSUPPORTED" || b.next != 3 {
			t.Fatalf("%v: got %+v, %d shapes", request, a, b.next)
		}
	}
}

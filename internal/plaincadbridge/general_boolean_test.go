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

// TestGeneralBooleanAdapter runs the three Boolean requests on two boxes
// that are apart, on a general bridge and on a default bridge, and
// requires the same outcome.
func TestGeneralBooleanAdapter(t *testing.T) {
	for _, op := range []string{"cut", "common", "fuse"} {
		t.Run(op, func(t *testing.T) {
			var got [2]answer
			for i, b := range [2]*Bridge{New(), newGeneral()} {
				x := box(t, b, vector{}, vector{2, 2, 2})
				y := box(t, b, vector{5, 0, 0}, vector{6, 1, 1})
				got[i] = ask(t, b, map[string]any{"op": op, "a": x.Handle, "b": y.Handle})
			}
			legacy, general := got[0], got[1]
			if op == "fuse" {
				// A result of two solids waits for owner decision D4.
				if !legacy.OK || general.OK || general.Error.Code != "UNSUPPORTED" {
					t.Fatalf("legacy %+v, general %+v", legacy, general.Error)
				}
				return
			}
			if !legacy.OK || !general.OK {
				t.Fatalf("legacy %+v, general %+v", legacy.Error, general.Error)
			}
			want, have := legacy.Result, general.Result
			if !have.GeometryAssertions.Valid || have.GeometryAssertions.SolidCount != want.GeometryAssertions.SolidCount {
				t.Fatalf("got %+v, want %+v", have.GeometryAssertions, want.GeometryAssertions)
			}
			close := func(a, b float64) bool { return math.Abs(a-b) <= 1e-7*math.Max(1, math.Abs(b)) }
			if !close(have.GeometryAssertions.Volume, want.GeometryAssertions.Volume) || !close(have.GeometryAssertions.SurfaceArea, want.GeometryAssertions.SurfaceArea) {
				t.Fatalf("got %+v, want %+v", have.GeometryAssertions, want.GeometryAssertions)
			}
			// trimmed.Solid.Bounds pads its box by the solid's tolerance,
			// which is the working tolerance 1e-7 here. Whether the adapter
			// removes the pad is part of owner decision D4.
			for i := range want.Bounds.Min {
				if math.Abs(have.Bounds.Min[i]-want.Bounds.Min[i]) > 2.5e-7 || math.Abs(have.Bounds.Max[i]-want.Bounds.Max[i]) > 2.5e-7 {
					t.Fatalf("got bounds %+v, want %+v", have.Bounds, want.Bounds)
				}
			}
		})
	}
}

// TestGeneralBooleanFailures fixes the bridge failure of each error of
// the general Boolean, and of a stored shape the adapter does not take.
func TestGeneralBooleanFailures(t *testing.T) {
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
		var f *failureError
		if err := generalFailure(fmt.Errorf("stage: %w", c.err)); !errors.As(err, &f) || f.code != c.code {
			t.Fatalf("%v maps to %v, want %s", c.err, err, c.code)
		}
	}
	b := newGeneral()
	x := box(t, b, vector{}, vector{2, 2, 2})
	y := shape(t, b, map[string]any{"op": "circle", "origin": vector{5, 5, 0}, "normal": vector{0, 0, 1}, "radius": 1, "height": 2})
	if a := ask(t, b, map[string]any{"op": "cut", "a": x.Handle, "b": y.Handle}); a.OK || a.Error.Code != "UNSUPPORTED" {
		t.Fatalf("got %+v", a)
	}
}

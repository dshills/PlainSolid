package boolean

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// snapshot is what P7 compares of a solid: its topology, bounds,
// tolerance and vertices.
func snapshot(t *testing.T, s trimmed.Solid) [4]any {
	t.Helper()
	var bounds geom.Box
	if s.Topology().Faces != 0 {
		var err error
		if bounds, err = s.Bounds(); err != nil {
			t.Fatal(err)
		}
	}
	return [4]any{s.Topology(), bounds, s.Tolerance(), s.Vertices()}
}

func TestContract(t *testing.T) {
	a, far := lifted(t, box(t, 0, 0, 0, 2, 2, 2)), lifted(t, box(t, 5, 0, 0, 6, 1, 1))
	good := matrixOptions
	with := func(change func(*Options)) Options {
		o := good
		change(&o)
		return o
	}
	scaled, err := a.Transformed(geom.Scaling(geom.Vec3{X: 1, Y: 2, Z: 1}))
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	nan, inf := math.NaN(), math.Inf(1)
	for _, c := range []struct {
		name string
		ctx  context.Context
		a, b trimmed.Solid
		op   Operation
		o    Options
		want error
	}{
		{"nil context", nil, a, far, Union, good, ErrInvalid},
		{"unknown operation", context.Background(), a, far, Cut + 1, good, ErrInvalid},
		{"no tolerance", context.Background(), a, far, Union, with(func(o *Options) { o.Linear, o.Relative = 0, 0 }), ErrInvalid},
		{"negative Linear", context.Background(), a, far, Union, with(func(o *Options) { o.Linear = -1 }), ErrInvalid},
		{"negative Relative", context.Background(), a, far, Union, with(func(o *Options) { o.Relative = -1 }), ErrInvalid},
		{"zero Angular", context.Background(), a, far, Union, with(func(o *Options) { o.Angular = 0 }), ErrInvalid},
		{"Angular above 0.5", context.Background(), a, far, Union, with(func(o *Options) { o.Angular = 0.6 }), ErrInvalid},
		{"zero Volume", context.Background(), a, far, Union, with(func(o *Options) { o.Volume = 0 }), ErrInvalid},
		{"negative Samples", context.Background(), a, far, Union, with(func(o *Options) { o.Samples = -1 }), ErrInvalid},
		{"negative MaxFaces", context.Background(), a, far, Union, with(func(o *Options) { o.MaxFaces = -1 }), ErrInvalid},
		{"negative MaxPairs", context.Background(), a, far, Union, with(func(o *Options) { o.MaxPairs = -1 }), ErrInvalid},
		{"negative MaxNodes", context.Background(), a, far, Union, with(func(o *Options) { o.MaxNodes = -1 }), ErrInvalid},
		{"negative MaxPieces", context.Background(), a, far, Union, with(func(o *Options) { o.MaxPieces = -1 }), ErrInvalid},
		{"MaxGrowth below 1", context.Background(), a, far, Union, with(func(o *Options) { o.MaxGrowth = 0.5 }), ErrInvalid},
		{"integration out of range", context.Background(), a, far, Union, with(func(o *Options) { o.Integration.RelativeTolerance = 2 }), ErrInvalid},
		{"NaN Linear", context.Background(), a, far, Union, with(func(o *Options) { o.Linear = nan }), ErrInvalid},
		{"infinite Relative", context.Background(), a, far, Union, with(func(o *Options) { o.Relative = inf }), ErrInvalid},
		{"NaN Angular", context.Background(), a, far, Union, with(func(o *Options) { o.Angular = nan }), ErrInvalid},
		{"infinite Volume", context.Background(), a, far, Union, with(func(o *Options) { o.Volume = inf }), ErrInvalid},
		{"NaN MaxGrowth", context.Background(), a, far, Union, with(func(o *Options) { o.MaxGrowth = nan }), ErrInvalid},
		{"NaN integration tolerance", context.Background(), a, far, Union, with(func(o *Options) { o.Integration.RelativeTolerance = nan }), ErrInvalid},
		{"tolerance not below L/100", context.Background(), a, far, Union, with(func(o *Options) { o.Linear = 0.5 }), ErrInvalid},
		{"face limit", context.Background(), a, far, Union, with(func(o *Options) { o.MaxFaces = 5 }), ErrBudget},
		{"face limit before the class", context.Background(), scaled, far, Union, with(func(o *Options) { o.MaxFaces = 5 }), ErrBudget},
		{"placement with a non-uniform scaling", context.Background(), scaled, far, Union, good, ErrUnsupported},
		{"cancelled", cancelled, a, far, Union, good, context.Canceled},
		{"cancelled with invalid options", cancelled, a, far, Union, with(func(o *Options) { o.Volume = 0 }), context.Canceled},
		{"cancelled with an unsupported operand", cancelled, scaled, far, Union, good, context.Canceled},
	} {
		t.Run(c.name, func(t *testing.T) {
			r, err := Combine(c.ctx, c.a, c.b, c.op, c.o)
			if !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
			if c.want == context.Canceled && err != context.Canceled {
				t.Fatalf("the context error is wrapped: %v", err)
			}
			if !reflect.DeepEqual(r, Result{}) {
				t.Fatalf("an error came with a result: %+v", r)
			}
		})
	}
	t.Run("piece limit", func(t *testing.T) {
		if _, err := Combine(context.Background(), a, far, Union, with(func(o *Options) { o.MaxPieces = 11 })); !errors.Is(err, ErrBudget) {
			t.Fatalf("got %v, want ErrBudget", err)
		}
	})
	// P7 on operands that do not interact and on operands that do.
	near := lifted(t, box(t, 1, 1, 1, 3, 3, 3))
	for name, b := range map[string]trimmed.Solid{"apart": far, "interacting": near} {
		for op := Union; op <= Cut; op++ {
			t.Run("operands unchanged/"+name+"/"+operationNames[op], func(t *testing.T) {
				before := [2][4]any{snapshot(t, a), snapshot(t, b)}
				if _, err := Combine(context.Background(), a, b, op, good); err != nil {
					t.Fatal(err)
				}
				if after := [2][4]any{snapshot(t, a), snapshot(t, b)}; !reflect.DeepEqual(before, after) {
					t.Fatal("Combine changed an operand")
				}
			})
			t.Run("equal calls give equal results/"+name+"/"+operationNames[op], func(t *testing.T) {
				var runs [2]Result
				for i := range runs {
					var err error
					if runs[i], err = Combine(context.Background(), a, b, op, good); err != nil {
						t.Fatal(err)
					}
				}
				if !sameResult(t, runs[0], runs[1]) {
					t.Fatal("two equal calls returned different results")
				}
			})
		}
	}
	t.Run("cancelled during the passes", func(t *testing.T) {
		// The context is cancelled by the first evaluation of an edge
		// of B, which is during intake; the passes then see it.
		ctx, stop := context.WithCancel(context.Background())
		tripped := restored(t, near, func(_ int, c geom.Curve, _, _ float64) geom.Curve { return tripwire{c, stop} })
		if _, err := Combine(ctx, a, tripped, Union, good); err != context.Canceled {
			t.Fatalf("got %v, want the context error", err)
		}
	})
}

// tripwire is a curve that cancels a context when it is evaluated.
type tripwire struct {
	geom.Curve
	stop context.CancelFunc
}

func (c tripwire) Eval(t float64) (geom.Vec3, error) {
	c.stop()
	return c.Curve.Eval(t)
}

// sameResult compares two results deeply, apart from the surface and
// curve evaluators of their solids.
func sameResult(t *testing.T, x, y Result) bool {
	t.Helper()
	strip := func(r Result) (Result, [4]any, [][]geom.Vec2) {
		var uv [][]geom.Vec2
		for _, f := range r.Solid.Faces() {
			var loop []geom.Vec2
			for _, l := range append([][]trimmed.Use{f.Outer}, f.Holes...) {
				for _, u := range l {
					loop = append(loop, u.UVStart, u.UVEnd)
				}
			}
			uv = append(uv, loop)
		}
		shot := snapshot(t, r.Solid)
		r.Solid = trimmed.Solid{}
		return r, shot, uv
	}
	rx, sx, ux := strip(x)
	ry, sy, uy := strip(y)
	return reflect.DeepEqual(rx, ry) && reflect.DeepEqual(sx, sy) && reflect.DeepEqual(ux, uy)
}

// TestKnownMeasures covers the measures a caller hands to Combine: V5
// takes the volume and area of an operand from them and does not
// integrate it, the result reports its own mass properties for the next
// call, and a value is trusted as given, so a wrong one fails V5.
func TestKnownMeasures(t *testing.T) {
	stock, tool := lifted(t, box(t, 0, 0, 0, 4, 4, 4)), tube(t, 2, 2, -1, 1, 6)
	plain, err := Combine(context.Background(), stock, tool, Cut, matrixOptions)
	if err != nil {
		t.Fatal(err)
	}
	wantArea := 96 - 2*math.Pi + 8*math.Pi
	if p := plain.Properties; math.Abs(p.Volume-plain.Volume) > 0 || math.Abs(p.Area-wantArea) > 1e-7 {
		t.Fatalf("the result reports volume %g and area %g, want %g and %g", p.Volume, p.Area, plain.Volume, wantArea)
	}
	o := matrixOptions
	o.KnownA, o.KnownB = &parametric.Properties{Volume: 64, Area: 96}, &parametric.Properties{Volume: 6 * math.Pi, Area: 14 * math.Pi}
	// The volumes the result reports for the operands are the given
	// ones to the last bit, which integration would not return.
	known, err := Combine(context.Background(), stock, tool, Cut, o)
	if err != nil || known.VolumeA != 64 || known.VolumeB != 6*math.Pi || math.Abs(known.Volume-plain.Volume) > 1e-9 {
		t.Fatalf("with known measures: %v, volumes %g %g %g", err, known.VolumeA, known.VolumeB, known.Volume)
	}
	// The result of one call as the operand of the next, with the
	// measures it reported.
	o.KnownA, o.KnownB = &known.Properties, nil
	if next, err := Combine(context.Background(), known.Solid, lifted(t, box(t, 3, 3, 3, 5, 5, 5)), Cut, o); err != nil || math.Abs(next.Volume-(known.Volume-1)) > 1e-7 {
		t.Fatalf("second cut: %v, volume %g", err, next.Volume)
	}
	o.KnownA = &parametric.Properties{Volume: 65, Area: 96}
	if _, err := Combine(context.Background(), stock, tool, Cut, o); !errors.Is(err, ErrValidation) {
		t.Fatalf("a wrong known volume gives %v, want a V5 failure", err)
	}
	o.KnownA = &parametric.Properties{Volume: math.Inf(1)}
	if _, err := Combine(context.Background(), stock, tool, Cut, o); !errors.Is(err, ErrInvalid) {
		t.Fatalf("a known volume that is not finite gives %v", err)
	}
}

package boolean

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

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
	t.Run("operands unchanged", func(t *testing.T) {
		before := [2][4]any{snapshot(t, a), snapshot(t, far)}
		if _, err := Combine(context.Background(), a, far, Union, good); err != nil {
			t.Fatal(err)
		}
		if after := [2][4]any{snapshot(t, a), snapshot(t, far)}; !reflect.DeepEqual(before, after) {
			t.Fatal("Combine changed an operand")
		}
	})
	t.Run("equal calls give equal results", func(t *testing.T) {
		var runs [2]Result
		for i := range runs {
			var err error
			if runs[i], err = Combine(context.Background(), a, far, Union, good); err != nil {
				t.Fatal(err)
			}
		}
		if !sameResult(t, runs[0], runs[1]) {
			t.Fatal("two equal calls returned different results")
		}
	})
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

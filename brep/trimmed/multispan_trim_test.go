package trimmed

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestMultispanRationalTrimTopology(t *testing.T) {
	ctx := context.Background()
	a, b, c, d := geom.Vec2{}, geom.Vec2{X: 4}, geom.Vec2{X: 4, Y: 3}, geom.Vec2{Y: 3}
	f := Face{Outer: []Use{{UVStart: a, UVEnd: b}, {UVStart: b, UVEnd: c},
		{UVStart: c, UVEnd: d}, {UVStart: d, UVEnd: a}}}
	curve := func(y float64) SplineUV {
		// At the one interior knot the Euclidean point is
		// (5/2, (y-3)/4) with these controls and weights.
		p, e := geom.NewBSpline(2, []geom.Vec3{{}, {X: 1, Y: y}, {X: 3, Y: -1}, {X: 4}},
			[]float64{0, 0, 0, .5, 1, 1, 1}, []float64{1, .25, .75, 1})
		if e != nil {
			t.Fatal(e)
		}
		return SplineUV{Curve: p, Range: UVInterval{Max: 1}}
	}
	f.Outer[0].Curve = curve(-2)
	r, e := ValidateFaceTrimLoops(ctx, f, trimvalidate.Options{})
	if e != nil || r.Status != trimvalidate.Certified {
		t.Fatal(r, e)
	}
	// The second piece starts at the exact rational join. A loop outside a
	// sampled approximation cannot be substituted for its hull proof.
	f.Outer[0].Curve = curve(15)
	r, e = ValidateFaceTrimLoops(ctx, f, trimvalidate.Options{})
	if e != nil || r.Status != trimvalidate.Invalid || r.EdgeA != 0 || r.EdgeB != 2 {
		t.Fatal(r, e)
	}
	f.Outer[0].Curve = curve(-2)
	for _, o := range []trimvalidate.Options{{MaxEdges: 4}, {MaxWork: 10}, {MaxBits: 16}} {
		r, e = ValidateFaceTrimLoops(ctx, f, o)
		if e != nil || r.Status != trimvalidate.Unresolved {
			t.Fatal(r, e)
		}
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	if r, e = ValidateFaceTrimLoops(cancelled, f, trimvalidate.Options{}); !errors.Is(e, context.Canceled) || r.Status == trimvalidate.Certified {
		t.Fatal(r, e)
	}
}

func TestMultispanRationalTrimWithNonbinaryKnotPoint(t *testing.T) {
	// At the internal knot, u=(1+2*3)/(1+2)=7/3. Rounding this point
	// would invalidate a literal comparison of both original pieces.
	p, e := geom.NewBSpline(2, []geom.Vec3{{}, {X: 1, Y: -1}, {X: 3, Y: -2}, {X: 4}},
		[]float64{0, 0, 0, .5, 1, 1, 1}, []float64{1, .25, .5, 1})
	if e != nil {
		t.Fatal(e)
	}
	f := Face{Outer: []Use{
		{UVEnd: geom.Vec2{X: 4}, Curve: SplineUV{Curve: p, Range: UVInterval{Max: 1}}},
		{UVStart: geom.Vec2{X: 4}, UVEnd: geom.Vec2{X: 4, Y: 2}},
		{UVStart: geom.Vec2{X: 4, Y: 2}, UVEnd: geom.Vec2{Y: 2}},
		{UVStart: geom.Vec2{Y: 2}},
	}}
	r, e := ValidateFaceTrimLoops(context.Background(), f, trimvalidate.Options{})
	if e != nil || r.Status != trimvalidate.Certified {
		t.Fatal(r, e)
	}
	// An approximate endpoint for a clipped range must remain unresolved.
	uv := f.Outer[0].Curve.(SplineUV)
	uv.Range.Max = .5
	z, e := uv.EvalUV(1)
	if e != nil {
		t.Fatal(e)
	}
	f.Outer[0].UVEnd, f.Outer[0].Curve, f.Outer[1].UVStart = z, uv, z
	r, e = ValidateFaceTrimLoops(context.Background(), f, trimvalidate.Options{})
	if e != nil || r.Status != trimvalidate.Unresolved || r.EdgeA != 0 {
		t.Fatal(r, e)
	}
}

func TestSplitFaceWithClippedRationalSplinePath(t *testing.T) {
	ctx := context.Background()
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	s, e := FromPlanar(b, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	f := s.Faces()[0]
	a, z := f.Outer[0].UVStart, f.Outer[2].UVStart
	// A degree-one rational support clipped across the interior knot. Its
	// first and last active parameters are interior and have exact endpoints.
	m := mix(a, z, .5)
	p := func(x geom.Vec2) geom.Vec3 { return geom.Vec3{X: x.X, Y: x.Y} }
	c, e := geom.NewBSpline(1, []geom.Vec3{p(mix(a, m, -1)), p(m), p(mix(m, z, 2))},
		[]float64{0, 0, .5, 1, 1}, []float64{1, 1, 1})
	if e != nil {
		t.Fatal(e)
	}
	out, h, e := s.SplitFacePath(ctx, 0, 0, 2, []CutSegment{{Start: a, End: z,
		Curve: &SplineUV{Curve: c, Range: UVInterval{Min: .25, Max: .75}}}})
	if e != nil || !h.UVTopologyCertified || len(h.Edges) != 1 {
		t.Fatal(h, e)
	}
	r, e := out.ValidateTrimLoops(ctx, trimvalidate.Options{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	mass, e := out.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(mass.Volume-16) > 1e-9 {
		t.Fatal(mass, e)
	}
}

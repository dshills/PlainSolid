package trimmed

import (
	"context"
	"math"
	"math/big"
	"testing"

	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestAlgebraicTubeLensIndependentAffineCharts(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	// Model XY=(2*u,3*v) has Jacobian six.
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 3}, {X: 2}, {X: 2, Y: 3, Z: 1.0 / 64}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	x, z := [3]float64{0, .5, 1}, [3]float64{.25, -.25, .25}
	p := make([]geom.Vec3, 9)
	for i := range x {
		for j := range x {
			u, v := x[2-j], x[i]
			p[3*i+j] = geom.Vec3{X: 2 * u, Y: 3 * v, Z: z[2-j] + z[i] - 25.0/256 + u*v/64}
		}
	}
	b, e := geom.NewBSplineSurface(2, 2, 3, 3, p, []float64{2, 2, 2, 4, 4, 4}, []float64{-1, -1, -1, 3, 3, 3}, nil)
	if e != nil {
		t.Fatal(e)
	}
	patches := [2]intersect.Patch{{Surface: a, Max: geom.Vec2{X: 1, Y: 1}},
		{Surface: b, Min: geom.Vec2{X: 2, Y: -1}, Max: geom.Vec2{X: 4, Y: 3}}}
	var f [2]Face
	for i, p := range patches {
		v := [4]geom.Vec2{p.Min, {X: p.Max.X, Y: p.Min.Y}, p.Max, {X: p.Min.X, Y: p.Max.Y}}
		f[i].Surface = p.Surface
		for j, x := range v {
			f[i].Outer = append(f[i].Outer, Use{Edge: j, UVStart: x, UVEnd: v[(j+1)%4]})
		}
	}
	o := AlgebraicTrimmedPairSectionOptions{}
	cuts := []float64{.25, .3125, .6875, .75}
	o.Trace.AffineGraphFallback.Enabled = true
	o.Trace.AffineGraphFallback.UCuts, o.Trace.AffineGraphFallback.VCuts = cuts, cuts
	o.Trace.AffineGraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	s, e := SectionTrimmedSplinePair(ctx, patches, f, nil, o)
	if e != nil || !s.Complete || len(s.Branches) != 1 {
		t.Fatalf("independent affine section branches=%d issues=%v err=%v", len(s.Branches), s.Trace.Issues, e)
	}
	l, e := s.SealAlgebraicTubeLens(ctx, AlgebraicTubeLensOptions{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := l.VerticalInterval(ctx, [2]*big.Rat{big.NewRat(1, 1), big.NewRat(3, 2)}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || !q.Present || !q.Complete || q.Parameters[1][0].Cmp(big.NewRat(3, 1)) != 0 ||
		q.Parameters[1][1].Cmp(big.NewRat(1, 1)) != 0 || q.Lower.Cmp(big.NewRat(-24, 256)) != 0 || q.Upper.Cmp(big.NewRat(1, 256)) != 0 {
		t.Fatalf("affine vertical interval %+v: %v", q, e)
	}
	v, e := l.BoundVolume(ctx, AlgebraicTubeLensVolumeOptions{MaxDepth: 9})
	if e != nil {
		t.Fatal(v, e)
	}
	lo, _ := v.Volume.Lower.Float64()
	hi, _ := v.Volume.Upper.Float64()
	want := 6 * math.Pi * 625 / 131072
	if lo <= 0 || lo > want || hi < want {
		t.Fatal(lo, hi, want)
	}
}

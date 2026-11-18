//go:build research

package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
)

func TestNativeCuspidalCubicCutConformingDisplay(t *testing.T) {
	ctx := context.Background()
	src := cuspidalCubicCommonFixture(t)
	for order, numerator := range [2]int64{1695, 243} {
		r, err := CutCuspidalCubicSplineGraphs(ctx, src[0], src[1+order], src[2-order], order, 1e-9, CuspidalCubicCutOptions{})
		if err != nil {
			t.Fatal(order, err)
		}
		want := big.NewRat(numerator, 22528)
		proof, err := prepareCuspidalCutDisplay(ctx, r)
		if err != nil || proof.volume.Cmp(want) != 0 {
			t.Fatal("independently integrated source volume differs from the preview certificate", order, proof.volume, err)
		}
		o := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000}
		m, err := r.Tessellate(ctx, o)
		if err != nil {
			t.Fatal(order, err)
		}
		native := r.Boundary().Vertices()
		if len(m.Vertices) < len(native) || !slices.Equal(m.Vertices[:len(native)], native) {
			t.Fatal("conforming display moved a native point", order)
		}
		loc, err := collision.NewMeshPointLocator(ctx, m, nil, collision.LocationOptions{})
		if err != nil || len(loc.Shells()) != 1 || !loc.BoundaryCollision().Complete || len(loc.BoundaryCollision().Contacts) != 0 {
			t.Fatal("conforming display is not one embedded outward shell", order, err)
		}
		volume, _ := want.Float64()
		relative := math.Abs(m.SignedVolume()/volume - 1)
		if !(relative <= .02) {
			t.Fatal("the conforming display misses the independently integrated source volume", order, m.SignedVolume())
		}
		t.Logf("order %d %d triangles volume %.15g (%.4f%% relative error)", order, len(m.Triangles), m.SignedVolume(), 100*relative)
		for _, y := range [2]float64{1 - 3.0/32, 1 + 3.0/32} {
			const x = 21.0 / 16
			found := 0
			for _, p := range m.Vertices {
				if p.X != x || p.Y != y {
					continue
				}
				found++
				if math.Abs(p.Z-(2+x/16+x*y/64)) > 1e-14 {
					t.Fatal("shared germ station left its original model edge", p)
				}
			}
			if found != 1 {
				t.Fatal("the two caps must share one model vertex at each half-clock", order, x, y, found)
			}
		}
		if order == 0 {
			tight := o
			tight.MaxTriangles, tight.MaxSubdivisions = 318, 8
			exactBudget, err := r.Tessellate(ctx, tight)
			if err != nil || !slices.Equal(exactBudget.Triangles, m.Triangles) {
				t.Fatal("the closed exterior candidate must fit its exact rail and triangle budgets", err)
			}
		}
		var receipt bytes.Buffer
		if err := r.WriteJSON(ctx, &receipt); err != nil {
			t.Fatal(err)
		}
		reloaded, err := ReadCuspidalCubicCutJSON(ctx, &receipt, JSONLimits{}, CuspidalCubicCutOptions{})
		if err != nil {
			t.Fatal(err)
		}
		replay, err := reloaded.Tessellate(ctx, o)
		if err != nil || !slices.Equal(replay.Vertices, m.Vertices) || !slices.Equal(replay.Triangles, m.Triangles) {
			t.Fatal("source replay produced different mesh indices", order, err)
		}
	}
}

func TestNativeCuspidalCubicCutConformingDisplayShiftedCusp(t *testing.T) {
	ctx := context.Background()
	// A−B=(3/16)*((v−1/2)^2 − (u−1/2)^3/2). Its normalized cubic
	// term has zero full-footprint integral. The full gap is 3/32,
	// and using width q³/2 on the model plane gives lobe 3/1408.
	src := nodalCubicStockFixture(t, [4]float64{-3.0 / 256, 3.0 / 256, -3.0 / 256, 3.0 / 256}, 3.0/64)
	d := src[2].Data()
	for i := 0; i < len(d.Poles); i += 2 {
		d.Poles[i], d.Poles[i+1] = d.Poles[i+1], d.Poles[i]
	}
	// Reflect the physical X direction of the second roof chart while
	// preserving its original surface image.
	var err error
	src[2], err = geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if err != nil {
		t.Fatal(err)
	}
	r, err := CutCuspidalCubicSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, CuspidalCubicCutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	p, err := prepareCuspidalCutDisplay(ctx, r)
	if err != nil || !p.reversed[1] || p.volume.Cmp(big.NewRat(135, 1408)) != 0 {
		t.Fatal("changed original chart or coefficients lost the exact signed volume", p.volume, err)
	}
	o := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000}
	m, err := r.TessellateWithVolumeTolerance(ctx, o, .006)
	if err != nil {
		t.Fatal(err)
	}
	want := 135.0 / 1408
	if !(math.Abs(m.SignedVolume()/want-1) <= .006) {
		t.Fatal("changed original cubic exceeds the requested volume bound", m.SignedVolume(), want)
	}
	cusp := m.Vertices[r.History().Vertices[0]]
	if cusp != (geom.Vec3{X: 1.5, Y: 1, Z: 271.0 / 128}) {
		t.Fatal("display moved the original cusp", cusp)
	}
	t.Logf("shifted cusp %d triangles volume %.15g (%.4f%% relative error)",
		len(m.Triangles), m.SignedVolume(), 100*math.Abs(m.SignedVolume()/want-1))
}

func TestNativeCuspidalCubicCutConformingDisplayRefusals(t *testing.T) {
	ctx := context.Background()
	src := cuspidalCubicCommonFixture(t)
	r, err := CutCuspidalCubicSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, CuspidalCubicCutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	o := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 8, MaxTriangles: 80_000}
	for _, trial := range []struct {
		options parametric.TessellationOptions
		volume  float64
		want    error
	}{
		{o, 1e-6, parametric.ErrBudget},
		{parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 4}, .02, parametric.ErrBudget},
		{parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxTriangles: 317}, .02, parametric.ErrBudget},
		{parametric.TessellationOptions{Deflection: .05, MaxTriangles: 1}, .02, parametric.ErrBudget},
		{parametric.TessellationOptions{Deflection: .05, AnalyticBound: true}, .02, parametric.ErrDeflectionUnsupported},
		{o, math.NaN(), geom.ErrInvalid},
	} {
		m, err := r.TessellateWithVolumeTolerance(ctx, trial.options, trial.volume)
		if !errors.Is(err, trial.want) || len(m.Vertices) != 0 || len(m.Triangles) != 0 {
			t.Fatal("unqualified preview published partial geometry or the wrong error", err, trial.want)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	m, err := r.Tessellate(canceled, o)
	if !errors.Is(err, context.Canceled) || len(m.Vertices) != 0 || len(m.Triangles) != 0 {
		t.Fatal("canceled preview published geometry", err)
	}
}

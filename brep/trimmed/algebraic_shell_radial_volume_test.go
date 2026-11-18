//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func radialMaterialForBounds(t *testing.T) AlgebraicShellBoolean {
	t.Helper()
	ctx := context.Background()
	a, b, hints := transverseSplineCapsDomain(t, true)
	options := AlgebraicShellSectionOptions{Seeds: []AlgebraicShellSectionSeed{{Faces: [2]int{0, 1}, Tubes: hints}}}
	options.Trace.Coverage.Isolation.SeparatingProjections = true
	options.Trace.Coverage.Isolation.ParameterTolerance = .25
	options.Trace.Coverage.Cell.MaxOperations = 16000000
	section, e := SectionSplineShellsAlgebraic(ctx, a, b, options)
	if e != nil {
		t.Fatal(e)
	}
	result, e := section.CombineAlgebraic(ctx, parametric.Intersection, AlgebraicShellBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return result
}

func TestBoundRadialTubeRegionVolume(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	result := radialMaterialForBounds(t)
	center := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	radii := []*big.Rat{big.NewRat(5, 16)}
	v, e := result.BoundRadialTubeRegionVolume(ctx, center, radii, geom.RadialGraphVolumeOptions{PrecisionBits: 80})
	if e != nil {
		t.Fatal(e)
	}
	// Integral pi*r^4/2 for z=r^2-q against the plane z=0.
	if v.Volume.PiCoefficient.Cmp(big.NewRat(625, 131072)) != 0 || len(v.Components) != 1 || v.Components[0].Band != 0 || v.Components[0].InnerBranch != -1 || v.Components[0].OriginalFaces != [2]int{0, 1} {
		t.Fatal(v)
	}
	width := new(big.Rat).Sub(v.Volume.Volume.Upper, v.Volume.Volume.Lower)
	if v.Volume.Volume.Lower.Sign() <= 0 || width.Cmp(new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))) > 0 {
		t.Fatal("width", width)
	}
	exactWork := v.Volume.ExactOperations
	if partial, e := result.BoundRadialTubeRegionVolume(ctx, center, radii, geom.RadialGraphVolumeOptions{PrecisionBits: 80, MaxOperations: exactWork - 1}); e == nil || partial.Volume.PiCoefficient != nil || len(partial.Components) != 0 {
		t.Fatal("aggregate one-operation deficit", partial, e)
	}
	if _, e := result.BoundRadialTubeRegionVolume(ctx, center, radii, geom.RadialGraphVolumeOptions{PrecisionBits: 80, MaxOperations: exactWork}); e != nil {
		t.Fatal("exact cumulative quota", e)
	}
	result.Faces = nil
	result.Edges = nil
	result.Vertices = nil
	if _, e = result.BoundRadialTubeRegionVolume(ctx, center, radii, geom.RadialGraphVolumeOptions{}); e != nil {
		t.Fatal("diagnostic copy", e)
	}
	for _, o := range []geom.RadialGraphVolumeOptions{{MaxOperations: 1}, {PrecisionBits: 15}, {MaxCoefficientBits: 63}} {
		v, e := result.BoundRadialTubeRegionVolume(ctx, center, radii, o)
		if e == nil || v.Volume.PiCoefficient != nil || len(v.Components) != 0 {
			t.Fatal("partial quota result", v, e)
		}
	}
	if v, e := result.BoundRadialTubeRegionVolume(ctx, center, []*big.Rat{big.NewRat(6, 16)}, geom.RadialGraphVolumeOptions{}); e == nil || len(v.Components) != 0 {
		t.Fatal("wrong radius", e)
	}
	bad := result
	bad.faces = append([]AlgebraicShellBooleanFace(nil), result.faces...)
	bad.faces[0].Reversed = !bad.faces[0].Reversed
	if v, e := bad.BoundRadialTubeRegionVolume(ctx, center, radii, geom.RadialGraphVolumeOptions{}); e == nil || len(v.Components) != 0 {
		t.Fatal("wrong material orientation", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if v, e := result.BoundRadialTubeRegionVolume(cancelled, center, radii, geom.RadialGraphVolumeOptions{}); e != context.Canceled || len(v.Components) != 0 {
		t.Fatal("cancel", e)
	}
	t.Log("original-source radial bound operations", v.Volume.ExactOperations)
}

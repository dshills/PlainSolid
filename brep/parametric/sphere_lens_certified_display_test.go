package parametric

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestCertifiedSphericalLensDilationDisplayRetainsExactRim(t *testing.T) {
	ctx := context.Background()
	a, b := Ball{Radius: 3}, Ball{Center: geom.Vec3{Y: -3}, Radius: 2}
	source, err := NewSpherePairSet(ctx, a, b, Intersection, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	const distance = .1
	if _, _, err := source.DilateSphericalLens(ctx, distance, 1e-9); !errors.Is(err, geom.ErrUnrepresentable) {
		t.Fatalf("strict native entry should refuse this irrational rim: %v", err)
	}
	result, err := source.ProveSphericalLensDilationDisplay(ctx, distance, 1e-9, geom.SphereLensMeasureOptions{})
	if err != nil {
		t.Fatal(err)
	}
	p := result.Proof()
	if p.OriginalCenter[1].Cmp(big.NewRat(-7, 3)) != 0 || p.OriginalRadiusSquared.Cmp(big.NewRat(32, 9)) != 0 ||
		p.ModelDeviation.Sign() <= 0 || p.ModelDeviation.Cmp(big.NewRat(1, 4000000000)) > 0 || p.DisplayCaps != [2]int{2, 0} || p.DisplayBand != 1 {
		t.Fatalf("unexpected display proof: %+v", p)
	}
	deviation, _ := p.ModelDeviation.Float64()
	p.OriginalCenter[1].SetInt64(999)
	p.ModelDeviation.SetInt64(999)
	if result.Proof().OriginalCenter[1].Cmp(big.NewRat(-7, 3)) != 0 || result.Proof().ModelDeviation.Cmp(big.NewRat(1, 1)) >= 0 {
		t.Fatal("display proof aliases caller mutations")
	}
	boundSource, displacement := result.Source()
	if displacement != distance {
		t.Fatal("source binding lost the exact float displacement")
	}
	inside, err := boundSource.ClassifyDilatedSphericalLens(ctx, geom.Vec3{Y: -2}, distance, geom.SphereFullBoundaryDistanceOptions{})
	if err != nil || inside.Location != geom.BoreInside {
		t.Fatalf("source-based exact membership failed: %+v %v", inside, err)
	}
	mesh, err := result.Tessellate(ctx, TessellationOptions{Deflection: .06})
	if err != nil || mesh.ValidateSolid() != nil || len(mesh.Triangles) == 0 {
		t.Fatalf("combined source/display deflection failed: %v, triangles %d", err, len(mesh.Triangles))
	}
	if m, err := result.Tessellate(ctx, TessellationOptions{Deflection: deviation / 2}); !errors.Is(err, ErrBudget) || len(m.Triangles) != 0 {
		t.Fatalf("deflection below model proof was accepted: %v", err)
	}
	props, err := result.DisplaySolid().MassProperties(ctx, IntegrationOptions{})
	if err != nil || props.Volume <= 0 {
		t.Fatal(props, err)
	}
	measure, err := boundSource.MeasureDilatedSphericalLens(ctx, distance, geom.SphereLensMeasureOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i, iv := range []geom.ExactParameterInterval{measure.Volume, measure.Area} {
		value, _ := iv.Lower.Float64()
		if math.Abs([2]float64{props.Volume, props.Area}[i]-value) > 1e-8 {
			t.Fatalf("display measure %d differs from the ideal source: %+v versus %g", i, props, value)
		}
	}
	if failed, err := source.ProveSphericalLensDilationDisplay(ctx, distance, 1e-20, geom.SphereLensMeasureOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || len(failed.DisplaySolid().Faces()) != 0 {
		t.Fatalf("impossible display model budget was accepted: %v", err)
	}
}

func TestCertifiedSphericalLensDilationDisplayNativeRecipe(t *testing.T) {
	ctx := context.Background()
	a := Ball{Center: geom.Vec3{X: 2, Y: 5, Z: -2}, Radius: 3}
	b := Ball{Center: geom.Vec3{X: 2, Y: 8, Z: -2}, Radius: 2}
	source, err := NewSpherePairSet(ctx, a, b, Intersection, 5e-10)
	if err != nil {
		t.Fatal(err)
	}
	result, err := source.ProveSphericalLensDilationDisplay(ctx, .1, 1e-9, geom.SphereLensMeasureOptions{PrecisionBits: 42})
	if err != nil {
		t.Fatal(err)
	}
	var original bytes.Buffer
	if err := result.WriteJSON(ctx, &original, SphericalLensDisplayJSONOptions{}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadCertifiedSphericalLensDilationJSON(ctx, bytes.NewReader(original.Bytes()), SphericalLensDisplayJSONOptions{})
	if err != nil || reloaded.Proof().OriginalCenter[1].Cmp(big.NewRat(22, 3)) != 0 {
		t.Fatalf("source recipe reload: %+v %v", reloaded.Proof(), err)
	}
	var emitted bytes.Buffer
	if err := reloaded.WriteJSON(ctx, &emitted, SphericalLensDisplayJSONOptions{}); err != nil || !bytes.Equal(emitted.Bytes(), original.Bytes()) {
		t.Fatal("source recipe did not reload byte deterministically", err)
	}
	m, err := reloaded.Tessellate(ctx, TessellationOptions{Deflection: .08})
	if err != nil || m.ValidateSolid() != nil {
		t.Fatalf("reloaded source did not yield a closed display: %v", err)
	}
	for _, mutate := range []func(*sphericalLensDisplayNative){
		func(r *sphericalLensDisplayNative) { r.Source.Distance *= 2 },
		func(r *sphericalLensDisplayNative) { r.Source.Balls[1].Center.Y++ },
		func(r *sphericalLensDisplayNative) { r.ModelDeviation = "0" },
		func(r *sphericalLensDisplayNative) { r.OriginalCenter[0] = "201/100" },
	} {
		var edited sphericalLensDisplayNative
		if err := json.Unmarshal(original.Bytes(), &edited); err != nil {
			t.Fatal(err)
		}
		mutate(&edited)
		raw, err := json.Marshal(edited)
		if err != nil {
			t.Fatal(err)
		}
		if failed, err := ReadCertifiedSphericalLensDilationJSON(ctx, bytes.NewReader(raw), SphericalLensDisplayJSONOptions{}); !errors.Is(err, geom.ErrInvalid) || len(failed.DisplaySolid().Faces()) != 0 {
			t.Fatalf("edited certificate was accepted: %v", err)
		}
	}
	var budget bytes.Buffer
	if err := result.WriteJSON(ctx, &budget, SphericalLensDisplayJSONOptions{MaxBytes: 32}); !errors.Is(err, ErrBudget) || budget.Len() != 0 {
		t.Fatalf("partial output on size exhaustion: %v", err)
	}
	if _, err := ReadCertifiedSphericalLensDilationJSON(ctx, bytes.NewReader(original.Bytes()), SphericalLensDisplayJSONOptions{Arithmetic: geom.SurfacePatchOptions{MaxOperations: 1}}); !errors.Is(err, geom.ErrConvergence) {
		t.Fatalf("reload did not re-prove arithmetic: %v", err)
	}
}

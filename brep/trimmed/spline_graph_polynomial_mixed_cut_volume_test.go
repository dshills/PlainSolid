package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativePolynomialMixedCutExactVolumes(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, crossingNumerator := range []int64{1, 3} {
		for axis := 0; axis < 2; axis++ {
			base, a, b := nativePolynomialMixedFixture(t, axis, crossingNumerator, 4)
			for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
				cut, e := CutSplineGraphPolynomialMixedComponents(ctx, base, roofs[0], roofs[1], axis, 1e-9, PolynomialNodeCutOptions{})
				if e != nil {
					t.Fatal(crossingNumerator, axis, order, e)
				}
				want := polynomialMixedCutVolumes(crossingNumerator, order)
				original := checkMixedExactCutVolumes(t, cut, want)
				var raw bytes.Buffer
				if e = cut.WriteJSON(ctx, &raw); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadPolynomialMixedCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, PolynomialNodeCutOptions{})
				if e != nil {
					t.Fatal(e)
				}
				after := checkMixedExactCutVolumes(t, loaded, want)
				if !reflect.DeepEqual(original, after) {
					t.Fatal("reloaded proof changed the exact volume ledger", crossingNumerator, axis, order)
				}
			}
		}
	}
}

func checkMixedExactCutVolumes(t *testing.T, cut *PolynomialMixedCut, want []*big.Rat) geom.SplineGraphPolynomialMixedVolumes {
	t.Helper()
	got, e := cut.ExactComponentVolumes(context.Background(), PolynomialMixedCutVolumeOptions{})
	if e != nil || !got.Arrangement.Proven || !slices.Equal(got.Regions, cut.Regions()) || len(got.Volumes) != len(want) {
		t.Fatal("source-bound exact component ledger", got, e)
	}
	total := new(big.Rat)
	for i, v := range got.Volumes {
		if v.Sign() <= 0 || v.Cmp(want[i]) != 0 {
			t.Fatal("independent exact positive component integral", i, v, want[i])
		}
		total.Add(total, want[i])
	}
	if got.Total.Cmp(total) != 0 {
		t.Fatal("exact material total", got.Total, total)
	}
	return got
}

func TestNativePolynomialMixedCutExactVolumesChangedSource(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	base, a, b := nativePolynomialMixedFixture(t, 1, 1, 4)
	// Scale the physical gap by 3/2 using the exact A-basis coefficients
	// of the lower-degree B roof. Both contact positions stay fixed.
	data := a.Data()
	for i, p := range data.Poles {
		x, y := new(big.Rat).SetFloat64(p.X), new(big.Rat).SetFloat64(p.Y)
		stock := new(big.Rat).Add(big.NewRat(3, 1), new(big.Rat).Quo(new(big.Rat).Mul(x, y), big.NewRat(64, 1)))
		z := new(big.Rat).Sub(new(big.Rat).SetFloat64(p.Z), stock)
		z.Mul(z, big.NewRat(3, 2)).Add(z, stock)
		value, exact := z.Float64()
		if !exact {
			t.Fatal("changed original coefficient must be representable", z)
		}
		data.Poles[i].Z = value
		data.Weights[i] = 2
	}
	roof, e := geom.NewBSplineSurfaceWithExactWeights(data.DegreeU, data.DegreeV, data.CountU, data.CountV, data.Poles, data.KnotsU, data.KnotsV, data.Weights)
	if e != nil {
		t.Fatal(e)
	}
	encoded, e := parametric.EncodeSurface(roof)
	if e != nil {
		t.Fatal(e)
	}
	for order, owners := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
		cut, e := CutSplineGraphPolynomialMixedComponents(ctx, base, owners[0], owners[1], 1, 1e-9, PolynomialNodeCutOptions{})
		if e != nil {
			t.Fatal(e)
		}
		var raw bytes.Buffer
		if e = cut.WriteJSON(ctx, &raw); e != nil {
			t.Fatal(e)
		}
		var recipe polynomialMixedCutRecipe
		if e = json.Unmarshal(raw.Bytes(), &recipe); e != nil {
			t.Fatal(e)
		}
		recipe.Sources[order+1] = encoded
		changed, e := json.Marshal(recipe)
		if e != nil {
			t.Fatal(e)
		}
		loaded, e := ReadPolynomialMixedCutJSON(ctx, bytes.NewReader(changed), JSONLimits{}, PolynomialNodeCutOptions{})
		if e != nil {
			t.Fatal("valid changed physical roof", e)
		}
		want := polynomialMixedCutVolumes(1, order)
		for _, v := range want {
			v.Mul(v, big.NewRat(3, 2))
		}
		checkMixedExactCutVolumes(t, loaded, want)
	}
}

func TestNativePolynomialMixedCutExactVolumeLimits(t *testing.T) {
	ctx := context.Background()
	base, a, b := nativePolynomialMixedFixture(t, 0, 1, 4)
	cut, e := CutSplineGraphPolynomialMixedComponents(ctx, base, b, a, 0, 1e-9, PolynomialNodeCutOptions{})
	if e != nil {
		t.Fatal(e)
	}
	want := polynomialMixedCutVolumes(1, 1)
	report := checkMixedExactCutVolumes(t, cut, want)
	report.Volumes[1].SetInt64(-1)
	report.Regions[1] = 4
	report.Total.SetInt64(-1)
	replayed := checkMixedExactCutVolumes(t, cut, want)
	for _, options := range []PolynomialMixedCutVolumeOptions{
		{Patch: geom.SurfacePatchOptions{MaxPoles: 4}},
		{Proof: geom.GraphPointContactOptions{MaxCells: 1}},
		{Proof: geom.GraphPointContactOptions{MaxOperations: replayed.ExactOperations - 1}},
	} {
		got, e := cut.ExactComponentVolumes(ctx, options)
		if !errors.Is(e, geom.ErrConvergence) || !reflect.DeepEqual(got, geom.SplineGraphPolynomialMixedVolumes{}) {
			t.Fatal("bounded exact volume must fail atomically", options, got, e)
		}
	}
	if out, e := cut.ExactComponentVolumes(ctx, PolynomialMixedCutVolumeOptions{
		Proof: geom.GraphPointContactOptions{MaxOperations: replayed.ExactOperations},
	}); e != nil || !reflect.DeepEqual(out, replayed) {
		t.Fatal("reported exact work boundary", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := cut.ExactComponentVolumes(canceled, PolynomialMixedCutVolumeOptions{}); !errors.Is(e, context.Canceled) || got.Total != nil {
		t.Fatal("canceled exact volume published", e)
	}
	forgedCap := *cut
	forgedCap.material.components = slices.Clone(cut.material.components)
	first := &forgedCap.material.components[0]
	first.faces = slices.Clone(first.faces)
	first.faces[0].Outer = slices.Clone(first.faces[0].Outer)
	first.faces[0].Outer[0].UVStart.X += .125
	if got, e := forgedCap.ExactComponentVolumes(ctx, PolynomialMixedCutVolumeOptions{}); !errors.Is(e, geom.ErrConvergence) || got.Total != nil {
		t.Fatal("exact measure admitted a stale original roof trim", e)
	}

	forged := *cut
	forged.material.contacts = slices.Clone(cut.material.contacts)
	forged.material.contacts[1].Vertices[0]++
	if got, e := forged.ExactComponentVolumes(ctx, PolynomialMixedCutVolumeOptions{}); !errors.Is(e, geom.ErrConvergence) || got.Total != nil {
		t.Fatal("exact measure admitted a stale native contact ledger", e)
	}
}

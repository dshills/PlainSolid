package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func anisotropicMassFixture(t *testing.T, asymmetric bool, translation geom.Vec3) Solid {
	t.Helper()
	ctx := context.Background()
	bands := []*geom.WrappedSurfacePatch{anisotropicStationBand(t, 0, 1, [2]float64{3, 1}, [2]float64{1, 2}, .75), anisotropicStationBand(t, 1, 3, [2]float64{1, 2}, [2]float64{2, 1}, .75)}
	for i, w := range bands {
		recipe, e := w.ExportRecipe()
		if e != nil {
			t.Fatal(e)
		}
		d := recipe.Surface.Data()
		if asymmetric {
			d.Weights[2], d.Weights[3] = .5, .5
		}
		for j := range d.Poles {
			d.Poles[j] = d.Poles[j].Add(translation)
		}
		s, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if e != nil {
			t.Fatal(e)
		}
		patch, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		bands[i], _, e = geom.NewWrappedSurfacePatch(ctx, patch, geom.WrappedSurfaceOptions{WrapU: true})
		if e != nil {
			t.Fatal(e)
		}
	}
	body, _, e := NewWrappedAnisotropicStationLoft(ctx, bands, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = WriteJSON(&raw, body); e != nil {
		t.Fatal(e)
	}
	body, e = ReadJSONContext(ctx, &raw, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	return body
}

func TestWrappedAnisotropicMass(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, asymmetric := range []bool{false, true} {
		translation := geom.Vec3{}
		if asymmetric {
			translation = geom.Vec3{X: -2, Y: 3, Z: -4}
		}
		body := anisotropicMassFixture(t, asymmetric, translation)
		var previous WrappedAnisotropicStationMassBounds
		for _, denom := range []int64{10, 1000} {
			p, e := body.BoundWrappedAnisotropicStationMass(ctx, WrappedRationalStationMassOptions{VolumeTolerance: r(1, denom)})
			if e != nil {
				t.Fatal(e)
			}
			expectedZ := r(10, 1)
			if asymmetric {
				expectedZ = r(-56, 3)
			}
			for i, pair := range [][2]*big.Rat{{p.AxialVolumeFactor, r(43, 6)}, {p.AxialFirstXYFactors[0], r(37, 3)}, {p.AxialFirstXYFactors[1], r(32, 3)}, {p.AxialFirstZFactor, expectedZ}} {
				if pair[0].Cmp(pair[1]) != 0 {
					t.Fatal("independent polynomial integral", i, pair)
				}
			}
			weights := [4]float64{.75, .75, .75, .75}
			if asymmetric {
				weights[0] = .5
			}
			area, mx, my := independentStationProfileMoments(weights, 8192)
			if !asymmetric {
				mx, my = 0, 0
			}
			volume := area * 43 / 6
			center := [3]float64{.25 + translation.X + (mx/area)*74/43, -.5 + translation.Y + (my/area)*64/43, 60./43 + translation.Z}
			first := [3]float64{(.25+translation.X)*volume + mx*37/3, (-.5+translation.Y)*volume + my*32/3, center[2] * volume}
			encloses := func(name string, q WrappedStationMassInterval, v float64) {
				t.Helper()
				x := new(big.Rat).SetFloat64(v)
				if q.Lower.Cmp(x) > 0 || q.Upper.Cmp(x) < 0 {
					t.Fatalf("%s: [%s,%s] misses %.16g", name, q.Lower, q.Upper, v)
				}
			}
			encloses("volume", p.Volume, volume)
			for axis := 0; axis < 3; axis++ {
				encloses("first moment", p.FirstMoments[axis], first[axis])
				if axis < 2 {
					encloses("centroid", p.Centroid[axis], center[axis])
				}
			}
			wantZ := r(60, 43)
			if asymmetric {
				wantZ = r(-112, 43)
			}
			if p.Centroid[2].Lower.Cmp(wantZ) != 0 || p.Centroid[2].Upper.Cmp(wantZ) != 0 {
				t.Fatal("axial centroid not exact")
			}
			if !asymmetric {
				for axis, want := range []*big.Rat{r(1, 4), r(-1, 2)} {
					if p.Centroid[axis].Lower.Cmp(want) != 0 || p.Centroid[axis].Upper.Cmp(want) != 0 {
						t.Fatal("symmetry centroid not exact")
					}
				}
			}
			if asymmetric && (p.FirstMoments[0].Upper.Sign() >= 0 || p.FirstMoments[1].Lower.Sign() <= 0 || p.FirstMoments[2].Upper.Sign() >= 0) {
				t.Fatal("signed transfer")
			}
			if new(big.Rat).Sub(p.Volume.Upper, p.Volume.Lower).Cmp(r(1, denom)) > 0 {
				t.Fatal("width contract")
			}
			if previous.Volume.Lower != nil {
				if p.Volume.Lower.Cmp(previous.Volume.Lower) < 0 || p.Volume.Upper.Cmp(previous.Volume.Upper) > 0 || p.Segments <= previous.Segments {
					t.Fatal("refinement not nested")
				}
			}
			previous = p
		}
		old := new(big.Rat).Set(previous.Volume.Lower)
		previous.ProfileArea.Lower.SetInt64(100)
		previous.Source.Center[0].SetInt64(100)
		if previous.Volume.Lower.Cmp(old) != 0 {
			t.Fatal("result field alias")
		}
		fresh, e := body.ValidateWrappedAnisotropicStationLoft(ctx, geom.SurfacePatchOptions{})
		if e != nil || fresh.Center[0].Cmp(new(big.Rat).SetFloat64(.25+translation.X)) != 0 {
			t.Fatal("mutated source", e)
		}
	}
}

func TestWrappedAnisotropicMassLimits(t *testing.T) {
	ctx := context.Background()
	body := anisotropicMassFixture(t, true, geom.Vec3{})
	for _, o := range []WrappedRationalStationMassOptions{
		{VolumeTolerance: big.NewRat(1, 1000000), MaxSegments: 4},
		{VolumeTolerance: big.NewRat(1, 1000), MaxOperations: 1},
		{VolumeTolerance: big.NewRat(1, 1000000000), MaxCoefficientBits: 64},
		{VolumeTolerance: big.NewRat(-1, 1)},
	} {
		p, e := body.BoundWrappedAnisotropicStationMass(ctx, o)
		if e == nil || p.Volume.Lower != nil || p.Source.Complete {
			t.Fatal("partial quota result", e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if p, e := body.BoundWrappedAnisotropicStationMass(cancelled, WrappedRationalStationMassOptions{VolumeTolerance: big.NewRat(1, 10)}); !errors.Is(e, context.Canceled) || p.Volume.Lower != nil {
		t.Fatal(p, e)
	}
}

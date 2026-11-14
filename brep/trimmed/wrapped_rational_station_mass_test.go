package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func independentStationProfileMoments(weights [4]float64, n int) (area, mx, my float64) {
	for quadrant, w := range weights {
		for i := 0; i <= n; i++ {
			t := float64(i) / float64(n)
			s := 1 - t
			b := 2 * w * t * s
			db := 2 * w * (1 - 2*t)
			den := s*s + b + t*t
			dd := -2*s + db + 2*t
			nx, ny := s*s+b, b+t*t
			dx, dy := ((-2*s+db)*den-nx*dd)/(den*den), ((db+2*t)*den-ny*dd)/(den*den)
			x, y := nx/den, ny/den
			for j := 0; j < quadrant; j++ {
				x, y = -y, x
				dx, dy = -dy, dx
			}
			c := 2.
			if i == 0 || i == n {
				c = 1
			} else if i%2 == 1 {
				c = 4
			}
			factor := c / (3 * float64(n))
			area += factor * (x*dy - y*dx) / 2
			mx += factor * x * x * dy / 2
			my -= factor * y * y * dx / 2
		}
	}
	return
}

func stationMassFixture(t *testing.T, asymmetric bool, shift float64) Solid {
	t.Helper()
	ctx := context.Background()
	bands := []*geom.WrappedSurfacePatch{rationalStationBand(t, shift, shift+1, 3, 1, .75), rationalStationBand(t, shift+1, shift+3, 1, 2, .75)}
	if asymmetric {
		for i, w := range bands {
			bs, e := w.FundamentalPatch().ToBSplineSurface()
			if e != nil {
				t.Fatal(e)
			}
			d := bs.Data()
			d.Weights[2], d.Weights[3] = .5, .5
			bs, e = geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
			if e != nil {
				t.Fatal(e)
			}
			p, e := geom.NewSurfacePatch(bs, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			bands[i], _, e = geom.NewWrappedSurfacePatch(ctx, p, geom.WrappedSurfaceOptions{WrapU: true})
			if e != nil {
				t.Fatal(e)
			}
		}
	}
	body, _, e := NewWrappedRationalStationLoft(ctx, bands, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var buf bytes.Buffer
	if e = WriteJSON(&buf, body); e != nil {
		t.Fatal(e)
	}
	body, e = ReadJSONContext(ctx, &buf, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	return body
}

func TestRationalStationCertifiedMass(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, asym := range []bool{false, true} {
		t.Run(map[bool]string{false: "symmetric", true: "asymmetric-signed"}[asym], func(t *testing.T) {
			shift := 0.
			weights := [4]float64{.75, .75, .75, .75}
			if asym {
				weights[0] = .5
				shift = -4
			}
			body := stationMassFixture(t, asym, shift)
			o := WrappedRationalStationMassOptions{VolumeTolerance: r(1, 25), MaxCoefficientBits: 65536}
			coarse, e := body.BoundWrappedRationalStationMass(ctx, o)
			if e != nil {
				t.Fatal(e)
			}
			o.VolumeTolerance = r(1, 100)
			fine, e := body.BoundWrappedRationalStationMass(ctx, o)
			if e != nil {
				t.Fatal(e)
			}
			if fine.Volume.Lower.Cmp(coarse.Volume.Lower) < 0 || fine.Volume.Upper.Cmp(coarse.Volume.Upper) > 0 || fine.Segments <= coarse.Segments {
				t.Fatal("nonmonotone exact enclosure")
			}
			if new(big.Rat).Sub(fine.Volume.Upper, fine.Volume.Lower).Cmp(o.VolumeTolerance) > 0 {
				t.Fatal("width not proved")
			}
			area, mx, my := independentStationProfileMoments(weights, 4096)
			a2, x2, y2 := independentStationProfileMoments(weights, 2048)
			if math.Abs(area-a2)+math.Abs(mx-x2)+math.Abs(my-y2) > 1e-10 {
				t.Fatal("independent integral convergence")
			}
			contains := func(q WrappedStationMassInterval, want float64) {
				t.Helper()
				lo, _ := q.Lower.Float64()
				hi, _ := q.Upper.Float64()
				if want < lo-1e-10 || want > hi+1e-10 {
					t.Fatal("independent integral outside certificate", lo, want, hi)
				}
			}
			contains(fine.ProfileArea, area)
			contains(fine.Volume, 9*area)
			zFactor := 71./6 + 9*shift
			for i, want := range []float64{.25*9*area + 17.5*mx, -.5*9*area + 17.5*my, zFactor * area} {
				contains(fine.FirstMoments[i], want)
			}
			for i, want := range []float64{.25 + 35*mx/(18*area), -.5 + 35*my/(18*area), shift + 71./54} {
				contains(fine.Centroid[i], want)
			}
			wantZ := r(71, 54)
			if asym {
				wantZ = r(-145, 54)
			}
			if fine.Centroid[2].Lower.Cmp(wantZ) != 0 || fine.Centroid[2].Upper.Cmp(wantZ) != 0 || fine.AxialVolumeFactor.Cmp(r(9, 1)) != 0 || fine.AxialCubicFactor.Cmp(r(35, 2)) != 0 {
				t.Fatal("exact axial transfer")
			}
			if !asym {
				if fine.Centroid[0].Lower.Cmp(r(1, 4)) != 0 || fine.Centroid[0].Upper.Cmp(r(1, 4)) != 0 || fine.Centroid[1].Lower.Cmp(r(-1, 2)) != 0 || fine.Centroid[1].Upper.Cmp(r(-1, 2)) != 0 {
					t.Fatal("central symmetry not exact")
				}
			} else if fine.Centroid[0].Upper.Cmp(r(1, 4)) >= 0 || fine.Centroid[1].Upper.Cmp(r(-1, 2)) >= 0 {
				t.Fatal("asymmetric first moments erased")
			}
			if len(fine.Source.History.WallFaces) != 2 || len(fine.Source.History.StationRims) != 3 {
				t.Fatal("original native history")
			}
			denied := o
			denied.MaxSegments = 4
			if got, e := body.BoundWrappedRationalStationMass(ctx, denied); e == nil || got.Volume.Lower != nil {
				t.Fatal("segment quota", e)
			}
			denied = o
			denied.MaxOperations = 1
			if got, e := body.BoundWrappedRationalStationMass(ctx, denied); e == nil || got.Volume.Lower != nil {
				t.Fatal("operation quota", e)
			}
			denied = o
			denied.MaxCoefficientBits = 64
			if got, e := body.BoundWrappedRationalStationMass(ctx, denied); e == nil || got.Volume.Lower != nil {
				t.Fatal("coefficient quota", e)
			}
			denied = o
			denied.Source.MaxOperations = 1
			if got, e := body.BoundWrappedRationalStationMass(ctx, denied); e == nil || got.Volume.Lower != nil {
				t.Fatal("source authentication quota", e)
			}
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			if got, e := body.BoundWrappedRationalStationMass(cancelled, o); e == nil || got.Volume.Lower != nil {
				t.Fatal("cancel")
			}
			t.Log("segments", coarse.Segments, fine.Segments, "operations", fine.ExactOperations, "area", area, "centroid shift", 35*mx/(18*area))
		})
	}
}

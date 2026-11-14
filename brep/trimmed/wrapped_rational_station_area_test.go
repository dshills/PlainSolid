package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func independentStationWallArea(t *testing.T, wall *geom.WrappedSurfacePatch, steps int) float64 {
	t.Helper()
	patch := wall.FundamentalPatch()
	spline, err := patch.ToBSplineSurface()
	if err != nil {
		t.Fatal(err)
	}
	u, _ := patch.KnotBreaks()
	domain := patch.Domain()
	simpson := func(i, n int) float64 {
		if i == 0 || i == n {
			return 1
		}
		if i%2 == 0 {
			return 2
		}
		return 4
	}
	var area float64
	for k := 0; k+1 < len(u); k++ {
		du := (u[k+1] - u[k]) / float64(steps)
		dv := (domain.VMax - domain.VMin) / 8
		for i := 0; i <= steps; i++ {
			x := u[k] + du*float64(i)
			if i == 0 || i == steps {
				// Use this quadrant's one-sided partial at a repeated knot.
				x = math.Nextafter(x, (u[k]+u[k+1])/2)
			}
			for j := 0; j <= 8; j++ {
				a, b, err := spline.Partials(x, domain.VMin+dv*float64(j))
				if err != nil {
					t.Fatal(err)
				}
				area += du * dv / 9 * simpson(i, steps) * simpson(j, 8) * a.Cross(b).Length()
			}
		}
	}
	return area
}

func checkStationArea(t *testing.T, body Solid, result WrappedRationalStationAreaBounds, capAreas map[int]float64, widths *big.Rat) {
	t.Helper()
	seen := make(map[int]bool)
	lower, upper := new(big.Rat), new(big.Rat)
	for _, face := range result.Faces {
		if face.Face < 0 || face.Face >= len(body.faces) || seen[face.Face] {
			t.Fatal("lost original area ownership", face.Face)
		}
		seen[face.Face] = true
		want, cap := capAreas[face.Face]
		if !cap {
			wall := body.faces[face.Face].Surface.(*geom.WrappedSurfacePatch)
			want = independentStationWallArea(t, wall, 512)
			coarse := independentStationWallArea(t, wall, 256)
			if math.Abs(want-coarse) > 2e-8 {
				t.Fatal("independent original patch integral failed to converge", face.Face, coarse, want)
			}
		}
		lo, _ := face.Area.Lower.Float64()
		hi, _ := face.Area.Upper.Float64()
		if lo > want || hi < want {
			t.Fatal("independent integral outside face area certificate", face.Face, lo, want, hi)
		}
		lower.Add(lower, face.Area.Lower)
		upper.Add(upper, face.Area.Upper)
	}
	if len(seen) != len(body.faces) || lower.Cmp(result.Total.Lower) != 0 || upper.Cmp(result.Total.Upper) != 0 || new(big.Rat).Sub(upper, lower).Cmp(widths) > 0 {
		t.Fatal("invalid total boundary enclosure", result)
	}
	t.Log("faces", len(seen), "profile spans", result.Segments, "exact operations", result.ExactOperations)
}

func TestRationalStationCertifiedBoundaryArea(t *testing.T) {
	ctx := context.Background()
	solid := stationMassFixture(t, true, -4)
	proof, err := solid.ValidateWrappedRationalStationLoft(ctx, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	profile, _, _ := independentStationProfileMoments([4]float64{.5, .75, .75, .75}, 4096)
	caps := map[int]float64{proof.History.CapFaces[0]: 9 * profile, proof.History.CapFaces[1]: 4 * profile}
	options := WrappedRationalStationAreaOptions{AreaTolerance: big.NewRat(1, 16), MaxCoefficientBits: 65536}
	coarse, err := solid.BoundWrappedRationalStationArea(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	options.AreaTolerance = big.NewRat(1, 64)
	precise, err := solid.BoundWrappedRationalStationArea(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	if precise.Total.Lower.Cmp(coarse.Total.Lower) < 0 || precise.Total.Upper.Cmp(coarse.Total.Upper) > 0 || precise.Segments <= coarse.Segments {
		t.Fatal("refinement widened area enclosure")
	}
	checkStationArea(t, solid, precise, caps, options.AreaTolerance)
	for _, adjust := range []func(*WrappedRationalStationAreaOptions){
		func(o *WrappedRationalStationAreaOptions) { o.MaxSegments = precise.Segments - 1 },
		func(o *WrappedRationalStationAreaOptions) { o.MaxOperations = precise.ExactOperations - 1 },
		func(o *WrappedRationalStationAreaOptions) { o.MaxCoefficientBits = 64 },
		func(o *WrappedRationalStationAreaOptions) { o.Source.MaxOperations = 1 },
	} {
		capped := options
		adjust(&capped)
		denied, err := solid.BoundWrappedRationalStationArea(ctx, capped)
		if !errors.Is(err, geom.ErrConvergence) || denied.Total.Lower != nil || len(denied.Faces) != 0 {
			t.Fatal("area budget must fail atomically", denied, err)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if denied, err := solid.BoundWrappedRationalStationArea(cancelled, options); !errors.Is(err, context.Canceled) || len(denied.Faces) != 0 {
		t.Fatal("cancelled area must fail atomically", denied, err)
	}
}

func TestRationalStationBoreCertifiedBoundaryArea(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	outer, _, err := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{
		rationalStationBand(t, 0, 1, 3, 2, .75), rationalStationBand(t, 1, 3, 2, 4, .75),
	}, 1e-8, o)
	if err != nil {
		t.Fatal(err)
	}
	inner, _, err := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{
		rationalStationBand(t, 0, 2, 1, 1, .75), rationalStationBand(t, 2, 3, 1, 2, .75),
	}, 1e-8, o)
	if err != nil {
		t.Fatal(err)
	}
	bore, history, err := CombineCoaxialWrappedRationalStationLofts(ctx, outer, inner, parametric.Difference, o)
	if err != nil {
		t.Fatal(err)
	}
	var native bytes.Buffer
	if err := WriteJSON(&native, bore); err != nil {
		t.Fatal(err)
	}
	bore, err = ReadJSONContext(ctx, &native, JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	options := WrappedRationalStationAreaOptions{AreaTolerance: big.NewRat(1, 20), MaxCoefficientBits: 65536}
	result, err := bore.BoundWrappedRationalStationBoreArea(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	profile, _, _ := independentStationProfileMoments([4]float64{.75, .75, .75, .75}, 4096)
	caps := map[int]float64{history.ResultCaps[0]: 8 * profile, history.ResultCaps[1]: 12 * profile}
	checkStationArea(t, bore, result, caps, options.AreaTolerance)
	if denied, err := bore.BoundWrappedRationalStationArea(ctx, options); !errors.Is(err, geom.ErrConvergence) || len(denied.Faces) != 0 {
		t.Fatal("filled area must reject original annular topology", denied, err)
	}
}

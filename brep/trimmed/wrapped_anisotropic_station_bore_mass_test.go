//go:build research

package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func anisotropicMeasuredBore(t *testing.T) Solid {
	t.Helper()
	ctx := context.Background()
	outer := anisotropicMassFixture(t, true, geom.Vec3{Z: -4})
	inner := anisotropicBoreInner(t, .75, [2]float64{.375, .25})
	p, bands, e := inner.validateWrappedAnisotropicStationLoft(ctx, geom.SurfacePatchOptions{})
	_ = p
	if e != nil {
		t.Fatal(e)
	}
	for i, w := range bands {
		recipe, e := w.ExportRecipe()
		if e != nil {
			t.Fatal(e)
		}
		d := recipe.Surface.Data()
		d.Weights[2], d.Weights[3] = .5, .5
		for j := range d.Poles {
			d.Poles[j].Z -= 4
		}
		raw, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if e != nil {
			t.Fatal(e)
		}
		patch, e := geom.NewSurfacePatch(raw, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		bands[i], _, e = geom.NewWrappedSurfacePatch(ctx, patch, geom.WrappedSurfaceOptions{WrapU: true})
		if e != nil {
			t.Fatal(e)
		}
	}
	inner, _, e = NewWrappedAnisotropicStationLoft(ctx, bands, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := CutWrappedAnisotropicStationLofts(ctx, outer, inner, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// Reindex every face, then require independent native reload authentication.
	faces := s.Faces()
	for i, j := 0, len(faces)-1; i < j; i, j = i+1, j-1 {
		faces[i], faces[j] = faces[j], faces[i]
	}
	s, e = NewSolidContext(ctx, s.Vertices(), s.Edges(), faces, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	var buf bytes.Buffer
	if e = WriteJSON(&buf, s); e != nil {
		t.Fatal(e)
	}
	s, e = ReadJSONContext(ctx, &buf, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	return s
}

// Independent Green integrals over authored rational quadratic arcs, followed
// by numerical Simpson integration over each wall's distinct axial grid.
func independentAnisotropicBoreMoments(n int) [10]float64 {
	powers := [10][3]int{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 0, 1}, {2, 0, 0}, {1, 1, 0}, {0, 2, 0}, {1, 0, 1}, {0, 1, 1}, {0, 0, 2}}
	var result [10]float64
	for k, p := range powers {
		prof := 0.0
		for quadrant, w := range []float64{.5, .75, .75, .75} {
			sum := 0.0
			for i := 0; i <= n; i++ {
				t := float64(i) / float64(n)
				a := 1 - t
				den := a*a + 2*w*a*t + t*t
				dd := -2*a + 2*w*(1-2*t) + 2*t
				nx, ny := a*a+2*w*a*t, 2*w*a*t+t*t
				dx, dy := -2*a+2*w*(1-2*t), 2*w*(1-2*t)+2*t
				x, y := nx/den, ny/den
				vx, vy := (dx*den-nx*dd)/(den*den), (dy*den-ny*dd)/(den*den)
				for q := 0; q < quadrant; q++ {
					x, y = -y, x
					vx, vy = -vy, vx
				}
				f := math.Pow(x, float64(p[0]+1)) * math.Pow(y, float64(p[1])) * vy / float64(p[0]+1)
				weight := 2.0
				if i == 0 || i == n {
					weight = 1
				} else if i%2 == 1 {
					weight = 4
				}
				sum += weight * f
			}
			prof += sum / (3 * float64(n))
		}
		// z is relative to lower cap -4. Source cuts are 0/1/3 and0/2/3.
		for side, stations := range [][][3]float64{{{0, 3, 1}, {1, 1, 2}, {3, 2, 1}}, {{0, .5, .25}, {2, .25, .5}, {3, .375, .25}}} {
			sign := 1.0
			if side == 1 {
				sign = -1
			}
			for j := 0; j+1 < len(stations); j++ {
				a, c := stations[j], stations[j+1]
				sum := 0.0
				for i := 0; i <= n; i++ {
					t := float64(i) / float64(n)
					z := a[0] + t*(c[0]-a[0])
					x := a[1] + t*(c[1]-a[1])
					y := a[2] + t*(c[2]-a[2])
					f := math.Pow(x, float64(p[0]+1)) * math.Pow(y, float64(p[1]+1)) * math.Pow(z, float64(p[2]))
					w := 2.0
					if i == 0 || i == n {
						w = 1
					} else if i%2 == 1 {
						w = 4
					}
					sum += w * f
				}
				result[k] += sign * prof * sum * (c[0] - a[0]) / (3 * float64(n))
			}
		}
	}
	return result
}

func TestAnisotropicBoreMassAndInertia(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	s := anisotropicMeasuredBore(t)
	o := WrappedAnisotropicBoreMassOptions{VolumeTolerance: big.NewRat(1, 100), InertiaTolerance: big.NewRat(1, 10), MaxOperations: 4000000}
	p, e := s.BoundWrappedAnisotropicStationBoreMass(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	truth := independentAnisotropicBoreMoments(2048)
	other := independentAnisotropicBoreMoments(1024)
	for i := range truth {
		if math.Abs(truth[i]-other[i]) > 1e-8 {
			t.Fatal("independent convergence", i)
		}
	}
	check := func(q WrappedStationMassInterval, want float64) {
		t.Helper()
		lo, _ := q.Lower.Float64()
		hi, _ := q.Upper.Float64()
		if want < lo-1e-10 || want > hi+1e-10 {
			t.Fatalf("[%g,%g] misses %g", lo, hi, want)
		}
	}
	check(p.Volume, truth[0])
	first := [3]float64{truth[1], truth[2], truth[3]}
	raw := [3][3]float64{{truth[4], truth[5], truth[7]}, {truth[5], truth[6], truth[8]}, {truth[7], truth[8], truth[9]}}
	var covariance [3][3]float64
	for i := 0; i < 3; i++ {
		check(p.FirstMoments[i], first[i])
		origin, _ := p.Origin[i].Float64()
		check(p.Centroid[i], origin+first[i]/truth[0])
		for j := 0; j < 3; j++ {
			check(p.SecondMoments[i][j], raw[i][j])
			covariance[i][j] = raw[i][j] - first[i]*first[j]/truth[0]
		}
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			want := -covariance[i][j]
			if i == j {
				want = covariance[(i+1)%3][(i+1)%3] + covariance[(i+2)%3][(i+2)%3]
			}
			check(p.CentroidalInertia[i][j], want)
			q := p.CentroidalInertia[i][j]
			if new(big.Rat).Sub(q.Upper, q.Lower).Cmp(o.InertiaTolerance) > 0 {
				t.Fatal("width")
			}
		}
	}
	for _, id := range p.Source.InnerWalls {
		if !s.faces[id].Reversed {
			t.Fatal("inner ownership")
		}
	}
	t.Log("segments/operations", p.Segments, p.ExactOperations)
	// Returned proof and scalar intervals are owned, not mutable source state.
	p.Source.Outer.Scales[0][0].SetInt64(999)
	p.SecondMoments[0][0].Lower.SetInt64(-999)
	o.MaxOperations = p.ExactOperations
	if _, e = s.BoundWrappedAnisotropicStationBoreMass(ctx, o); e != nil {
		t.Fatal("exact quota", e)
	}
	o.MaxOperations--
	fail, e := s.BoundWrappedAnisotropicStationBoreMass(ctx, o)
	if e == nil || fail.Volume.Lower != nil || fail.Source.Complete {
		t.Fatal("non-atomic quota", e)
	}
	o.MaxOperations = 4000000
	o.MaxSegments = 4
	if fail, e = s.BoundWrappedAnisotropicStationBoreMass(ctx, o); e == nil || fail.Volume.Lower != nil {
		t.Fatal("segments", e)
	}
	cancel, c := context.WithCancel(ctx)
	c()
	if _, e = s.BoundWrappedAnisotropicStationBoreMass(cancel, o); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

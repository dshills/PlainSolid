package geom_test

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func coverageCircleSources(t *testing.T) ([2]*geom.SurfacePatch, *geom.SplineGraphRadialContours) {
	return coverageCircleRadius(t, big.NewRat(1, 4))
}
func coverageCircleRadius(t *testing.T, radius *big.Rat) ([2]*geom.SurfacePatch, *geom.SplineGraphRadialContours) {
	t.Helper()
	var a, b [][4]*big.Rat
	q := [3]*big.Rat{big.NewRat(1, 4), big.NewRat(-1, 4), big.NewRat(1, 4)}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			z := new(big.Rat).Sub(new(big.Rat).Add(q[i], q[j]), new(big.Rat).Mul(radius, radius))
			a = append(a, [4]*big.Rat{big.NewRat(int64(i), 2), big.NewRat(int64(j), 2), new(big.Rat), big.NewRat(1, 1)})
			b = append(b, [4]*big.Rat{big.NewRat(int64(i), 2), big.NewRat(int64(j), 2), z, big.NewRat(1, 1)})
		}
	}
	var p [2]*geom.SurfacePatch
	var e error
	p[0], e = geom.NewExactHomogeneousBezierSurface(context.Background(), 2, 2, a, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p[1], e = geom.NewExactHomogeneousBezierSurface(context.Background(), 2, 2, b, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := geom.CertifySplineGraphRadialContours(context.Background(), p[0], p[1], [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, []*big.Rat{radius}, geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p, r
}
func coverageCircleTubes(t *testing.T, p [2]*geom.SurfacePatch) []geom.VerifiedSurfaceTube {
	return coverageCircleTubesOverlap(t, p, .0078125)
}
func coverageCircleTubesOverlap(t *testing.T, p [2]*geom.SurfacePatch, overlap float64) []geom.VerifiedSurfaceTube {
	t.Helper()
	var tubes []geom.VerifiedSurfaceTube
	// Thirty-two arcs are proposed using square roots, then independently proved. Values are
	// original represented coordinates; each proposal is independently proved.
	for axis := 0; axis < 2; axis++ {
		for _, sign := range []int{-1, 1} {
			for part := 0; part < 8; part++ {
				lo := .3125 + float64(part)*.046875 - overlap
				hi := .3125 + float64(part+1)*.046875 + overlap
				a0, a1 := math.Abs(lo-.5), math.Abs(hi-.5)
				near, far := math.Min(a0, a1), math.Max(a0, a1)
				if lo <= .5 && hi >= .5 {
					near = 0
				}
				low, high := .5+math.Sqrt(.0625-far*far), .5+math.Sqrt(.0625-near*near)
				if sign < 0 {
					low, high = 1-high, 1-low
				}
				low -= .01
				high += .01
				da := geom.ParameterRectangle{UMin: lo - .01, UMax: hi + .01, VMin: low, VMax: high}
				db := geom.ParameterRectangle{UMin: lo, UMax: hi, VMin: low, VMax: high}
				if axis == 1 {
					da = geom.ParameterRectangle{UMin: low, UMax: high, VMin: lo - .01, VMax: hi + .01}
					db = geom.ParameterRectangle{UMin: low, UMax: high, VMin: lo, VMax: hi}
				}
				a, e := p[0].Trim(da)
				if e != nil {
					t.Fatal(e)
				}
				b, e := p[1].Trim(db)
				if e != nil {
					t.Fatal(e)
				}
				tube, e := geom.NewVerifiedSurfaceTube(a, b, geom.SurfacePairTubeOptions{FreeAxis: axis + 2, MaxOperations: 8000000})
				if e != nil {
					t.Fatal("uniform arc", axis, sign, part, e)
				}
				tubes = append(tubes, tube)
			}
		}
	}
	return tubes
}
func TestExactRadialTubeCoverage(t *testing.T) {
	p, r := coverageCircleSources(t)
	tubes := coverageCircleTubes(t, p)
	cover, e := r.CertifyTubeCoverage(context.Background(), tubes, geom.RadialTubeCoverageOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if cover.TubeCount() != 32 {
		t.Fatal("tube occurrences")
	}
	t.Log("coverage operations", cover.ExactOperations(), "neighbor tests", cover.NeighborTests())
}

func TestExactRadialTubeCoverageAtomic(t *testing.T) {
	slowtest.Skip(t)
	p, r := coverageCircleSources(t)
	tubes := coverageCircleTubes(t, p)
	c, e := r.CertifyTubeCoverage(context.Background(), tubes, geom.RadialTubeCoverageOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name    string
		tubes   []geom.VerifiedSurfaceTube
		options geom.RadialTubeCoverageOptions
	}{
		{"partial", tubes[:1], geom.RadialTubeCoverageOptions{}},
		{"tube budget", tubes, geom.RadialTubeCoverageOptions{MaxTubes: 31}},
		{"neighbor budget", tubes, geom.RadialTubeCoverageOptions{MaxNeighborTests: c.NeighborTests() - 1}},
		{"operation budget", tubes, geom.RadialTubeCoverageOptions{MaxOperations: c.ExactOperations() - 1}},
		{"endpoint only", coverageCircleTubesOverlap(t, p, 0), geom.RadialTubeCoverageOptions{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, e := r.CertifyTubeCoverage(context.Background(), tc.tubes, tc.options)
			if got != nil || !errors.Is(e, geom.ErrConvergence) {
				t.Fatalf("non-atomic rejection: %v %v", got, e)
			}
		})
	}
	if _, e = r.CertifyTubeCoverage(context.Background(), tubes, geom.RadialTubeCoverageOptions{MaxOperations: c.ExactOperations(), MaxNeighborTests: c.NeighborTests()}); e != nil {
		t.Fatal("exact budgets", e)
	}
	_, other := coverageCircleRadius(t, big.NewRat(1, 8))
	if got, e := other.CertifyTubeCoverage(context.Background(), tubes, geom.RadialTubeCoverageOptions{}); got != nil || !errors.Is(e, geom.ErrDomain) {
		t.Fatalf("other radius/source accepted: %v %v", got, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, e := r.CertifyTubeCoverage(ctx, tubes, geom.RadialTubeCoverageOptions{}); got != nil || !errors.Is(e, context.Canceled) {
		t.Fatal("cancellation", got, e)
	}
	owners := c.RadiusOwners()
	owners[0] = 99
	neighbors := c.EndpointNeighbors()
	neighbors[0] = [2]int{-1, -1}
	if c.RadiusOwners()[0] != 0 || c.EndpointNeighbors()[0][0] < 0 {
		t.Fatal("mutable public slices")
	}
	for i, j := 0, len(tubes)-1; i < j; i, j = i+1, j-1 {
		tubes[i], tubes[j] = tubes[j], tubes[i]
	}
	if _, e := r.CertifyTubeCoverage(context.Background(), tubes, geom.RadialTubeCoverageOptions{}); e != nil {
		t.Fatal("reordered occurrences", e)
	}
}

// Independent power expansion of two circles: a tube on one exact factor
// cannot establish coverage of the other factor on the very same supports.
func TestExactRadialTubeCoverageMissingRadius(t *testing.T) {
	var power [5][5]*big.Rat
	for i := range power {
		for j := range power[i] {
			power[i][j] = new(big.Rat)
		}
	}
	type term struct {
		i, j int
		q    *big.Rat
	}
	factor := func(r *big.Rat) []term {
		return []term{{2, 0, big.NewRat(1, 1)}, {0, 2, big.NewRat(1, 1)}, {1, 0, big.NewRat(-1, 1)}, {0, 1, big.NewRat(-1, 1)}, {0, 0, new(big.Rat).Sub(big.NewRat(1, 2), new(big.Rat).Mul(r, r))}}
	}
	for _, a := range factor(big.NewRat(1, 8)) {
		for _, b := range factor(big.NewRat(1, 4)) {
			power[a.i+b.i][a.j+b.j].Add(power[a.i+b.i][a.j+b.j], new(big.Rat).Mul(a.q, b.q))
		}
	}
	choose := func(n, k int) int64 {
		v := int64(1)
		for j := 1; j <= k; j++ {
			v = v * int64(n-j+1) / int64(j)
		}
		return v
	}
	var controls [2][][4]*big.Rat
	for i := 0; i <= 4; i++ {
		for j := 0; j <= 4; j++ {
			z := new(big.Rat)
			for k := 0; k <= i; k++ {
				for l := 0; l <= j; l++ {
					q := new(big.Rat).Mul(power[k][l], big.NewRat(choose(i, k)*choose(j, l), choose(4, k)*choose(4, l)))
					z.Add(z, q)
				}
			}
			for owner := 0; owner < 2; owner++ {
				h := new(big.Rat)
				if owner == 1 {
					h.Set(z)
				}
				controls[owner] = append(controls[owner], [4]*big.Rat{big.NewRat(int64(i), 4), big.NewRat(int64(j), 4), h, big.NewRat(1, 1)})
			}
		}
	}
	var p [2]*geom.SurfacePatch
	for owner := range p {
		var e error
		p[owner], e = geom.NewExactHomogeneousBezierSurface(context.Background(), 4, 4, controls[owner], geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	r, e := geom.CertifySplineGraphRadialContours(context.Background(), p[0], p[1], [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, []*big.Rat{big.NewRat(1, 8), big.NewRat(1, 4)}, geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var tubes []geom.VerifiedSurfaceTube
	for _, radius := range []float64{.25, .125} {
		a, e := p[0].Trim(geom.ParameterRectangle{UMin: .5 - radius/16, UMax: .5 + radius/16, VMin: .5 + radius - radius/64, VMax: .5 + radius + radius/64})
		if e != nil {
			t.Fatal(e)
		}
		b, e := p[1].Trim(geom.ParameterRectangle{UMin: .5 - radius/32, UMax: .5 + radius/32, VMin: .5 + radius - radius/64, VMax: .5 + radius + radius/64})
		if e != nil {
			t.Fatal(e)
		}
		tube, e := geom.NewVerifiedSurfaceTube(a, b, geom.SurfacePairTubeOptions{FreeAxis: 2, MaxOperations: 8000000})
		if e != nil {
			t.Fatal(e)
		}
		tubes = append(tubes, tube)
	}
	for _, subset := range [][]geom.VerifiedSurfaceTube{tubes[:1], tubes} {
		if got, e := r.CertifyTubeCoverage(context.Background(), subset, geom.RadialTubeCoverageOptions{}); got != nil || !errors.Is(e, geom.ErrConvergence) {
			t.Fatal("missing radius or disconnected arcs accepted", got, e)
		}
	}
}

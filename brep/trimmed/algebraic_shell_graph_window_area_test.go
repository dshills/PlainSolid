package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestGraphWindowSurfaceAreaSigned(t *testing.T) {
	s := graphMaterialBooleanFixture(t, false, true)
	axes := [2]int{0, 2}
	q, e := s.PrepareGraphMaterialQuery(context.Background(), AlgebraicGraphMaterialOptions{Projection: &axes})
	if e != nil {
		t.Fatal(e)
	}
	w := graphWindow(big.NewRat(27, 16), big.NewRat(9, 2), big.NewRat(1, 1000))
	o := AlgebraicGraphWindowAreaOptions{Refinement: AlgebraicGraphWindowRefineOptions{MaxCells: 1}}
	r, e := q.BoundWindowSurfaceArea(context.Background(), w, o)
	if e != nil {
		t.Fatal(e)
	}
	area := big.NewRat(1, 250000)
	if !r.Partition.Complete || r.Partition.ProjectionAxes != axes || r.SurfaceCells != 2 {
		t.Fatal("certified source area", r)
	}
	for _, x := range r.Owners[0].Area {
		if x.Cmp(area) != 0 {
			t.Fatal("horizontal exact area", r.Owners[0].Area)
		}
	}
	// sqrt(1+t) lies between1+t/2−t²/8 and1+t/2 for t>=0.
	// Here t=4(x²+y²), E[t]=8d²/3 and E[t²]=448d⁴/45.
	d2 := big.NewRat(1, 1000000)
	mean := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(big.NewRat(4, 3), d2))
	analyticHi := new(big.Rat).Mul(area, mean)
	analyticLo := new(big.Rat).Mul(area, new(big.Rat).Sub(mean, new(big.Rat).Mul(big.NewRat(56, 45), new(big.Rat).Mul(d2, d2))))
	if r.Owners[1].Area[0].Cmp(analyticLo) > 0 || r.Owners[1].Area[1].Cmp(analyticHi) < 0 {
		t.Fatal("independent area integral interval")
	}
	hi, _ := r.Owners[1].Area[1].Float64()
	if hi > 4e-6*math.Sqrt(1+8e-6)+1e-18 {
		t.Fatal("linear gradient bound lost", hi)
	}
	for _, owner := range r.Owners {
		if len(owner.NativeFaces) != 1 {
			t.Fatal("unused/missing native owners", owner)
		}
	}
	rim := graphWindow(big.NewRat(27, 16), big.NewRat(9, 2), big.NewRat(3, 32))
	mixed, e := q.BoundWindowSurfaceArea(context.Background(), rim, AlgebraicGraphWindowAreaOptions{Refinement: AlgebraicGraphWindowRefineOptions{MaxCells: 85, MaxDepth: 3}})
	if e != nil {
		t.Fatal(e)
	}
	if mixed.Owners[0].Area[0].Cmp(mixed.Partition.MaterialArea) != 0 || mixed.Owners[0].Area[1].Cmp(new(big.Rat).Add(mixed.Partition.MaterialArea, mixed.Partition.UnresolvedArea)) != 0 {
		t.Fatal("unknown rim incorrectly charged as material")
	}
	rr := 25.0 / 4096
	disk := math.Pi * rr
	roof := math.Pi / 6 * (math.Pow(1+4*rr, 1.5) - 1)
	for owner, exact := range []float64{disk, roof} {
		lo, _ := mixed.Owners[owner].Area[0].Float64()
		hi, _ := mixed.Owners[owner].Area[1].Float64()
		if exact < lo || exact > hi {
			t.Fatal("whole disk area", owner, lo, hi, exact)
		}
	}
	for _, badOptions := range []AlgebraicGraphWindowAreaOptions{{Refinement: o.Refinement, MaxSurfaceCells: 1}, {Refinement: o.Refinement, CellOperations: 1}} {
		bad, e := q.BoundWindowSurfaceArea(context.Background(), w, badOptions)
		if !errors.Is(e, geom.ErrConvergence) || bad.Partition.HasBounds || bad.Total[0] != nil {
			t.Fatal("atomic area limits", e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bad, e := q.BoundWindowSurfaceArea(ctx, w, o)
	if !errors.Is(e, context.Canceled) || bad.Partition.HasBounds {
		t.Fatal("cancel", e)
	}
	t.Logf("horizontal%s; curved%s..%s; rim planar%s..%s", area, r.Owners[1].Area[0], r.Owners[1].Area[1], mixed.Owners[0].Area[0], mixed.Owners[0].Area[1])
}
func TestGraphWindowSurfaceAreaNestedHole(t *testing.T) {
	s := graphMaterialBooleanFixture(t, true, false)
	q, e := s.PrepareGraphMaterialQuery(context.Background(), AlgebraicGraphMaterialOptions{})
	if e != nil {
		t.Fatal(e)
	}
	hole := graphWindow(big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(1, 100))
	r, e := q.BoundWindowSurfaceArea(context.Background(), hole, AlgebraicGraphWindowAreaOptions{})
	if e != nil || !r.Partition.Complete || r.SurfaceCells != 0 || r.Total[1].Sign() != 0 {
		t.Fatal("excluded nested hole charged area", e, r)
	}
	material := graphWindow(big.NewRat(2, 3), big.NewRat(7, 13), big.NewRat(1, 4096))
	r, e = q.BoundWindowSurfaceArea(context.Background(), material, AlgebraicGraphWindowAreaOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a := big.NewRat(1, 4194304)
	if !r.Partition.Complete || r.Owners[0].Area[0].Cmp(a) != 0 || r.Owners[0].Area[1].Cmp(a) != 0 || r.Owners[1].Area[0].Cmp(a) < 0 {
		t.Fatal("retained annulus area", r)
	}
}
func TestGraphAreaSqrtEnclosure(t *testing.T) {
	for _, x := range []*big.Rat{big.NewRat(1, 1), big.NewRat(2, 1), big.NewRat(1, 3), big.NewRat(81, 16)} {
		lo, hi, e := graphAreaSqrt(x, 32, 1024)
		if e != nil || new(big.Rat).Mul(lo, lo).Cmp(x) > 0 || new(big.Rat).Mul(hi, hi).Cmp(x) < 0 {
			t.Fatal(x, lo, hi, e)
		}
	}
}

func TestGraphAreaPositiveRationalWeights(t *testing.T) {
	// Exact X=u,Y=v,Z=1/(1+u), represented with positive varying weights.
	var controls [][4]*big.Rat
	for i := 0; i < 3; i++ {
		for j := 0; j < 2; j++ {
			weight := big.NewRat(int64(2+i), 2)
			x := []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 2), big.NewRat(2, 1)}[i]
			controls = append(controls, [4]*big.Rat{x, new(big.Rat).Mul(big.NewRat(int64(j), 1), weight), big.NewRat(1, 1), weight})
		}
	}
	p, e := geom.NewExactHomogeneousBezierSurface(context.Background(), 2, 1, controls, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	g, e := geom.NewAffineProjectedGraph(context.Background(), p, [2]int{0, 1}, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	q := AlgebraicGraphMaterialQuery{patches: [2]*geom.SurfacePatch{p, p}, graphs: [2]*geom.AffineProjectedGraph{g, g}, axes: [2]int{0, 1}, residual: 2}
	w := graphWindow(big.NewRat(3, 8), big.NewRat(3, 8), big.NewRat(1, 8))
	density, e := q.graphCellAreaDensity(context.Background(), 0, w, 65536, 16384, 64)
	if e != nil {
		t.Fatal(e)
	}
	minimum := new(big.Rat).Add(big.NewRat(1, 1), big.NewRat(16, 81))
	maximum := new(big.Rat).Add(big.NewRat(1, 1), big.NewRat(256, 625))
	if new(big.Rat).Mul(density[0], density[0]).Cmp(minimum) > 0 || new(big.Rat).Mul(density[1], density[1]).Cmp(maximum) < 0 {
		t.Fatal("weighted quotient derivative enclosure", density)
	}
}

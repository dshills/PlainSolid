package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestGraphAffineWholeSurfaceArea(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	s := graphMaterialBooleanFixture(t, false, true)
	axes := [2]int{0, 2}
	q, e := s.PrepareGraphMaterialQuery(ctx, AlgebraicGraphMaterialOptions{Projection: &axes})
	if e != nil {
		t.Fatal(e)
	}
	o := AlgebraicGraphAffineAreaOptions{Original: AlgebraicGraphWindowAreaOptions{Refinement: AlgebraicGraphWindowRefineOptions{MaxCells: 341, MaxDepth: 4}}}
	diagonal := geom.Affine{{2, 0, 0, 1}, {0, -3, 0, -2}, {0, 0, 4, 5}}
	d, e := q.BoundAffineWholeSurfaceArea(ctx, diagonal, o)
	if e != nil {
		t.Fatal(e)
	}
	p := d.Original.Boundary.Partition
	capUpper := new(big.Rat).Add(p.MaterialArea, p.UnresolvedArea)
	if d.Owners[0].Area[0].Cmp(new(big.Rat).Mul(big.NewRat(8, 1), p.MaterialArea)) != 0 || d.Owners[0].Area[1].Cmp(new(big.Rat).Mul(big.NewRat(8, 1), capUpper)) != 0 {
		t.Fatal("plane needs XZ area scale8, not determinant24", d.Owners[0].Area)
	}
	if !reflect.DeepEqual(d.Owners[0].NativeFaces, d.Original.Boundary.Owners[0].NativeFaces) || len(d.Owners[0].NativeFaces) != 2 {
		t.Fatal("original native owners lost")
	}
	rr := 25.0 / 4096
	area := 2 * math.Pi * rr
	contains := func(label string, b [2]*big.Rat, lo, hi float64) {
		t.Helper()
		a, _ := b[0].Float64()
		z, _ := b[1].Float64()
		if a > lo || z < hi {
			t.Fatalf("%s reference[%g,%g] not contained in[%g,%g]", label, lo, hi, a, z)
		}
	}
	contains("diagonal plane", d.Owners[0].Area, 8*area, 8*area)
	shear := geom.Affine{{2, 0, .5, 1}, {.25, 3, 0, -2}, {0, 0, 1, 5}}
	r, e := q.BoundAffineWholeSurfaceArea(ctx, shear, o)
	if e != nil {
		t.Fatal(e)
	}
	// Independently derived cofactor columns: Cx=(3,0,-1.5),
	// Cy=(-.25,2,.125), Cz=(0,0,6). On each paraboloid disk,
	// hx²+hz²<=4r². Triangle/reverse-triangle plus Frobenius bounds
	// therefore enclose the exact transformed density by |Cy| ±2r*sqrt(47.25).
	base := math.Sqrt(261) / 8
	delta := 2 * (5.0 / 64) * math.Sqrt(47.25)
	contains("sheared plane", r.Owners[0].Area, area*base, area*base)
	contains("sheared curved analytic enclosure", r.Owners[1].Area, area*(base-delta), area*(base+delta))
	if !reflect.DeepEqual(r.Original.Boundary.Partition.Leaves, p.Leaves) || r.ImageCells != d.ImageCells {
		t.Fatal("placement changed original partition")
	}
	for end := 0; end < 2; end++ {
		if r.Total[end].Cmp(new(big.Rat).Add(r.Owners[0].Area[end], r.Owners[1].Area[end])) != 0 {
			t.Fatal("area sum")
		}
	}
	id := r.Owners[0].NativeFaces[0]
	r.Original.Boundary.Owners[0].NativeFaces[0] = -1
	if r.Owners[0].NativeFaces[0] != id {
		t.Fatal("owner alias")
	}
	badOptions := o
	badOptions.MaxImageCells = 1
	bad, e := q.BoundAffineWholeSurfaceArea(ctx, shear, badOptions)
	if !errors.Is(e, geom.ErrConvergence) || bad.Total[0] != nil || bad.Original.Volume[0] != nil {
		t.Fatal("atomic image quota", e)
	}
	singular := shear
	singular[2] = singular[0]
	bad, e = q.BoundAffineWholeSurfaceArea(ctx, singular, o)
	if !errors.Is(e, geom.ErrConvergence) || bad.Total[0] != nil {
		t.Fatal("singular", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e = q.BoundAffineWholeSurfaceArea(cancelled, shear, o)
	if !errors.Is(e, context.Canceled) || bad.Total[0] != nil {
		t.Fatal("cancel", e)
	}
	t.Logf("one original partition%d attempts, %d image owner cells, area[%s,%s]", p.CellAttempts, r.ImageCells, r.Total[0], r.Total[1])
}

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
	"reflect"
	"testing"
)

func TestAlgebraicGraphOccurrenceRationalArea(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	source := graphMaterialBooleanFixture(t, false, false)
	emptyPose := func() (p [3][4]*big.Rat) {
		for i := range p {
			for j := range p[i] {
				p[i][j] = new(big.Rat)
			}
		}
		return p
	}
	diagonal := emptyPose()
	diagonal[0][0] = big.NewRat(2, 3)
	diagonal[1][1] = big.NewRat(3, 5)
	diagonal[2][2] = big.NewRat(-7, 11)
	occurrence, e := NewAlgebraicGraphOccurrence(ctx, source, diagonal, [2]int{0, 1}, AlgebraicGraphMaterialOptions{})
	if e != nil {
		t.Fatal(e)
	}
	options := AlgebraicGraphOccurrenceAreaOptions{Original: AlgebraicGraphWindowAreaOptions{Refinement: AlgebraicGraphWindowRefineOptions{MaxCells: 341, MaxDepth: 4}}}
	d, e := occurrence.BoundWholeSurfaceArea(ctx, options)
	if e != nil {
		t.Fatal(e)
	}
	partition := d.Original.Boundary.Partition
	expected := [2]*big.Rat{new(big.Rat).Mul(big.NewRat(2, 5), partition.MaterialArea), new(big.Rat).Mul(big.NewRat(2, 5), new(big.Rat).Add(partition.MaterialArea, partition.UnresolvedArea))}
	rootError := new(big.Rat).Mul(partition.ProjectedArea, new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 64)))
	for i := 0; i < 2; i++ {
		difference := new(big.Rat).Sub(d.Owners[0].Area[i], expected[i])
		if i == 0 {
			difference.Neg(difference)
		}
		if difference.Sign() < 0 || difference.Cmp(rootError) > 0 {
			t.Fatal("planar area must scale by exact2/5, not volume14/55", difference, rootError)
		}
	}
	shear := emptyPose()
	shear[0][0] = big.NewRat(2, 1)
	shear[0][1] = big.NewRat(1, 3)
	shear[0][3] = big.NewRat(1, 7)
	shear[1][1] = big.NewRat(1, 1)
	shear[1][3] = big.NewRat(2, 7)
	shear[2][0] = big.NewRat(1, 5)
	shear[2][2] = big.NewRat(-3, 1)
	shear[2][3] = big.NewRat(-1, 11)
	placed, e := NewAlgebraicGraphOccurrence(ctx, source, shear, [2]int{0, 1}, AlgebraicGraphMaterialOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := placed.BoundWholeSurfaceArea(ctx, options)
	if e != nil {
		t.Fatal(e)
	}
	// Independent exact cofactor columns are(-3,1,0),(0,-6,0),
	// (-1/5,1/15,2). Disk slopes obey hx²+hy²<=4r², so
	// reverse-triangle/triangle bounds enclose curved density by sqrt910/15±2r√46.
	rr := 25.0 / 4096
	area := 2 * math.Pi * rr
	base, delta := math.Sqrt(910)/15, 2*(5.0/64)*math.Sqrt(46)
	contains := func(label string, b [2]*big.Rat, lo, hi float64) {
		t.Helper()
		x, _ := b[0].Float64()
		y, _ := b[1].Float64()
		if x > lo || y < hi {
			t.Fatal(label, lo, hi, x, y)
		}
	}
	contains("rational diagonal cap", d.Owners[0].Area, area*.4, area*.4)
	contains("rational sheared cap", r.Owners[0].Area, area*base, area*base)
	contains("rational curved enclosure", r.Owners[1].Area, area*(base-delta), area*(base+delta))
	if !reflect.DeepEqual(r.Original.Boundary.Partition.Leaves, partition.Leaves) || r.ImageCells != d.ImageCells {
		t.Fatal("rational placement changed source partition")
	}
	for i := 0; i < 2; i++ {
		if !reflect.DeepEqual(r.Owners[i].NativeFaces, r.Original.Boundary.Owners[i].NativeFaces) || len(r.Owners[i].NativeFaces) != 2 {
			t.Fatal("native face owners")
		}
	}
	var native bytes.Buffer
	if e = WriteAlgebraicGraphOccurrenceJSON(&native, placed); e != nil {
		t.Fatal(e)
	}
	section := AlgebraicShellSectionOptions{Trace: nestedTubeSectionOptions().Trace}
	section.Trace.Coverage.OverlappingAffineXYFallback = true
	boolean := AlgebraicShellBooleanOptions{}
	boolean.Imprint.Exact.MaxOperations = 512000000
	back, e := ReadAlgebraicGraphOccurrenceJSON(ctx, &native, JSONLimits{}, section, boolean, AlgebraicGraphMaterialOptions{})
	if e != nil {
		t.Fatal(e)
	}
	again, e := back.BoundWholeSurfaceArea(ctx, options)
	if e != nil || !reflect.DeepEqual(again.Owners, r.Owners) || !reflect.DeepEqual(again.Total, r.Total) || !reflect.DeepEqual(again.Pose, r.Pose) {
		t.Fatal("exact rational area/ancestry reload", e)
	}
	r.Pose[0][1].SetInt64(999)
	r.Original.Boundary.Owners[0].NativeFaces[0] = -1
	if placed.Pose()[0][1].Cmp(big.NewRat(1, 3)) != 0 || r.Owners[0].NativeFaces[0] < 0 {
		t.Fatal("mutable result changed source/owner")
	}
	fail := options
	fail.MaxImageCells = 1
	bad, e := placed.BoundWholeSurfaceArea(ctx, fail)
	if !errors.Is(e, geom.ErrConvergence) || bad.Total[0] != nil || bad.Original.Volume[0] != nil {
		t.Fatal("atomic image quota", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e = placed.BoundWholeSurfaceArea(cancelled, options)
	if !errors.Is(e, context.Canceled) || bad.Total[0] != nil {
		t.Fatal("cancel", e)
	}
	t.Logf("nonbinary cofactor; %d original attempts,%d image owner cells; rational cap area scale2/5 independently bounded", partition.CellAttempts, r.ImageCells)
}

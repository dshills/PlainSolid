package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"sync"
	"testing"
)

func TestSurfaceTubeCoverageCache(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{X: u, Y: v} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{X: u, Y: .5, Z: v - .5} })
	regions := []SurfacePairTubeRegion{{Box: SurfacePairParameterBox{A: ParameterRectangle{.2, .8, .4, .6}, B: ParameterRectangle{.1, .9, .4, .6}}}}
	cell := regions[0].Box
	cache, e := NewSurfacePairTubeCoverageCache(a, b, 0)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	plain, e := CertifySurfacePairCellInTubesContext(ctx, a, b, cell, regions, SurfaceTubeUnionCoverageOptions{})
	if e != nil || !plain.Covered {
		t.Fatal(plain, e)
	}
	for i := 0; i < 2; i++ {
		r, e := cache.CertifyCell(ctx, cell, regions, SurfaceTubeUnionCoverageOptions{})
		if e != nil || !reflect.DeepEqual(r, plain) {
			t.Fatal(r, e, plain)
		}
	}
	stats := cache.Stats()
	if stats.Stored != 1 || stats.Hits != 1 || stats.SavedOperations <= 0 {
		t.Fatal(stats)
	}
	// Diagnostics are value-only; mutating caller results cannot edit authority.
	r, _ := cache.CertifyCell(ctx, cell, regions, SurfaceTubeUnionCoverageOptions{})
	r.Tubes[0].Test.KrawczykBounds[0].Sign = 77
	next, _ := cache.CertifyCell(ctx, cell, regions, SurfaceTubeUnionCoverageOptions{})
	if !reflect.DeepEqual(next, plain) {
		t.Fatal("aliased certificate")
	}
	for _, limit := range []int{1, plain.ExactOperations - 1, plain.ExactOperations, plain.ExactOperations + 1} {
		o := SurfaceTubeUnionCoverageOptions{MaxOperations: limit}
		want, we := CertifySurfacePairCellInTubesContext(ctx, a, b, cell, regions, o)
		got, ge := cache.CertifyCell(ctx, cell, regions, o)
		if !errors.Is(ge, we) || !reflect.DeepEqual(want, got) {
			t.Fatal("budget", limit, we, ge, want, got)
		}
	}
	before := cache.Stats()
	_, _ = cache.CertifyCell(ctx, cell, regions, SurfaceTubeUnionCoverageOptions{MaxCoefficientBits: 64})
	if cache.Stats().Hits != before.Hits {
		t.Fatal("foreign bit limit reused")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	r, e = cache.CertifyCell(canceled, cell, regions, SurfaceTubeUnionCoverageOptions{})
	if !errors.Is(e, context.Canceled) || r.Covered || len(r.Pending) != 1 {
		t.Fatal(r, e)
	}
	duplicates := append(append([]SurfacePairTubeRegion{}, regions...), regions...)
	want, we := CertifySurfacePairCellInTubesContext(ctx, a, b, cell, duplicates, SurfaceTubeUnionCoverageOptions{})
	got, ge := cache.CertifyCell(ctx, cell, duplicates, SurfaceTubeUnionCoverageOptions{})
	if we != nil || ge != nil || !reflect.DeepEqual(want, got) {
		t.Fatal("duplicates", we, ge)
	}

	for _, limit := range []int{want.ExactOperations - 1, want.ExactOperations, want.ExactOperations + 1} {
		o := SurfaceTubeUnionCoverageOptions{MaxOperations: limit}
		old, oe := CertifySurfacePairCellInTubesContext(ctx, a, b, cell, duplicates, o)
		cached, ce := cache.CertifyCell(ctx, cell, duplicates, o)
		if !errors.Is(ce, oe) || !reflect.DeepEqual(old, cached) {
			t.Fatal("duplicate budget", limit, oe, ce, old, cached)
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := cache.CertifyCell(ctx, cell, regions, SurfaceTubeUnionCoverageOptions{})
			if e != nil || !r.Covered {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	foreign := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{X: u, Y: .5, Z: v + 5} })
	other, _ := NewSurfacePairTubeCoverageCache(a, foreign, 1)
	if r, e := other.CertifyCell(ctx, cell, regions, SurfaceTubeUnionCoverageOptions{}); e == nil || r.Covered || other.Stats().Stored != 0 {
		t.Fatal("foreign proof reused", r, e)
	}
	bad := append([]SurfacePairTubeRegion(nil), regions...)
	bad[0].FreeAxis = 9
	if r, e := cache.CertifyCell(ctx, cell, bad, SurfaceTubeUnionCoverageOptions{}); !errors.Is(e, ErrInvalid) || r.Covered {
		t.Fatal("bad axis", r, e)
	}
}

func TestTubeCoverageCacheAnchors(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{X: u, Y: v, Z: u * v} })
	a, _ = a.Trim(ParameterRectangle{UMax: .125, VMax: .25})
	s, e := NewBSplineSurface(1, 1, 2, 2, []Vec3{{X: -.25, Z: -.25}, {X: -.25, Y: 1, Z: -1.25}, {X: .25, Z: .25}, {X: .25, Y: 1, Z: -.75}}, []float64{-.25, -.25, .25, .25}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, _ = b.Trim(ParameterRectangle{UMin: -.25, UMax: .25, VMax: .25})
	box := SurfacePairParameterBox{A: a.Domain(), B: b.Domain()}
	anchor := &SurfaceTubeBoundaryOptions{}
	for i := range anchor.Anchor {
		anchor.Anchor[i] = new(big.Rat)
	}
	regions := []SurfacePairTubeRegion{{Box: box, Boundary: anchor}}
	cache, _ := NewSurfacePairTubeCoverageCache(a, b, 1)
	ctx := context.Background()
	original, e := cache.CertifyCell(ctx, box, regions, SurfaceTubeUnionCoverageOptions{})
	if e != nil || !original.Covered {
		t.Fatal(original, e)
	}
	anchor.Anchor[0].SetInt64(1)
	if r, e := cache.CertifyCell(ctx, box, regions, SurfaceTubeUnionCoverageOptions{}); e == nil || r.Covered {
		t.Fatal("changed anchor accepted", r, e)
	}
	anchor.Anchor[0].SetInt64(0)
	r, e := cache.CertifyCell(ctx, box, regions, SurfaceTubeUnionCoverageOptions{})
	if e != nil || !reflect.DeepEqual(original, r) || cache.Stats().Hits != 1 {
		t.Fatal("anchor cache corrupted", e, cache.Stats())
	}
}

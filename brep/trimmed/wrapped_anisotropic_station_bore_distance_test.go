//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestAnisotropicBoreCapDistance(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	s := anisotropicMeasuredBore(t)
	r := big.NewRat
	p := [3]*big.Rat{r(1, 1), r(-1, 2), r(-17, 4)}
	o := WrappedAnisotropicBoreDistanceOptions{Search: geom.SurfaceDistanceOptions{SquaredTolerance: 1e-4}}
	q, e := s.BoundWrappedAnisotropicStationBoreDistanceRational(ctx, p, o)
	if e != nil {
		t.Fatal(e)
	}
	source, e := s.ValidateWrappedAnisotropicStationBore(ctx, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if q.Lower.Cmp(r(1, 16)) != 0 || q.Upper.Cmp(r(1, 16)) != 0 || q.WitnessFace != source.Caps[0] || len(q.PerFace) != len(s.faces) || q.VisitedCells != 0 {
		t.Fatal("exact cap distance", q)
	}
	if q.Witness[0].Cmp(p[0]) != 0 || q.Witness[1].Cmp(p[1]) != 0 || q.Witness[2].Cmp(r(-4, 1)) != 0 {
		t.Fatal("attained cap projection")
	}
	for _, face := range q.PerFace {
		d := new(big.Rat)
		for i, v := range face.Witness {
			delta := new(big.Rat).Sub(p[i], v)
			d.Add(d, new(big.Rat).Mul(delta, delta))
		}
		if d.Cmp(face.Upper) != 0 {
			t.Fatal("attained upper")
		}
	}
	o.Search.MaxOperations = q.ExactOperations
	if _, e = s.BoundWrappedAnisotropicStationBoreDistanceRational(ctx, p, o); e != nil {
		t.Fatal("exact quota", e)
	}
	o.Search.MaxOperations--
	bad, e := s.BoundWrappedAnisotropicStationBoreDistanceRational(ctx, p, o)
	if e == nil || bad.Lower != nil || len(bad.PerFace) != 0 {
		t.Fatal("atomic budget", e)
	}
	o.Search.MaxOperations = 0
	o.MaxPointOperations = q.PointOperations - 1
	if bad, e = s.BoundWrappedAnisotropicStationBoreDistanceRational(ctx, p, o); e == nil || bad.Lower != nil {
		t.Fatal("point budget", e)
	}
	t.Log("exact cap operations", q.ExactOperations, q.PointOperations)
}

func TestAnisotropicBoreVoidDistance(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	build := func(scale [2]float64) Solid {
		w := anisotropicStationBand(t, -1, 1, scale, scale, .75)
		s, _, e := NewWrappedAnisotropicStationLoft(ctx, []*geom.WrappedSurfacePatch{w}, 1e-8, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	s, _, e := CutWrappedAnisotropicStationLofts(ctx, build([2]float64{3, 2}), build([2]float64{1, .5}), geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p := [3]*big.Rat{r(1, 4), r(-1, 2), r(0, 1)}
	o := WrappedAnisotropicBoreDistanceOptions{Search: geom.SurfaceDistanceOptions{SquaredTolerance: .01}}
	q, e := s.BoundWrappedAnisotropicStationBoreDistanceRational(ctx, p, o)
	if e != nil {
		t.Fatal(e)
	}
	source, e := s.ValidateWrappedAnisotropicStationBore(ctx, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// For w=3/4 the homogeneous squared-radius residual is
	// (4w²−2)t²(1−t)² >=0. Every normalized rim point has norm>=1.
	// Independent diagonal scales (1,1/2) imply distance>=1/2;
	// the minor-axis wall point at the query's height attains equality.
	if q.Lower.Cmp(r(1, 4)) > 0 || q.Upper.Cmp(r(1, 4)) < 0 || new(big.Rat).Sub(q.Upper, q.Lower).Cmp(ratInset(.01)) > 0 || q.WitnessFace != source.InnerWalls[0] {
		t.Fatal("noncircular void distance", q)
	}
	loc, e := s.LocateWrappedAnisotropicStationBoreRational(ctx, q.Witness, WrappedRationalStationPointOptions{})
	if e != nil || loc != parametric.Boundary {
		t.Fatal("native retained witness", loc, e)
	}
	for _, face := range q.PerFace {
		if face.Face == source.Caps[0] || face.Face == source.Caps[1] {
			if face.Lower.Cmp(r(1, 1)) <= 0 {
				t.Fatal("removed cap disk selected", face)
			}
		}
	}
	t.Log("void interval cells operations", q.Lower.FloatString(8), q.Upper.FloatString(8), q.VisitedCells, q.ExactOperations)
	o.Search.MaxCells = 1
	bad, e := s.BoundWrappedAnisotropicStationBoreDistanceRational(ctx, p, o)
	if e == nil || bad.Lower != nil || len(bad.PerFace) != 0 {
		t.Fatal("cell quota", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if bad, e = s.BoundWrappedAnisotropicStationBoreDistanceRational(cancelled, p, o); e == nil || bad.Lower != nil {
		t.Fatal("cancelled")
	}
}

package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestCircularSectorAxisIntervals(t *testing.T) {
	s, _ := NewCircularSector(Vec3{}, Vec3{Z: 1}, Vec3{X: 1}, Vec3{Y: 1}, 5, -2, 4, 1)
	for _, scale := range []float64{3, -3} {
		q, e := IntersectCircularSectorAxisLine(context.Background(), s, Vec3{X: 1, Y: 1, Z: 1}, scale, [2]float64{-10, 10}, SurfacePatchEvaluationOptions{})
		if e != nil || !q.Complete || q.Empty || q.AlongBoundary || q.Singleton {
			t.Fatal(q, e)
		}
		if q.Parameters[0].Cmp(big.NewRat(-1, 1)) != 0 || q.Parameters[1].Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatal(q)
		}
		low, high := SectorLowerCap, SectorUpperCap
		if scale < 0 {
			low, high = high, low
		}
		if q.EndpointBoundary != [2]CircularSectorBoundary{low, high} {
			t.Fatal(q)
		}
	}
	for _, c := range []struct {
		p     Vec3
		f     CircularSectorBoundary
		empty bool
	}{
		{Vec3{}, SectorAxis | SectorStartWall | SectorEndWall, false},
		{Vec3{X: 3, Y: 4}, SectorOuterWall, false},
		{Vec3{X: 1}, SectorStartWall, false},
		{Vec3{X: 3, Y: math.Nextafter(4, 5)}, 0, true},
		{Vec3{X: 1, Y: -math.SmallestNonzeroFloat64}, 0, true},
	} {
		q, e := IntersectCircularSectorAxisLine(context.Background(), s, c.p, 3, [2]float64{-10, 10}, SurfacePatchEvaluationOptions{})
		if e != nil || q.Empty != c.empty || q.InteriorBoundary != c.f {
			t.Fatal(q, e)
		}
	}
	q, e := IntersectCircularSectorAxisLine(context.Background(), s, Vec3{X: 1, Y: 1}, 3, [2]float64{0, 1}, SurfacePatchEvaluationOptions{})
	if e != nil || q.Parameters[1].Cmp(big.NewRat(1, 1)) != 0 || q.EndpointBoundary[1] != 0 {
		t.Fatal(q, e)
	}
	q, e = IntersectCircularSectorAxisLine(context.Background(), s, Vec3{X: 1, Y: 1}, 3, [2]float64{1, 1}, SurfacePatchEvaluationOptions{})
	if e != nil || !q.Singleton {
		t.Fatal(q, e)
	}
	q, e = IntersectCircularSectorAxisLine(context.Background(), s, Vec3{X: 1, Y: 1}, 3, [2]float64{2, 3}, SurfacePatchEvaluationOptions{})
	if e != nil || !q.Empty {
		t.Fatal(q, e)
	}
	q, e = IntersectCircularSectorAxisLine(context.Background(), s, Vec3{X: 1, Y: 1}, 3, [2]float64{-10, 10}, SurfacePatchEvaluationOptions{})
	if e != nil || q.Parameters[0].Cmp(big.NewRat(-2, 3)) != 0 || q.Parameters[1].Cmp(big.NewRat(4, 3)) != 0 {
		t.Fatal(q, e)
	}
	if _, e = IntersectCircularSectorAxisLine(context.Background(), s, Vec3{}, 1, [2]float64{0, 1}, SurfacePatchEvaluationOptions{MaxOperations: 1}); e == nil {
		t.Fatal("budget")
	}
}

func TestCircularSectorAxisIntervalOrientations(t *testing.T) {
	for _, axis := range []Vec3{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {Z: 1}, {Z: -1}} {
		start := Vec3{X: 1}
		if axis.X != 0 {
			start = Vec3{Y: 1}
		}
		end := axis.Cross(start)
		s, e := NewCircularSector(Vec3{}, axis, start, end, 2, 0, 4, 1)
		if e != nil {
			t.Fatal(e)
		}
		origin := start.Scale(.5).Add(end.Scale(.5)).Add(axis.Scale(1))
		q, e := IntersectCircularSectorAxisLine(context.Background(), s, origin, 2, [2]float64{-5, 5}, SurfacePatchEvaluationOptions{})
		if e != nil || q.Empty || q.Parameters[0].Cmp(big.NewRat(-1, 2)) != 0 || q.Parameters[1].Cmp(big.NewRat(3, 2)) != 0 {
			t.Fatal(q, e)
		}
	}
}

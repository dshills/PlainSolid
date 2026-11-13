package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestCircularSectorHorizontal(t *testing.T) {
	s, _ := NewCircularSector(Vec3{}, Vec3{Z: 1}, Vec3{X: 1}, Vec3{Y: 1}, 2, -1, 1, 1)
	q, e := IntersectCircularSectorHorizontalLine(context.Background(), s, Vec3{Y: 1}, Vec3{X: 1}, [2]float64{-4, 4}, SurfacePatchEvaluationOptions{})
	if e != nil || q.Empty || !q.Complete || q.Singleton {
		t.Fatal(q, e)
	}
	if q.Endpoints[0].Sign != 0 || q.Endpoints[0].Offset.Sign() != 0 || q.Endpoints[1].Radicand.Cmp(big.NewRat(3, 1)) != 0 || q.Endpoints[1].Sign != 1 {
		t.Fatal(q)
	}
	if q.EndpointBoundary != [2]CircularSectorBoundary{SectorEndWall, SectorOuterWall} {
		t.Fatal(q)
	}
	for _, bound := range q.Endpoints[1].Bounds {
		if bound.Sign() <= 0 {
			t.Fatal(bound)
		}
	}
	q, e = IntersectCircularSectorHorizontalLine(context.Background(), s, Vec3{Y: 2}, Vec3{X: 1}, [2]float64{-4, 4}, SurfacePatchEvaluationOptions{})
	if e != nil || !q.Singleton || q.Empty {
		t.Fatal(q, e)
	}
	q, e = IntersectCircularSectorHorizontalLine(context.Background(), s, Vec3{Y: 3}, Vec3{X: 1}, [2]float64{-4, 4}, SurfacePatchEvaluationOptions{})
	if e != nil || !q.Empty {
		t.Fatal(q, e)
	}
	q, e = IntersectCircularSectorHorizontalLine(context.Background(), s, Vec3{Y: 1, Z: 1}, Vec3{X: -2}, [2]float64{-1, -.5}, SurfacePatchEvaluationOptions{})
	if e != nil || q.Empty || q.Endpoints[0].Sign != -1 || q.Endpoints[1].Offset.Cmp(big.NewRat(-1, 2)) != 0 || q.InteriorBoundary != SectorUpperCap {
		t.Fatal(q, e)
	}
	h, _ := NewCircularSector(Vec3{}, Vec3{Z: 1}, Vec3{X: 1}, Vec3{X: -1}, 2, -1, 1, -1)
	q, e = IntersectCircularSectorHorizontalLine(context.Background(), h, Vec3{Y: -1}, Vec3{X: 1}, [2]float64{-4, 4}, SurfacePatchEvaluationOptions{})
	if e != nil || q.Empty || q.Endpoints[0].Sign != -1 || q.Endpoints[1].Sign != 1 {
		t.Fatal(q, e)
	}
}

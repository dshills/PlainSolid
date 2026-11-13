package geom

import (
	"context"
	"math"
	"testing"
)

func TestCircularSectorExactPoint(t *testing.T) {
	for _, hand := range []int{-1, 1} {
		for _, half := range []bool{false, true} {
			end := Vec3{Y: float64(hand) * 7}
			if half {
				end = Vec3{X: -7}
			}
			s, e := NewCircularSector(Vec3{}, Vec3{Z: 1}, Vec3{X: 3}, end, 5, -2, 2, hand)
			if e != nil {
				t.Fatal(e)
			}
			cases := []struct {
				p Vec3
				l CircularSectorLocation
				f CircularSectorBoundary
			}{
				{Vec3{X: 1, Y: float64(hand)}, SectorInterior, 0},
				{Vec3{X: 3, Y: float64(hand) * 4}, SectorBoundary, SectorOuterWall},
				{Vec3{X: 3}, SectorBoundary, SectorStartWall},
				{Vec3{}, SectorBoundary, SectorAxis | SectorStartWall | SectorEndWall},
				{Vec3{X: 1, Y: float64(hand), Z: 2}, SectorBoundary, SectorUpperCap},
				{Vec3{X: 1, Y: float64(hand), Z: -2}, SectorBoundary, SectorLowerCap},
				{Vec3{X: 1, Y: -float64(hand) * math.SmallestNonzeroFloat64}, SectorOutside, 0},
				{Vec3{X: 3, Y: float64(hand) * math.Nextafter(4, 5)}, SectorOutside, 0},
			}
			if half {
				cases = append(cases, struct {
					p Vec3
					l CircularSectorLocation
					f CircularSectorBoundary
				}{Vec3{X: -3}, SectorBoundary, SectorEndWall})
			} else {
				cases = append(cases, struct {
					p Vec3
					l CircularSectorLocation
					f CircularSectorBoundary
				}{Vec3{Y: float64(hand) * 3}, SectorBoundary, SectorEndWall})
			}
			for _, c := range cases {
				q, e := ClassifyCircularSectorPoint(context.Background(), s, c.p, SurfacePatchEvaluationOptions{})
				if e != nil || !q.Complete || q.Location != c.l || q.Boundary != c.f {
					t.Fatalf("hand %d half %v p %v: %+v %v", hand, half, c.p, q, e)
				}
			}
		}
	}
}
func TestCircularSectorAxesLimits(t *testing.T) {
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
		p := start.Scale(.5).Add(end.Scale(.5)).Add(axis.Scale(2))
		q, e := ClassifyCircularSectorPoint(context.Background(), s, p, SurfacePatchEvaluationOptions{})
		if e != nil || q.Location != SectorInterior {
			t.Fatal(q, e)
		}
		_, e = ClassifyCircularSectorPoint(context.Background(), s, p, SurfacePatchEvaluationOptions{MaxOperations: 1})
		if e == nil {
			t.Fatal("budget")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, e = ClassifyCircularSectorPoint(ctx, s, p, SurfacePatchEvaluationOptions{}); e != context.Canceled {
			t.Fatal(e)
		}
	}
	if _, e := NewCircularSector(Vec3{}, Vec3{Z: 1}, Vec3{X: 1}, Vec3{X: 1, Y: 1}, 2, 0, 1, 1); e != ErrDomain {
		t.Fatal(e)
	}
}

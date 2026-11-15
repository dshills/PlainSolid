package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func convexOverlapFixture(t *testing.T) (Solid, Solid) {
	a, b := coincidenceFaceFixture(t, true, false, false)
	a.faces[0].Holes = nil
	b.faces[0].Holes = nil
	a.faces[0].Outer = orthogonalOverlapLoop([]geom.Vec2{{}, {X: 1}, {Y: .75}}, 10)
	// Original shared triangle (0,1/2),(1/2,0),(1,1/2), mapped
	// independently by B=(2+3v,-1+5u), with its positive UV winding restored.
	b.faces[0].Outer = orthogonalOverlapLoop([]geom.Vec2{{X: 3.5, Y: 4}, {X: 2, Y: 1.5}, {X: 3.5, Y: -1}}, 100)
	return a, b
}
func TestCoincidentConvexSlantedRationalOverlap(t *testing.T) {
	a, b := convexOverlapFixture(t)
	r, e := a.IntersectCoincidentConvexTrims(context.Background(), 0, b, 0, CoincidentConvexOverlapOptions{})
	if e != nil || !r.Complete || r.Dimension != 2 || len(r.Vertices) != 4 || len(r.Boundary) != 4 || r.UVArea[0].Cmp(big.NewRat(13, 84)) != 0 || r.UVArea[1].Cmp(big.NewRat(65, 28)) != 0 {
		t.Fatal(r, e)
	}
	wanted := [][2]*big.Rat{{new(big.Rat), big.NewRat(1, 2)}, {big.NewRat(1, 2), new(big.Rat)}, {big.NewRat(5, 7), big.NewRat(3, 14)}, {big.NewRat(1, 3), big.NewRat(1, 2)}}
	for _, p := range wanted {
		found := false
		for _, v := range r.Vertices {
			if p[0].Cmp(v.A[0]) == 0 && p[1].Cmp(v.A[1]) == 0 {
				found = true
			}
		}
		if !found {
			t.Fatal("missing independent rational crossing", p)
		}
	}
	for _, edge := range r.Boundary {
		if len(edge.Sources) == 0 {
			t.Fatal("lost original owners")
		}
		for _, source := range edge.Sources {
			face, points := a.faces[0], edge.A
			if source.Owner.Source == 1 {
				face, points = b.faces[0], edge.B
			}
			u := face.Outer[source.Owner.Use]
			if u.Edge != source.Owner.Edge {
				t.Fatal("original edge identity")
			}
			for i, parameter := range source.Range {
				for k, endpoints := range [][2]float64{{u.UVStart.X, u.UVEnd.X}, {u.UVStart.Y, u.UVEnd.Y}} {
					value := new(big.Rat).Add(ratInset(endpoints[0]), new(big.Rat).Mul(parameter, new(big.Rat).Sub(ratInset(endpoints[1]), ratInset(endpoints[0]))))
					if value.Cmp(points[i][k]) != 0 {
						t.Fatal("original range mismatch")
					}
				}
			}
		}
		for side, c := range []*geom.ExactSplineCurve{edge.CurveA, edge.CurveB} {
			net, _, e := c.BezierControlNet(context.Background(), geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			points := edge.A
			if side == 1 {
				points = edge.B
			}
			for i, p := range points {
				for k, v := range p {
					if net.Control[i][k].Cmp(v) != 0 {
						t.Fatal("rounded native line")
					}
				}
			}
		}
	}
}
func TestCoincidentConvexClosedContactAndLateClip(t *testing.T) {
	a, b := coincidenceFaceFixture(t, false, false, false)
	b.faces[0].Surface = a.faces[0].Surface
	a.faces[0].Holes = nil
	b.faces[0].Holes = nil
	a.faces[0].Outer = orthogonalOverlapLoop([]geom.Vec2{{}, {X: 1}, {Y: 1}}, 10)
	for _, tc := range []struct {
		points    []geom.Vec2
		dimension int
	}{{[]geom.Vec2{{X: 1}, {X: 1, Y: 1}, {Y: 1}}, 1}, {[]geom.Vec2{{X: .5, Y: .5}, {X: .75, Y: .5}, {X: .5, Y: .75}}, 0}, {[]geom.Vec2{{X: .75, Y: .75}, {X: 1, Y: .75}, {X: .75, Y: 1}}, -1}} {
		b.faces[0].Outer = orthogonalOverlapLoop(tc.points, 100)
		r, e := a.IntersectCoincidentConvexTrims(context.Background(), 0, b, 0, CoincidentConvexOverlapOptions{})
		if e != nil || !r.Complete || r.Dimension != tc.dimension || r.UVArea[0].Sign() != 0 {
			t.Fatal(tc, r, e)
		}
		if tc.dimension == 1 && (len(r.Boundary) != 1 || len(r.Boundary[0].Sources) != 2) {
			t.Fatal("diagonal seam owners", r)
		}
	}
	// First clipping halfplane y>=1/2 collapses A to(0,1/2); the
	// final diagonal x-y>=0 removes it. All original halfplanes must run.
	a.faces[0].Outer = orthogonalOverlapLoop([]geom.Vec2{{}, {X: 1}, {Y: .5}}, 10)
	b.faces[0].Outer = orthogonalOverlapLoop([]geom.Vec2{{X: .5, Y: .5}, {X: 1, Y: .5}, {X: 1, Y: 1}}, 100)
	r, e := a.IntersectCoincidentConvexTrims(context.Background(), 0, b, 0, CoincidentConvexOverlapOptions{})
	if e != nil || !r.Complete || r.Dimension != -1 || len(r.Vertices) != 0 {
		t.Fatal("late halfplane ignored", r, e)
	}

}
func TestCoincidentConvexLimitsAndScope(t *testing.T) {
	a, b := convexOverlapFixture(t)
	if r, e := a.IntersectCoincidentConvexTrims(context.Background(), 0, b, 0, CoincidentConvexOverlapOptions{MaxWork: 1}); e == nil || r.Complete || len(r.Vertices) > 0 || r.UVArea[0] != nil {
		t.Fatal("atomic arrangement quota", r, e)
	}
	a.faces[0].Holes = [][]Use{overlapRectangle(.125, .25, .125, .25, 20, true)}
	if r, e := a.IntersectCoincidentConvexTrims(context.Background(), 0, b, 0, CoincidentConvexOverlapOptions{}); e == nil || r.Complete {
		t.Fatal("unsupported holes")
	}
	a, b = convexOverlapFixture(t)
	a.faces[0].Outer = orthogonalOverlapLoop([]geom.Vec2{{}, {X: 1}, {X: .25, Y: .25}, {Y: 1}}, 10)
	if r, e := a.IntersectCoincidentConvexTrims(context.Background(), 0, b, 0, CoincidentConvexOverlapOptions{}); e == nil || r.Complete {
		t.Fatal("nonconvex admitted")
	}
	a, b = convexOverlapFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, e := a.IntersectCoincidentConvexTrims(ctx, 0, b, 0, CoincidentConvexOverlapOptions{}); !errors.Is(e, context.Canceled) || r.Complete {
		t.Fatal(r, e)
	}
}

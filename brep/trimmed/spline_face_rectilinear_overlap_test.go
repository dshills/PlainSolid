package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func overlapRectangle(x0, x1, y0, y1 float64, edge int, hole bool) []Use {
	p := []geom.Vec2{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}
	if hole {
		p[1], p[3] = p[3], p[1]
	}
	var r []Use
	for i, v := range p {
		r = append(r, Use{Edge: edge + i, UVStart: v, UVEnd: p[(i+1)%4]})
	}
	return r
}
func TestCoincidentRectangularOverlapRationalMappedHole(t *testing.T) {
	a, b := coincidenceFaceFixture(t, true, false, false)
	a.faces[0].Outer = overlapRectangle(.125, .875, .125, .875, 10, false)
	a.faces[0].Holes = [][]Use{overlapRectangle(.25, .5, .375, .625, 20, true)}
	b.faces[0].Outer = overlapRectangle(3, 4, 0, 2, 100, false)
	b.faces[0].Holes = nil
	r, e := a.IntersectCoincidentRectangularTrims(context.Background(), 0, b, 0, CoincidentRectangleOverlapOptions{})
	if e != nil || !r.Complete || r.UVArea[0].Cmp(big.NewRat(17, 240)) != 0 || r.UVArea[1].Cmp(big.NewRat(17, 16)) != 0 || len(r.Tiles) == 0 || len(r.Segments) != 0 || len(r.Points) != 0 {
		t.Fatal(r, e)
	}
	seenHole := false
	seenNonbinary := false
	for _, tile := range r.Tiles {
		for _, owners := range tile.BoundaryOwners {
			for _, owner := range owners {
				if owner.Source == 0 && owner.Loop == 1 {
					seenHole = true
					if owner.Edge < 20 || owner.Edge > 23 {
						t.Fatal(owner)
					}
				}
			}
		}
		for _, x := range tile.A.Min {
			if x.Cmp(big.NewRat(1, 5)) == 0 || x.Cmp(big.NewRat(1, 3)) == 0 {
				seenNonbinary = true
			}
		}
		// Independent original chart relation B=(2+3v,-1+5u).
		if tile.B.Min[0].Cmp(new(big.Rat).Add(big.NewRat(2, 1), new(big.Rat).Mul(big.NewRat(3, 1), tile.A.Min[1]))) != 0 || tile.B.Min[1].Cmp(new(big.Rat).Add(big.NewRat(-1, 1), new(big.Rat).Mul(big.NewRat(5, 1), tile.A.Min[0]))) != 0 {
			t.Fatal("mapped original ranges")
		}
	}
	if !seenHole || !seenNonbinary {
		t.Fatal("lost hole/parameter provenance")
	}
	r.UVArea[0].SetInt64(99)
	r.Tiles[0].A.Min[0].SetInt64(99)
	again, e := a.IntersectCoincidentRectangularTrims(context.Background(), 0, b, 0, CoincidentRectangleOverlapOptions{})
	if e != nil || again.UVArea[0].Cmp(big.NewRat(17, 240)) != 0 {
		t.Fatal("result mutated source", e)
	}
}
func TestCoincidentRectangularOverlapDimensions(t *testing.T) {
	a, b := coincidenceFaceFixture(t, false, false, false)
	b.faces[0].Surface = a.faces[0].Surface
	a.faces[0].Holes = nil
	b.faces[0].Holes = nil
	a.faces[0].Outer = overlapRectangle(0, .5, 0, .5, 10, false)
	for _, tc := range []struct {
		x, y          float64
		edges, points int
	}{{.5, 0, 1, 0}, {.5, .5, 0, 1}, {.75, .75, 0, 0}, {math.Nextafter(.5, 1), 0, 0, 0}} {
		b.faces[0].Outer = overlapRectangle(tc.x, 1, tc.y, tc.y+.5, 100, false)
		// Keep the disjoint test inside the independently proved support domain.
		if tc.y+.5 > 1 {
			b.faces[0].Outer = overlapRectangle(tc.x, 1, tc.y, 1, 100, false)
		}
		r, e := a.IntersectCoincidentRectangularTrims(context.Background(), 0, b, 0, CoincidentRectangleOverlapOptions{})
		if e != nil || !r.Complete || r.UVArea[0].Sign() != 0 || len(r.Segments) != tc.edges || len(r.Points) != tc.points {
			t.Fatal(tc, r, e)
		}
		if tc.points == 1 && len(r.Points[0].Owners) != 4 {
			t.Fatal("corner owners", r.Points)
		}
		if tc.edges == 1 && len(r.Segments[0].Owners) != 2 {
			t.Fatal("edge owners", r.Segments)
		}
	}
	a.faces[0].Outer = overlapRectangle(0, 1, 0, 1, 10, false)
	a.faces[0].Holes = [][]Use{overlapRectangle(.25, .75, .25, .75, 20, true)}
	b.faces[0].Outer = overlapRectangle(.25, .75, .25, .75, 100, false)
	r, e := a.IntersectCoincidentRectangularTrims(context.Background(), 0, b, 0, CoincidentRectangleOverlapOptions{})
	if e != nil || r.UVArea[0].Sign() != 0 || len(r.Segments) != 4 || len(r.Points) != 0 {
		t.Fatal("hole boundary counted as area", r, e)
	}
	b.faces[0].Outer = overlapRectangle(.375, .625, .375, .625, 100, false)
	r, e = a.IntersectCoincidentRectangularTrims(context.Background(), 0, b, 0, CoincidentRectangleOverlapOptions{})
	if e != nil || len(r.Tiles)+len(r.Segments)+len(r.Points) != 0 {
		t.Fatal("hole interior", r, e)
	}
}
func TestCoincidentRectangularOverlapLimits(t *testing.T) {
	a, b := coincidenceFaceFixture(t, false, false, false)
	b.faces[0].Holes = nil
	a.faces[0].Outer = overlapRectangle(.125, .875, .125, .875, 10, false)
	b.faces[0].Outer = overlapRectangle(3, 4, 0, 2, 100, false)
	for _, o := range []CoincidentRectangleOverlapOptions{{MaxCells: 1}, {MaxLoops: 2}, {Coincidence: intersect.SplineCoincidenceOptions{MaxOperations: 1}}} {
		r, e := a.IntersectCoincidentRectangularTrims(context.Background(), 0, b, 0, o)
		if e == nil || r.Complete || r.UVArea[0] != nil || len(r.Tiles) > 0 {
			t.Fatal("atomic quota", r, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, e := a.IntersectCoincidentRectangularTrims(ctx, 0, b, 0, CoincidentRectangleOverlapOptions{}); !errors.Is(e, context.Canceled) || r.Complete {
		t.Fatal(r, e)
	}

}

func TestCoincidentRectangularOverlapRejectsFoldedChart(t *testing.T) {
	a, b := coincidenceFaceFixture(t, false, false, false)
	var poles []geom.Vec3
	for _, x := range []float64{.25, -.25, .25} {
		for _, y := range []float64{0, 1} {
			poles = append(poles, geom.Vec3{X: x, Y: y})
		}
	}
	s, e := geom.NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	a.faces[0] = Face{Surface: s, Outer: overlapRectangle(0, .25, .25, .75, 10, false)}
	b.faces[0] = Face{Surface: s, Outer: overlapRectangle(.75, 1, .25, .75, 20, false)}
	// Parameter rectangles are disjoint, but the folded support maps them onto
	// the same physical area. A parameter-only disjoint answer would be false.
	r, e := a.IntersectCoincidentRectangularTrims(context.Background(), 0, b, 0, CoincidentRectangleOverlapOptions{})
	if e == nil || r.Complete || r.UVArea[0] != nil {
		t.Fatal("unproved injectivity", r, e)
	}
}

func TestCoincidentRectangularOverlapNativeReload(t *testing.T) {
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	b, _ := makeOpenSplineBox(t, [3]float64{.25, .25, 0}, [3]float64{.75, .75, 1}, 0)
	b.faces[4].Surface = a.faces[4].Surface

	var e error
	b, e = NewSolidContext(ctx, b.vertices, b.edges, b.faces, b.tolerance)
	if e != nil {
		t.Fatal(e)
	}
	var stream bytes.Buffer
	if e = WriteJSON(&stream, b); e != nil {
		t.Fatal(e)
	}
	b, e = ReadJSONContext(ctx, &stream, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := a.IntersectCoincidentRectangularTrims(ctx, 4, b, 4, CoincidentRectangleOverlapOptions{})
	if e != nil || !r.Complete || r.Faces != [2]int{4, 4} || r.UVArea[0].Cmp(big.NewRat(1, 4)) != 0 || len(r.Tiles) != 1 {
		t.Fatal(r, e)
	}
	for _, owners := range r.Tiles[0].BoundaryOwners {
		if len(owners) != 1 || owners[0].Source != 1 || owners[0].Face != 4 || owners[0].Edge != b.faces[4].Outer[owners[0].Use].Edge {
			t.Fatal("native retained owner", owners)
		}
	}
}

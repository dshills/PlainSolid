package trimmed

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestMassKnotTrianglePartitionAreaAndOrientation(t *testing.T) {
	triangle := [3]geom.Vec2{{}, {X: 1}, {X: .125, Y: 1}}
	u, v := []float64{0, .25, .5, .75, 1}, []float64{0, 1.0 / 3, 2.0 / 3, 1}
	for _, reverse := range []bool{false, true} {
		tri := triangle
		if reverse {
			tri[1], tri[2] = tri[2], tri[1]
		}
		pieces, e := splitMassTriangleAtKnots(context.Background(), tri, u, v, 100)
		if e != nil || len(pieces) < 5 {
			t.Fatal(len(pieces), e)
		}
		area := 0.0
		for _, piece := range pieces {
			cross := (piece[1].X-piece[0].X)*(piece[2].Y-piece[0].Y) - (piece[1].Y-piece[0].Y)*(piece[2].X-piece[0].X)
			if reverse && cross >= 0 || !reverse && cross <= 0 {
				t.Fatal(piece)
			}
			area += cross / 2
			for axis, knots := range [2][]float64{u, v} {
				for _, k := range knots {
					low, high := math.Inf(1), math.Inf(-1)
					for _, p := range piece {
						x := p.X
						if axis == 1 {
							x = p.Y
						}
						low = min(low, x)
						high = max(high, x)
					}
					if low < k && high > k {
						t.Fatal("triangle crosses a derivative crease", piece, k)
					}
				}
			}
		}
		want := .5
		if reverse {
			want = -.5
		}
		if math.Abs(area-want) > 2e-15 {
			t.Fatal(area)
		}
	}
	if _, e := splitMassTriangleAtKnots(context.Background(), triangle, u, v, 1); !errors.Is(e, parametric.ErrBudget) {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := splitMassTriangleAtKnots(ctx, triangle, u, v, 100); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestMassPropertiesPiecewiseBilinearCreasedRoof(t *testing.T) {
	// Independent closed boundary under z=1+|x-.5|+|y-.5|. This tests mass
	// integration directly, independent of any Boolean or shell constructor.
	points := []float64{0, .5, 1}
	knots := []float64{0, 0, .5, 1, 1}
	var roofPoles []geom.Vec3
	for _, u := range points {
		for _, v := range points {
			roofPoles = append(roofPoles, geom.Vec3{X: u, Y: v, Z: 1 + math.Abs(u-.5) + math.Abs(v-.5)})
		}
	}
	roof, _ := geom.NewBSplineSurface(1, 1, 3, 3, roofPoles, knots, knots, nil)
	bottom, _ := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	solid := Solid{vertices: []geom.Vec3{{}}, faces: []Face{{Surface: roof}, {Surface: bottom, Reversed: true}}}
	for axis := 0; axis < 2; axis++ {
		for side := 0; side < 2; side++ {
			var poles []geom.Vec3
			for _, s := range points {
				for _, height := range []float64{0, 1.5 + math.Abs(s-.5)} {
					p := geom.Vec3{X: float64(side), Y: s, Z: height}
					if axis == 1 {
						p.X, p.Y = s, float64(side)
					}
					poles = append(poles, p)
				}
			}
			wall, _ := geom.NewBSplineSurface(1, 1, 3, 2, poles, knots, []float64{0, 0, 1, 1}, nil)
			reverse := axis == 0 && side == 0 || axis == 1 && side == 1
			solid.faces = append(solid.faces, Face{Surface: wall, Reversed: reverse})
		}
	}
	for range solid.faces {
		solid.seeds = append(solid.seeds, []seed{{uv: [3]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}}}, {uv: [3]geom.Vec2{{}, {X: 1, Y: 1}, {Y: 1}}}})
	}
	p, e := solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10, MaxSubdivisions: 8, MaxSamples: 200000})
	if e != nil || math.Abs(p.Volume-1.5) > 1e-11 || math.Abs(p.Area-(8+math.Sqrt(3))) > 1e-10 || math.Abs(p.Centroid.X-.5) > 1e-11 || math.Abs(p.Centroid.Y-.5) > 1e-11 || math.Abs(p.Centroid.Z-55.0/72) > 1e-11 {
		t.Fatal(p, e)
	}
}

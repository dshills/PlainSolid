//go:build research

package trimmed

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestBuildProjectiveSplineRoofDifferenceWithAdjacentSideTerminals(t *testing.T) {
	ctx := context.Background()
	b := projectiveContourImprintFace(t, false, false, false)
	a := b
	// Independent laws A=1+xy+x+y-1/2 and B=1+xy. The section
	// connects (0,1/2) to (1/2,0) on adjacent footprint edges.
	knots := []float64{0, 0, 1, 1}
	var poles []geom.Vec3
	for u := range 2 {
		for v := range 2 {
			x, y := float64(u), float64(v)
			poles = append(poles, geom.Vec3{X: x, Y: y, Z: 1 + x*y + x + y - .5})
		}
	}
	raw, err := geom.NewBSplineSurface(1, 1, 2, 2, poles, knots, knots, nil)
	if err != nil {
		t.Fatal(err)
	}
	a.Surface = raw
	for _, axes := range [][2]int{{0, 1}, {1, 0}} {
		for i, tc := range []struct {
			pair         [2]Face
			vol, xy      float64
			faces, edges int
			verts, walls int
		}{
			{[2]Face{a, b}, 25.0 / 48, 129.0 / 200, 6, 12, 8, 4},
			{[2]Face{b, a}, 1.0 / 48, 1.0 / 8, 4, 6, 4, 2},
		} {
			q, err := BuildProjectiveSplineRoofDifference(ctx, tc.pair[0], tc.pair[1], axes, 0, ProjectiveRoofSolidOptions{})
			if err != nil || !q.Complete || len(q.Walls) != tc.walls {
				t.Fatal("both original sides of the oblique seam must close", axes, i, err)
			}
			s := q.Solid
			if len(s.faces) != tc.faces || len(s.edges) != tc.edges || len(s.vertices) != tc.verts {
				t.Fatal("adjacent section terminals changed closed topology", axes, i, len(s.faces), len(s.edges), len(s.vertices))
			}
			p, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if err != nil || math.Abs(p.Volume-tc.vol) > 1e-10 || math.Abs(p.Centroid.X-tc.xy) > 1e-10 ||
				math.Abs(p.Centroid.Y-tc.xy) > 1e-10 {
				t.Fatal("independent triangle and exterior integrals do not match native material", axes, i, p, err)
			}
		}
	}
}

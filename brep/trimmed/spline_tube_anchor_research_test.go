//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestCertifySplineTubeEdgeJunctionDifferentPairs(t *testing.T) {
	ctx := context.Background()
	s, _, _ := transverseSplineCaps(t)
	makeTool := func(y float64) *geom.BSplineSurface {
		t.Helper()
		b, e := geom.NewBSplineSurface(1, 1, 2, 2,
			[]geom.Vec3{{X: -.5, Y: y, Z: -1}, {X: -.5, Y: y, Z: 2}, {X: 1.5, Y: y, Z: -1}, {X: 1.5, Y: y, Z: 2}},
			[]float64{-.5, -.5, 1.5, 1.5}, []float64{-1, -1, 2, 2}, nil)
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	build := func(face, free int, y float64) intersect.TubeSegment {
		t.Helper()
		a := s.faces[face].Surface.(*geom.BSplineSurface)
		b := makeTool(y)
		da := geom.ParameterRectangle{UMax: 1, VMax: 1}
		db := geom.ParameterRectangle{UMin: -.5, UMax: 1.5, VMin: -1, VMax: 2}
		segments, e := intersect.RebuildTubeCurves(ctx, intersect.TubeRecipeData{
			A:        intersect.Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}},
			B:        intersect.Patch{Surface: b, Min: geom.Vec2{X: -.5, Y: -1}, Max: geom.Vec2{X: 1.5, Y: 2}},
			FreeAxis: free,
			Cells:    []intersect.TubeRecipeCell{{DomainA: da, DomainB: db}},
		})
		if e != nil {
			t.Fatal(e)
		}
		return segments[0]
	}
	end := func(face, axis int, side geom.SurfaceEdge, y int64) SplineTubeEdgeEnd {
		return SplineTubeEdgeEnd{Face: face, Segment: build(face, axis, float64(y)/4), Endpoint: SplineTubeEndpoint{Side: side,
			Hint: [4]*big.Rat{big.NewRat(0, 1), big.NewRat(y, 4), big.NewRat(0, 1), big.NewRat(0, 1)}}}
	}
	a := end(0, 0, geom.SurfaceEdgeUMin, 1)
	b := end(2, 1, geom.SurfaceEdgeVMin, 1)
	b.Endpoint.Hint[0], b.Endpoint.Hint[1] = b.Endpoint.Hint[1], b.Endpoint.Hint[0]
	r, e := s.CertifySplineTubeEdgeJunction(ctx, a, b, geom.TubeEndpointEdgeOptions{})
	if e != nil || !r.First.Proven || !r.Last.Proven || r.Anchor.Parameter != .75 {
		t.Fatal(r, e)
	}
	b = end(2, 1, geom.SurfaceEdgeVMin, 2)
	b.Endpoint.Hint[0], b.Endpoint.Hint[1] = b.Endpoint.Hint[1], b.Endpoint.Hint[0]
	if _, e = s.CertifySplineTubeEdgeJunction(ctx, a, b, geom.TubeEndpointEdgeOptions{}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal(e)
	}
}

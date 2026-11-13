package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestSplitSplineTubeWireAcrossFourFaces(t *testing.T) {
	ctx := context.Background()
	s, _, _ := transverseSplineCaps(t)
	k := []float64{-.5, -.5, 1.5, 1.5}
	tool, e := geom.NewBSplineSurface(1, 1, 2, 2,
		[]geom.Vec3{{X: -.5, Y: -.5, Z: .25}, {X: -.5, Y: 1.5, Z: .25}, {X: 1.5, Y: -.5, Z: .25}, {X: 1.5, Y: 1.5, Z: .25}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	var cuts []SplineTubeFaceCut
	for face := 2; face < 6; face++ {
		sp := s.faces[face].Surface.(*geom.BSplineSurface)
		d := geom.ParameterRectangle{UMax: 1, VMax: 1}
		toolDomain := geom.ParameterRectangle{UMin: -.5, UMax: 1.5, VMin: -.5, VMax: 1.5}
		seg, e := intersect.RebuildTubeCurves(ctx, intersect.TubeRecipeData{
			A:     intersect.Patch{Surface: sp, Max: geom.Vec2{X: 1, Y: 1}},
			B:     intersect.Patch{Surface: tool, Min: geom.Vec2{X: -.5, Y: -.5}, Max: geom.Vec2{X: 1.5, Y: 1.5}},
			Cells: []intersect.TubeRecipeCell{{DomainA: d, DomainB: toolDomain}},
		})
		if e != nil {
			t.Fatal(face, e)
		}
		end := func(u int64, side geom.SurfaceEdge) SplineTubeEndpoint {
			p, e := sp.Eval(float64(u), .25)
			if e != nil {
				t.Fatal(e)
			}
			return SplineTubeEndpoint{Side: side, Hint: [4]*big.Rat{big.NewRat(u, 1), big.NewRat(1, 4),
				new(big.Rat).SetFloat64(p.X), new(big.Rat).SetFloat64(p.Y)}}
		}
		cuts = append(cuts, SplineTubeFaceCut{Face: face, Segments: seg,
			Start: end(0, geom.SurfaceEdgeUMin), End: end(1, geom.SurfaceEdgeUMax)})
	}
	out, h, e := s.SplitSplineTubeWire(ctx, cuts, geom.TubeEndpointEdgeOptions{})
	if e != nil || len(h.OriginalEdges) != 4 || len(h.EdgeSplits) != 4 || len(h.Faces) != 4 || len(out.faces) != 10 || out.Topology().EulerCharacteristic != 2 {
		t.Fatal(h, out.Topology(), e)
	}
	seen := map[int]bool{}
	for _, p := range h.EdgeSplits {
		if seen[p.Vertex] || out.vertices[p.Vertex].Z != .25 {
			t.Fatal(p, out.vertices[p.Vertex])
		}
		seen[p.Vertex] = true
	}
	m, e := out.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(m.Volume-1) > 1e-9 {
		t.Fatal(m, e)
	}
	if _, _, e := s.SplitSplineTubeWire(ctx, cuts[:3], geom.TubeEndpointEdgeOptions{}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal(e)
	}
}

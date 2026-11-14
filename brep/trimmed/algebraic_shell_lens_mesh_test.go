package trimmed

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func TestAlgebraicBooleanCertifiedLensFill(t *testing.T) {
	t.Run("XY", func(t *testing.T) { checkAlgebraicBooleanCertifiedLensFill(t, false) })
	t.Run("YZ", func(t *testing.T) { checkAlgebraicBooleanCertifiedLensFill(t, true) })
}
func checkAlgebraicBooleanCertifiedLensFill(t *testing.T, rotated bool) {
	ctx := context.Background()
	a, b, hints := transverseSplineCaps(t)
	o := AlgebraicShellSectionOptions{Seeds: []AlgebraicShellSectionSeed{{Faces: [2]int{0, 1}, Tubes: hints}}}
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Trace.Coverage.Cell.MaxOperations = 16000000

	axes := [2]int{0, 1}
	if rotated {
		permute := func(s Solid) Solid {
			var faces []SewFace
			for _, f := range s.Faces() {
				d := f.Surface.(*geom.BSplineSurface).Data()
				for i, p := range d.Poles {
					d.Poles[i] = geom.Vec3{X: p.Z, Y: p.X, Z: p.Y}
				}
				support, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
				if e != nil {
					t.Fatal(e)
				}
				face, e := RectangularSplineFace(support, f.Reversed)
				if e != nil {
					t.Fatal(e)
				}
				faces = append(faces, face)
			}
			out, _, e := Sew(ctx, faces, SewOptions{Tolerance: 1e-8})
			if e != nil {
				t.Fatal(e)
			}
			return out
		}
		a, b = permute(a), permute(b)
		axes = [2]int{1, 2}
		o.Seeds = nil
		cuts := []float64{.25, .3125, .6875, .75}
		o.Trace.ProjectedGraphFallback = intersect.SplinePlanePairDiscoveryOptions{Enabled: true, UCuts: cuts, VCuts: cuts}
		o.Trace.ProjectedGraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
		o.Trace.GraphProjectionAxes = axes
	}
	section, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil {
		t.Fatal(e)
	}
	solid, e := section.CombineAlgebraic(ctx, parametric.Intersection, AlgebraicShellBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !sameShellMeshTube(solid.source.branches[0].Segments[0], solid.source.branches[0].Segments[0]) || sameShellMeshTube(solid.source.branches[0].Segments[0], solid.source.branches[0].Segments[1]) {
		t.Fatal("source segment ownership")
	}
	solid.Vertices = nil
	solid.Edges = nil
	solid.Faces = nil
	options := AlgebraicShellLensMeshOptions{}
	options.Section.Trace = o.Trace
	options.Section.Clip.Projection = &axes
	options.Mesh.Closed.Boundary.Run.Display.Deflection = .04
	options.Mesh.Closed.Interior.Deflection = .02
	result, e := solid.TessellateLensCertified(ctx, options)
	if e != nil {
		t.Fatal("filled Boolean lens", e)
	}
	if e := result.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if !result.Collision.Complete || len(result.Collision.Contacts) != 0 {
		t.Fatal("unproved display embedding")
	}
	if len(result.BoundaryEdges) != len(solid.index) || len(result.OriginalFace) != len(result.Mesh.Triangles) {
		t.Fatal("missing ancestry")
	}
	for edge, path := range result.BoundaryEdges {
		native := solid.index[edge]
		if path[0] != result.TopologyVertices[native.Start] || path[len(path)-1] != result.TopologyVertices[native.End] {
			t.Fatal("native corner ownership")
		}

		for _, vertex := range path {
			if vertex < 0 || vertex >= len(result.Mesh.Vertices) {
				t.Fatal("invalid shared vertex")
			}
		}
	}
	for id, vertex := range result.BoundaryVertices {
		if result.Mesh.Vertices[vertex] != result.Boundary.Run.Vertices[id] {
			t.Fatal("moved common seam")
		}
	}
	// Integral of r²−x²−y² over the disk r=5/16, independent of mesh.
	want := math.Pi * math.Pow(5.0/16, 4) / 2
	volume, _ := result.SignedDisplayVolume.Float64()
	if volume <= 0 || math.Abs(volume-want) > .15*want {
		t.Fatal("analytic lens volume", volume, want)
	}
	bound, _ := result.SourceImageSquaredDistance.Float64()
	if !(bound > 0) || math.IsInf(bound, 0) {
		t.Fatal("invalid source-distance proof")
	}
	for i, p := range result.Mesh.Vertices {
		if rotated {
			p = geom.Vec3{X: p.Y, Y: p.Z, Z: p.X}
		}
		// The common rounded seam may lie slightly off either support; all
		// displayed vertices must remain inside the independently certified bound.
		z := 25.0/256 - (p.X-.5)*(p.X-.5) - (p.Y-.5)*(p.Y-.5)
		if math.Min(math.Abs(p.Z), math.Abs(p.Z-z)) > math.Sqrt(bound)+1e-12 {
			t.Fatal("source witness", i, p, bound)
		}
	}
	t.Logf("triangles=%d edges=%d volume=%.9g exact=%.9g distance<=%.9g", len(result.Mesh.Triangles), len(result.BoundaryEdges), volume, want, math.Sqrt(bound))
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if bad, e := solid.TessellateLensCertified(cancelled, options); !errors.Is(e, context.Canceled) || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("cancelled fill", e)
	}
	if rotated {
		return
	}
	// Explicitly reject an otherwise valid unsupported material topology.
	union, e := section.CombineAlgebraic(ctx, parametric.Union, AlgebraicShellBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if bad, e := union.TessellateLensCertified(ctx, options); e == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("union accepted as lens", e)
	}
	tiny := options
	tiny.Mesh.Closed.Interior.MaxTriangles = 1
	if bad, e := solid.TessellateLensCertified(ctx, tiny); e == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("partial bounded fill", e)
	}
}

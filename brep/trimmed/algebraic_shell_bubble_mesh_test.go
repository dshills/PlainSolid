package trimmed

import (
	"context"
	"errors"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func TestBubbleCertifiedSharedBoundary(t *testing.T) {
	ctx := context.Background()
	a, _ := crossingFootprintRoofShellShape(t, false, true)
	b, _ := crossingFootprintRoofShellShape(t, true, true)
	o := AlgebraicShellSectionOptions{}
	o.Trace.UniformOverlapGraphFallback = intersect.UniformGraphDiscoveryOptions{GridDivisions: 16, AxisDivisions: [2]int{8, 16}}
	o.Trace.UniformOverlapGraphFallback.Enabled = true
	o.Trace.UniformOverlapGraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.UniformOverlapGraphFallback.MaxArcs = 1024
	o.Trace.UniformOverlapGraphFallback.MaxTubes = 1024
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	o.Trace.Branches.PruneSeparatedEndpoints = true
	o.Trace.Branches.MaxComparisons = 4096
	o.Trace.MaxTopologyPairs = 1000000
	o.Trace.Coverage.MaxCandidateTests = 4096
	o.Trace.Coverage.IsoPlaneFallback = true
	o.Trace.PlaneFallback.Enabled = true
	o.Trace.PlaneFallback.Plane.Arcs.ResolveSubdivisionCorners = true

	section, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil || !section.Complete {
		t.Fatal(e)
	}
	bc := AlgebraicShellBooleanOptions{}
	bc.Imprint.Exact.MaxOperations = 128000000
	solid, e := section.CombineAlgebraic(ctx, parametric.Intersection, bc)
	if e != nil {
		t.Fatal(e)
	}
	options := AlgebraicShellBoundaryOptions{Deflection: .02, MaxOperations: 128000000, AutoSectionProjection: true, PlanarSectionFallback: true}
	embedded, e := solid.TessellateEmbeddedBoundaryCertified(ctx, AlgebraicShellBoundaryEmbeddingOptions{Boundary: options})
	if e != nil {
		t.Fatal(e)
	}
	net := embedded.Boundary
	if len(net.Edges) != len(solid.index) || len(net.Coedges) != 2*len(net.Edges) || len(net.PlanarSections) == 0 || len(net.SectionProjections) == 0 || len(net.SourceCertificates) == 0 {
		t.Fatal("incomplete mixed-support network")
	}
	if embedded.PairChecks != net.Segments*(net.Segments-1)/2 {
		t.Fatal("missing display pair proof")
	}
	counts := make([]int, len(net.Edges))
	signs := make([]int, len(net.Edges))
	for _, use := range net.Coedges {
		counts[use.Edge]++
		sign := 1
		if use.Reversed {
			sign = -1
		}
		signs[use.Edge] += sign
		path := net.Edges[use.Edge]
		if len(use.Vertices) != len(path) {
			t.Fatal("incomplete use")
		}
		for i, v := range use.Vertices {
			j := i
			if use.Reversed {
				j = len(path) - 1 - i
			}
			if v != path[j] {
				t.Fatal("unshared display seam")
			}
		}
	}
	for i, path := range net.Edges {
		edge := solid.index[i]
		if counts[i] != 2 || signs[i] != 0 || path[0] != net.TopologyVertices[edge.Start] || path[len(path)-1] != net.TopologyVertices[edge.End] {
			t.Fatal("gap or inconsistent indexed incidence", i)
		}
	}
	for ei, cert := range net.SectionCertificates {
		edge := solid.index[ei]
		ids := [2]int{edge.Start, edge.End}
		if cert.TraversalReversed {
			ids[0], ids[1] = ids[1], ids[0]
		}
		for end, sample := range []CertifiedTubeGraphSample{cert.Samples[0], cert.Samples[len(cert.Samples)-1]} {
			expected := solid.vertices[ids[end]].SectionPoint
			if expected == nil {
				t.Fatal("missing original event")
			}
			same, _, err := expected.CompareModelPoint(ctx, sample.Identity.ModelEvent, geom.ExactCurvePlaneEventOptions{})
			if err != nil || !same {
				t.Fatal("independent endpoint model identity", ei, end, err)
			}
		}
	}
	for _, bad := range []AlgebraicShellBoundaryOptions{
		{Deflection: .02, PlanarSectionFallback: true},
		{Deflection: .02, AutoSectionProjection: true, Section: RationalTubePolylineOptions{PlanarPointEvaluation: true}},
		{Deflection: .02, AutoSectionProjection: true, MaxOperations: 1},
		{Deflection: .02, AutoSectionProjection: true, MaxSegments: 1},
	} {
		out, err := solid.TessellateBoundaryCertified(ctx, bad)
		if err == nil || len(out.Vertices) != 0 || len(out.Edges) != 0 {
			t.Fatal("non-atomic reject", err)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	out, err := solid.TessellateBoundaryCertified(cancelled, options)
	if !errors.Is(err, context.Canceled) || len(out.Vertices) != 0 {
		t.Fatal("cancel", err)
	}
	t.Logf("vertices=%d edges=%d segments=%d projections=%d planar=%d operations=%d pairs=%d", len(net.Vertices), len(net.Edges), net.Segments, len(net.SectionProjections), len(net.PlanarSections), net.ExactOperations, embedded.PairChecks)
}

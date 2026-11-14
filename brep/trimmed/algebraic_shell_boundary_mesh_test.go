package trimmed

import (
	"context"
	"errors"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestAlgebraicShellBooleanSharedBoundary(t *testing.T) {
	ctx := context.Background()
	a, b, hints := transverseSplineCaps(t)
	o := AlgebraicShellSectionOptions{Seeds: []AlgebraicShellSectionSeed{{Faces: [2]int{0, 1}, Tubes: hints}}}
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	section, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil {
		t.Fatal(e)
	}
	solid, e := section.CombineAlgebraic(ctx, parametric.Union, AlgebraicShellBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	options := AlgebraicShellBoundaryOptions{Deflection: .02, MaxOperations: 32000000}
	network, e := solid.TessellateBoundaryCertified(ctx, options)
	if e != nil {
		t.Fatal(e)
	}
	if len(network.SourceCertificates) == 0 || len(network.SectionCertificates) == 0 || len(network.Coedges) != 2*len(network.Edges) {
		t.Fatal("missing source/section ownership")
	}
	for _, c := range network.Coedges {
		p := network.Edges[c.Edge]
		for i, v := range c.Vertices {
			j := i
			if c.Reversed {
				j = len(p) - 1 - i
			}
			if v != p[j] {
				t.Fatal("unshared seam")
			}
		}
	}
	// This source section is the exact circle at z=0 with radius 5/16.
	// Every tube display lies within its independently enclosed representative
	// error of that circle, without assuming the polyline is a face fill.
	for _, c := range network.SectionCertificates {
		for _, v := range c.Samples {
			p := v.Point
			if p.Z != 0 {
				t.Fatal("nonplanar circle display", p)
			}
			dx, dy := p.X-.5, p.Y-.5
			res := dx*dx + dy*dy - 25.0/256
			if res < -0.002 || res > .002 {
				t.Fatal("circle witness", p, res)
			}
		}
	}
	solid.Vertices = nil
	solid.Edges = nil
	solid.Faces = nil
	solid.Topology = AlgebraicSubdivisionTopology{}
	embedded, e := solid.TessellateEmbeddedBoundaryCertified(ctx, AlgebraicShellBoundaryEmbeddingOptions{Boundary: options})
	if e != nil || !reflect.DeepEqual(network, embedded.Boundary) || embedded.PairChecks != network.Segments*(network.Segments-1)/2 {
		t.Fatal("public diagnostics changed immutable proof", e)
	}
	limitedEmbedding, err := solid.TessellateEmbeddedBoundaryCertified(ctx, AlgebraicShellBoundaryEmbeddingOptions{Boundary: options, MaxPairs: 1})
	if !errors.Is(err, geom.ErrConvergence) || len(limitedEmbedding.Boundary.Vertices) != 0 || limitedEmbedding.PairChecks != 0 {
		t.Fatal("partial embedding certificate", err)
	}
	options.MaxSegments = 1
	bad, e := solid.TessellateBoundaryCertified(ctx, options)
	if !errors.Is(e, geom.ErrConvergence) || len(bad.Vertices) != 0 {
		t.Fatal("partial budget result", e)
	}
	for _, limited := range []AlgebraicShellBoundaryOptions{
		{Deflection: .02, MaxCoedges: 1},
		{Deflection: .02, MaxOperations: 1},
	} {
		bad, err := solid.TessellateBoundaryCertified(ctx, limited)
		if err == nil || len(bad.Vertices) != 0 {
			t.Fatal("non-atomic bounded failure", err)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := solid.TessellateBoundaryCertified(cancelled, AlgebraicShellBoundaryOptions{Deflection: .02}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	t.Logf("vertices=%d edges=%d segments=%d operations=%d", len(network.Vertices), len(network.Edges), network.Segments, network.ExactOperations)
}

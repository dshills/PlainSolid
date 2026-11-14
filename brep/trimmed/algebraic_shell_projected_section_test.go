package trimmed

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestAlgebraicShellProjectedSharedBoundary(t *testing.T) {
	ctx := context.Background()
	a, b, _ := transverseSplineCaps(t)
	// A proper cyclic coordinate permutation maps the XY circle to the YZ plane.
	permute := func(s Solid) Solid {
		var faces []SewFace
		for _, f := range s.Faces() {
			d := f.Surface.(*geom.BSplineSurface).Data()
			for i, p := range d.Poles {
				d.Poles[i] = geom.Vec3{X: p.Z, Y: p.X, Z: p.Y}
			}
			surface, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
			if e != nil {
				t.Fatal(e)
			}
			face, e := RectangularSplineFace(surface, f.Reversed)
			if e != nil {
				t.Fatal(e)
			}
			faces = append(faces, face)
		}
		result, _, e := Sew(ctx, faces, SewOptions{Tolerance: 1e-8})
		if e != nil {
			t.Fatal(e)
		}
		return result
	}
	a, b = permute(a), permute(b)
	axes := [2]int{1, 2}
	cuts := []float64{.25, .3125, .6875, .75}
	d := intersect.SplinePlanePairDiscoveryOptions{Enabled: true, UCuts: cuts, VCuts: cuts}
	d.Plane.Arcs.ResolveSubdivisionCorners = true
	o := AlgebraicShellSectionOptions{}
	o.Trace.ProjectedGraphFallback = d
	o.Trace.GraphProjectionAxes = axes
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	section, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil {
		t.Fatalf("projected native section: pairs=%v error=%v", section.Pairs, e)
	}
	if !section.Complete || len(section.Branches) != 1 || !section.Branches[0].Closed {
		t.Fatal(section.Pairs, section.Complete)
	}
	result, e := section.CombineAlgebraic(ctx, parametric.Union, AlgebraicShellBooleanOptions{})
	if e != nil || result.Topology.EulerCharacteristic != 2 {
		t.Fatal("rotated shell Boolean must close on both originals", result.Topology, e)
	}
	mesh, e := result.TessellateBoundaryCertified(ctx, AlgebraicShellBoundaryOptions{Deflection: .04, MaxOperations: 64000000,
		Section: RationalTubePolylineOptions{Projection: &axes, Interval: geom.SurfaceTubeIntervalOptions{MaxIterations: 2}}})
	if e != nil || len(mesh.SourceCertificates) == 0 || len(mesh.SectionCertificates) == 0 || len(mesh.Coedges) != 2*len(mesh.Edges) {
		t.Fatal("rotated display lost shared topology or original ancestry", e)
	}
	for _, curve := range mesh.SectionCertificates {
		for _, x := range curve.Samples {
			p := x.Point
			dy, dz := p.Y-.5, p.Z-.5
			// Exact junction displays are outward representatives, not
			// exact evaluations in the YZ plane. Verify their distance to
			// the independent circle at the vertex allocation (.04/16).
			distance := math.Hypot(p.X, math.Hypot(dy, dz)-5.0/16)
			if distance > .04/16 {
				t.Fatal("display does not follow the independent YZ circle", p)
			}
		}
	}
}

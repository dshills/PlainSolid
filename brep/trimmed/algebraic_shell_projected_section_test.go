package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestAlgebraicShellProjectedSharedBoundary(t *testing.T) {
	slowtest.Skip(t)
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

func TestAlgebraicShellSectionRetainsAffinePlacement(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, b, _ := transverseSplineCaps(t)
	// Reflect the cyclic YZ instance and translate it without replacing
	// the original UV chart or its homogeneous spline coefficients.
	m := geom.Affine{{0, 0, -1, 3}, {1, 0, 0, -2}, {0, 1, 0, 1}}
	for _, s := range []*Solid{&a, &b} {
		q, e := s.Transformed(m)
		if e != nil {
			t.Fatal(e)
		}
		*s = q
	}
	axis := [2]int{1, 2}
	o := AlgebraicShellSectionOptions{}
	o.Trace.GraphProjectionAxes = axis
	o.Trace.ProjectedGraphFallback.Enabled = true
	o.Trace.ProjectedGraphFallback.UCuts = []float64{.25, .3125, .6875, .75}
	o.Trace.ProjectedGraphFallback.VCuts = o.Trace.ProjectedGraphFallback.UCuts
	o.Trace.ProjectedGraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	section, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil || !section.Complete || len(section.Branches) != 1 || !section.Branches[0].Closed {
		t.Fatal("complete closed section of affine originals", section.Pairs, e)
	}
	check := func(s AlgebraicShellSectionResult) {
		t.Helper()
		for _, edge := range s.Branches[0].Segments {
			base, placement := parametric.SurfaceMapping(edge.TrimB().SourceSurface())
			if _, ok := base.(*geom.BSplineSurface); !ok || placement != m {
				t.Fatal("lost exact placed original chart", placement)
			}
		}
		common, e := s.CombineAlgebraic(ctx, parametric.Intersection, AlgebraicShellBooleanOptions{})
		if e != nil || common.Topology.Faces != 2 || common.Topology.EulerCharacteristic != 2 {
			t.Fatal("closed placed common", common.Topology, e)
		}
	}
	check(section)
	var native bytes.Buffer
	if e := WriteAlgebraicShellSectionJSON(&native, section); e != nil {
		t.Fatal(e)
	}
	var tampered nativeAlgebraicShellSection
	if e := json.Unmarshal(native.Bytes(), &tampered); e != nil || len(tampered.Pairs) != 1 {
		t.Fatal(e)
	}
	// All proposals continue to agree with one another after this change,
	// but their placement no longer belongs to the stored original shell.
	for i := range tampered.Pairs[0].Tubes {
		q := &tampered.Pairs[0].Tubes[i].A.Surface
		var meta map[string]json.RawMessage
		if e = json.Unmarshal(*q, &meta); e != nil {
			t.Fatal(e)
		}
		var move geom.Affine
		if e = json.Unmarshal(meta["Transform"], &move); e != nil {
			t.Fatal(e)
		}
		move[1][3] += .125
		meta["Transform"], _ = json.Marshal(move)
		*q, _ = json.Marshal(meta)
	}
	bad, _ := json.Marshal(tampered)
	if _, e := ReadAlgebraicShellSectionJSON(ctx, bytes.NewReader(bad), JSONLimits{}, o); e == nil {
		t.Fatal("changed recipe placement bound to the original shell")
	}
	read, e := ReadAlgebraicShellSectionJSON(ctx, &native, JSONLimits{}, o)
	if e != nil || !read.Complete || len(read.Branches) != 1 {
		t.Fatal("placed original chart binding on reload", read.Pairs, e)
	}
	check(read)
}

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

func TestCertifiedDisjointBooleanLensFill(t *testing.T) {
	checkDisjointBooleanLensFill(t, false, false)
}
func TestCertifiedExactOwnerDisjointBooleanLensFill(t *testing.T) {
	checkDisjointBooleanLensFill(t, true, false)
}
func TestCertifiedYZDisjointBooleanLensFill(t *testing.T) {
	checkDisjointBooleanLensFill(t, false, true)
}
func checkDisjointBooleanLensFill(t *testing.T, exact, rotated bool) {
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{0, 0, 0}, [3]float64{1, 1, 1}, 0)
	b := disjointLoopRoof(t)
	if exact {
		a = exactPlaneOwnerForRegions(t, a)
	}
	// The stock builder stores its lower Z rectangle at index four.
	aFace, bFace := a.faces[4], b.faces[1]
	d, e := faceUVBounds(bFace)
	if e != nil {
		t.Fatal(e)
	}
	// The tube proposals are discovered separately on each dome span;
	// Section rechecks coverage across both full original face domains.
	o := AlgebraicShellSectionOptions{}
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	var hints intersect.SplineSeedReport
	for i, center := range []float64{5.0 / 16, 11.0 / 16} {
		span := d
		if i == 0 {
			span.UMax = .5
		} else {
			span.UMin = .5
		}
		p, e := algebraicShellTracePatch(aFace, span)
		if e != nil {
			t.Fatal(e)
		}
		q, e := algebraicShellTracePatch(bFace, span)
		if e != nil {
			t.Fatal(e)
		}
		opts := o.Trace.GraphFallback
		opts.UCuts = []float64{center - 1.0/16, center - 3.0/64, center + 3.0/64, center + 1.0/16}
		opts.VCuts = []float64{.5 - 1.0/16, .5 - 3.0/64, .5 + 3.0/64, .5 + 1.0/16}
		opts.Plane.Arcs.ResolveSubdivisionCorners = true
		r, e := intersect.DiscoverCommonXYGraphPairTubes(ctx, p, q, geom.SurfacePatchOptions{}, opts)
		if e != nil || len(r.Attempts) < 4 {
			t.Fatal("dome branch proposals", i, len(r.Attempts), e)
		}
		hints.Attempts = append(hints.Attempts, r.Attempts...)
	}
	axes := [2]int{0, 1}
	if rotated {
		rotate := func(s Solid) Solid {
			var faces []SewFace
			for _, f := range s.Faces() {
				d := f.Surface.(*geom.BSplineSurface).Data()
				for i, p := range d.Poles {
					d.Poles[i] = geom.Vec3{X: 2 - p.Z, Y: -3 - p.X, Z: 4 + p.Y}
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
			out, _, e := Sew(ctx, faces, SewOptions{Tolerance: 1e-9})
			if e != nil {
				t.Fatal(e)
			}
			return out
		}
		a, b = rotate(a), rotate(b)
		axes = [2]int{1, 2}
		o.Trace.GraphProjectionAxes = axes
		o.Trace.Coverage.OverlappingAffineXYFallback = true
		for i := range hints.Attempts {
			hints.Attempts[i].Sample = nil
			hints.Attempts[i].Section = nil
		}
	}
	o.Seeds = []AlgebraicShellSectionSeed{{Faces: [2]int{4, 1}, Tubes: hints}}
	r, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil || !r.Complete || len(r.Wires) != 2 || len(r.Branches) != 2 {
		t.Fatal("two complete closed sections", r.Complete, r.Pairs, len(r.Wires), len(r.Branches), e)
	}

	common, e := r.CombineAlgebraic(ctx, parametric.Intersection, AlgebraicShellBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	options := AlgebraicShellRegionsMeshOptions{Section: AlgebraicTrimmedPairSectionOptions{Trace: o.Trace}}
	options.Mesh.AffineCharts = true
	options.Section.Trace.GraphProjectionAxes = axes
	options.Section.Clip.Projection = &axes
	if exact {
		options.Mesh.Span.MaxDegree = 3
	}
	options.Mesh.Boundary.Loop.Run.Display.Deflection = .01
	options.Mesh.Component.Interior.Deflection = .002
	common.Faces = nil
	common.Edges = nil
	common.Vertices = nil
	m, e := common.TessellateDisjointLensesCertified(ctx, options)
	if e != nil {
		t.Fatal("multi-lens fill", e)
	}
	if m.ProjectionAxes != axes {
		t.Fatal("source projection lost")
	}
	if len(m.Components) != 2 || !m.Collision.Complete || len(m.Collision.Contacts) != 0 {
		t.Fatal("global collection embedding")
	}
	if e := m.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if len(m.TriangleFaces) != len(m.Mesh.Triangles) || len(m.BoundaryEdges) != len(common.index) {
		t.Fatal("native ancestry")
	}
	usedFaces := map[int]bool{}
	for _, f := range m.TriangleFaces {
		if f < 0 || f >= len(common.faces) {
			t.Fatal("face ID")
		}
		usedFaces[f] = true
	}
	if len(usedFaces) != 4 {
		t.Fatal("lost a native fragment")
	}
	for edge, path := range m.BoundaryEdges {
		native := common.index[edge]
		if path[0] != m.TopologyVertices[native.Start] || path[len(path)-1] != m.TopologyVertices[native.End] {
			t.Fatal("native endpoint mismatch")
		}
	}
	for component, ids := range m.ComponentVertices {
		for local, global := range ids {
			if m.Mesh.Vertices[global] != m.Components[component].Mesh.Vertices[local] {
				t.Fatal("post-welded component")
			}
		}
	}
	for _, component := range m.Components {
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, p := range component.Mesh.Vertices {
			x := p.X
			if rotated {
				x = -p.Y - 3
				if p.X > 2+1e-7 || p.Y >= -3 || p.Z <= 4 {
					t.Fatal("signed world restoration", p)
				}
			}
			lo = math.Min(lo, x)
			hi = math.Max(hi, x)
		}
		if !(hi < .5 || lo > .5) {
			t.Fatal("independent spatial separation", lo, hi)
		}
	}
	want := math.Pi * math.Pow(5.0/64, 4)
	volume, _ := m.SignedDisplayVolume.Float64()
	if volume <= 0 || math.Abs(volume-want) > .15*want {
		t.Fatal("two-lobe analytic integral", volume, want)
	}
	bound, _ := m.SourceImageSquaredDistance.Float64()
	if !(bound > 0) || math.IsInf(bound, 0) {
		t.Fatal("source bound")
	}
	t.Logf("components=%d triangles=%d volume=%g analytic=%g source-distance<=%g contact-pairs=%d", len(m.Components), len(m.Mesh.Triangles), volume, want, math.Sqrt(bound), m.ContactPairs)
	for _, bad := range []AlgebraicShellRegionsMeshOptions{{MaxComponents: 1}, {MaxContactPairs: 1}} {
		out, e := common.TessellateDisjointLensesCertified(ctx, bad)
		if !errors.Is(e, geom.ErrConvergence) || len(out.Mesh.Vertices) != 0 {
			t.Fatal("non-atomic budget", e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := common.TessellateDisjointLensesCertified(cancelled, options); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestDisjointBooleanMeshInvalidLimits(t *testing.T) {
	s := AlgebraicShellBoolean{source: &algebraicShellSectionSource{}}
	for _, loops := range []int{-1, 65} {
		o := AlgebraicShellRegionsMeshOptions{}
		o.Regions.MaxLoops = loops
		if _, e := s.TessellateDisjointLensesCertified(context.Background(), o); !errors.Is(e, geom.ErrInvalid) {
			t.Fatal("invalid authored loop bound was silently capped", e)
		}
	}
	o := AlgebraicShellRegionsMeshOptions{}
	o.Mesh.Component.Collision.MaxPairs = -1
	if _, e := s.TessellateDisjointLensesCertified(context.Background(), o); !errors.Is(e, geom.ErrInvalid) {
		t.Fatal(e)
	}
}

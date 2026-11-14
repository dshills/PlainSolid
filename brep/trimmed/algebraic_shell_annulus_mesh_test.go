package trimmed

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func TestCertifiedBooleanAnnulusFill(t *testing.T)   { checkCertifiedBooleanAnnulusFill(t, false) }
func TestCertifiedYZBooleanAnnulusFill(t *testing.T) { checkCertifiedBooleanAnnulusFill(t, true) }
func checkCertifiedBooleanAnnulusFill(t *testing.T, rotated bool) {
	if testing.Short() {
		t.Skip("complete nested Boolean section and certified annulus")
	}
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{-.125, -.125, 0}, [3]float64{1.125, 1.125, 1}, 0)
	b := nestedQuarticRoofSolid(t)
	x, e := algebraicShellTracePatch(a.faces[4], geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1})
	if e != nil {
		t.Fatal(e)
	}
	y, e := algebraicShellTracePatch(b.faces[1], geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1})
	if e != nil {
		t.Fatal(e)
	}
	h, e := intersect.DiscoverRadialGraphPairTubes(ctx, x, y, geom.SurfacePatchOptions{}, intersect.SplinePlanePairDiscoveryOptions{MaxTubes: 128})
	if e != nil {
		t.Fatal("nested loop proposals", e)
	}
	o := AlgebraicShellSectionOptions{Trace: nestedTubeSectionOptions().Trace, Seeds: []AlgebraicShellSectionSeed{{Faces: [2]int{4, 1}, Tubes: h}}}
	o.Trace.Coverage.OverlappingAffineXYFallback = true
	axes := [2]int{0, 1}
	if rotated {
		rotate := func(s Solid) Solid {
			var faces []SewFace
			for _, f := range s.Faces() {
				d := f.Surface.(*geom.BSplineSurface).Data()
				for i, p := range d.Poles {
					d.Poles[i] = geom.Vec3{X: -p.Z, Y: -p.X, Z: p.Y}
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
		// These unchanged UV cells/free axes are proposals only. The original
		// signed rotated supports must independently reprove every tube/coverage.
		for i := range h.Attempts {
			h.Attempts[i].Sample = nil
			h.Attempts[i].Section = nil
		}
		axes = [2]int{1, 2}
		o.Trace.GraphProjectionAxes = axes
		o.Seeds = []AlgebraicShellSectionSeed{{Faces: [2]int{4, 1}, Tubes: h}}
	}

	section, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil || !section.Complete || len(section.Wires) != 2 {
		t.Fatal("complete nested original shell section", section.Complete, len(section.Wires), e)
	}
	measure := AlgebraicShellBooleanOptions{}
	measure.Imprint.Exact.MaxOperations = 512000000
	common, e := section.CombineAlgebraic(ctx, parametric.Intersection, measure)
	if e != nil || len(common.faces) != 2 || common.Topology.InnerWires != 2 || common.Topology.EulerCharacteristic != 0 {
		t.Fatal("common retains one annular material region", common.Topology, e)
	}

	options := AlgebraicShellAnnulusMeshOptions{Section: nestedTubeSectionOptions(), Regions: nestedTubeRegionOptions()}
	options.Section.Trace.Coverage.OverlappingAffineXYFallback = true
	options.Section.Trace.GraphProjectionAxes = axes
	options.Section.Clip.Projection = &axes
	options.Mesh = AlgebraicTubeRegionMeshOptions{Footprint: AlgebraicTubeFootprintMeshOptions{Boundary: AlgebraicTubeRegionsBoundaryOptions{MaxOperations: 256000000, Loop: AlgebraicTubeLensBoundaryOptions{Run: AlgebraicTubeRunMeshOptions{Display: RationalTubePolylineOptions{Deflection: .03, MaxOperations: 64000000}, MaxPieces: 512}, MaxOwnerOperations: 64000000}}, Polygon: trimvalidate.Options{MaxEdges: 1024, MaxWork: 2000000}}, Surface: MultiSpanTubeLensMeshOptions{AffineCharts: true, Closed: AlgebraicTubeLensMeshOptions{Collision: collision.Options{MaxPairs: 16000000}, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .001}}, Span: MultiSpanTriangleBoundOptions{MaxDegree: 4}}}
	options.Mesh.Footprint.Boundary.Loop.Run.Partition.MaxOperations = 64000000
	common.Faces = nil
	common.Edges = nil
	common.Vertices = nil
	display, e := common.TessellateAnnulusCertified(ctx, options)
	if e != nil {
		t.Fatal("Boolean annular fill", e)
	}
	if display.ProjectionAxes != axes {
		t.Fatal("projection identity")
	}
	if rotated && (display.Region != -1 || display.Footprint.Region != -1) {
		t.Fatal("scratch region exposed as original")
	}
	if rotated {
		minX := 0.0
		for _, p := range display.Mesh.Vertices {
			minX = math.Min(minX, p.X)
			if p.X > 1e-7 || p.Y >= 0 || p.Z <= 0 {
				t.Fatal("signed world restoration", p)
			}
		}
		if minX >= -1e-5 {
			t.Fatal("lost nonplanar retained roof")
		}
		for _, p := range display.Footprint.Mesh.Vertices {
			if p.Z != 0 || p.X >= 0 || p.Y <= 0 {
				t.Fatal("YZ footprint coordinates", p)
			}
		}
	}

	if e := display.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if !display.Collision.Complete || len(display.Collision.Contacts) != 0 {
		t.Fatal("incomplete embedding")
	}
	edges := map[[2]int]bool{}
	for _, tri := range display.Mesh.Triangles {
		for i, a := range tri {
			b := tri[(i+1)%3]
			if a > b {
				a, b = b, a
			}
			edges[[2]int{a, b}] = true
		}
	}
	if len(display.Mesh.Vertices)-len(edges)+len(display.Mesh.Triangles) != 0 {
		t.Fatal("annular Euler characteristic")
	}
	if len(display.BoundaryEdges) != len(common.index) || len(display.TriangleFaces) != len(display.Mesh.Triangles) {
		t.Fatal("lost native ancestry")
	}
	owners := map[int]bool{}
	for _, f := range display.TriangleFaces {
		if f < 0 || f >= 2 {
			t.Fatal("invalid native face")
		}
		owners[f] = true
	}
	if len(owners) != 2 {
		t.Fatal("missing original chart")
	}
	for edge, path := range display.BoundaryEdges {
		original := common.index[edge]
		if len(path) < 2 || path[0] != display.TopologyVertices[original.Start] || path[len(path)-1] != display.TopologyVertices[original.End] {
			t.Fatal("native endpoint identity")
		}
	}
	want := math.Pi * (9.0 / 16) * math.Pow(25.0/256-25.0/1024, 3) / 6
	volume, _ := display.SignedDisplayVolume.Float64()
	if volume <= 0 || math.Abs(volume-want) > .2*want {
		t.Fatal("independent annular volume", volume, want)
	}
	bound, _ := display.SourceImageSquaredDistance.Float64()
	if bound <= 0 || math.IsInf(bound, 0) {
		t.Fatal("source distance")
	}
	t.Log("vertices", len(display.Mesh.Vertices), "triangles", len(display.Mesh.Triangles), "volume", volume, "analytic", want, "distance", math.Sqrt(bound))
	limited := options
	limited.Regions.MaxLoops = 1
	fail, e := common.TessellateAnnulusCertified(ctx, limited)
	if !errors.Is(e, geom.ErrConvergence) || len(fail.Mesh.Vertices) != 0 {
		t.Fatal("insufficient loop quota", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	fail, e = common.TessellateAnnulusCertified(cancelled, options)
	if !errors.Is(e, context.Canceled) || len(fail.Mesh.Vertices) != 0 {
		t.Fatal("atomic cancellation", e)
	}
	malformed := common
	malformed.faces = append([]AlgebraicShellBooleanFace(nil), common.faces...)
	malformed.faces[0].Holes = nil
	fail, e = malformed.TessellateAnnulusCertified(ctx, options)
	if !errors.Is(e, geom.ErrConvergence) || len(fail.Mesh.Vertices) != 0 {
		t.Fatal("missing ring accepted", e)
	}
}

// Public diagnostics cannot manufacture the immutable proof required by this
// bridge. Exercise atomic option/cancellation rejection without a costly source
// reconstruction, independently of the positive integration fixture.
func TestAnnulusMeshProofAndLimitGuards(t *testing.T) {
	ctx := context.Background()
	forged := AlgebraicShellBoolean{Faces: make([]AlgebraicShellBooleanFace, 2)}
	out, e := forged.TessellateAnnulusCertified(ctx, AlgebraicShellAnnulusMeshOptions{})
	if !errors.Is(e, geom.ErrInvalid) || len(out.Mesh.Vertices) != 0 {
		t.Fatal("public faces accepted as proof", e)
	}
	pending := AlgebraicShellBoolean{source: &algebraicShellSectionSource{}}
	for _, o := range []AlgebraicShellAnnulusMeshOptions{{Regions: AlgebraicTubeRegionOptions{MaxLoops: -1}}, {Endpoints: geom.ExactCurvePlaneEventOptions{MaxOperations: -1}}, {Mesh: AlgebraicTubeRegionMeshOptions{Surface: MultiSpanTubeLensMeshOptions{MaxSpanCells: -1}}}} {
		out, e = pending.TessellateAnnulusCertified(ctx, o)
		if !errors.Is(e, geom.ErrInvalid) || len(out.Mesh.Vertices) != 0 {
			t.Fatal("invalid quota", e)
		}
	}
	out, e = pending.TessellateAnnulusCertified(ctx, AlgebraicShellAnnulusMeshOptions{Regions: AlgebraicTubeRegionOptions{MaxLoops: 1}})
	if !errors.Is(e, geom.ErrConvergence) || len(out.Mesh.Vertices) != 0 {
		t.Fatal("one-loop quota", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	out, e = pending.TessellateAnnulusCertified(cancelled, AlgebraicShellAnnulusMeshOptions{})
	if !errors.Is(e, context.Canceled) || len(out.Mesh.Vertices) != 0 {
		t.Fatal("cancelled source", e)
	}
}

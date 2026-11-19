//go:build research

package trimmed

import (
	"context"
	"math"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestCertifiedOverlappingDisjointRegionsMesh(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1.0 / 64}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	xs := []float64{.0625, .28125, .5, .71875, .9375}
	ys := []float64{.25, .5, .75}
	var poles []geom.Vec3
	for i, x := range xs {
		xx := .21875 * .21875
		if i%2 == 1 {
			xx = -xx
		}
		for j, y := range ys {
			yy := .0625
			if j == 1 {
				yy = -yy
			}
			poles = append(poles, geom.Vec3{X: x, Y: y, Z: 25.0/1024 - xx - yy + x*y/64})
		}
	}
	b, e := geom.NewBSplineSurface(2, 2, 5, 3, poles, []float64{.0625, .0625, .0625, .5, .5, .9375, .9375, .9375}, []float64{.25, .25, .25, .75, .75, .75}, nil)
	if e != nil {
		t.Fatal(e)
	}
	pa := intersect.Patch{Surface: a, Min: geom.Vec2{X: .09375}, Max: geom.Vec2{X: .90625, Y: 1}}
	pb := intersect.Patch{Surface: b, Min: geom.Vec2{X: .0625, Y: .25}, Max: geom.Vec2{X: .9375, Y: .75}}
	o := intersect.OverlappingGraphTraceOptions{}
	o.Discovery.UCuts = []float64{0, .0625, .09375, .90625, .9375, 1, .375, .15625, .1875, .375, .40625, .59375, .625, .8125, .84375}
	o.Discovery.VCuts = []float64{0, .25, .75, 1, .375, .40625, .59375, .625}
	o.Discovery.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 16000000

	patches := [2]intersect.Patch{pa, pb}
	var faces [2]Face
	for i, p := range patches {
		points := [4]geom.Vec2{p.Min, {X: p.Max.X, Y: p.Min.Y}, p.Max, {X: p.Min.X, Y: p.Max.Y}}
		faces[i].Surface = p.Surface
		for j, q := range points {
			faces[i].Outer = append(faces[i].Outer, Use{Edge: j, UVStart: q, UVEnd: points[(j+1)%4]})
		}
	}
	section := AlgebraicTrimmedPairSectionOptions{Trace: o.Trace}
	regions, e := SealOverlappingAffineGraphRegions(ctx, patches, faces, o.Discovery, section, AlgebraicTubeRegionOptions{})
	if e != nil {
		t.Fatal(e)
	}

	options := AlgebraicTubeRegionsMeshOptions{Boundary: AlgebraicTubeRegionsBoundaryOptions{Loop: AlgebraicTubeLensBoundaryOptions{Run: AlgebraicTubeRunMeshOptions{Display: RationalTubePolylineOptions{Deflection: .025}}}}, Component: AlgebraicTubeLensMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .01}}}
	m, e := regions.TessellateDisjointClosedCertified(ctx, options)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Components) != 2 || !m.Collision.Complete || len(m.Collision.Contacts) != 0 || m.Mesh.ValidateSolid() != nil {
		t.Fatal("unproved aggregate")
	}
	for i, c := range m.Components {
		if c.Mesh.ValidateSolid() != nil || c.SourceImageSquaredDistance.Sign() < 0 || c.SourceImageSquaredDistance.Cmp(m.SourceImageSquaredDistance) > 0 {
			t.Fatal("component certificate", i)
		}
		for j, p := range c.Mesh.Vertices {
			if m.Mesh.Vertices[m.ComponentVertices[i][j]] != p {
				t.Fatal("component vertex ownership")
			}
		}
	}
	seen := map[int]int{}
	for i, region := range m.OriginalRegion {
		if region < 0 || region >= 2 || m.OriginalChart[i] < 0 || m.OriginalChart[i] >= 2 {
			t.Fatal("ancestry")
		}
		seen[region]++
	}
	if len(seen) != 2 || len(m.OriginalRegion) != len(m.Mesh.Triangles) {
		t.Fatal("missing component ancestry")
	}
	for loop, indices := range m.Boundary.LoopVertices {
		for local, global := range indices {
			if m.Mesh.Vertices[global] != m.Boundary.Loops[loop].Run.Vertices[local] {
				t.Fatal("rim changed")
			}
		}
	}
	want := math.Pi * math.Pow(5.0/32, 4)
	volume, _ := m.SignedDisplayVolume.Float64()
	if math.Abs(volume-want) > .15*want {
		t.Fatal("two independent cap integrals", volume, want)
	}
	bound, _ := m.SourceImageSquaredDistance.Float64()
	t.Logf("two components, %d vertices/%d triangles; image distance upper %.9g; display volume %.12g vs analytic %.12g", len(m.Mesh.Vertices), len(m.Mesh.Triangles), math.Sqrt(bound), volume, want)
	options.Component.Interior.MaxTriangles = 1
	bad, e := regions.TessellateDisjointClosedCertified(ctx, options)
	if e == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("partial aggregate on limit", e)
	}
	// The early aggregate gate must preserve nested rim semantics rather than
	// silently flattening them. Internal deliberately invalid topology exercises
	// rejection without spending time reconstructing an unrelated quartic fixture.
	nested := regions
	nested.loops = append([]AlgebraicTubeRegionLoop(nil), regions.loops...)
	nested.loops[1].Parent = 0
	bad, e = nested.TessellateDisjointClosedCertified(ctx, AlgebraicTubeRegionsMeshOptions{})
	if e == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("nested rim flattened", e)
	}
}

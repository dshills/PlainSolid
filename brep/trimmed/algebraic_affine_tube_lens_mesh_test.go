package trimmed

import (
	"context"
	"math"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func algebraicAffineRunMeshFixture(t *testing.T) AlgebraicTrimmedPairSection {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1.0 / 64}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	x, z := [3]float64{0, .5, 1}, [3]float64{.25, -.25, .25}
	var poles []geom.Vec3
	for i := range x {
		for j := range x {
			poles = append(poles, geom.Vec3{X: x[i], Y: x[j], Z: z[i] + z[j] - 25.0/256 + x[i]*x[j]/64})
		}
	}
	k = []float64{0, 0, 0, 1, 1, 1}
	b, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}

	placed := func(s *geom.BSplineSurface, reverse bool) *geom.BSplineSurface {
		d := s.Data()
		for i := range d.Poles {
			d.Poles[i].X *= 2
			d.Poles[i].Y *= .5
		}
		if reverse {
			for i := 0; i < d.CountU/2; i++ {
				for j := 0; j < d.CountV; j++ {
					a, b := i*d.CountV+j, (d.CountU-1-i)*d.CountV+j
					d.Poles[a], d.Poles[b] = d.Poles[b], d.Poles[a]
				}
			}
		}
		s, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	a, b = placed(a, true), placed(b, false)
	patches := [2]intersect.Patch{{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, {Surface: b, Max: geom.Vec2{X: 1, Y: 1}}}
	loop := func(p [4]geom.Vec2) []Use {
		r := make([]Use, 4)
		for i := range r {
			r[i] = Use{Edge: i, UVStart: p[i], UVEnd: p[(i+1)%4]}
		}
		return r
	}
	outer := loop([4]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}})
	faces := [2]Face{{Surface: a, Outer: outer}, {Surface: b, Outer: outer}}
	o := AlgebraicTrimmedPairSectionOptions{}
	o.Trace.Coverage.AffineXYFallback = true
	cuts := []float64{.25, .3125, .6875, .75}
	o.Trace.AffineGraphFallback.UCuts, o.Trace.AffineGraphFallback.VCuts = cuts, cuts
	o.Trace.AffineGraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	// Circle (u-1/2)^2+(v-1/2)^2=25/256; exact graph discovery uses
	// rational 3-4-5 switch points, followed by independent coverage.
	h, e := intersect.DiscoverAffineXYGraphPairTubes(ctx, patches[0], patches[1], geom.SurfacePatchOptions{}, o.Trace.AffineGraphFallback)
	if e != nil || len(h.Attempts) < 4 {
		t.Fatal(len(h.Attempts), e)
	}
	r, e := SectionTrimmedSplinePair(ctx, patches, faces, &h, o)
	if e != nil || !r.Complete || len(r.Branches) != 1 || len(r.Branches[0].Runs) != 1 ||
		!r.Branches[0].Runs[0].Closed || len(r.Branches[0].Runs[0].Intervals) != len(h.Attempts) {
		t.Fatalf("complete=%v branches=%d trace=%v err=%v", r.Complete, len(r.Branches), r.Trace.Issues, e)
	}
	return r
}
func TestCertifiedAffineTubeLensMesh(t *testing.T) {
	ctx := context.Background()
	r := algebraicAffineRunMeshFixture(t)
	l, e := r.SealAlgebraicTubeLens(ctx, AlgebraicTubeLensOptions{})
	if e != nil {
		t.Fatal(e)
	}
	o := AlgebraicTubeLensMeshOptions{Boundary: AlgebraicTubeLensBoundaryOptions{Run: AlgebraicTubeRunMeshOptions{Display: RationalTubePolylineOptions{Deflection: .06}}}, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .02}}
	if _, e := l.TessellateClosedCertified(ctx, o); e == nil {
		t.Fatal("identity chart gate")
	}
	m, e := l.TessellateAffineClosedCertified(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	if !m.Collision.Complete || len(m.Collision.Contacts) > 0 {
		t.Fatal("contact")
	}
	if e := m.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	v, _ := m.SignedDisplayVolume.Float64()
	want := math.Pi * math.Pow(5.0/16, 4) / 2
	if math.Abs(v-want) > .15*want {
		t.Fatal(v, want)
	}
	for id, index := range m.BoundaryVertices {
		if m.Mesh.Vertices[index] != m.Boundary.Run.Vertices[id] {
			t.Fatal("seam identity")
		}
	}
	t.Log(len(m.Mesh.Triangles), v, m.TriangleChecks)
}

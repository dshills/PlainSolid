package trimmed

import (
	"context"
	"fmt"
	"math"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"plainsolid/intersect"
	"testing"
)

func TestCertifiedProjectedTubeLensXZ(t *testing.T) {
	slowtest.Skip(t)
	for _, axes := range [][2]int{{0, 2}, {1, 2}, {2, 0}} {
		t.Run(fmt.Sprint(axes), func(t *testing.T) { checkCertifiedProjectedTubeLens(t, axes) })
	}
}
func checkCertifiedProjectedTubeLens(t *testing.T, axes [2]int) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Z: 1}, {X: 1}, {X: 1, Y: 1.0 / 64, Z: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	x, z := [3]float64{0, .5, 1}, [3]float64{.25, -.25, .25}
	var net []geom.Vec3
	for i := range x {
		for j := range x {
			net = append(net, geom.Vec3{X: x[i], Z: x[j], Y: z[i] + z[j] - 25.0/256 + x[i]*x[j]/64})
		}
	}
	k = []float64{0, 0, 0, 1, 1, 1}
	b, e := geom.NewBSplineSurface(2, 2, 3, 3, net, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}

	permute := func(s *geom.BSplineSurface) *geom.BSplineSurface {
		d := s.Data()
		for i, p := range d.Poles {
			var q [3]float64
			q[axes[0]], q[axes[1]], q[3-axes[0]-axes[1]] = p.X, p.Z, p.Y
			d.Poles[i] = geom.Vec3{X: q[0], Y: q[1], Z: q[2]}
		}
		out, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if e != nil {
			t.Fatal(e)
		}
		return out
	}
	a, b = permute(a), permute(b)
	unit := geom.Vec2{X: 1, Y: 1}
	patch := func(s *geom.BSplineSurface) intersect.Patch { return intersect.Patch{Surface: s, Max: unit} }
	cuts := []float64{.25, .3125, .6875, .75}
	d := intersect.SplinePlanePairDiscoveryOptions{Enabled: true, UCuts: cuts, VCuts: cuts}
	d.Plane.Arcs.ResolveSubdivisionCorners = true
	v := [4]geom.Vec2{{}, {X: 1}, unit, {Y: 1}}
	var uses []Use
	for i, x := range v {
		uses = append(uses, Use{Edge: i, UVStart: x, UVEnd: v[(i+1)%4]})
	}
	o := AlgebraicTrimmedPairSectionOptions{}
	o.Trace.ProjectedGraphFallback = d
	o.Trace.GraphProjectionAxes = axes
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	s, e := SectionTrimmedSplinePair(ctx, [2]intersect.Patch{patch(a), patch(b)}, [2]Face{{Surface: a, Outer: uses}, {Surface: b, Outer: uses}}, nil, o)
	if e != nil || !s.Complete {
		t.Fatalf("XZ section issues %v err=%v", s.Trace.Issues, e)
	}
	l, e := s.SealProjectedAlgebraicTubeLens(ctx, AlgebraicTubeLensOptions{})
	if e != nil || l.Topology().EulerCharacteristic != 2 || l.ResidualAxis() != 3-axes[0]-axes[1] || l.ProjectionAxes() != axes {
		t.Fatalf("XZ lens topology %+v err=%v", l.Topology(), e)
	}

	options := MultiSpanTubeLensMeshOptions{Closed: AlgebraicTubeLensMeshOptions{Boundary: AlgebraicTubeLensBoundaryOptions{Run: AlgebraicTubeRunMeshOptions{Display: RationalTubePolylineOptions{Deflection: .04}}}, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .02}}}
	m, e := l.TessellateClosedCertified(ctx, options)
	if e != nil {
		t.Fatal(e)
	}
	if e := m.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if !m.Collision.Complete || len(m.Collision.Contacts) != 0 {
		t.Fatal("embedding")
	}
	for id, local := range m.BoundaryVertices {
		if m.Mesh.Vertices[local] != m.Boundary.Run.Vertices[id] {
			t.Fatal("seam changed")
		}
	}
	want := math.Pi * math.Pow(5.0/16, 4) / 2
	volume, _ := m.SignedDisplayVolume.Float64()
	if math.Abs(volume-want) > .15*want {
		t.Fatal(volume, want)
	}
	if m.Mesh.SignedVolume() <= 0 {
		t.Fatal("physical orientation")
	}
	if m.Boundary.Owners[0][0].Reversed == m.Boundary.Owners[1][0].Reversed {
		t.Fatal("incidence")
	}
	t.Log(len(m.Mesh.Vertices), len(m.Mesh.Triangles), volume, want)
	options.Closed.Interior.MaxTriangles = 1
	bad, e := l.TessellateClosedCertified(ctx, options)
	if e == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("partial limit result", e)
	}
}

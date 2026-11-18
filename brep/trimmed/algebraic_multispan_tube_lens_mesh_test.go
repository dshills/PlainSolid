package trimmed

import (
	"context"
	"math"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"plainsolid/intersect"
	"testing"
)

// The true C0 knot is outside the closed section but inside the original
// support. Interior knot crossings remain a source-tube discovery prerequisite.
func algebraicMultiSpanRunMeshFixture(t *testing.T, crease bool) AlgebraicTrimmedPairSection {
	return algebraicKnotRunMeshFixture(t, crease, .0625)
}
func algebraicKnotRunMeshFixture(t *testing.T, crease bool, knot float64) AlgebraicTrimmedPairSection {
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

	refine := func(s *geom.BSplineSurface) *geom.BSplineSurface {
		p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		p, e = p.InsertUKnot(knot, s.Data().DegreeU, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		s, e = p.ToBSplineSurface()
		if e != nil {
			t.Fatal(e)
		}
		if crease {
			d := s.Data()
			for i, p := range d.Poles {
				d.Poles[i].Z += math.Abs(p.X-knot) / 8
			}
			s, e = geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
			if e != nil {
				t.Fatal(e)
			}
		}
		return s
	}
	a, b = refine(a), refine(b)
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
	cuts := []float64{.25, .3125, .6875, .75}
	o.Trace.GraphFallback.UCuts, o.Trace.GraphFallback.VCuts = cuts, cuts
	o.Trace.GraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	// Circle (u-1/2)^2+(v-1/2)^2=25/256; exact graph discovery uses
	// rational 3-4-5 switch points, followed by independent coverage.
	h, e := intersect.DiscoverCommonXYGraphPairTubes(ctx, patches[0], patches[1], geom.SurfacePatchOptions{}, o.Trace.GraphFallback)
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
func TestCertifiedMultiSpanTubeLensMesh(t *testing.T) {
	slowtest.Skip(t)
	for _, crease := range []bool{false, true} {
		r := algebraicMultiSpanRunMeshFixture(t, crease)
		l, e := r.SealAlgebraicTubeLens(context.Background(), AlgebraicTubeLensOptions{})
		if e != nil {
			t.Fatal(crease, e)
		}
		o := MultiSpanTubeLensMeshOptions{Closed: AlgebraicTubeLensMeshOptions{Boundary: AlgebraicTubeLensBoundaryOptions{Run: AlgebraicTubeRunMeshOptions{Display: RationalTubePolylineOptions{Deflection: .04}}}, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .02}}}
		m, e := l.TessellateMultiSpanClosedCertified(context.Background(), o)
		if e != nil {
			t.Fatal(crease, e)
		}
		if m.SpanCellChecks < 2 || m.SpanPieceChecks == 0 {
			t.Fatal("missing span certificate")
		}
		if !m.Collision.Complete || len(m.Collision.Contacts) > 0 {
			t.Fatal("display contacts")
		}
		if e := m.Mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		for id, local := range m.BoundaryVertices {
			if m.Mesh.Vertices[local] != m.Boundary.Run.Vertices[id] {
				t.Fatal("changed seam")
			}
		}
		v, _ := m.SignedDisplayVolume.Float64()
		radius := 5.0 / 16
		want := math.Pi * math.Pow(radius, 4) / 2
		if math.Abs(v-want) > .15*want {
			t.Fatal("common crease must preserve source volume", crease, v, want)
		}

		if crease {
			patch, e := geom.NewSurfacePatch(l.faces[0].Support, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			uv := [3]geom.Vec2{{}, {X: .125}, {Y: .5}}
			var model [3]geom.Vec3
			exact := func(u, v float64) geom.Vec3 { return geom.Vec3{X: u, Y: v, Z: u*v/64 + math.Abs(u-.0625)/8} }
			for i, p := range uv {
				model[i] = exact(p.X, p.Y)
			}
			certificate, e := BoundMultiSpanSplineTriangleDeviation(context.Background(), patch, uv, model, MultiSpanTriangleBoundOptions{})
			if e != nil || certificate.Cells != 2 || !certificate.BothDirections {
				t.Fatal("independent crease residual", certificate, e)
			}
			bound, _ := certificate.SquaredDistance.Float64()
			for i := 0; i <= 20; i++ {
				for j := 0; j <= 20-i; j++ {
					u, v := float64(i)/20, float64(j)/20
					p := exact(.125*u, .5*v)
					q := model[0].Scale(1 - u - v).Add(model[1].Scale(u)).Add(model[2].Scale(v))
					if p.Sub(q).Dot(p.Sub(q)) > bound+1e-15 {
						t.Fatal("crease enclosure")
					}
				}
			}
		}
		t.Log(crease, len(m.Mesh.Triangles), m.SpanCellChecks, m.SpanPieceChecks, v)
		o.MaxSpanCells = 1
		if partial, e := l.TessellateMultiSpanClosedCertified(context.Background(), o); e == nil || len(partial.Mesh.Vertices) > 0 {
			t.Fatal("partial span budget", e)
		}
	}
}

func TestCertifiedTubeLensActualCrossKnotCrease(t *testing.T) {
	slowtest.Skip(t)
	r := algebraicKnotRunMeshFixture(t, true, .5)
	l, e := r.SealAlgebraicTubeLens(context.Background(), AlgebraicTubeLensOptions{})
	if e != nil {
		t.Fatal(e)
	}
	o := MultiSpanTubeLensMeshOptions{Closed: AlgebraicTubeLensMeshOptions{Boundary: AlgebraicTubeLensBoundaryOptions{Run: AlgebraicTubeRunMeshOptions{Display: RationalTubePolylineOptions{Deflection: .04}}}, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .02}}}
	m, e := l.TessellateMultiSpanClosedCertified(context.Background(), o)
	if e != nil {
		t.Fatal(e)
	}
	if !m.Collision.Complete || len(m.Collision.Contacts) > 0 {
		t.Fatal("contacts")
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
			t.Fatal("moved seam")
		}
	}
	cross := 0
	for _, tri := range m.Mesh.Triangles {
		lo, hi := 1., 0.
		for _, id := range tri {
			p := m.Mesh.Vertices[id]
			lo = math.Min(lo, p.X)
			hi = math.Max(hi, p.X)
		}
		if lo < .5 && hi > .5 {
			cross++
		}
	}
	if cross == 0 || m.SpanPieceChecks <= m.TriangleChecks {
		t.Fatal("missing genuine cross-knot triangle qualification", cross, m.SpanPieceChecks, m.TriangleChecks)
	}
	t.Log(len(m.Mesh.Triangles), cross, m.SpanCellChecks, m.SpanPieceChecks, v)
}

// Loose endpoint representatives can lie outside the true circle even though
// the continuous polyline distance is certified. A shell certificate must not
// promote that independent boundary certificate to an embedded material mesh.
func TestClosedTubeLensRejectsOutwardDisplay(t *testing.T) {
	slowtest.Skip(t)
	r := algebraicKnotRunMeshFixture(t, true, .5)
	l, e := r.SealAlgebraicTubeLens(context.Background(), AlgebraicTubeLensOptions{})
	if e != nil {
		t.Fatal(e)
	}
	o := MultiSpanTubeLensMeshOptions{Closed: AlgebraicTubeLensMeshOptions{Boundary: AlgebraicTubeLensBoundaryOptions{Run: AlgebraicTubeRunMeshOptions{Display: RationalTubePolylineOptions{Deflection: .04, Point: geom.TubeAlgebraicGraphEndpointOptions{Tolerance: .0025}}}}, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .02}}}
	m, e := l.TessellateMultiSpanClosedCertified(context.Background(), o)
	if e == nil || len(m.Mesh.Vertices) > 0 {
		t.Fatal("unsafe outward display accepted", e)
	}
}

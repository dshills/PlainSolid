package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func algebraicRunMeshFixture(t *testing.T) AlgebraicTrimmedPairSection {
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
func TestCertifiedAlgebraicTubeRunMesh(t *testing.T) {
	r := algebraicRunMeshFixture(t)
	o := AlgebraicTubeRunMeshOptions{Display: RationalTubePolylineOptions{Deflection: .08, MaxOperations: 32000000}}
	m, e := r.TessellateTubeRunCertified(context.Background(), 0, 0, 0, o)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.PieceVertices) < 4 {
		t.Fatal("missing piece provenance")
	}
	for i, ids := range m.PieceVertices {
		next := m.PieceVertices[(i+1)%len(m.PieceVertices)]
		if ids[len(ids)-1] != next[0] {
			t.Fatal("unshared junction")
		}
		for j, p := range m.Samples[i] {
			if m.Vertices[ids[j]] != p.Point {
				t.Fatal("display identity")
			}
			radius := math.Hypot(p.Point.X-.5, p.Point.Y-.5)
			if math.Abs(radius-5.0/16) > .005 {
				t.Fatal(radius)
			}
			if math.Abs(p.Point.Z-p.Point.X*p.Point.Y/64) > .005 {
				t.Fatal(p.Point)
			}
		}
		for _, q := range m.SquaredErrors[i] {
			v, _ := q.Float64()
			if v > .08*.08 {
				t.Fatal(v)
			}
		}
	}
	t.Log(len(m.Vertices), len(m.PieceVertices), m.ExactOperations)
	// Public reports are not authoritative endpoint bindings.
	r.Branches = nil
	if _, e := r.TessellateTubeRunCertified(context.Background(), 0, 0, 0, o); e != nil {
		t.Fatal("mutable report changed proof", e)
	}
	o.Display.MaxOperations = 1
	if m, e := r.TessellateTubeRunCertified(context.Background(), 0, 0, 0, o); e == nil || len(m.Vertices) != 0 {
		t.Fatal("work limit returned partial", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := r.TessellateTubeRunCertified(ctx, 0, 0, 0, AlgebraicTubeRunMeshOptions{Display: RationalTubePolylineOptions{Deflection: .08}}); e == nil {
		t.Fatal("cancel")
	}
}

func TestCertifiedTubeRunIrrationalEnds(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 0, 1, 1, 1}
	var poles []geom.Vec3
	for i := range 3 {
		for j := range 3 {
			z := 0.
			if i == 2 {
				z++
			}
			if j == 2 {
				z++
			}
			poles = append(poles, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z})
		}
	}
	a, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	k = []float64{0, 0, 1, 1}
	b, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Z: .5}, {Y: 1, Z: .5}, {X: 1, Z: .5}, {X: 1, Y: 1, Z: .5}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	d := geom.ParameterRectangle{UMin: 7.0 / 32, UMax: .25, VMin: 5.0 / 8, VMax: 11.0 / 16}
	other := geom.ParameterRectangle{UMin: 3.0 / 16, UMax: 9.0 / 32, VMin: 19.0 / 32, VMax: 23.0 / 32}
	chart := func(s *geom.BSplineSurface, r geom.ParameterRectangle) intersect.Patch {
		return intersect.Patch{Surface: s, Min: geom.Vec2{X: r.UMin, Y: r.VMin}, Max: geom.Vec2{X: r.UMax, Y: r.VMax}}
	}
	pa, e := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pb, e := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pa, e = pa.Trim(d)
	if e != nil {
		t.Fatal(e)
	}
	pb, e = pb.Trim(other)
	if e != nil {
		t.Fatal(e)
	}
	proof, e := geom.CertifySurfacePairTube(pa, pb, geom.SurfacePairTubeOptions{FreeAxis: 0})
	if e != nil || !proof.ProvenBranch {
		t.Fatal(proof, e)
	}
	seeds := intersect.SplineSeedReport{Attempts: []intersect.SplineSeedCell{{Cell: geom.SurfaceSurfaceCell{DomainA: d, DomainB: other}, Tube: &proof}}}
	x, y := chart(a, d), chart(b, other)

	rect := func(minU, minV, maxU, maxV float64) []Use {
		p := [4]geom.Vec2{{X: minU, Y: minV}, {X: maxU, Y: minV}, {X: maxU, Y: maxV}, {X: minU, Y: maxV}}
		r := make([]Use, 4)
		for i := range r {
			r[i] = Use{Edge: i, UVStart: p[i], UVEnd: p[(i+1)%4]}
		}
		return r
	}
	faces := [2]Face{{Surface: a, Outer: rect(d.UMin, 85.0/128, d.UMax, 43.0/64)}, {Surface: b, Outer: rect(other.UMin, other.VMin, other.UMax, other.VMax)}}
	r, e := SectionTrimmedSplinePair(ctx, [2]intersect.Patch{x, y}, faces, &seeds, AlgebraicTrimmedPairSectionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := r.TessellateTubeRunCertified(ctx, 0, 0, 0, AlgebraicTubeRunMeshOptions{Display: RationalTubePolylineOptions{Deflection: .004}})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Samples) != 1 {
		t.Fatal(len(m.Samples))
	}
	samples := m.Samples[0]
	for _, s := range []AlgebraicTubeRunSample{samples[0], samples[len(samples)-1]} {
		if s.Free == nil || s.Free.Interval().Lower.Cmp(s.Free.Interval().Upper) == 0 {
			t.Fatal("lost irrational free root")
		}
		if math.Abs(s.Point.X*s.Point.X+s.Point.Y*s.Point.Y-.5) > .0003 {
			t.Fatal(s.Point)
		}
	}
	for _, s := range samples {
		if math.Abs(s.Point.Z-.5) > .0003 {
			t.Fatal(s.Point)
		}
	}

	free := func(s AlgebraicTubeRunSample) float64 {
		if s.Rational != nil {
			v, _ := s.Rational.Float64()
			return v
		}
		b, e := s.Free.Enclose(ctx, big.NewRat(1, 1000000000000), geom.ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		v, _ := b.Interval.Lower.Float64()
		return v
	}
	for i := 0; i+1 < len(samples); i++ {
		a, b := samples[i], samples[i+1]
		u, v := free(a), free(b)
		for j := 0; j <= 16; j++ {
			t0 := float64(j) / 16
			x := u + (v-u)*t0
			exact := geom.Vec3{X: x, Y: math.Sqrt(.5 - x*x), Z: .5}
			chord := a.Point.Scale(1 - t0).Add(b.Point.Scale(t0))
			if exact.Sub(chord).Length() > .004000001 {
				t.Fatal("independent correspondence", exact, chord)
			}
		}
	}
	for _, o := range []RationalTubePolylineOptions{{Deflection: .004, MaxSegments: 1}, {Deflection: 1e-10, MaxDepth: 1}, {Deflection: .004, EndpointPoints: &[2]geom.Vec3{}}} {
		m, e := r.TessellateTubeRunCertified(ctx, 0, 0, 0, AlgebraicTubeRunMeshOptions{Display: o})
		if e == nil || len(m.Vertices) > 0 {
			t.Fatal("accepted unsupported display or exhausted budget", e)
		}
	}
	t.Log(len(samples), m.ExactOperations)
}

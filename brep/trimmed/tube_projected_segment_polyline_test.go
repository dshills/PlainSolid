package trimmed

import (
	"context"
	"math"

	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func TestProjectedNativeSectionSegmentPolyline(t *testing.T) {
	axes := [2]int{0, 2}
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
	count := 0
	for _, branch := range s.partitionSource.segments {
		for _, segment := range branch {
			_, domain, reversed, e := segment.ExactFreeDomain()
			if e != nil {
				t.Fatal(e)
			}
			curve, e := TessellateRationalTubeSegmentCertified(ctx, segment, domain.Lower, domain.Upper, RationalTubePolylineOptions{Deflection: .04, Projection: &axes, MaxOperations: 32000000, Interval: geom.SurfaceTubeIntervalOptions{MaxIterations: 2}})
			if e != nil {
				t.Fatalf("free %v reverse %v: %v", domain, reversed, e)
			}
			if curve.TraversalReversed != reversed {
				t.Fatal("lost original traversal")
			}
			for _, sample := range curve.Samples {
				q, _ := sample.Free.Float64()
				dep := math.Sqrt(25.0/256 - (q-.5)*(q-.5))
				x, z := q, sample.Point.Z
				if curve.FreeAxis%2 == 0 {
					if z < .5 {
						dep = -dep
					}
					z = .5 + dep
				} else {
					x = sample.Point.X
					if x < .5 {
						dep = -dep
					}
					x = .5 + dep
					z = q
				}
				actual := geom.Vec3{X: x, Y: x * z / 64, Z: z}
				if actual.Sub(sample.Point).Length() > sample.Model.ErrorUpper+1e-12 {
					t.Fatal("independent XZ warped-circle witness", actual, sample.Point, sample.Model.ErrorUpper)
				}
			}
			count += len(curve.SquaredErrors)
		}
	}
	if count < 16 {
		t.Fatal("missing projected segments", count)
	}
	t.Logf("certified projected section chords=%d", count)
}

package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestAlgebraicProjectedTubeLensXZ(t *testing.T) {
	slowtest.Skip(t)
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
	axes := [2]int{0, 2}
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
	if e != nil || l.Topology().EulerCharacteristic != 2 || l.ResidualAxis() != 1 || l.ProjectionAxes() != axes {
		t.Fatalf("XZ lens topology %+v err=%v", l.Topology(), e)
	}
	original := l.Faces()
	if !original[0].Reversed || original[1].Reversed {
		t.Fatal("physical Y orientation", original)
	}
	q, e := l.ProjectionInterval(ctx, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || !q.Present || q.Lower.Cmp(big.NewRat(-24, 256)) != 0 || q.Upper.Cmp(big.NewRat(1, 256)) != 0 {
		t.Fatal(q, e)
	}
	w, e := l.LocateRational(ctx, [3]*big.Rat{big.NewRat(1, 2), big.NewRat(-1, 32), big.NewRat(1, 2)}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || w != parametric.Inside {
		t.Fatal(w, e)
	}
	line, e := l.ClipMaterialLine(ctx, geom.Vec3{Y: -1.0 / 32, Z: .5}, geom.Vec3{X: 1},
		[2]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1)}, AlgebraicLensLineOptions{})
	if e != nil || !line.Complete || len(line.Intervals) != 1 || len(line.Isolated) != 0 {
		t.Fatal(line, e)
	}
	for _, v := range []AlgebraicTubeLensLineEvent{line.Intervals[0].Start, line.Intervals[0].End} {
		c, e := v.Parameter.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
		if e != nil || c.Kind != geom.ExactRootKindIrrational || v.BoundaryMask != 2 {
			t.Fatal("lower support root", c, v.BoundaryMask, e)
		}
	}
	plane, e := l.SectionProjectionPlane(ctx, [2]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 2)},
		[2]*big.Rat{big.NewRat(1, 1), big.NewRat(0, 1)}, [2]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1)}, AlgebraicLensLineOptions{})
	if e != nil || !plane.Complete || len(plane.Regions) != 1 || len(plane.Isolated) != 0 {
		t.Fatal(plane, e)
	}
	r := plane.Regions[0]
	for i, f := range r.Curves {
		if !f.Complete || len(f.Cells) != 1 || r.Clipped != [2]bool{} {
			t.Fatal(i, r.Clipped, f)
		}
		v, e := f.Cells[0].BindRoot(ctx, r.Parameter[0], geom.ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for j, want := range [3]*big.Rat{big.NewRat(3, 16), big.NewRat(3, 2048), big.NewRat(1, 2)} {
			q, _, e := v.CoordinateRoot(ctx, j, geom.ExactCurvePlaneEventOptions{})
			if e != nil {
				t.Fatal(e)
			}
			c, e := q.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
			if e != nil || c.Kind != geom.ExactRootKindRational || c.Rational.Cmp(want) != 0 {
				t.Fatal("original XYZ rim", i, j, c, e)
			}
		}
	}
	distance, e := l.BoundBoundaryDistance(ctx, [3]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 256), big.NewRat(1, 2)},
		AlgebraicTubeLensDistanceOptions{MaxCells: 1})
	if e != nil || !distance.Converged || distance.Squared.Lower.Sign() != 0 || distance.Squared.Upper.Sign() != 0 || distance.WitnessChart != 0 {
		t.Fatal("exact original upper face witness", distance, e)
	}
	m, e := l.BoundMassProperties(ctx, AlgebraicTubeLensVolumeOptions{MaxDepth: 9})
	if e != nil {
		t.Fatal(m, e)
	}
	want := [3]*big.Rat{big.NewRat(1, 2), big.NewRat(-11, 384), big.NewRat(1, 2)}
	for i, p := range m.Centroid {
		if p.Lower == nil || p.Lower.Cmp(want[i]) > 0 || p.Upper.Cmp(want[i]) < 0 {
			t.Fatalf("original XYZ centroid coordinate %d bound %+v", i, p)
		}
	}
	solid, e := l.Solid(ctx, 1e-7, edgecheck.Options{})
	if e != nil || solid.Topology().EulerCharacteristic != 2 {
		t.Fatal(solid.Topology(), e)
	}
	var f bytes.Buffer
	if e := WriteAlgebraicProjectedTubeLensJSON(&f, l); e != nil {
		t.Fatal(e)
	}
	o.Trace.ProjectedGraphFallback = intersect.SplinePlanePairDiscoveryOptions{}
	o.Trace.GraphProjectionAxes = [2]int{}
	back, e := ReadAlgebraicProjectedTubeLensJSON(ctx, bytes.NewReader(f.Bytes()), JSONLimits{}, o, AlgebraicTubeLensOptions{})
	if e != nil || back.ProjectionAxes() != axes || back.Topology() != l.Topology() {
		t.Fatal(back.Topology(), e)
	}
}

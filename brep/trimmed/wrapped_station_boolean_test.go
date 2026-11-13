package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCoaxialStationLoftBooleanCrossing(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	makeLoft := func(a, b float64) Solid {
		t.Helper()
		s, _, e := LoftWrappedCircularStations(ctx, geom.Vec2{}, []WrappedLoftStation{{0, a}, {4, b}}, 1e-9, o)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	a, b := makeLoft(1, 3), makeLoft(3, 1)
	if s, h, e := CombineSplineShells(ctx, a, b, parametric.Intersection, SplineShellCombineOptions{}); e != nil ||
		h.Method != "coaxial-station-loft" || len(s.faces) != 4 || len(h.FacesA[1]) != 1 || len(h.FacesB[2]) != 1 ||
		len(h.FacesB[1]) != 0 || len(h.FacesA[2]) != 0 {
		t.Fatal(s.Topology(), h, e)
	}
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection} {
		s, h, e := CombineCoaxialWrappedStationLofts(ctx, a, b, op, o)
		if e != nil || len(h.Bands) != 2 || h.Bands[0].UpperZ.Cmp(big.NewRat(2, 1)) != 0 ||
			h.Bands[1].LowerZ.Cmp(big.NewRat(2, 1)) != 0 || !h.Bands[0].BasisPreserved || !h.Bands[1].BasisPreserved {
			t.Fatal(h, e)
		}
		first, second := [2]int{-1, 0}, [2]int{0, -1}
		if op == parametric.Intersection {
			first, second = second, first
		}
		if h.Bands[0].SourceWalls != first || h.Bands[1].SourceWalls != second {
			t.Fatal("source wall history", h)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		want := [2]float64{76 * math.Pi / 3, 28 * math.Pi / 3}[i]
		if e != nil || math.Abs(p.Volume-want) > 1e-8 || math.Abs(p.Centroid.Z-2) > 1e-8 {
			t.Fatal(p, want, e)
		}
		q, e := s.ValidateWrappedCircularStationLoft(ctx, o)
		if e != nil || len(q.Heights) != 3 || q.Radii[1].Cmp(big.NewRat(2, 1)) != 0 {
			t.Fatal(q, e)
		}
		r, e := s.SectionWrappedCircularStationLoft(ctx, 2, o)
		if e != nil || r.Cap != -1 || r.Outer.Section.Cap != 0 || r.Inner != nil || len(r.Outer.Faces) != 2 ||
			r.Outer.Section.Radius.Cmp(big.NewRat(2, 1)) != 0 {
			t.Fatal("interior crease section", r, e)
		}
		lo, hi, split, e := s.SplitWrappedCircularStationLoftAtZ(ctx, 2, o)
		if e != nil || !split.ExistingStation || split.LowerWall[0] < 0 || split.UpperWall[1] < 0 ||
			split.LowerWall[1] >= 0 || split.UpperWall[0] >= 0 ||
			lo.Faces()[split.LowerWall[0]].Surface != s.Faces()[split.OriginalWall[0]].Surface ||
			hi.Faces()[split.UpperWall[1]].Surface != s.Faces()[split.OriginalWall[1]].Surface {
			t.Fatal(lo.Topology(), hi.Topology(), split, e)
		}
	}
	if s, _, e := CombineCoaxialWrappedStationLofts(ctx, makeLoft(1, 1), b, parametric.Difference, o); e != nil || len(s.faces) != 0 {
		t.Fatal("nested subtraction should be empty", e)
	}
	if s, _, e := CombineCoaxialWrappedStationLofts(ctx, a, makeLoft(2, 2), parametric.Difference, o); !errors.Is(e, geom.ErrConvergence) || len(s.faces) != 0 {
		t.Fatal("unrepresented annular difference", e)
	}
	tube, h, e := CombineCoaxialWrappedStationLofts(ctx, makeLoft(2, 3), makeLoft(1, 2), parametric.Difference, o)
	if e != nil || len(h.Bands) != 1 || len(h.InnerBands) != 1 || tube.Topology().InnerWires != 2 ||
		tube.Topology().EulerCharacteristic != 0 {
		t.Fatal(h, tube.Topology(), e)
	}
	p, e := tube.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-16*math.Pi) > 1e-8 || math.Abs(p.Centroid.Z-13.0/6) > 1e-8 {
		t.Fatal(p, e)
	}
	var encoded bytes.Buffer
	if e := WriteJSON(&encoded, tube); e != nil {
		t.Fatal(e)
	}
	tube, e = ReadJSONContext(ctx, &encoded, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := tube.ValidateWrappedCircularStationBore(ctx, o)
	if e != nil || !q.Complete || len(q.InnerWalls) != 1 || q.InnerWalls[0] != h.InnerBands[0].ResultWall {
		t.Fatal(q, e)
	}
	for _, c := range []struct {
		point geom.Vec3
		want  parametric.Location
	}{
		{geom.Vec3{Z: 2}, parametric.Outside},
		{geom.Vec3{X: 1.5, Z: 2}, parametric.Boundary},
		{geom.Vec3{X: 2, Z: 2}, parametric.Inside},
		{geom.Vec3{X: 1.5}, parametric.Boundary},
		{geom.Vec3{X: .5}, parametric.Outside},
	} {
		if r, e := tube.LocateWrappedCircularStationBore(ctx, c.point, o); e != nil || r != c.want {
			t.Fatal(c, r, e)
		}
	}
	line := geom.ToroidLineOptions{ParameterTolerance: 1e-11}
	r, e := tube.IntersectWrappedCircularStationBoreLine(ctx, geom.Vec3{Z: -1}, geom.Vec3{Z: 1}, 0, 6, line)
	if e != nil || len(r.Hits) != 0 {
		t.Fatal("axis must pass through both cap openings", r, e)
	}
	r, e = tube.IntersectWrappedCircularStationBoreLine(ctx, geom.Vec3{X: 1.5, Z: -1}, geom.Vec3{Z: 1}, 0, 6, line)
	if e != nil || len(r.Hits) != 2 || r.Hits[0].Parameter.Min.Cmp(big.NewRat(1, 1)) != 0 ||
		r.Hits[0].Parts != geom.CylinderLowerCap || r.Hits[1].Parameter.Min.Cmp(big.NewRat(3, 1)) > 0 ||
		r.Hits[1].Parameter.Max.Cmp(big.NewRat(3, 1)) < 0 || r.Hits[1].Faces[0] != q.InnerWalls[0] {
		t.Fatal("cap annulus then inward wall", r, e)
	}
	r, e = tube.IntersectWrappedCircularStationBoreLine(ctx, geom.Vec3{X: -4, Y: .5, Z: 2}, geom.Vec3{X: 1}, 0, 8, line)
	if e != nil || len(r.Hits) != 4 {
		t.Fatal("horizontal cut should meet two inner and two outer wall points", r, e)
	}
	for i, id := range []int{q.OuterWalls[0], q.InnerWalls[0], q.InnerWalls[0], q.OuterWalls[0]} {
		if len(r.Hits[i].Faces) != 1 || r.Hits[i].Faces[0] != id {
			t.Fatal(r.Hits[i], id)
		}
	}
	r, e = tube.IntersectWrappedCircularStationBoreLine(ctx, geom.Vec3{X: -4, Y: .5}, geom.Vec3{X: 1}, 0, 8, line)
	if e != nil || len(r.Hits) != 4 || len(r.Overlaps) != 2 {
		t.Fatal("two algebraic chords separated by the cap hole", r, e)
	}
	for i, id := range []int{q.OuterWalls[0], q.InnerWalls[0], q.InnerWalls[0], q.OuterWalls[0]} {
		hit := r.Hits[i]
		if hit.Parts != geom.CylinderLowerCap|geom.CylinderLateral || len(hit.Faces) != 2 ||
			hit.Faces[0] != min(id, q.Caps[0]) || hit.Faces[1] != max(id, q.Caps[0]) ||
			r.Overlaps[i/2].Face != q.Caps[0] {
			t.Fatal(hit, r.Overlaps[i/2], id)
		}
	}
	d := geom.FrustumDistanceOptions{SquaredTolerance: 1e-12}
	distance, e := tube.BoundWrappedCircularStationBoreDistance(ctx, geom.Vec3{Z: -1}, d)
	if e != nil || distance.Face != q.InnerWalls[0] && distance.Face != q.Caps[0] ||
		distance.Lower.Cmp(big.NewRat(2, 1)) != 0 || distance.Upper.Cmp(big.NewRat(2, 1)) != 0 ||
		distance.Witness.Radius.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("annular cap must not include a fictitious disk", distance, e)
	}
	distance, e = tube.BoundWrappedCircularStationBoreDistance(ctx, geom.Vec3{X: 1.5, Y: .25, Z: -.5}, d)
	if e != nil || distance.Face != q.Caps[0] || distance.Lower.Cmp(big.NewRat(1, 4)) > 0 ||
		distance.Upper.Cmp(big.NewRat(1, 4)) < 0 || distance.Witness.Height.Sign() != 0 {
		t.Fatal("expected cap-interior projection", distance, e)
	}
	cut, e := tube.SectionWrappedCircularStationBore(ctx, 0, o)
	if e != nil || cut.Cap != q.Caps[0] || cut.Inner == nil || !cut.Inner.Hole ||
		cut.Outer.Section.Radius.Cmp(big.NewRat(2, 1)) != 0 || cut.Inner.Section.Radius.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("annular cap section", cut, e)
	}
	cut, e = tube.SectionWrappedCircularStationBore(ctx, 1, o)
	if e != nil || cut.Cap != -1 || cut.Inner == nil || cut.Inner.Section.Radius.Cmp(big.NewRat(5, 4)) != 0 ||
		cut.Outer.Section.Radius.Cmp(big.NewRat(9, 4)) != 0 || cut.Outer.Section.Curve == nil || cut.Inner.Section.Curve == nil {
		t.Fatal("exact annular interior section", cut, e)
	}
	lo, hi, sh, e := tube.SplitWrappedCircularStationBoreAtZ(ctx, 1, o)
	if e != nil || sh.LowerCut < 0 || sh.UpperCut < 0 || lo.Topology().InnerWires != 2 || hi.Topology().InnerWires != 2 ||
		sh.Outer.LowerWall[0] == sh.Inner.LowerWall[0] || sh.Outer.OriginalWall[0] != q.OuterWalls[0] ||
		sh.Inner.OriginalWall[0] != q.InnerWalls[0] {
		t.Fatal(lo.Topology(), hi.Topology(), sh, e)
	}
	for j, part := range []Solid{lo, hi} {
		mass, e := part.MassProperties(ctx, parametric.IntegrationOptions{})
		want := [2]float64{3.25 * math.Pi, 12.75 * math.Pi}[j]
		if e != nil || math.Abs(mass.Volume-want) > 1e-8 {
			t.Fatal(mass, want, e)
		}
	}
	// The exact crossing is at z=8/3 and cannot be stored as a station.
	if s, _, e := CombineCoaxialWrappedStationLofts(ctx, makeLoft(1, 4), makeLoft(3, 3), parametric.Union, o); !errors.Is(e, geom.ErrUnrepresentable) || len(s.faces) != 0 {
		t.Fatal("rounded a nonbinary crossing", e)
	}
}

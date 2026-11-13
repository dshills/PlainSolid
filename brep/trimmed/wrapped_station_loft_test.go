package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestWrappedRationalStationLoft(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec2{X: 1, Y: -2}
	z := []WrappedLoftStation{{Z: 0, Radius: 2}, {Z: 3, Radius: 1}, {Z: 5, Radius: 2}}
	s, h, e := LoftWrappedCircularStations(ctx, center, z, 1e-9, geom.SurfacePatchOptions{})
	if e != nil || len(h.StationRims) != 3 || len(h.WallFaces) != 2 || h.CapFaces != ([2]int{2, 3}) {
		t.Fatal(h, e)
	}
	topology := s.Topology()
	if topology.Vertices != 12 || topology.Edges != 14 || topology.Faces != 4 || topology.EulerCharacteristic != 2 {
		t.Fatal(topology)
	}
	for _, edge := range h.StationRims[1] {
		count := 0
		for _, face := range h.WallFaces {
			for _, u := range s.faces[face].Outer {
				if u.Edge == edge {
					count++
				}
			}
		}
		if count != 2 {
			t.Fatal("station rim is not shared", edge, count)
		}
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 2e-6})
	wantV, wantA := 35*math.Pi/3, (8+3*(math.Sqrt(10)+math.Sqrt(5)))*math.Pi
	// Integrating z*r(z)^2 over the two bands gives 335/12,
	// divided by the volume factor 35/3.
	wantCentroid := geom.Vec3{X: 1, Y: -2, Z: 67.0 / 28}
	if e != nil || math.Abs(p.Volume-wantV) > 1e-5 || math.Abs(p.Area-wantA) > 1e-5 || p.Centroid.Distance(wantCentroid) > 1e-6 {
		t.Fatal(p, wantV, wantA, wantCentroid, e)
	}
	var buf bytes.Buffer
	if e = WriteJSON(&buf, s); e != nil {
		t.Fatal(e)
	}
	s, e = ReadJSON(&buf, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	agreement, e := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !agreement.Complete {
		t.Fatal(agreement, e)
	}
	proof, e := s.ValidateWrappedCircularStationLoft(ctx, geom.SurfacePatchOptions{})
	if e != nil || !proof.Complete || proof.Center[0].Cmp(big.NewRat(1, 1)) != 0 ||
		proof.Center[1].Cmp(big.NewRat(-2, 1)) != 0 || proof.Heights[1].Cmp(big.NewRat(3, 1)) != 0 ||
		proof.Radii[1].Cmp(big.NewRat(1, 1)) != 0 || proof.History.StationRims[1] != h.StationRims[1] {
		t.Fatal(proof, e)
	}
	faces := s.Faces()
	faces[0], faces[3] = faces[3], faces[0]
	faces[1].Outer = append(faces[1].Outer[3:], faces[1].Outer[:3]...)
	renumbered, e := NewSolid(s.Vertices(), s.Edges(), faces, s.Tolerance())
	if e != nil {
		t.Fatal(e)
	}
	proof, e = renumbered.ValidateWrappedCircularStationLoft(ctx, geom.SurfacePatchOptions{})
	if e != nil || !proof.Complete || proof.History.WallFaces[0] != 3 || proof.History.CapFaces != ([2]int{2, 0}) {
		t.Fatal(proof, e)
	}
	for i := range faces {
		faces[i].Reversed = !faces[i].Reversed
	}
	inward, e := NewSolid(s.Vertices(), s.Edges(), faces, s.Tolerance())
	if e != nil {
		t.Fatal(e)
	}
	if proof, e = inward.ValidateWrappedCircularStationLoft(ctx, geom.SurfacePatchOptions{}); e == nil || proof.Complete {
		t.Fatal("accepted inward orientation", proof, e)
	}
}

func TestWrappedStationLoftExactQueries(t *testing.T) {
	ctx := context.Background()
	s, history, e := LoftWrappedCircularStations(ctx, geom.Vec2{}, []WrappedLoftStation{{0, 2}, {3, 1}, {5, 2}}, 1e-9, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, x := range []struct {
		point geom.Vec3
		want  parametric.Location
	}{
		{geom.Vec3{Z: 3}, parametric.Inside},
		{geom.Vec3{X: 1, Z: 3}, parametric.Boundary},
		{geom.Vec3{X: 1.5, Z: 3}, parametric.Outside},
		{geom.Vec3{Z: 5}, parametric.Boundary},
		{geom.Vec3{X: 1.5, Z: 4}, parametric.Boundary},
	} {
		r, e := s.LocateWrappedCircularStationLoft(ctx, x.point, geom.SurfacePatchOptions{})
		if e != nil || r != x.want {
			t.Fatal(x, r, e)
		}
	}
	o := geom.ToroidLineOptions{ParameterTolerance: 1e-11}
	axis, e := s.IntersectWrappedCircularStationLoftLine(ctx, geom.Vec3{Z: -1}, geom.Vec3{Z: 2}, 0, 4, o)
	if e != nil || len(axis.Hits) != 2 || len(axis.Overlaps) != 0 {
		t.Fatal(axis, e)
	}
	for i, w := range []*big.Rat{big.NewRat(1, 2), big.NewRat(3, 1)} {
		h := axis.Hits[i]
		if h.Parameter.Min.Cmp(w) != 0 || h.Parameter.Max.Cmp(w) != 0 ||
			len(h.Faces) != 1 || h.Faces[0] != history.CapFaces[i] {
			t.Fatal(h, w)
		}
	}
	// At the internal station, y=1/2 cuts two algebraic points.
	// Both original wall faces meet at each point and no disk exists.
	rim, e := s.IntersectWrappedCircularStationLoftLine(ctx, geom.Vec3{X: -2, Y: .5, Z: 3}, geom.Vec3{X: 1}, 0, 4, o)
	if e != nil || len(rim.Hits) != 2 || len(rim.Overlaps) != 0 {
		t.Fatal(rim, e)
	}
	for i, w := range []float64{2 - math.Sqrt(.75), 2 + math.Sqrt(.75)} {
		h := rim.Hits[i]
		q := new(big.Rat).SetFloat64(w)
		if h.Parameter.Min.Cmp(q) > 0 || h.Parameter.Max.Cmp(q) < 0 ||
			len(h.Faces) != 2 || h.Faces[0] != history.WallFaces[0] || h.Faces[1] != history.WallFaces[1] ||
			h.Parts != geom.CylinderLateral {
			t.Fatal(h, w)
		}
	}
	generator, e := s.IntersectWrappedCircularStationLoftLine(ctx, geom.Vec3{X: 2}, geom.Vec3{X: -1, Z: 3}, 0, 2, o)
	if e != nil || len(generator.Overlaps) != 1 || generator.Overlaps[0].Face != history.WallFaces[0] ||
		generator.Overlaps[0].Start.Min.Sign() != 0 || generator.Overlaps[0].End.Max.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(generator, e)
	}
}

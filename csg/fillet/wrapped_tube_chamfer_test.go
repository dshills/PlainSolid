package fillet

import (
	"bytes"
	"context"
	"math"
	"reflect"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestIndependentTubeInnerOuterChamfers(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	outer, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 5, 8, o)
	if e != nil {
		t.Fatal(e)
	}
	inner, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 2, 8, o)
	if e != nil {
		t.Fatal(e)
	}
	source, _, e := trimmed.NewWrappedCylinderTube(ctx, outer, inner, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	faces := source.Faces()
	faces = append(faces[2:], faces[:2]...)
	source, e = trimmed.NewSolid(source.Vertices(), source.Edges(), faces, source.Tolerance())
	if e != nil {
		t.Fatal(e)
	}
	before, e := source.ValidateCanonicalWrappedBore(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	selection := CircularTubeChamfers{Outer: [2]CircularRimChamfer{{1, 2}, {.5, 1}}, Inner: [2]CircularRimChamfer{{.5, 1}, {.75, 2}}}
	result, e := ChamferWrappedCylinderTubeRims(ctx, source, selection, o)
	if e != nil {
		t.Fatal(e)
	}
	if result.Solid.Topology().EulerCharacteristic != 0 || len(result.Walls[0]) != 5 || len(result.Walls[1]) != 5 {
		t.Fatal(result.Solid.Topology(), result)
	}
	for end, cap := range result.Caps {
		if !reflect.DeepEqual(result.SourceFaces[cap], []int{before.OuterFaces[end+1]}) || len(result.Solid.Faces()[cap].Holes) != 1 {
			t.Fatal("annular cap ancestry", result)
		}
	}
	seen := map[int]bool{}
	for side, ids := range result.Walls {
		old := before.OuterFaces[0]
		if side == 1 {
			old = before.InnerFaces[0]
		}
		for _, id := range ids {
			if seen[id] || result.SourceFaces[id][0] != old {
				t.Fatal("wall ownership", id)
			}
			seen[id] = true
		}
		for end, bevels := range result.Bevels[side] {
			if len(bevels) == 0 {
				t.Fatal("missing bevel")
			}
			for _, id := range bevels {
				if !reflect.DeepEqual(result.SourceFaces[id], []int{old, before.OuterFaces[end+1]}) {
					t.Fatal("bevel ownership", id)
				}
			}
		}
	}
	volume := 168 * math.Pi
	area := 0.0
	for side, ends := range [][2]CircularRimChamfer{selection.Outer, selection.Inner} {
		R := 5.0
		if side == 1 {
			R = 2
		}
		area += 2 * math.Pi * R * (8 - ends[0].Axial - ends[1].Axial)
		for _, c := range ends {
			sign := -1.0
			if side == 1 {
				sign = 1
			}
			volume -= math.Pi * c.Axial * (R*c.Radial + sign*c.Radial*c.Radial/3)
			area += math.Pi * (2*R + sign*c.Radial) * math.Hypot(c.Radial, c.Axial)
		}
	}
	for end := 0; end < 2; end++ {
		a := 5 - selection.Outer[end].Radial
		b := 2 + selection.Inner[end].Radial
		area += math.Pi * (a*a - b*b)
	}
	mass, e := result.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil || math.Abs(mass.Volume-volume) > 1e-7 || math.Abs(mass.Area-area) > 1e-7 || math.Abs(volume-3643*math.Pi/24) > 1e-10 {
		t.Fatal(mass, volume, area, e)
	}
	agreement, e := result.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !agreement.Complete {
		t.Fatal(agreement, e)
	}
	proof, e := result.Solid.ValidateWrappedCircularStationBore(ctx, o)
	if e != nil || !proof.Complete {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, result.Solid); e != nil {
		t.Fatal(e)
	}
	back, e := trimmed.ReadJSONContext(ctx, &native, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	proof, e = back.ValidateWrappedCircularStationBore(ctx, o)
	if e != nil || !proof.Complete {
		t.Fatal(e)
	}
	for _, bad := range []CircularTubeChamfers{
		{Outer: [2]CircularRimChamfer{{1, 2}, {}}, Inner: [2]CircularRimChamfer{{2, 1}, {}}},
		{Outer: [2]CircularRimChamfer{{1.5, 2}, {}}, Inner: [2]CircularRimChamfer{{2, 1}, {}}},
		{Inner: [2]CircularRimChamfer{{.5, 4}, {.5, 4}}},
		{Outer: [2]CircularRimChamfer{{5, 1}, {}}},
		{},
	} {
		r, e := ChamferWrappedCylinderTubeRims(ctx, source, bad, o)
		if e == nil || r.Solid.Topology().Faces != 0 {
			t.Fatal("atomic contact/runout", bad, e)
		}
	}
}

func TestTubeChamferLimits(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	a, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 5, 8, o)
	if e != nil {
		t.Fatal(e)
	}
	b, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 2, 8, o)
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := trimmed.NewWrappedCylinderTube(ctx, a, b, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	selection := CircularTubeChamfers{Outer: [2]CircularRimChamfer{{1, 2}, {}}}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := ChamferWrappedCylinderTubeRims(cancelled, s, selection, o); e != context.Canceled || r.Solid.Topology().Faces != 0 {
		t.Fatal(e)
	}
	if r, e := ChamferWrappedCylinderTubeRims(ctx, s, selection, geom.SurfacePatchOptions{MaxOperations: 1}); e == nil || r.Solid.Topology().Faces != 0 {
		t.Fatal("proof budget", e)
	}
	selection.Outer[0].Radial = .1
	if r, e := ChamferWrappedCylinderTubeRims(ctx, s, selection, o); e == nil || r.Solid.Topology().Faces != 0 {
		t.Fatal("nonrepresentable cap rim", e)
	}
}

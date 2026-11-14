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

func TestIndependentCircularRimChamfers(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 4, 8, o)
	if e != nil {
		t.Fatal(e)
	}
	source, _, e := trimmed.NewWrappedCylinder(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	f := source.Faces()
	source, e = trimmed.NewSolid(source.Vertices(), source.Edges(), []trimmed.Face{f[2], f[0], f[1]}, source.Tolerance())
	if e != nil {
		t.Fatal(e)
	}
	for _, selection := range [][2]CircularRimChamfer{{{1, 2}, {.5, 1}}, {{}, {.5, 1}}, {{1, 2}, {}}} {
		result, e := ChamferWrappedCylinderRims(ctx, source, selection[0], selection[1], o)
		if e != nil {
			t.Fatal(e)
		}
		faces := 3
		for _, c := range selection {
			if c.Axial > 0 {
				faces++
			}
		}
		if result.Solid.Topology().Faces != faces || result.Solid.Topology().EulerCharacteristic != 2 || !reflect.DeepEqual(result.SourceFaces[result.Lateral], []int{1}) {
			t.Fatal(result)
		}
		volume := 128 * math.Pi
		area := 2 * math.Pi * 4 * (8 - selection[0].Axial - selection[1].Axial)
		for end, c := range selection {
			oldCap := 2
			if end == 1 {
				oldCap = 0
			}
			if !reflect.DeepEqual(result.SourceFaces[result.Caps[end]], []int{oldCap}) || len(result.SourceRims[end]) != 4 {
				t.Fatal("cap ownership", result)
			}
			oldPlane := source.Faces()[oldCap].Surface.(geom.PlaneSurface)
			newPlane := result.Solid.Faces()[result.Caps[end]].Surface.(geom.PlaneSurface)
			if oldPlane.Frame() != newPlane.Frame() {
				t.Fatal("original cap support changed")
			}
			if c.Axial == 0 {
				if result.Chamfers[end] != -1 || result.CapRims[end] != result.ShoulderRims[end] {
					t.Fatal(result)
				}
			} else {
				if !reflect.DeepEqual(result.SourceFaces[result.Chamfers[end]], []int{oldCap, 1}) || result.CapRims[end] == result.ShoulderRims[end] {
					t.Fatal(result)
				}
			}
			volume -= math.Pi * c.Axial * (4*c.Radial - c.Radial*c.Radial/3)
			area += math.Pi*(4-c.Radial)*(4-c.Radial) + math.Pi*(8-c.Radial)*math.Hypot(c.Radial, c.Axial)
		}
		mass, e := result.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil || math.Abs(mass.Volume-volume) > 1e-7 || math.Abs(mass.Area-area) > 1e-7 {
			t.Fatal(mass, volume, area, e)
		}
		agreement, e := result.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !agreement.Complete {
			t.Fatal(agreement, e)
		}
		var b bytes.Buffer
		if e = trimmed.WriteJSON(&b, result.Solid); e != nil {
			t.Fatal(e)
		}
		back, e := trimmed.ReadJSONContext(ctx, &b, trimmed.JSONLimits{})
		if e != nil || back.Topology().Faces != faces {
			t.Fatal(e)
		}
	}
	for _, selection := range [][2]CircularRimChamfer{{{}, {}}, {{1, 0}, {}}, {{4, 1}, {}}, {{1, 4}, {1, 4}}, {{1, 5}, {1, 4}}, {{math.NaN(), 1}, {}}, {{.1, 1}, {}}} {
		out, e := ChamferWrappedCylinderRims(ctx, source, selection[0], selection[1], o)
		if e == nil || out.Solid.Topology().Faces != 0 {
			t.Fatal("atomic invalid blade/runout/representation", selection, e)
		}
	}
	if out, e := ChamferWrappedCylinderRims(ctx, source, CircularRimChamfer{1, 2}, CircularRimChamfer{}, geom.SurfacePatchOptions{MaxOperations: 1}); e == nil || out.Solid.Topology().Faces != 0 {
		t.Fatal("proof budget", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if out, e := ChamferWrappedCylinderRims(cancelled, source, CircularRimChamfer{1, 2}, CircularRimChamfer{}, o); e != context.Canceled || out.Solid.Topology().Faces != 0 {
		t.Fatal(e)
	}
}

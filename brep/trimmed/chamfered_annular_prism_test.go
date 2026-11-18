package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestChamferedAnnularPrism(t *testing.T) {
	slowtest.Skip(t)
	for _, axis := range []geom.Vec3{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {Z: 1}, {Z: -1}} {
		f, e := geom.FrameFromNormal(geom.Vec3{X: 3, Y: 5, Z: 7}, axis)
		if e != nil {
			t.Fatal(e)
		}
		for _, cuts := range [][2]float64{{0, 0}, {.6, 0}, {0, .6}, {.6, .4}} {
			p := ChamferedAnnularPrism{Frame: f, OuterRadius: 78, InnerRadius: 26, Height: 12, LowerChamfer: cuts[0], UpperChamfer: cuts[1]}
			result, e := p.Build(context.Background())
			if e != nil {
				t.Fatal(axis, cuts, e)
			}
			wantV, _ := p.NominalVolume()
			wantA, _ := p.SurfaceArea()
			mass, e := result.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(mass.Volume-wantV) > wantV*1e-10 || math.Abs(mass.Area-wantA) > wantA*1e-10 {
				t.Fatal(mass, wantV, wantA)
			}
			baseV := math.Pi * (78*78 - 26*26) * 12
			removedMoment := math.Pi * 104 * (cuts[0]*cuts[0]*cuts[0]/3 + 12*cuts[1]*cuts[1] - cuts[1]*cuts[1]*cuts[1]/3)
			wantCenter := f.ToWorld(geom.Vec3{Z: (baseV*6 - removedMoment) / wantV})
			if mass.Centroid.Distance(wantCenter) > 1e-8 {
				t.Fatal("centroid", mass.Centroid, wantCenter)
			}
			faces := result.Solid.Faces()
			for _, id := range []int{result.LowerFace, result.UpperFace} {
				if _, ok := faces[id].Surface.(geom.PlaneSurface); !ok || len(faces[id].Outer) != 4 || len(faces[id].Holes) != 1 || len(faces[id].Holes[0]) != 4 {
					t.Fatal("cap is not one shared planar annulus", faces[id])
				}
			}
		}
	}
}

func TestChamferedAnnularMeshAndPersistence(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	p := ChamferedAnnularPrism{Frame: f, OuterRadius: 78, InnerRadius: 26, Height: 12, LowerChamfer: .6}
	s, e := p.Solid(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	var data bytes.Buffer
	if e = WriteJSON(&data, s); e != nil {
		t.Fatal(e)
	}
	restored, e := ReadJSON(&data, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	volume, _ := p.NominalVolume()
	mass, e := restored.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(mass.Volume-volume) > volume*1e-10 {
		t.Fatal(mass, e)
	}
	o := parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 384}
	m, e := p.Tessellate(context.Background(), o)
	if e != nil || len(m.Triangles) != 384 {
		t.Fatal(len(m.Triangles), e)
	}
	mm, e := m.MassProperties()
	if e != nil || math.Abs(mm.Volume-volume) > volume*.01 {
		t.Fatal(mm, e)
	}
	o.MaxTriangles = 383
	if _, e = p.Tessellate(context.Background(), o); e != parametric.ErrBudget {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = p.Build(ctx); e != context.Canceled {
		t.Fatal(e)
	}
	p.LowerChamfer = 6
	p.UpperChamfer = 6
	if _, e = p.Build(context.Background()); e == nil {
		t.Fatal("overlapping axial chamfers accepted")
	}
	p.LowerChamfer = 26
	p.UpperChamfer = 0
	if _, e = p.Build(context.Background()); e == nil {
		t.Fatal("vanished cap accepted")
	}
}

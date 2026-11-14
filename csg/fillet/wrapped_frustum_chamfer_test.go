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

func TestCircularFrustumRimChamfers(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	for _, descending := range []bool{false, true} {
		w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 4, 8, o)
		if e != nil {
			t.Fatal(e)
		}
		lo, hi := 0.0, 2.0
		if descending {
			lo, hi = 2, 0
		}
		w, _, e = geom.DraftWrappedFrustum(ctx, w, lo, hi, o)
		if e != nil {
			t.Fatal(e)
		}
		source, _, e := trimmed.NewWrappedFrustum(ctx, w, 1e-9, o)
		if e != nil {
			t.Fatal(e)
		}
		f := source.Faces()
		source, e = trimmed.NewSolid(source.Vertices(), source.Edges(), []trimmed.Face{f[2], f[0], f[1]}, source.Tolerance())
		if e != nil {
			t.Fatal(e)
		}
		R0, R1 := 4+lo, 4+hi
		s := (R1 - R0) / 8
		for _, selection := range [][2]CircularRimChamfer{{{1, 2}, {.5, 1}}, {{}, {.5, 1}}, {{1, 2}, {}}} {
			out, e := ChamferWrappedFrustumRims(ctx, source, selection[0], selection[1], o)
			if e != nil {
				t.Fatal(descending, e)
			}
			a, b, c, d := selection[0].Radial, selection[0].Axial, selection[1].Radial, selection[1].Axial
			removed := math.Pi * (b*(R0*a+s*a*b/3-a*a/3) + d*(R1*c-s*c*d/3-c*c/3))
			volume := math.Pi*8*(R0*R0+R0*R1+R1*R1)/3 - removed
			shoulder0, shoulder1 := R0+s*b, R1-s*d
			area := math.Pi * ((R0-a)*(R0-a) + (R1-c)*(R1-c) + (shoulder0+shoulder1)*math.Hypot(8-b-d, shoulder1-shoulder0))
			area += math.Pi*(R0-a+shoulder0)*math.Hypot(b, shoulder0-(R0-a)) + math.Pi*(R1-c+shoulder1)*math.Hypot(d, (R1-c)-shoulder1)
			mass, e := out.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if e != nil || math.Abs(mass.Volume-volume) > 1e-7 || math.Abs(mass.Area-area) > 1e-7 {
				t.Fatal(mass, volume, area, e)
			}
			if !reflect.DeepEqual(out.SourceFaces[out.Lateral], []int{1}) || out.Solid.Topology().EulerCharacteristic != 2 {
				t.Fatal(out)
			}
			central := out.Solid.Faces()[out.Lateral].Surface.(*geom.WrappedSurfacePatch)
			proof, e := geom.CertifyWrappedFrustumEmbedding(ctx, central, o)
			if e != nil {
				t.Fatal(e)
			}
			lowerZ, _ := proof.LowerZ.Float64()
			upperZ, _ := proof.UpperZ.Float64()
			lowerR, _ := proof.LowerRadius.Float64()
			upperR, _ := proof.UpperRadius.Float64()
			if lowerR != R0+s*lowerZ || upperR != R0+s*upperZ {
				t.Fatal("original wall equation lost", proof)
			}
			for end, cap := range out.Caps {
				old := 2
				if end == 1 {
					old = 0
				}
				if !reflect.DeepEqual(out.SourceFaces[cap], []int{old}) || out.Solid.Faces()[cap].Surface.(geom.PlaneSurface).Frame() != source.Faces()[old].Surface.(geom.PlaneSurface).Frame() {
					t.Fatal("cap support", out)
				}
			}
			agreement, e := out.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
			if e != nil || !agreement.Complete {
				t.Fatal(agreement, e)
			}
			var buf bytes.Buffer
			if e = trimmed.WriteJSON(&buf, out.Solid); e != nil {
				t.Fatal(e)
			}
			back, e := trimmed.ReadJSONContext(ctx, &buf, trimmed.JSONLimits{})
			if e != nil || back.Topology().Faces != out.Solid.Topology().Faces {
				t.Fatal(e)
			}
		}
		for _, bad := range [][2]CircularRimChamfer{{{R0, 1}, {}}, {{}, {R1, 1}}, {{1, 4}, {1, 4}}, {{.1, 1}, {}}, {{1, 0}, {}}} {
			r, e := ChamferWrappedFrustumRims(ctx, source, bad[0], bad[1], o)
			if e == nil || r.Solid.Topology().Faces != 0 {
				t.Fatal("invalid cap/runout", bad, e)
			}
		}
	}
}

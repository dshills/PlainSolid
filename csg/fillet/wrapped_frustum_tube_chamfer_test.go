package fillet

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestAnnularFrustumFourRimChamfers(t *testing.T) {
	for _, descending := range []bool{false, true} {
		name := "ascending"
		if descending {
			name = "descending"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			o := geom.SurfacePatchOptions{}
			R0, R1, r0, r1 := 5.0, 7.0, 2.0, 3.0
			if descending {
				R0, R1, r0, r1 = 7, 5, 3, 2
			}
			makeWall := func(a, b float64) *geom.WrappedSurfacePatch {
				w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, a, 8, o)
				if e != nil {
					t.Fatal(e)
				}
				w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, b-a, o)
				if e != nil {
					t.Fatal(e)
				}
				return w
			}
			source, _, e := trimmed.NewWrappedCircularFrustumBore(ctx, makeWall(R0, R1), makeWall(r0, r1), 1e-9, o)
			if e != nil {
				t.Fatal(e)
			}
			f := source.Faces()
			f = append(f[2:], f[:2]...)
			source, e = trimmed.NewSolid(source.Vertices(), source.Edges(), f, source.Tolerance())
			if e != nil {
				t.Fatal(e)
			}
			before, e := source.ValidateCanonicalWrappedFrustumBore(ctx, o)
			if e != nil {
				t.Fatal(e)
			}
			selection := CircularTubeChamfers{Outer: [2]CircularRimChamfer{{1, 2}, {.5, 1}}, Inner: [2]CircularRimChamfer{{.5, 1}, {.75, 2}}}
			result, e := ChamferWrappedFrustumTubeRims(ctx, source, selection, o)
			if e != nil {
				t.Fatal(e)
			}
			if result.Solid.Topology().EulerCharacteristic != 0 || len(result.Walls[0]) != 5 || len(result.Walls[1]) != 5 {
				t.Fatal(result.Solid.Topology())
			}
			volume := 8 * math.Pi * (R0*R0 + R0*R1 + R1*R1 - r0*r0 - r0*r1 - r1*r1) / 3
			area := 0.0
			for side, ends := range [][2]CircularRimChamfer{selection.Outer, selection.Inner} {
				a, b := R0, R1
				sign := -1.0
				original := before.OuterFaces[0]
				if side == 1 {
					a, b, sign = r0, r1, 1
					original = before.InnerFaces[0]
				}
				s := (b - a) / 8
				q0, q1 := a+s*ends[0].Axial, b-s*ends[1].Axial
				area += math.Pi * (q0 + q1) * math.Hypot(8-ends[0].Axial-ends[1].Axial, q1-q0)
				for end, c := range ends {
					R, signedSlope := a, s
					shoulder := q0
					if end == 1 {
						R, signedSlope, shoulder = b, -s, q1
					}
					cap := R + sign*c.Radial
					volume -= math.Pi * c.Axial * (R*c.Radial + signedSlope*c.Radial*c.Axial/3 + sign*c.Radial*c.Radial/3)
					area += math.Pi * (cap + shoulder) * math.Hypot(c.Axial, cap-shoulder)
				}
				for _, id := range result.Walls[side] {
					if result.SourceFaces[id][0] != original {
						t.Fatal("wall ancestry")
					}
					if len(result.SourceFaces[id]) == 1 {
						p, e := geom.CertifyWrappedFrustumEmbedding(ctx, result.Solid.Faces()[id].Surface.(*geom.WrappedSurfacePatch), o)
						if e != nil {
							t.Fatal(e)
						}
						z0, _ := p.LowerZ.Float64()
						z1, _ := p.UpperZ.Float64()
						x0, _ := p.LowerRadius.Float64()
						x1, _ := p.UpperRadius.Float64()
						if x0 != a+s*z0 || x1 != a+s*z1 {
							t.Fatal("original equation")
						}
					}
				}
			}
			for end, cap := range result.Caps {
				a, b := R0, r0
				if end == 1 {
					a, b = R1, r1
				}
				a -= selection.Outer[end].Radial
				b += selection.Inner[end].Radial
				area += math.Pi * (a*a - b*b)
				if result.SourceFaces[cap][0] != before.OuterFaces[end+1] {
					t.Fatal("cap ancestry")
				}
			}
			mass, e := result.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if e != nil || math.Abs(mass.Volume-volume) > 1e-7 || math.Abs(mass.Area-area) > 1e-7 {
				t.Fatal(mass, volume, area, e)
			}
			if !descending && math.Abs(volume-10613*math.Pi/48) > 1e-10 {
				t.Fatal(volume)
			}
			agreement, e := result.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
			if e != nil || !agreement.Complete {
				t.Fatal(agreement, e)
			}
			var buf bytes.Buffer
			if e = trimmed.WriteJSON(&buf, result.Solid); e != nil {
				t.Fatal(e)
			}
			back, e := trimmed.ReadJSONContext(ctx, &buf, trimmed.JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			proof, e := back.ValidateWrappedCircularStationBore(ctx, o)
			if e != nil || !proof.Complete {
				t.Fatal(e)
			}
			bad := selection
			bad.Outer[0].Radial = R0 - r0 - selection.Inner[0].Radial
			if r, e := ChamferWrappedFrustumTubeRims(ctx, source, bad, o); e == nil || r.Solid.Topology().Faces != 0 {
				t.Fatal("cap contact accepted", e)
			}
		})
	}
}

func TestAnnularFrustumChamferScope(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	outer, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 5, 8, o)
	if e != nil {
		t.Fatal(e)
	}
	inner, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: .25}, 2, 8, o)
	if e != nil {
		t.Fatal(e)
	}
	source, _, e := trimmed.NewWrappedEccentricCircularFrustumBore(ctx, outer, inner, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	selection := CircularTubeChamfers{Outer: [2]CircularRimChamfer{{1, 2}, {}}}
	if r, e := ChamferWrappedFrustumTubeRims(ctx, source, selection, o); e == nil || r.Solid.Topology().Faces != 0 {
		t.Fatal("eccentric source outside proof", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := ChamferWrappedFrustumTubeRims(cancelled, source, selection, o); e != context.Canceled || r.Solid.Topology().Faces != 0 {
		t.Fatal(e)
	}
	if r, e := ChamferWrappedFrustumTubeRims(ctx, source, selection, geom.SurfacePatchOptions{MaxOperations: 1}); e == nil || r.Solid.Topology().Faces != 0 {
		t.Fatal("budget", e)
	}
}

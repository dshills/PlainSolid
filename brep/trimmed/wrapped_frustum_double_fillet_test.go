package trimmed

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestSimultaneousConicalRollingFillets(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	for _, fixture := range []struct {
		descending  bool
		lowerRadius float64
	}{{false, 15.0 / 8}, {true, 15.0 / 8}, {false, 15.0 / 16}, {true, 15.0 / 16}} {
		descending := fixture.descending
		r0, r1 := fixture.lowerRadius, 15.0/8
		lower, upper := r0, r1
		if descending {
			lower, upper = upper, lower
		}
		R, delta := 8.0, 8.0
		if descending {
			R, delta = 16, -8
		}
		w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, R, 6, o)
		if e != nil {
			t.Fatal(e)
		}
		w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, delta, o)
		if e != nil {
			t.Fatal(e)
		}
		source, _, e := NewWrappedFrustum(ctx, w, 1e-9, o)
		if e != nil {
			t.Fatal(e)
		}
		f := source.Faces()
		source, e = NewSolid(source.Vertices(), source.Edges(), []Face{f[2], f[0], f[1]}, source.Tolerance())
		if e != nil {
			t.Fatal(e)
		}
		out, h, e := FilletWrappedFrustumRims(ctx, source, lower, upper, o)
		if e != nil {
			t.Fatal(descending, e)
		}
		if out.Topology().Faces != 5 || out.Topology().EulerCharacteristic != 2 || len(h.SourceFaces) != 5 || h.SourceFaces[h.Lateral][0] != 1 {
			t.Fatal(out.Topology(), h)
		}
		for end := 0; end < 2; end++ {
			oldCap := 2
			if end == 1 {
				oldCap = 0
			}
			if h.SourceFaces[h.Caps[end]][0] != oldCap || len(h.SourceFaces[h.Blends[end]]) != 2 {
				t.Fatal("source provenance", h)
			}
		}
		// Independent meridian integrals for ascending stock. Descending stock is
		// its exact reflected body, so both total measures agree independently.
		M0, M1 := 8-r0/3, 16-3*r1
		theta := -math.Atan2(4, 3)
		volumeArc := func(r, M, a, b float64) float64 {
			F := func(x float64) float64 {
				s, c := math.Sin(x), math.Cos(x)
				return math.Pi * r * (M*M*s + M*r*(x+s*c) + r*r*(s-s*s*s/3))
			}
			return F(b) - F(a)
		}
		areaArc := func(r, M, a, b float64) float64 { return 2 * math.Pi * r * (M*(b-a) + r*(math.Sin(b)-math.Sin(a))) }
		z0, z1 := r0/5, 6-9*r1/5
		R0, R1 := 8+4*z0/3, 8+4*z1/3
		V := volumeArc(r0, M0, -math.Pi/2, theta) + volumeArc(r1, M1, theta, math.Pi/2) + math.Pi*(z1-z0)*(R0*R0+R0*R1+R1*R1)/3
		A := areaArc(r0, M0, -math.Pi/2, theta) + areaArc(r1, M1, theta, math.Pi/2) + math.Pi*(R0+R1)*math.Hypot(z1-z0, R1-R0) + math.Pi*(M0*M0+M1*M1)
		mass, e := out.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil || math.Abs(mass.Volume-V) > 1e-7 || math.Abs(mass.Area-A) > 1e-7 {
			t.Fatal(mass, V, A, e)
		}
		if h.Circles[0].Tangency[1].Cmp(h.Circles[1].Tangency[1]) >= 0 {
			t.Fatal("runout order")
		}
		for end, rows := range h.Rims {
			for role, row := range rows {
				for _, edge := range row {
					count := 0
					for _, face := range out.Faces() {
						for _, use := range face.Outer {
							if use.Edge == edge {
								count++
							}
						}
					}
					if count != 2 {
						t.Fatal("unique full rim ownership", end, role, edge, count)
					}
				}
			}
		}
		var b bytes.Buffer
		if e = WriteJSON(&b, out); e != nil {
			t.Fatal(e)
		}
		back, e := ReadJSONContext(ctx, &b, JSONLimits{})
		if e != nil || back.Topology().Faces != 5 {
			t.Fatal(e)
		}
	}
	// For s=4/3 lower runout=r/5 and upper runout=9r/5. With r=15/8
	// their sum is15/4: exact equality must fail, before any tolerance merge.
	for _, H := range []float64{15.0 / 4, 3} {
		w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 8, H, o)
		if e != nil {
			t.Fatal(e)
		}
		w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, H*4/3, o)
		if e != nil {
			t.Fatal(e)
		}
		s, _, e := NewWrappedFrustum(ctx, w, 1e-9, o)
		if e != nil {
			t.Fatal(e)
		}
		bad, h, e := FilletWrappedFrustumRims(ctx, s, 15.0/8, 15.0/8, o)
		if e == nil || bad.Topology().Faces != 0 || h.Circles[0].Radius != nil {
			t.Fatal("touching/overlapping runouts", H, e)
		}
	}
}

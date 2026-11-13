package exchange

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

// Every proper signed coordinate frame retains its original recipe coefficients
// through portable native exchange. Reflections are not silently interpreted as
// proper rigid frame data: X/Z determine the right-handed Y axis explicitly.
func TestSphereThroughBoreNativeSignedAxisPlacements(t *testing.T) {
	ctx := context.Background()
	identity := sphereThroughBoreFixture()
	var raw bytes.Buffer
	if e := WriteSphereThroughBoreRecipeJSON(ctx, &raw, identity); e != nil {
		t.Fatal(e)
	}
	_, base, e := ReadSphereThroughBoreRecipeJSON(ctx, &raw, 0)
	if e != nil {
		t.Fatal(e)
	}
	mass, e := base.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
	if e != nil {
		t.Fatal(e)
	}
	axes := []geom.Vec3{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {Z: 1}, {Z: -1}}
	count := 0
	for _, x := range axes {
		for _, z := range axes {
			if x.Dot(z) != 0 {
				continue
			}
			count++
			r := identity
			r.Name = "posed off-axis bore"
			r.Origin = geom.Vec3{X: 8, Y: -4, Z: 2}
			r.XAxis = x
			r.ZAxis = z
			var data bytes.Buffer
			if e = WriteSphereThroughBoreRecipeJSON(ctx, &data, r); e != nil {
				t.Fatal(e)
			}
			recipe, body, e := ReadSphereThroughBoreRecipeJSON(ctx, &data, 0)
			if e != nil || recipe != r {
				t.Fatal("changed exact recipe", e)
			}
			f, e := geom.NewFrame(r.Origin, x, z)
			if e != nil {
				t.Fatal(e)
			}
			apply := func(p geom.Vec3) geom.Vec3 {
				return r.Origin.Add(f.XAxis().Scale(p.X)).Add(f.YAxis().Scale(p.Y)).Add(f.ZAxis().Scale(p.Z))
			}
			for i, face := range base.Solid.Faces() {
				u, v := face.U.Min+.37*(face.U.Max-face.U.Min), face.V.Min+.61*(face.V.Max-face.V.Min)
				p, e := face.Surface.Eval(u, v)
				if e != nil {
					t.Fatal(e)
				}
				got, e := body.Solid.Faces()[i].Surface.Eval(u, v)
				if e != nil || got.Sub(apply(p)).Length() > 1e-13 {
					t.Fatal("changed original chart placement", i, e)
				}
			}
			got, e := body.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(got.Volume-mass.Volume) > 1e-6 || math.Abs(got.Area-mass.Area) > 1e-6 || got.Centroid.Sub(apply(mass.Centroid)).Length() > 1e-7 {
				t.Fatal("placement changed material", got, mass)
			}
		}
	}
	if count != 24 {
		t.Fatal(count)
	}
}

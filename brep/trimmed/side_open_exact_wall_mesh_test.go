//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestNonbinaryCircularWallCertifiedDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	discs := [2]ExtrusionCircle{{Center: geom.Vec2{X: 20}, Radius: 13}, {Center: geom.Vec2{X: 24}, Radius: 15}}
	removed := 24 + (169*(math.Pi/2-math.Asin(5./13))+225*(math.Pi/2+math.Asin(9./15)))/2
	for _, depth := range []float64{3, 6} {
		s, e := CutGeneralSideOpenDualDisksWithExactWalls(ctx, frame, geom.Vec2{}, geom.Vec2{X: 50, Y: 22}, discs, 6, depth, 1e-10, trimvalidate.Options{})
		if e != nil {
			t.Fatal(e)
		}
		o := SideOpenDualDiskMeshOptions{Deflection: .5, AngularTurn: .2}
		a, e := s.TessellateCertified(ctx, o)
		if e != nil {
			t.Fatal(depth, e)
		}
		v, _ := a.DisplayVolume.Float64()
		want := 6600 - depth*removed
		if math.Abs(v-want) > 5 {
			t.Fatal(v, want)
		}
		var native bytes.Buffer
		if e = WriteJSON(&native, s.Solid); e != nil {
			t.Fatal(e)
		}
		if !bytes.Contains(native.Bytes(), []byte("26/3")) {
			t.Fatal("nonbinary source missing")
		}
		restored, e := ReadJSONContext(ctx, &native, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		b, e := restored.TessellateDualCircularWallCertified(ctx, o)
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(a.Mesh, b.Mesh) || a.SquaredDistance.Cmp(b.SquaredDistance) != 0 {
			t.Fatal("native reload changed certified display")
		}
		for fi, f := range a.Network.Faces {
			for _, tri := range f.Mesh.Triangles {
				var u, v float64
				for _, id := range tri {
					x, _ := f.ExactUV[id][0].Float64()
					y, _ := f.ExactUV[id][1].Float64()
					u += x / 3
					v += y / 3
				}
				du, dv, e := s.Solid.faces[fi].Surface.Partials(u, v)
				if e != nil {
					t.Fatal(e)
				}
				n, _ := du.Cross(dv).Unit()
				if s.Solid.faces[fi].Reversed {
					n = n.Scale(-1)
				}
				p, q, r := f.Mesh.Vertices[tri[0]], f.Mesh.Vertices[tri[1]], f.Mesh.Vertices[tri[2]]
				display, _ := q.Sub(p).Cross(r.Sub(p)).Unit()
				if n.Dot(display) < math.Cos(.1)-1e-10 {
					t.Fatal("independent exact-adapter normal", fi)
				}
			}
		}
		t.Log(depth, len(a.Mesh.Triangles), v, want)
		fi := s.WallFaces[0][0]
		recipe, e := s.Solid.faces[fi].Surface.(*geom.ExactBezierSurface).ExactPatch().ExportExactBezierRecipe()
		if e != nil {
			t.Fatal(e)
		}
		x, _ := new(big.Rat).SetString(recipe.Controls[2][0])
		recipe.Controls[2][0] = x.Add(x, big.NewRat(1, 1000000000000)).RatString()
		patch, e := geom.ImportExactBezierSurfaceRecipe(ctx, recipe, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		badSurface, e := geom.NewExactBezierSurface(patch)
		if e != nil {
			t.Fatal(e)
		}
		bad := s.Solid
		bad.faces = append([]Face(nil), bad.faces...)
		bad.faces[fi].Surface = badSurface
		if out, e := bad.TessellateDualCircularWallCertified(ctx, o); e == nil || len(out.Mesh.Vertices) != 0 {
			t.Fatal("perturbed rational circle accepted")
		}
	}
}

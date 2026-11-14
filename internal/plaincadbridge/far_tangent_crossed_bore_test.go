package plaincadbridge

import (
	"encoding/base64"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestFarTangentCrossedBoreScopeAndSTEP(t *testing.T) {
	for _, op := range []bool{false, true} {
		b := New()
		stock := box(t, b, vector{0, 0, 0}, vector{20, 16, 12})
		x := circle(t, b, vector{0, 8, 6}, vector{1, 0, 0}, 2, 20)
		entry, sign := 0., 1.
		if op {
			entry, sign = 12, -1
		}
		z := circle(t, b, vector{10, 8, entry}, vector{0, 0, sign}, 2, 8)
		both, err := b.cutMany(stock.Handle, []string{z.Handle, x.Handle})
		if err != nil {
			t.Fatal(op, err)
		}
		near(t, both.GeometryAssertions.Volume, 3840-112*math.Pi+128./3)
		near(t, both.GeometryAssertions.SurfaceArea, 1504+104*math.Pi-64)
		file, err := b.exportSTEP([]string{both.Handle})
		if err != nil {
			t.Fatal(op, err)
		}
		source, err := base64.StdEncoding.DecodeString(file.Base64)
		if err != nil {
			t.Fatal(err)
		}
		got, err := b.importSTEP(string(source))
		if err != nil || got.Bounds != both.Bounds {
			t.Fatal(op, got, err)
		}
		floor, err := b.shapes[got.Handle].bore.planarFace(geom.Vec3{X: 10, Y: 8, Z: entry + 8*sign}, geom.Vec3{Z: sign})
		if err != nil || floor.Face != 6 || floor.Normal != (vector{0, 0, -sign}) {
			t.Fatal(op, floor, err)
		}
		near(t, floor.Area, 4*math.Pi)
		bad := circle(t, b, vector{10, 8, entry}, vector{0, 0, sign}, 2, 4)
		if _, err := b.cutMany(stock.Handle, []string{bad.Handle, x.Handle}); err == nil {
			t.Fatal("accepted a nonmanifold tangent diameter")
		}
	}
}

package plaincadbridge

import (
	"encoding/base64"
	"fmt"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestClippedCrossedBoreScopeAndSTEP(t *testing.T) {
	for _, opposite := range []bool{false, true} {
		t.Run(fmt.Sprint(opposite), func(t *testing.T) {
			b := New()
			stock := box(t, b, vector{0, 0, 0}, vector{20, 16, 12})
			x := circle(t, b, vector{0, 8, 6}, vector{1, 0, 0}, 2, 20)
			entry, direction := 0., 1.
			if opposite {
				entry, direction = 12., -1.
			}
			z := circle(t, b, vector{10, 8, entry}, vector{0, 0, direction}, 2, 7.2)
			q := math.Sqrt((2. - 1.2) * (2. + 1.2))
			theta := math.Acos(1.2 / 2)
			common := 128./3 - (2*q*q*q/3 - 8*1.2*theta + 8*q)
			floor := 2 * (4*math.Asin(1.2/2) - 1.2*q)
			wantVolume := 3840 - 27.2*4*math.Pi + common
			wantArea := 1504 + 27.2*4*math.Pi - 64 + 16*q - 8*1.2*theta - 12*math.Pi + floor
			before := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": x.Handle})
			near(t, overlapMeasure(t, b, before.Handle, z.Handle), 7.2*4*math.Pi-common)
			after := shape(t, b, map[string]any{"op": "cut", "a": before.Handle, "b": z.Handle})
			group, err := b.cutMany(stock.Handle, []string{z.Handle, x.Handle})
			if err != nil || group.GeometryAssertions != after.GeometryAssertions {
				t.Fatal(group, after, err)
			}
			near(t, after.GeometryAssertions.Volume, wantVolume)
			near(t, after.GeometryAssertions.SurfaceArea, wantArea)
			file, err := b.exportSTEP([]string{after.Handle})
			if err != nil {
				t.Fatal(err)
			}
			p, err := base64.StdEncoding.DecodeString(file.Base64)
			if err != nil {
				t.Fatal(err)
			}
			read, err := b.importSTEP(string(p))
			if err != nil || read.Bounds != after.Bounds {
				t.Fatal(read, err)
			}
			near(t, read.GeometryAssertions.Volume, wantVolume)
			near(t, read.GeometryAssertions.SurfaceArea, wantArea)
			near(t, overlapMeasure(t, b, read.Handle, z.Handle), 0)
			body := b.shapes[read.Handle].bore
			if _, err := body.planarFace(geom.Vec3{X: 10, Y: 8, Z: entry + 7.2*direction}, geom.Vec3{Z: 1}); err == nil {
				t.Fatal("resolved two disconnected floor faces as one support")
			}
		})
	}
}

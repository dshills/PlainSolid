package plaincadbridge

import (
	"encoding/base64"
	"fmt"
	"math"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestClippedCrossedBoreScopeAndSTEP(t *testing.T) {
	slowtest.Skip(t)
	for _, depth := range []float64{4.8, 6, 7.2} {
		for _, opposite := range []bool{false, true} {
			t.Run(fmt.Sprintf("%g/%t", depth, opposite), func(t *testing.T) {
				b := New()
				stock := box(t, b, vector{0, 0, 0}, vector{20, 16, 12})
				x := circle(t, b, vector{0, 8, 6}, vector{1, 0, 0}, 2, 20)
				entry, direction := 0., 1.
				if opposite {
					entry, direction = 12., -1.
				}
				z := circle(t, b, vector{10, 8, entry}, vector{0, 0, direction}, 2, depth)
				h := depth - 6
				q := math.Sqrt((2 - h) * (2 + h))
				theta := math.Acos(math.Abs(h) / 2)
				common := 2*q*q*q/3 - 8*math.Abs(h)*theta + 8*q
				if h >= 0 {
					common = 128./3 - common
				}
				floor := 2 * (4*math.Asin(math.Abs(h)/2) - math.Abs(h)*q)
				buriedX := 32 - 8*q
				if h < 0 {
					buriedX = 8 * q
				}
				buriedZ := buriedX + 8*h*theta
				wantVolume := 3840 - (20+depth)*4*math.Pi + common
				wantArea := 1504 + (20+depth)*4*math.Pi - buriedX - buriedZ - 12*math.Pi + floor
				before := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": x.Handle})
				near(t, overlapMeasure(t, b, before.Handle, z.Handle), depth*4*math.Pi-common)
				after := shape(t, b, map[string]any{"op": "cut", "a": before.Handle, "b": z.Handle})
				group, err := b.cutMany(stock.Handle, []string{z.Handle, x.Handle})
				if err != nil {
					t.Fatal(err)
				}
				sameShape(t, group, after)
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
				if _, err := planarFace(t, b, read.Handle, vector{10, 8, entry + depth*direction}, vector{0, 0, 1}); err == nil {
					t.Fatal("resolved two disconnected floor faces as one support")
				}
			})
		}
	}
}

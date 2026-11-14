package plaincadbridge

import (
	"encoding/base64"
	"fmt"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestBlindTripleCrossedBoreScopeAndSTEP(t *testing.T) {
	for axis := 0; axis < 3; axis++ {
		for _, side := range []float64{1, -1} {
			t.Run(fmt.Sprintf("%d/%v", axis, side), func(t *testing.T) {
				b := New()
				min, max, center := vector{2, 4, -2}, vector{34, 40, 38}, vector{17, 19, 16}
				stock := box(t, b, min, max)
				var handles []string
				var floor, normal geom.Vec3
				sum := 108.
				for i := 0; i < 3; i++ {
					entry, direction := center, vector{}
					entry[i], direction[i] = min[i], 1
					depth := max[i] - min[i]
					if i == axis {
						if side < 0 {
							entry[i], direction[i] = max[i], side
						}
						depth -= 5.2
						sum -= 5.2
						stop := entry
						stop[i] += side * depth
						floor = stop.vec()
						normal = direction.vec().Scale(-1)
					}
					c := circle(t, b, entry, direction, 3, depth)
					handles = append(handles, c.Handle)
				}
				out, err := b.cutMany(stock.Handle, []string{handles[2], handles[0], handles[1]})
				if err != nil {
					t.Fatal(err)
				}
				wantVolume := 32.*36*40 - sum*9*math.Pi + 216*math.Sqrt2
				wantArea := 2.*(32*36+32*40+36*40) + (sum*6-36)*math.Pi - 216*math.Sqrt2
				near(t, out.GeometryAssertions.Volume, wantVolume)
				near(t, out.GeometryAssertions.SurfaceArea, wantArea)
				saved, err := b.exportSTEP([]string{out.Handle})
				if err != nil {
					t.Fatal(err)
				}
				raw, err := base64.StdEncoding.DecodeString(saved.Base64)
				if err != nil {
					t.Fatal(err)
				}
				got, err := b.importSTEP(string(raw))
				if err != nil || got.Bounds != stock.Bounds {
					t.Fatal(got, err)
				}
				near(t, got.GeometryAssertions.Volume, wantVolume)
				near(t, got.GeometryAssertions.SurfaceArea, wantArea)
				f, err := b.shapes[got.Handle].bore.planarFace(floor, normal)
				if err != nil || f.Face != 6 || f.Normal != coordinates(normal) {
					t.Fatal(f, err)
				}
				near(t, f.Area, 9*math.Pi)
				for _, h := range handles {
					near(t, overlapMeasure(t, b, got.Handle, h), 0)
				}
			})
		}
	}
}

package plaincadbridge

import (
	"encoding/base64"
	"fmt"
	"math"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
)

func TestFiniteUnequalCrossedBoreScopeAndSTEP(t *testing.T) {
	slowtest.Skip(t)
	for _, q := range [][4]float64{{3, 2, 0, 11.2}, {2, 3, 14.4, 0}, {3, 2, 14.4, 11.2}} {
		t.Run(fmt.Sprint(q), func(t *testing.T) {
			b := New()
			stock := box(t, b, vector{0, 0, 0}, vector{20, 16, 14})
			xLength, zLength := q[2], q[3]
			if xLength == 0 {
				xLength = 20
			}
			if zLength == 0 {
				zLength = 14
			}
			x := circle(t, b, vector{0, 8, 7}, vector{1, 0, 0}, q[0], xLength)
			z := circle(t, b, vector{10, 8, 14}, vector{0, 0, -1}, q[1], zLength)
			fullX := circle(t, b, vector{0, 8, 7}, vector{1, 0, 0}, q[0], 20)
			fullZ := circle(t, b, vector{10, 8, 14}, vector{0, 0, -1}, q[1], 14)
			full, err := b.cutMany(stock.Handle, []string{fullX.Handle, fullZ.Handle})
			if err != nil {
				t.Fatal(err)
			}
			before := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": x.Handle})
			wantVolume := full.GeometryAssertions.Volume + math.Pi*(q[0]*q[0]*(20-xLength)+q[1]*q[1]*(14-zLength))
			wantArea := full.GeometryAssertions.SurfaceArea - 2*math.Pi*(q[0]*(20-xLength)+q[1]*(14-zLength))
			if q[2] != 0 {
				wantArea += 2 * math.Pi * q[0] * q[0]
			}
			if q[3] != 0 {
				wantArea += 2 * math.Pi * q[1] * q[1]
			}
			near(t, overlapMeasure(t, b, before.Handle, z.Handle), before.GeometryAssertions.Volume-wantVolume)
			successive := shape(t, b, map[string]any{"op": "cut", "a": before.Handle, "b": z.Handle})
			grouped, err := b.cutMany(stock.Handle, []string{z.Handle, x.Handle})
			if err != nil {
				t.Fatal(grouped, successive, err)
			}
			sameShape(t, grouped, successive)
			file, err := b.exportSTEP([]string{grouped.Handle})
			if err != nil {
				t.Fatal(err)
			}
			data, err := base64.StdEncoding.DecodeString(file.Base64)
			if err != nil {
				t.Fatal(err)
			}
			got, err := b.importSTEP(string(data))
			if err != nil || got.Bounds != stock.Bounds {
				t.Fatal(got, err)
			}
			near(t, got.GeometryAssertions.Volume, wantVolume)
			near(t, got.GeometryAssertions.SurfaceArea, wantArea)
			body := b.shapes[got.Handle].bore
			if q[2] != 0 {
				floor, err := body.planarFace(geom.Vec3{X: q[2], Y: 8, Z: 7}, geom.Vec3{X: 1})
				if err != nil || floor.Face < 16 {
					t.Fatal(floor, err)
				}
			}
			if q[3] != 0 {
				floor, err := body.planarFace(geom.Vec3{X: 10, Y: 8, Z: 14 - q[3]}, geom.Vec3{Z: 1})
				if err != nil || floor.Face < 16 {
					t.Fatal(floor, err)
				}
			}
		})
	}
}

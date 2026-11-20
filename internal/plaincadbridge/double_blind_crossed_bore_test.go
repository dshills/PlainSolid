package plaincadbridge

import (
	"encoding/base64"
	"fmt"
	"math"
	"testing"
)

func TestDoubleBlindCrossedBoreScopeAndSTEP(t *testing.T) {
	for _, xOpposite := range []bool{false, true} {
		for _, zOpposite := range []bool{false, true} {
			t.Run(fmt.Sprintf("%t/%t", xOpposite, zOpposite), func(t *testing.T) {
				b := New()
				stock := box(t, b, vector{0, 0, 0}, vector{20, 24, 18})
				// Decimal stops exercise represented stock floor resolution.
				const xDepth, zDepth = 15.4, 15.2
				xOrigin, xSign := 0., 1.
				if xOpposite {
					xOrigin, xSign = 20., -1.
				}
				zOrigin, zSign := 0., 1.
				if zOpposite {
					zOrigin, zSign = 18., -1.
				}
				x := circle(t, b, vector{xOrigin, 9, 7}, vector{xSign, 0, 0}, 3, xDepth)
				z := circle(t, b, vector{8, 9, zOrigin}, vector{0, 0, zSign}, 3, zDepth)
				before := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": x.Handle})
				near(t, overlapMeasure(t, b, before.Handle, z.Handle), zDepth*9*math.Pi-144)
				successive := shape(t, b, map[string]any{"op": "cut", "a": before.Handle, "b": z.Handle})
				group, err := b.cutMany(stock.Handle, []string{z.Handle, x.Handle})
				if err != nil {
					t.Fatal(err)
				}
				sameShape(t, group, successive)
				if group.Bounds != stock.Bounds {
					t.Fatal(group, stock)
				}
				near(t, group.GeometryAssertions.Volume, 20.*24*18-(xDepth+zDepth)*9*math.Pi+144)
				near(t, group.GeometryAssertions.SurfaceArea, 2.*(20*24+20*18+24*18)+(xDepth+zDepth)*6*math.Pi-144)
				file, err := b.exportSTEP([]string{group.Handle})
				if err != nil {
					t.Fatal(err)
				}
				data, err := base64.StdEncoding.DecodeString(file.Base64)
				if err != nil {
					t.Fatal(err)
				}
				reloaded, err := b.importSTEP(string(data))
				if err != nil || reloaded.Bounds != group.Bounds {
					t.Fatal(reloaded, err)
				}
				near(t, reloaded.GeometryAssertions.Volume, group.GeometryAssertions.Volume)
				near(t, reloaded.GeometryAssertions.SurfaceArea, group.GeometryAssertions.SurfaceArea)
				near(t, overlapMeasure(t, b, reloaded.Handle, z.Handle), 0)
				for _, floor := range []struct{ origin, normal vector }{
					{vector{xOrigin + xSign*xDepth, 9, 7}, vector{-xSign, 0, 0}},
					{vector{8, 9, zOrigin + zSign*zDepth}, vector{0, 0, -zSign}},
				} {
					face, err := planarFace(t, b, reloaded.Handle, floor.origin, floor.normal)
					if err != nil || face.Normal != floor.normal {
						t.Fatal(face, err)
					}
					near(t, face.Area, 9*math.Pi)
				}
			})
		}
	}
}

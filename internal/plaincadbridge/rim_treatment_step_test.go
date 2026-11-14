package plaincadbridge

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestCylinderRimFilletSTEP(t *testing.T) {
	testCylinderRimSTEP(t, "filletCylinderRims", "radius")
}

func TestCylinderRimChamferSTEP(t *testing.T) {
	testCylinderRimSTEP(t, "chamferCylinderRims", "distance")
}

func testCylinderRimSTEP(t *testing.T, operation, dimension string) {
	for _, axis := range []vector{{0, 0, 1}, {0, 0, -1}, {0, 1, 0}, {0, -1, 0}, {1, 0, 0}, {-1, 0, 0}} {
		b := New()
		start := vector{1, 2, 3}
		stock := circle(t, b, start, axis, 10, 10)
		end := coordinates(start.vec().Add(axis.vec().Scale(10)))
		for _, rims := range [][]vector{{end}, {start}, {start, end}} {
			body := shape(t, b, map[string]any{"op": operation, "handle": stock.Handle,
				"normal": axis, "rimCenters": rims, dimension: 1})
			file, err := b.exportSTEP([]string{body.Handle})
			if err != nil {
				t.Fatal(axis, rims, err)
			}
			raw, err := base64.StdEncoding.DecodeString(file.Base64)
			if err != nil || !bytes.Contains(raw, []byte("B_SPLINE_SURFACE_WITH_KNOTS")) {
				t.Fatal(err)
			}
			got, err := b.importSTEP(string(raw))
			if err != nil {
				t.Fatal(err)
			}
			near(t, got.GeometryAssertions.Volume, body.GeometryAssertions.Volume)
			near(t, got.GeometryAssertions.SurfaceArea, body.GeometryAssertions.SurfaceArea)
			again, err := b.exportSTEP([]string{got.Handle})
			if err != nil || again.Base64 != file.Base64 {
				t.Fatal(err)
			}
		}
	}
}

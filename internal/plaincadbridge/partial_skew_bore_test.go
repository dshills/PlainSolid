package plaincadbridge

import (
	"encoding/base64"
	"math"
	"plainsolid/internal/slowtest"
	"strings"
	"testing"
)

func TestPartialSkewBoreScopeSTEPAndSTL(t *testing.T) {
	slowtest.Skip(t)
	for _, radii := range [][2]float64{{5, 4}, {5, 5}} {
		b := New()
		stock := box(t, b, vector{}, vector{40, 40, 40})
		x := circle(t, b, vector{0, 16, 20}, vector{1, 0, 0}, radii[0], 40)
		z := circle(t, b, vector{20, 21, 40}, vector{0, 0, -1}, radii[1], 40)
		first := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": x.Handle})
		next := shape(t, b, map[string]any{"op": "cut", "a": first.Handle, "b": z.Handle})
		wantVolume, wantArea := 59020.039219249535, 11408.296380716538
		if radii[0] == radii[1] {
			wantVolume, wantArea = 58007.189499960914, 11530.344011235435
		}
		m := next.GeometryAssertions
		if !m.Valid || m.SolidCount != 1 || math.Abs(m.Volume-wantVolume) > 1e-6 || math.Abs(m.SurfaceArea-wantArea) > 1e-6 {
			t.Fatal(radii, m)
		}
		near(t, overlapMeasure(t, b, first.Handle, z.Handle), first.GeometryAssertions.Volume-m.Volume)
		grouped, err := b.cutMany(stock.Handle, []string{z.Handle, x.Handle, z.Handle})
		if err != nil || grouped.Bounds != stock.Bounds {
			t.Fatal(grouped, err)
		}
		sameShape(t, grouped, next)
		near(t, overlapMeasure(t, b, grouped.Handle, x.Handle), 0)
		mesh, err := b.shapes[grouped.Handle].tessellate(request{})
		if err != nil {
			t.Fatal(err)
		}
		if err := mesh.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
		stepOut, err := b.exportSTEP([]string{grouped.Handle})
		if err != nil {
			t.Fatal(err)
		}
		source, err := base64.StdEncoding.DecodeString(stepOut.Base64)
		if err != nil || len(source) >= 512*1024 {
			t.Fatal("STEP source exceeds application limits", len(source), err)
		}
		same, err := b.importSTEP(string(source))
		if err != nil || same.Bounds != stock.Bounds {
			t.Fatal(same, err)
		}
		sameShape(t, same, next)
		near(t, overlapMeasure(t, b, same.Handle, z.Handle), 0)
		// The axis of the second bore lies on the wall of the first, so
		// a turn of the curve in which they meet falls on a seam, and
		// the general Boolean does not take such a body back. Its file
		// is therefore still written from the bore history (D6 of
		// specs/general-boolean-quartic), reads back as a legacy bore
		// body and exports to the same bytes.
		again, err := b.exportSTEP([]string{same.Handle})
		if err != nil || again.Base64 != stepOut.Base64 {
			t.Fatal("canonical STEP changed on re-export", err)
		}
		if strings.Contains(string(source), "INTERSECTION_CURVE") {
			t.Fatalf("radii %v: the file is the boundary of the body, and D6 no longer holds", radii)
		}
	}
}

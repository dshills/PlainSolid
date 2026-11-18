package plaincadbridge

import (
	"bytes"
	"encoding/base64"
	"math"
	"os"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
)

func TestProvedSTEPRevolutionImportAndSourceExport(t *testing.T) {
	slowtest.Skip(t)
	for _, tc := range []struct {
		name   string
		volume float64
		faces  int
	}{
		{"occt-eccentric-annular-chamfer", 726.98826535488854, 8},
		{"occt-conical-double-rim-eccentric-bore", 2282.4014040828897, 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "exchange", "step", "testdata")
			original, err := os.ReadFile(filepath.Join(path, tc.name+"-exact.step"))
			if err != nil {
				t.Fatal(err)
			}
			b := New()
			shape, err := b.importSTEP(string(original))
			if err != nil || !shape.GeometryAssertions.Valid || shape.GeometryAssertions.SolidCount != 1 ||
				math.Abs(shape.GeometryAssertions.Volume-tc.volume) > 1e-5 {
				t.Fatalf("independent solid failed the ordinary import: %+v, %v", shape, err)
			}
			body := b.shapes[shape.Handle]
			if body.analytic == nil || body.analytic.stepRevolution == nil || len(body.analytic.solid.Faces()) != tc.faces || body.analytic.solid.Topology().InnerWires != 2 {
				t.Fatal("ordinary import did not retain the proved original shell and both cap holes")
			}
			m, err := body.tessellate(request{})
			if err != nil {
				t.Fatal("native display", err)
			}
			if err := m.ValidateSolid(); err != nil || m.SignedVolume() <= 0 || math.Abs(m.SignedVolume()/tc.volume-1) > .015 {
				t.Fatal("display lost analytic solid closure or signed volume", m.SignedVolume(), err)
			}
			export, err := b.exportSTEP([]string{shape.Handle})
			if err != nil || len(export.Before) != 1 || len(export.After) != 1 || export.Before[0] != export.After[0] {
				t.Fatal("original source did not pass STEP export readback", err)
			}
			raw, err := base64.StdEncoding.DecodeString(export.Base64)
			if err != nil || !bytes.Equal(raw, original) {
				t.Fatal("source-preserving export changed independently authored STEP bytes", err)
			}
			shift := geom.Identity()
			shift[0][3] = 1
			shifted, err := body.analytic.transformed(shift)
			if err == nil && shifted.stepRevolution != nil {
				t.Fatal("placement retained stale world-coordinate STEP")
			}
			rounded, err := os.ReadFile(filepath.Join(path, tc.name+"-rounded.step"))
			if err != nil {
				t.Fatal(err)
			}
			before := len(b.shapes)
			if _, err := b.importSTEP(string(rounded)); err == nil || len(b.shapes) != before {
				t.Fatal("inconsistent rounded source published a shape", err)
			}
		})
	}
}

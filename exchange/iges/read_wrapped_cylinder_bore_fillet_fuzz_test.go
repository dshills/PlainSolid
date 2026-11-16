package iges

import (
	"bytes"
	"context"
	"os"
	"testing"

	"plainsolid/geom"
)

func FuzzReadWrappedCylinderBoreRimFilletIGES(f *testing.F) {
	limits := Limits{MaxBytes: 32 << 10}
	options := geom.SurfacePatchOptions{MaxOperations: 2000000}
	for _, variant := range []string{"lower", "upper", "both", "off-support"} {
		data, err := os.ReadFile("testdata/occt-model-only-eccentric-bore-rim-" + variant + ".igs")
		if err != nil {
			f.Fatal(err)
		}
		if variant == "both" {
			if _, err := ReadWrappedCylinderBoreRimFillet(context.Background(), bytes.NewReader(data), limits, options); err != nil {
				f.Fatal("positive seed exceeds the fuzz proof limits", err)
			}
		}
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 32<<10 {
			return
		}
		result, err := ReadWrappedCylinderBoreRimFillet(context.Background(), bytes.NewReader(raw), limits, options)
		if err != nil {
			if len(result.SourceIGES()) != 0 || len(result.Solid.Faces()) != 0 || len(result.NativeCurveSourceIDs) != 0 {
				t.Fatal("unproved material or provenance escaped a failed read")
			}
			return
		}
		topology := result.Solid.Topology()
		if !bytes.Equal(result.SourceIGES(), raw) || result.SourceSolidID <= 0 || result.SourceShellID <= 0 || result.LengthUnit <= 0 ||
			len(topology.Shells) != 1 || topology.EulerCharacteristic != 0 ||
			len(result.NativeFaceSourceIDs) != len(result.Solid.Faces()) ||
			len(result.NativeCurveSourceIDs) != len(result.Solid.Edges()) ||
			len(result.NativeEdgeSourceIndices) != len(result.Solid.Edges()) ||
			len(result.NativeVertexSourceIndices) != len(result.Solid.Vertices()) {
			t.Fatal("incomplete eccentric bore topology or source occurrence mapping")
		}
	})
}

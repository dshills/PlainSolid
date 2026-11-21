package plaincadbridge

import (
	"bytes"
	"compress/gzip"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// TestLegacySkewSTEPStillImports reads STEP files that exportStep wrote
// for bores of unequal radius or with skew axes while the legacy bore
// engine still built them: each edge where two bores meet is a spline on
// a SURFACE_CURVE with a p-curve on each cylinder. The code that wrote
// and read them is gone (section 11 of specs/general-boolean-quartic).
// The default bridge now reads each as the boundary it states, to the
// volume and area the legacy body was published with, as a result of the
// general Boolean that can be cut and exported again.
func TestLegacySkewSTEPStillImports(t *testing.T) {
	type q = map[string]any
	for _, c := range []struct {
		name         string
		volume, area float64
	}{
		{"unequal-through", 6966.106463150456, 3070.157153872642},
		{"skew-through", 6963.389287337086, 3072.436494923965},
		{"partial-through", 6336.676160108042, 3171.057542966776},
		{"skew-blind", 6988.522028565804, 3072.436494923965},
		{"skew-companion", 6906.840619572469, 3179.250645146018},
	} {
		t.Run(c.name, func(t *testing.T) {
			packed, err := os.ReadFile(filepath.Join("testdata", "legacy-step", c.name+".step.gz"))
			if err != nil {
				t.Fatal(err)
			}
			unpacked, err := gzip.NewReader(bytes.NewReader(packed))
			if err != nil {
				t.Fatal(err)
			}
			source, err := io.ReadAll(unpacked)
			if err != nil {
				t.Fatal(err)
			}
			b := New()
			read := shape(t, b, q{"op": "importStep", "source": string(source)})
			if got := read.GeometryAssertions; !got.Valid || math.Abs(got.Volume-c.volume) > 1e-5 || math.Abs(got.SurfaceArea-c.area) > 1e-5 {
				t.Fatalf("read as %+v, want volume %.10g and area %.10g", got, c.volume, c.area)
			}
			if body := b.shapes[read.Handle].analytic; body == nil || !body.general {
				t.Fatal("the file did not read as a result of the general Boolean")
			}
			slab := box(t, b, vector{-1, -1, 15}, vector{21, 25, 19})
			cut := shape(t, b, q{"op": "cut", "a": read.Handle, "b": slab.Handle})
			if cut.GeometryAssertions.Volume >= read.GeometryAssertions.Volume {
				t.Fatal("a cut of the body read removed nothing")
			}
			generalSTEPExported(t, b, read.Handle)
		})
	}
}

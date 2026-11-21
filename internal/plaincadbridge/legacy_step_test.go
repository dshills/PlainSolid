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

// TestLegacyBoreSTEPStillImports reads STEP files that exportStep wrote
// for crossed bores while the legacy bore engine still built them, with
// a p-curve on each surface of every edge. The first five are bores of
// unequal radius or with skew axes, where each edge between two bores is
// a spline (section 11 of specs/general-boolean-quartic). The rest are
// bores of equal radius whose axes meet, one file for each family that
// built them (section 13 of specs/general-boolean-crossing). The code
// that wrote and read these files is gone. The default bridge now reads
// each as the boundary it states, to the volume and area the legacy body
// was published with, as a result of the general Boolean that can be cut
// and exported again.
func TestLegacyBoreSTEPStillImports(t *testing.T) {
	type q = map[string]any
	skew := [2]vector{{-1, -1, 15}, {21, 25, 19}}
	small := [2]vector{{-1, -1, 10}, {21, 17, 13}}
	triple := [2]vector{{1, 3, 30}, {35, 41, 39}}
	for _, c := range []struct {
		name         string
		volume, area float64
		slab         [2]vector // a box that overlaps the body
	}{
		{"unequal-through", 6966.106463150456, 3070.157153872642, skew},
		{"skew-through", 6963.389287337086, 3072.436494923965, skew},
		{"partial-through", 6336.676160108042, 3171.057542966776, skew},
		{"skew-blind", 6988.522028565804, 3072.436494923965, skew},
		{"skew-companion", 6906.840619572469, 3179.250645146018, skew},
		{"crossed-through", 3480.542807007173, 1791.858377202057, small},
		{"crossed-blind", 3511.958733543071, 1785.575191894877, small},
		{"crossed-far-tangent", 3530.80828946461, 1766.725635973339, small},
		{"crossed-clipped", 3534.431732412852, 1761.652815845798, small},
		{"crossed-center", 3534.607697359994, 1761.026524130261, small},
		{"crossed-before", 3534.783662307136, 1761.849609594021, small},
		{"crossed-double-blind", 3574.790586614867, 1747.8760800518, small},
		{"crossed-triple", 43331.84207018331, 9304.635906759748, triple},
		{"crossed-triple-blind", 43614.58540900639, 9172.689015308977, triple},
		{"crossed-companion", 61379.39846484098, 11328.38922153952, [2]vector{{-1, -1, 30}, {41, 41, 41}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
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
			slab := box(t, b, c.slab[0], c.slab[1])
			cut := shape(t, b, q{"op": "cut", "a": read.Handle, "b": slab.Handle})
			if cut.GeometryAssertions.Volume >= read.GeometryAssertions.Volume {
				t.Fatal("a cut of the body read removed nothing")
			}
			generalSTEPExported(t, b, read.Handle)
		})
	}
}

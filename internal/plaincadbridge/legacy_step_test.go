package plaincadbridge

import (
	"bytes"
	"compress/gzip"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"plainsolid/exchange/step"
)

// TestLegacyBoreSTEPStillImports reads STEP files that exportStep wrote
// for crossed bores while the legacy bore engine still built them, with
// a p-curve on each surface of every edge. The first five are bores of
// unequal radius or with skew axes, where each edge between two bores is
// a spline (section 11 of specs/general-boolean-quartic). The rest are
// bores of equal radius whose axes meet, one file for each family that
// built them (section 13 of specs/general-boolean-crossing), and the
// last eight are bores with an oblique axis, alone, across a bore along
// a coordinate axis, and apart from one (section 14). The code that
// wrote and read these files is gone. The last ten are plain bores, which
// their own readers still recognise; those readers now have the general
// Boolean cut the stock by the bores they recover (section 17). The default bridge now reads
// each as the boundary it states, to the volume and area the legacy body
// was published with, as a result of the general Boolean that can be cut
// and exported again.
func TestLegacyBoreSTEPStillImports(t *testing.T) {
	type q = map[string]any
	skew := [2]vector{{-1, -1, 15}, {21, 25, 19}}
	small := [2]vector{{-1, -1, 10}, {21, 17, 13}}
	triple := [2]vector{{1, 3, 30}, {35, 41, 39}}
	plate := [2]vector{{-1, -1, 5}, {5, 6, 7}}
	plain := [2]vector{{-1, -1, 17}, {41, 31, 21}}
	tall := [2]vector{{-1, -1, 36}, {21, 17, 41}}
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
		{"oblique-through", 117.56602587488, 161.0970988637412, plate},
		{"oblique-adjacent-caps", 118.7091902931713, 154.085245760764, plate},
		{"oblique-crossed-through", 12036.51924339999, 4200.26542216873, tall},
		{"oblique-crossed-reciprocal", 11949.57726605811, 4267.846535109757, tall},
		{"oblique-crossed-blind", 12187.3156907723, 4074.601716025138, tall},
		{"oblique-crossed-floor", 12115.05905973973, 4149.999939711293, tall},
		{"oblique-crossed-double-blind", 12265.85550711204, 4024.336233567702, tall},
		{"oblique-disjoint", 7872.402555211128, 2642.435145098857, [2]vector{{-1, -1, 17}, {21, 21, 21}}},
		{"plain-through", 23434.513322353836, 5520.442450666158, plain},
		{"plain-blind-top", 23660.707993412303, 5426.194671058465, plain},
		{"plain-blind-bottom", 23773.805328941533, 5350.7964473723105, plain},
		{"plain-two-through", 23183.18591006665, 5746.637121724622, plain},
		{"plain-two-blind", 23597.876140340504, 5489.026524130261, plain},
		{"plain-x-through", 22869.026644707676, 5897.433569096934, plain},
		{"plain-y-blind", 23604.159325647688, 5463.893782901542, plain},
		{"plain-mixed-axes", 23246.01776313845, 5903.716754404114, plain},
		{"plain-mixed-axes-blind", 23296.283245595885, 5878.584013175397, plain},
		{"plain-near-the-edge", 23748.672587712816, 5426.194671058464, plain},
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

// TestLegacySideSTEPStillImports reads STEP files that exportStep wrote
// for cylinders that break out of the side or the corner of planar stock,
// while the legacy families still built those bodies: the side notch
// product, which also has the corner and the notch with bores; the side
// opening cuts, for two or more openings on one side; and the products
// for two discs that cross, one for each arrangement of their centres
// and radii (section 18 of specs/general-boolean-crossing). The bridge
// no longer has their readers. It reads each as the boundary it states,
// to the volume and area the legacy body had, to nine figures, as a
// result of the general Boolean that can be cut and exported again.
func TestLegacySideSTEPStillImports(t *testing.T) {
	type q = map[string]any
	stock := [2]vector{{-1, -1, 17}, {41, 31, 21}}
	for _, c := range []struct {
		name         string
		volume, area float64
		slab         [2]vector // a box that overlaps the body
	}{
		{"notch-through", 23717.2567, 5240.22123, stock},
		{"notch-blind-inset", 23839.807, 5246.45556, stock},
		{"corner-through", 23858.6283, 5160.11061, stock},
		{"corner-blind", 23943.4513, 5189.69911, stock},
		{"notch-blind-then-bores", 23535.0443, 5554.12386, stock},
		{"dual-unequal", 23611.4909, 5232.4765, stock},
		{"dual-blind-unequal", 23816.7987, 5238.72978, stock},
		{"openings-apart", 23773.8053, 5254.79645, stock},
		{"corner-then-side", 23830.354, 5217.09734, stock},
		{"dd-equal-blind", 309.481923, 321.214462, [2]vector{{-1, 3, 5}, {11, 11, 7}}},
		{"dd-equal-through", 258.963846, 296.750205, [2]vector{{-1, 3, 5}, {11, 11, 7}}},
		{"dd-unequal", 6422.3982, 3674.3013, [2]vector{{-1, -1, 5}, {61, 23, 7}}},
		{"dd-offcenter", 3250.85545, 1971.52613, [2]vector{{-1, -1, 5}, {51, 13, 7}}},
		{"dd-exactwall-blind", 5482.5573, 3113.50759, [2]vector{{-1, -1, 5}, {51, 23, 7}}},
		{"dd-offset-blind", 2282.90274, 1405.43257, [2]vector{{-1, -1, 5}, {41, 11, 7}}},
		{"dd-unequal-center-through", 1973.07114, 1530.13283, [2]vector{{-1, -1, 5}, {61, 11, 7}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			packed, err := os.ReadFile(filepath.Join("testdata", "legacy-step", "side-"+c.name+".step.gz"))
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
			if got := read.GeometryAssertions; !got.Valid || math.Abs(got.Volume-c.volume) > 1e-8*c.volume || math.Abs(got.SurfaceArea-c.area) > 1e-8*c.area {
				t.Fatalf("read as %+v, want volume %.9g and area %.9g", got, c.volume, c.area)
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

// TestDisjointBoresSTEPInCentimetres reads two of those files in the
// product for bores along more than one axis, states their lengths in
// centimetres instead, and imports them: the body is ten times the size,
// with the volume and the area that follow.
func TestDisjointBoresSTEPInCentimetres(t *testing.T) {
	for _, c := range []struct {
		name         string
		volume, area float64
	}{
		{"plain-mixed-axes", 23246.01776313845, 5903.716754404114},
		{"plain-mixed-axes-blind", 23296.283245595885, 5878.584013175397},
	} {
		packed, err := os.ReadFile(filepath.Join("testdata", "legacy-step", c.name+".step.gz"))
		if err != nil {
			t.Fatal(err)
		}
		unpacked, err := gzip.NewReader(bytes.NewReader(packed))
		if err != nil {
			t.Fatal(err)
		}
		model, err := step.ReadDisjointBores(unpacked, stepLimits())
		if err != nil {
			t.Fatal(c.name, err)
		}
		model.LengthUnit = .01
		var scaled bytes.Buffer
		if err := step.WriteDisjointBores(&scaled, model); err != nil {
			t.Fatal(err)
		}
		b := New()
		got, err := b.importSTEP(scaled.String())
		if err != nil {
			t.Fatal(c.name, err)
		}
		near(t, got.GeometryAssertions.Volume, c.volume*1000)
		near(t, got.GeometryAssertions.SurfaceArea, c.area*100)
		if body := b.shapes[got.Handle].analytic; body == nil || !body.general {
			t.Fatal("the file did not read as a result of the general Boolean")
		}
	}
}

package step

import (
	"bytes"
	"context"
	"errors"
	"math"
	"plainsolid/geom"
	"strings"
	"testing"
)

func nativeEllipsePrism(t testing.TB) EllipticalPrismMaterial {
	t.Helper()
	o := EllipticalWireRepairOptions{MaxDistance: 1e-11}
	s, e := RepairRoundedEllipticalPrismShell(bytes.NewReader(ellipticalPrismFixture(t)), Limits{}, o)
	if e != nil {
		t.Fatal(e)
	}
	m, e := PromoteRepairedEllipticalPrism(context.Background(), s.Source, Limits{}, o, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestWriteEllipticalPrismMaterialSTEP(t *testing.T) {
	t.Parallel()
	m := nativeEllipsePrism(t)
	for _, o := range []EllipticalPrismSTEPOptions{{}, {Name: "authored 'ellipse'", Translation: geom.Vec3{X: 8, Y: -2, Z: 4}}} {
		var b bytes.Buffer
		if e := WriteEllipticalPrismMaterialSTEP(context.Background(), &b, m, o); e != nil {
			t.Fatal(e)
		}
		f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil || len(f.Entities) != 76 {
			t.Fatal(e)
		}
		for _, en := range f.Entities {
			for _, r := range en.Records {
				if r.Type == "PCURVE" || r.Type == "SURFACE_CURVE" || r.Type == "SEAM_CURVE" {
					t.Fatal("pcurve retained")
				}
			}
		}
		if strings.Contains(b.String(), "6.283185") {
			t.Fatal("rounded periodic trim")
		}
		if _, e := RepairRoundedEllipticalPrismShell(bytes.NewReader(b.Bytes()), Limits{}, EllipticalWireRepairOptions{MaxDistance: 1e-11}); e == nil {
			t.Fatal("old source gate widened")
		}
		en, e := simple(f, 21, "EDGE_CURVE", 5)
		if e != nil || en.Args[3].Ref != 27 {
			t.Fatal("model seam", e)
		}
		p, e := mappedExactCoordinate(f, 35, "CARTESIAN_POINT", 3)
		if e != nil || p != (geom.Vec3{X: 1, Y: 2, Z: 3}).Add(o.Translation) {
			t.Fatal(p, e)
		}
	}
	for _, o := range []EllipticalPrismSTEPOptions{{Name: strings.Repeat("x", 1025)}, {Name: "bad\nname"}, {Translation: geom.Vec3{X: math.NaN()}}, {Translation: geom.Vec3{X: math.SmallestNonzeroFloat64}}} {
		var b bytes.Buffer
		e := WriteEllipticalPrismMaterialSTEP(context.Background(), &b, m, o)
		if e == nil || b.Len() != 0 {
			t.Fatal("nonatomic rejection", o)
		}
		if o.Translation.X == math.SmallestNonzeroFloat64 && !errors.Is(e, geom.ErrUnrepresentable) {
			t.Fatal(e)
		}
	}
	original, _ := mappedPlanarSourceBytes(m.Shell.Source)
	var b bytes.Buffer
	if e := WriteEllipticalPrismMaterialSTEP(context.Background(), &b, m, EllipticalPrismSTEPOptions{Translation: geom.Vec3{X: 4}}); e != nil {
		t.Fatal(e)
	}
	after, _ := mappedPlanarSourceBytes(m.Shell.Source)
	if !bytes.Equal(original, after) {
		t.Fatal("mutated source")
	}
	m.NativeRimEdges[0][0]++
	b.Reset()
	if WriteEllipticalPrismMaterialSTEP(context.Background(), &b, m, EllipticalPrismSTEPOptions{}) == nil || b.Len() != 0 {
		t.Fatal("stale material")
	}
}

func FuzzWriteEllipticalPrismMaterialSTEP(f *testing.F) {
	m := nativeEllipsePrism(f)
	f.Add("native", 0., 0., 0.)
	f.Add("quoted 'native'", 8., -2., 4.)
	f.Fuzz(func(t *testing.T, name string, x, y, z float64) {
		if len(name) > 1024 {
			return
		}
		var b bytes.Buffer
		e := WriteEllipticalPrismMaterialSTEP(context.Background(), &b, m, EllipticalPrismSTEPOptions{Name: name, Translation: geom.Vec3{X: x, Y: y, Z: z}})
		if e != nil {
			if b.Len() != 0 {
				t.Fatal("partial invalid output")
			}
			return
		}
		g, e := Parse(bytes.NewReader(b.Bytes()), Limits{MaxBytes: 65536, MaxEntities: 76, MaxValues: 4096, MaxDepth: 16})
		if e != nil || len(g.Entities) != 76 {
			t.Fatal("invalid generated graph", e)
		}
	})
}

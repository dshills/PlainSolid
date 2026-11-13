package cylindercells

import (
	"bytes"
	"encoding/json"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"reflect"
	"strings"
	"testing"
)

func native(t *testing.T) parametric.Solid {
	t.Helper()
	f, e := geom.NewFrame(geom.Vec3{X: -3, Y: 5, Z: 2}, geom.Vec3{Y: 1}, geom.Vec3{Z: -1})
	if e != nil {
		t.Fatal(e)
	}
	s, e := parametric.Cylinder(f, 2, 3)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestCylinderAssetChain(t *testing.T) {
	for _, dialect := range []string{Half, Quarter} {
		s := native(t)
		a, e := FromNative(s, "part 'named'", SIUnit{-3}, dialect)
		if e != nil {
			t.Fatal(e)
		}
		result, e := a.Validate()
		if e != nil || result.Source != nil {
			t.Fatal(result, e)
		}
		var stepBytes bytes.Buffer
		if e = WriteSTEP(&stepBytes, a); e != nil {
			t.Fatal(e)
		}
		authored := stepBytes.String()
		a, e = FromSTEP(&stepBytes, dialect)
		if e != nil || a.Name != "part 'named'" || a.Unit.MetresExponent != -3 || a.SourceSTEP != authored {
			t.Fatal(a, e)
		}
		result, e = a.Validate()
		if e != nil || result.Source == nil || !reflect.DeepEqual(result.Solid, s) {
			t.Fatal(result, e)
		}
		n := 4
		if dialect == Quarter {
			n = 6
		}
		if len(result.Source.Faces) != n || len(result.Solid.Faces()) != 3 {
			t.Fatal("source/native ancestry conflated")
		}
		result.Source.Faces[0] = 999
		again, e := a.Validate()
		if e != nil || again.Source.Faces[0] == 999 {
			t.Fatal("source IDs alias")
		}
		var b bytes.Buffer
		if e = WriteNative(&b, a); e != nil {
			t.Fatal(e)
		}
		raw := append([]byte(nil), b.Bytes()...)
		back, e := ReadNative(&b)
		if e != nil || back != a {
			t.Fatal(back, e)
		}
		if e = WriteNative(&b, back); e != nil || !bytes.Equal(raw, b.Bytes()) {
			t.Fatal("unstable", e)
		}
		if e = WriteSTEP(&stepBytes, back); e != nil || stepBytes.String() != authored {
			t.Fatal("canonical source chain changed", e)
		}
	}
}
func TestCylinderAssetTamper(t *testing.T) {
	a, e := FromNative(native(t), "part", SIUnit{-3}, Quarter)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	_ = WriteSTEP(&b, a)
	a, e = FromSTEP(&b, Quarter)
	if e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"name", "unit", "radius", "pose", "dialect", "source"} {
		bad := a
		switch mode {
		case "name":
			bad.Name = "changed"
		case "unit":
			bad.Unit = SIUnit{0}
		case "radius":
			bad.Radius = 3
		case "pose":
			bad.Origin.X++
		case "dialect":
			bad.Dialect = Half
		case "source":
			bad.SourceSTEP = strings.Replace(bad.SourceSTEP, "'mechanical'", "'other'", 1)
		}
		b.Reset()
		if e = WriteNative(&b, bad); e == nil || b.Len() != 0 {
			t.Fatal("accepted tamper", mode)
		}
		raw, _ := json.Marshal(envelope{"plainsolid-cylinder-cells", 1, bad})
		if got, e := ReadNative(bytes.NewReader(raw)); e == nil || got.Dialect != "" {
			t.Fatal("accepted forged valid JSON", mode)
		}
	}
	b.Reset()
	_ = WriteNative(&b, a)
	raw := b.String()
	for _, bad := range []string{strings.Replace(raw, `"Version":1`, `"Version":2`, 1), strings.Replace(raw, `"Version":1`, `"Version":1,"version":1`, 1), strings.Replace(raw, `"Version":1`, `"Version":1,"extra":1`, 1), strings.Replace(raw, `"Version":1`, `"Version":1,"Verſion":1`, 1), raw + `{}`, `null`, strings.Repeat("[", 10) + strings.Repeat("]", 10), strings.Repeat(" ", MaxNativeBytes+1)} {
		if got, e := ReadNative(strings.NewReader(bad)); e == nil || got.Dialect != "" {
			t.Fatal("accepted malformed")
		}
	}
	if _, e = FromNative(native(t), "part", SIUnit{-4}, Quarter); e == nil {
		t.Fatal("non-SI exponent")
	}
	if _, e = FromSTEP(strings.NewReader(a.SourceSTEP), Half); e == nil {
		t.Fatal("dialect fallback")
	}
}

func TestCylinderAssetStrictSourceUnits(t *testing.T) {
	a, e := FromNative(native(t), "unit part", SIUnit{-3}, Half)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteSTEP(&b, a); e != nil {
		t.Fatal(e)
	}
	raw := b.String()
	for _, pair := range [][2]string{{".MILLI.,.METRE.", ".VENDOR.,.METRE."}, {".MILLI.,.METRE.", ".MILLI.,.RADIAN."}, {"SI_UNIT($,.RADIAN.)", "SI_UNIT(.MILLI.,.RADIAN.)"}, {"GEOMETRIC_REPRESENTATION_CONTEXT(3)", "GEOMETRIC_REPRESENTATION_CONTEXT(3.00000000000000000001)"}} {
		bad := strings.Replace(raw, pair[0], pair[1], 1)
		if bad == raw {
			t.Fatal("missing mutation")
		}
		if got, e := FromSTEP(strings.NewReader(bad), Half); e == nil || got.Dialect != "" {
			t.Fatal("accepted loose unit/context", pair)
		}
	}
}
func FuzzCylinderAssetNative(f *testing.F) {
	f.Add([]byte(`{"Format":"plainsolid-cylinder-cells","Version":1,"Asset":{}}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > MaxNativeBytes {
			return
		}
		a, e := ReadNative(bytes.NewReader(b))
		if e == nil {
			var out bytes.Buffer
			if e = WriteNative(&out, a); e != nil {
				t.Fatal(e)
			}
		}
	})
}

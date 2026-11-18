package step

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestBoxPocketSTEPFullGraphRoundTrip(t *testing.T) {
	t.Parallel()
	for _, axis := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		frame, e := geom.FrameFromNormal(geom.Vec3{X: 26, Y: -17, Z: 9}, axis)
		if e != nil {
			t.Fatal(e)
		}
		b := BoxPocketBody{Name: "pocket stock", Frame: frame, Size: geom.Vec3{X: 20, Y: 20, Z: 10}, Pockets: []trimmed.BoxBlindPocket{{Footprint: trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 1, Y: 2}, Max: geom.Vec2{X: 7, Y: 8}}, Floor: 6}, {Footprint: trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 2, Y: 3}, Max: geom.Vec2{X: 6, Y: 7}}, Floor: 4}}, Bores: []trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 13, Y: 12}, Radius: 2}, {Center: geom.Vec2{X: 17, Y: 17}, Radius: 1}}}
		var out bytes.Buffer
		if e = WriteBoxPocketExtrusions(&out, BoxPocketModel{Bodies: []BoxPocketBody{b}, LengthUnit: .001}); e != nil {
			t.Fatal(axis, e)
		}
		got, e := ReadBoxPocketExtrusions(bytes.NewReader(out.Bytes()), Limits{})
		if e != nil {
			t.Fatal(axis, e)
		}
		q := got.Bodies[0]
		if q.Frame != b.Frame || q.Size != b.Size || len(q.Pockets) != 2 || len(q.Bores) != 2 || got.LengthUnit != .001 {
			t.Fatal(q)
		}
		volume := q.Size.X * q.Size.Y * q.Size.Z
		top := q.Size.Z
		for _, p := range q.Pockets {
			volume -= (p.Footprint.Max.X - p.Footprint.Min.X) * (p.Footprint.Max.Y - p.Footprint.Min.Y) * (top - p.Floor)
			top = p.Floor
		}
		for _, c := range q.Bores {
			volume -= math.Pi * c.Radius * c.Radius * q.Size.Z
		}
		if math.Abs(volume-(4000-144-32-50*math.Pi)) > 1e-10 {
			t.Fatal(volume)
		}
		source := out.String()
		// A flipped edge sense changes real trim orientation while leaving every
		// inferred box, pocket and bore dimension unchanged. Full graph proof must
		// reject it rather than accept the candidate recipe alone.
		at := strings.Index(source, "EDGE_CURVE(")
		if at < 0 {
			t.Fatal("no edge")
		}
		end := strings.Index(source[at:], ";") + at
		edge := source[at:end]
		sense := ",.T.)"
		opposite := ",.F.)"
		if strings.HasSuffix(edge, opposite) {
			sense, opposite = opposite, sense
		}
		altered := source[:at] + strings.Replace(edge, sense, opposite, 1) + source[end:]
		if _, e = ReadBoxPocketExtrusions(strings.NewReader(altered), Limits{}); e == nil {
			t.Fatal("accepted corrupt edge sense")
		}
		if _, e = ReadBoxPocketExtrusions(bytes.NewReader(out.Bytes()), Limits{MaxFaces: 4}); e == nil {
			t.Fatal("ignored face budget")
		}
	}
}

func TestBoxPocketSTEPFaceLimit(t *testing.T) {
	t.Parallel()
	frame, err := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	b := BoxPocketBody{Frame: frame, Size: geom.Vec3{X: 100, Y: 100, Z: 10}}
	for i := 0; i < 8; i++ {
		b.Pockets = append(b.Pockets, trimmed.BoxBlindPocket{Footprint: trimmed.ExtrusionRectangle{Min: geom.Vec2{X: float64(i + 1), Y: float64(i + 1)}, Max: geom.Vec2{X: float64(20 - i), Y: float64(20 - i)}}, Floor: float64(9 - i)})
	}
	for i := 0; i < 3; i++ {
		b.Bores = append(b.Bores, trimmed.ExtrusionCircle{Center: geom.Vec2{X: 30 + 5*float64(i), Y: 50}, Radius: 1})
	}
	m := BoxPocketModel{Bodies: []BoxPocketBody{b}, LengthUnit: .001}
	var out bytes.Buffer
	if err := WriteBoxPocketExtrusions(&out, m); err != nil {
		t.Fatal("58-face writer", err)
	}
	if _, err := ReadBoxPocketExtrusions(bytes.NewReader(out.Bytes()), Limits{}); err != nil {
		t.Fatal("58-face reader", err)
	}
	if _, err := ReadBoxPocketExtrusions(bytes.NewReader(out.Bytes()), Limits{MaxFaces: 57}); err == nil {
		t.Fatal("ignored caller face limit")
	}
	// The next bore exceeds the five-wire cap limit. Preserve existing bytes.
	m.Bodies[0].Bores = append(m.Bodies[0].Bores, trimmed.ExtrusionCircle{Center: geom.Vec2{X: 85, Y: 50}, Radius: 1})
	before := out.String()
	if err := WriteBoxPocketExtrusions(&out, m); err == nil {
		t.Fatal("accepted oversized recipe")
	}
	if out.String() != before {
		t.Fatal("published partial output")
	}
	// Invalid numeric geometry cannot be reached before the count rejection.
	m.Bodies[0].Bores = make([]trimmed.ExtrusionCircle, 10000)
	if err := WriteBoxPocketExtrusions(&out, m); err != geom.ErrDomain {
		t.Fatal("count not rejected first", err)
	}
	if out.String() != before {
		t.Fatal("published rejected recipe")
	}
	if boundedBoxPocketRecipe(2, int(^uint(0)>>1)) {
		t.Fatal("count overflow")
	}
}

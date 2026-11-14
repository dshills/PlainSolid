package step

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func TestTiltedEllipticalPrism(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/occt-tilted-elliptical-prism.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadTiltedEllipticalPrism(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if m.Height != 5 || m.MajorRadius != 5 || m.MinorRadius != 2 {
		t.Fatal("dimensions")
	}
	if _, e = ReadExactControlEllipticalPrism(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("cardinal reader widened")
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, m.Solid); e != nil {
		t.Fatal(e)
	}
	s, e := trimmed.ReadJSON(&native, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	before, e := m.Solid.RationalVertices(ctx)
	if e != nil {
		t.Fatal(e)
	}
	after, e := s.RationalVertices(ctx)
	if e != nil || len(after) != 8 {
		t.Fatal(e)
	}
	for i := range before {
		for j := 0; j < 3; j++ {
			if before[i].Point[j].Cmp(after[i].Point[j]) != 0 {
				t.Fatal("rounded vertex")
			}
		}
	}
	p, e := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !p.Complete {
		t.Fatal(e)
	}
	for _, u := range p.Uses {
		if u.Report.SquaredUpperBound.Sign() != 0 {
			t.Fatal("nonexact shared edge")
		}
	}
	d := TiltedEllipticalPrismDefinition{Name: "tilted", Origin: [3]*big.Rat{big.NewRat(1, 10), big.NewRat(2, 1), big.NewRat(3, 1)}, Reference: geom.Vec3{X: 1}, Axis: geom.Vec3{Y: 3, Z: 4}, MajorRadius: 5, MinorRadius: 2, Height: 5}
	var out bytes.Buffer
	if e = WriteTiltedEllipticalPrismSTEP(ctx, &out, d, 1e-9); e != nil {
		t.Fatal(e)
	}
	n, e := ReadTiltedEllipticalPrism(ctx, bytes.NewReader(out.Bytes()), Limits{}, 1e-9)
	if e != nil || n.Name != "tilted" {
		t.Fatal(e)
	}
	if path := os.Getenv("PLAINSOLID_TILTED_STEP_OUTPUT"); path != "" {
		if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
	}
	for _, pair := range [][2]string{{"0.,0.6,0.8", "0.,0.6,0.800000000000001"}, {"5.,2.", "6.,2."}, {"5.1,2.,3.", "5.2,2.,3."}} {
		bad := bytes.Replace(raw, []byte(pair[0]), []byte(pair[1]), 1)
		if bytes.Equal(bad, raw) {
			t.Fatalf("missing mutation %s", pair[0])
		}
		if _, e = ReadTiltedEllipticalPrism(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
			t.Fatal("forged support accepted")
		}
	}
	out.Reset()
	d.Axis = geom.Vec3{Y: .6, Z: .8}
	if e = WriteTiltedEllipticalPrismSTEP(ctx, &out, d, 1e-9); e == nil || out.Len() != 0 {
		t.Fatal("rounded direction accepted")
	}
	out.Reset()
	d.Axis = geom.Vec3{Z: 1}
	if e = WriteTiltedEllipticalPrismSTEP(ctx, &out, d, 1e-9); e == nil || out.Len() != 0 {
		t.Fatal("cardinal domain accepted")
	}
}

func FuzzTiltedEllipticalPrism(f *testing.F) {
	raw, e := os.ReadFile("testdata/occt-tilted-elliptical-prism.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Add([]byte("ISO-10303-21;END-ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 65536 {
			t.Skip()
		}
		_, _ = ReadTiltedEllipticalPrism(context.Background(), bytes.NewReader(raw), Limits{MaxBytes: 65536, MaxEntities: 76, MaxValues: 4096, MaxDepth: 16}, 1e-9)
	})
}

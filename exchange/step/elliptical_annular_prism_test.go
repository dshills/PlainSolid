package step

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestEllipticalAnnularPrism(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/occt-elliptical-annular-prism.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadEllipticalAnnularPrism(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if m.Height != 5 || m.InnerMajorRadius != 2.5 || m.InnerMinorRadius != 1 {
		t.Fatal("dimensions")
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, m.Solid); e != nil {
		t.Fatal(e)
	}
	s, e := trimmed.ReadJSON(&native, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	v, e := s.RationalVertices(ctx)
	if e != nil || len(v) != 16 {
		t.Fatal(len(v), e)
	}
	before, e := m.Solid.RationalVertices(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for i := range before {
		for j := 0; j < 3; j++ {
			if before[i].Point[j].Cmp(v[i].Point[j]) != 0 {
				t.Fatal("rounded native reload")
			}
		}
	}
	if _, e = ReadTiltedEllipticalPrism(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("old disk reader widened")
	}
	if _, e = ReadEllipticalAnnularPrism(ctx, bytes.NewReader(raw), Limits{MaxFaces: 9}, 1e-9); e == nil {
		t.Fatal("native limits ignored")
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
	d := EllipticalAnnularPrismDefinition{Name: "annulus", Origin: [3]*big.Rat{big.NewRat(1, 10), big.NewRat(2, 1), big.NewRat(3, 1)}, Reference: geom.Vec3{X: 1}, Axis: geom.Vec3{Y: 3, Z: 4}, MajorRadius: 5, MinorRadius: 2, InnerMajorRadius: 2.5, InnerMinorRadius: 1, Height: 5}
	var out bytes.Buffer
	if e = WriteEllipticalAnnularPrismSTEP(ctx, &out, d, 1e-9); e != nil {
		t.Fatal(e)
	}
	n, e := ReadEllipticalAnnularPrism(ctx, bytes.NewReader(out.Bytes()), Limits{}, 1e-9)
	if e != nil || n.Name != "annulus" {
		t.Fatal(e)
	}
	if path := os.Getenv("PLAINSOLID_ANNULAR_STEP_OUTPUT"); path != "" {
		if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
	}
	for _, pair := range [][2]string{{"2.5,1.", "5.,2."}, {"2.6,2.,3.", "2.7,2.,3."}, {"#81,.T.", "#81,.F."}, {"#75,.T.", "#39,.T."}} {
		bad := bytes.Replace(raw, []byte(pair[0]), []byte(pair[1]), 1)
		if bytes.Equal(bad, raw) {
			t.Fatal("missing mutation", pair[0])
		}
		if _, e = ReadEllipticalAnnularPrism(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
			t.Fatal("tamper accepted")
		}
	}
	out.Reset()
	d.InnerMinorRadius = .9
	if e = WriteEllipticalAnnularPrismSTEP(ctx, &out, d, 1e-9); e == nil || out.Len() != 0 {
		t.Fatal("nonhomothetic accepted")
	}
}

func FuzzEllipticalAnnularPrism(f *testing.F) {
	raw, e := os.ReadFile("testdata/occt-elliptical-annular-prism.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		_, _ = ReadEllipticalAnnularPrism(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 118, MaxValues: 4096, MaxDepth: 16}, 1e-9)
	})
}

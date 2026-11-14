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

func TestOrthogonalEllipticalAnnularPrism(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/occt-orthogonal-elliptical-annular-prism.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadOrthogonalEllipticalAnnularPrism(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if m.Height != 5 || m.InnerMajorRadius != 1.5 || m.InnerMinorRadius != .5 {
		t.Fatal("dimensions")
	}
	outer, inner := m.RationalFrame(), m.InnerRationalFrame()
	for i := 0; i < 3; i++ {
		if inner[1][i].Cmp(outer[2][i]) != 0 || inner[2][i].Cmp(new(big.Rat).Neg(outer[1][i])) != 0 {
			t.Fatal("inner frame")
		}
	}
	inner[1][1].SetInt64(7)
	if m.InnerRationalFrame()[1][1].Cmp(big.NewRat(4, 5)) != 0 {
		t.Fatal("frame alias")
	}
	if _, e = ReadNestedEllipticalAnnularPrism(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("nonhomothetic aligned reader widened")
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
	if _, e = ReadEllipticalAnnularPrism(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("old aligned reader widened")
	}
	if _, e = ReadOrthogonalEllipticalAnnularPrism(ctx, bytes.NewReader(raw), Limits{MaxFaces: 9}, 1e-9); e == nil {
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
	d := OrthogonalEllipticalAnnularPrismDefinition{Name: "annulus", Origin: [3]*big.Rat{big.NewRat(1, 10), big.NewRat(2, 1), big.NewRat(3, 1)}, Reference: geom.Vec3{X: 1}, Axis: geom.Vec3{Y: 3, Z: 4}, MajorRadius: 5, MinorRadius: 2, InnerMajorRadius: 1.5, InnerMinorRadius: .5, Height: 5}
	var out bytes.Buffer
	if e = WriteOrthogonalEllipticalAnnularPrismSTEP(ctx, &out, d, 1e-9); e != nil {
		t.Fatal(e)
	}
	n, e := ReadOrthogonalEllipticalAnnularPrism(ctx, bytes.NewReader(out.Bytes()), Limits{}, 1e-9)
	if e != nil || n.Name != "annulus" {
		t.Fatal(e)
	}
	if path := os.Getenv("PLAINSOLID_ORTHOGONAL_ANNULAR_STEP_OUTPUT"); path != "" {
		if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
	}
	for _, pair := range [][2]string{{"1.5,0.5", "5.,2."}, {"0.1,3.2,2.1", "0.2,3.2,2.1"}, {"#81,.T.", "#81,.F."}, {"#75,.T.", "#39,.T."}, {"-0.,0.8,-0.6", "0.,-0.8,0.6"}} {
		bad := bytes.Replace(raw, []byte(pair[0]), []byte(pair[1]), 1)
		if bytes.Equal(bad, raw) {
			t.Fatal("missing mutation", pair[0])
		}
		if _, e = ReadOrthogonalEllipticalAnnularPrism(ctx, bytes.NewReader(bad), Limits{}, 1e-9); e == nil {
			t.Fatal("tamper accepted")
		}
	}
	out.Reset()
	d.InnerMajorRadius = 2
	if e = WriteOrthogonalEllipticalAnnularPrismSTEP(ctx, &out, d, 1e-9); e == nil || out.Len() != 0 {
		t.Fatal("touching inner ellipse accepted")
	}
}

func FuzzOrthogonalEllipticalAnnularPrism(f *testing.F) {
	raw, e := os.ReadFile("testdata/occt-orthogonal-elliptical-annular-prism.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		_, _ = ReadOrthogonalEllipticalAnnularPrism(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 118, MaxValues: 4096, MaxDepth: 16}, 1e-9)
	})
}

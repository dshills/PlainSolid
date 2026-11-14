package parametric

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestNativeTorusCylinderSet(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	for _, op := range []geom.TorusCylinderOperation{geom.TorusCylinderCommon, geom.TorusCylinderDifference} {
		s, e := NewTorusCylinderSet(ctx, 5, 2, 6, -3, 4, op, 1e-9)
		if e != nil {
			t.Fatal(op, e)
		}
		solid := s.Solid()
		if len(solid.Vertices()) != 2 || len(solid.Edges()) != 4 || len(solid.Faces()) != 2 {
			t.Fatal("native Euler")
		}
		radius, h2, roots, e := s.RimProof()
		if e != nil || radius.Cmp(r(6, 1)) != 0 || h2.Cmp(r(3, 1)) != 0 {
			t.Fatal("rim proof", e)
		}
		for _, root := range roots {
			v, e := root.SignPolynomial(ctx, []*big.Rat{r(-3, 1), r(0, 1), r(1, 1)}, geom.ExactCurvePlaneEventOptions{})
			if e != nil || v.Sign != 0 {
				t.Fatal("irrational seam", e)
			}
		}
		radius.SetInt64(99)
		again, _, _, _ := s.RimProof()
		if again.Cmp(r(6, 1)) != 0 {
			t.Fatal("proof alias")
		}
		p, e := solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil {
			t.Fatal(e)
		}
		volume, e := s.Query().BoundVolume(ctx, geom.TorusCylinderVolumeOptions{VolumeTolerance: r(1, 100)})
		if e != nil {
			t.Fatal(e)
		}
		area, e := s.Query().BoundArea(ctx, geom.TorusCylinderAreaOptions{AreaTolerance: r(1, 1000000)})
		if e != nil {
			t.Fatal(e)
		}
		check := func(x float64, l, h *big.Rat) {
			lo, _ := l.Float64()
			hi, _ := h.Float64()
			if x < lo-1e-6 || x > hi+1e-6 {
				t.Fatal("native independent measure", x, lo, hi)
			}
		}
		check(p.Volume, volume.Lower, volume.Upper)
		check(p.Area, area.Lower, area.Upper)
		if p.Centroid.Length() > 1e-8 {
			t.Fatal(p)
		}
		for i, f := range solid.Faces() {
			for _, u := range []float64{0, .7, 3, 2 * math.Pi} {
				for _, v := range []float64{-1, -.3, 0, .7, 1} {
					p, e := f.Surface.Eval(u, v)
					if e != nil {
						t.Fatal(e)
					}
					rho := math.Hypot(p.X, p.Y)
					residual := (rho-5)*(rho-5) + p.Z*p.Z - 4
					if i == 1 {
						residual = rho*rho - 36
					}
					if math.Abs(residual) > 1e-12 {
						t.Fatal("original support", i, residual)
					}
				}
			}
		}
		var data bytes.Buffer
		if e = WriteJSON(&data, solid); e != nil {
			t.Fatal(e)
		}
		restored, e := ReadJSON(bytes.NewReader(data.Bytes()), JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		var canonical bytes.Buffer
		if e = WriteJSON(&canonical, restored); e != nil || !bytes.Equal(data.Bytes(), canonical.Bytes()) {
			t.Fatal("reload", e)
		}
		tampered := bytes.ReplaceAll(data.Bytes(), []byte("\"Radius\":6"), []byte("\"Radius\":7"))
		if bytes.Equal(tampered, data.Bytes()) {
			t.Fatal("tamper not applied")
		}
		if _, e := ReadJSON(bytes.NewReader(tampered), JSONLimits{}); e == nil {
			t.Fatal("tangent source accepted")
		}
		if _, e := restored.Bounds(); e != nil {
			t.Fatal(e)
		}
		m, e := restored.Tessellate(ctx, TessellationOptions{Deflection: .5, AnalyticBound: true})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		if m.SignedVolume() <= 0 || math.Abs(m.SignedVolume()-p.Volume) > p.Volume*.04 {
			t.Fatal("mesh mass", m.SignedVolume(), p.Volume)
		}
		report, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
		if e != nil || !report.Complete || len(report.Contacts) != 0 {
			t.Fatal("display contacts", len(report.Contacts), e)
		}
		t.Log(op, len(m.Triangles), p.Volume, p.Area)
	}
	if _, e := NewTorusCylinderSet(ctx, 5, 2, 7, -3, 4, geom.TorusCylinderCommon, 1e-9); e == nil {
		t.Fatal("tangent")
	}
	if _, e := NewTorusCylinderSet(ctx, 5, 2, 6, -3, 4, geom.TorusCylinderUnion, 1e-9); e == nil {
		t.Fatal("unsupported union")
	}
}

func TestNativeTorusCylinderRationalMeridianIdentity(t *testing.T) {
	r := big.NewRat
	for _, sigma := range []int64{-1, 1} {
		K := new(big.Rat).Quo(new(big.Rat).Sub(r(2, 1), r(sigma, 1)), new(big.Rat).Add(r(2, 1), r(sigma, 1)))
		for _, v := range []*big.Rat{r(-1, 1), r(-2, 7), r(0, 1), r(3, 5), r(1, 1)} {
			kv := new(big.Rat).Mul(K, new(big.Rat).Mul(v, v))
			den := new(big.Rat).Add(r(1, 1), kv)
			dr := new(big.Rat).Quo(new(big.Rat).Mul(r(2*sigma, 1), new(big.Rat).Sub(r(1, 1), kv)), den)
			z2 := new(big.Rat).Quo(new(big.Rat).Mul(r(16, 1), kv), new(big.Rat).Mul(den, den))
			identity := new(big.Rat).Add(new(big.Rat).Mul(dr, dr), z2)
			if identity.Cmp(r(4, 1)) != 0 {
				t.Fatal("original torus identity")
			}
			if new(big.Rat).Abs(v).Cmp(r(1, 1)) == 0 && (dr.Cmp(r(1, 1)) != 0 || z2.Cmp(r(3, 1)) != 0) {
				t.Fatal("exact common model seam")
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := NewTorusCylinderSet(ctx, 5, 2, 6, -3, 4, geom.TorusCylinderCommon, 1e-9); e == nil {
		t.Fatal("cancel")
	}
}

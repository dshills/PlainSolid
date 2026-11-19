package step

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func curvedOvalFixture(t testing.TB) CurvedOvalFace {
	t.Helper()
	p := []geom.Vec3{}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			z := []float64{.25, -.25, .25}[i] + []float64{.25, -.25, .25}[j]
			p = append(p, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: 3 * z})
		}
	}
	s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	return CurvedOvalFace{Name: "curved rational oval", Geometry: s, Outer: QuadraticOval{Center: geom.Vec2{X: .5, Y: .5}, Radii: geom.Vec2{X: 3. / 16, Y: 3. / 16}, QuarterWeight: .5}, LengthUnit: .001}
}
func TestCurvedOvalComposition(t *testing.T) {
	t.Parallel()
	o, _ := (CurvedOvalOptions{}).defaults()
	_, curves, e := certifyCurvedOval(curvedOvalFixture(t), o)
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range curves {
		p, w, e := exactBezierFloats(c, o.patch())
		if e != nil {
			t.Fatal(e)
		}
		if len(p) != 5 || len(w) != 5 {
			t.Fatal(len(p), len(w))
		}
	}
}
func TestCurvedOvalFaceRoundtrip(t *testing.T) {
	t.Parallel()
	m := curvedOvalFixture(t)
	var b bytes.Buffer
	if e := WriteCurvedOvalFace(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	got, e := ReadCurvedOvalFace(bytes.NewReader(b.Bytes()), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if got.Name != m.Name || got.Outer != m.Outer || got.LengthUnit != m.LengthUnit {
		t.Fatal(got)
	}
	if _, e = ReadQuadraticOvalFaces(bytes.NewReader(b.Bytes()), Limits{}); e == nil {
		t.Fatal("affine reader broadened")
	}
	for _, c := range got.ModelCurves {
		net, _, e := c.BezierControlNet(context.Background(), geom.SurfacePatchOptions{})
		if e != nil || len(net.Control) != 5 {
			t.Fatal(e)
		}
	}
}

func TestCurvedOvalRationalSupport(t *testing.T) {
	t.Parallel()
	m := curvedOvalFixture(t)
	d := m.Geometry.Data()
	wu := []float64{1, 1.5, 2}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			k := 3*i + j
			d.Poles[k].X = []float64{0, 1, 3}[i]
			d.Poles[k].Z /= wu[i]
		}
	}
	weights := []float64{}
	for _, w := range wu {
		for j := 0; j < 3; j++ {
			weights = append(weights, w)
		}
	}
	var e error
	m.Geometry, e = geom.NewBSplineSurface(2, 2, 3, 3, d.Poles, d.KnotsU, d.KnotsV, weights)
	if e != nil {
		t.Fatal(e)
	}
	o, _ := (CurvedOvalOptions{}).defaults()
	_, curves, e := certifyCurvedOval(m, o)
	if e != nil {
		t.Fatal(e)
	}
	scale := big.NewInt(1)
	for _, c := range curves {
		net, _, e := c.BezierControlNet(context.Background(), o.patch())
		if e != nil {
			t.Fatal(e)
		}
		for _, h := range net.Control {
			for j := 0; j < 3; j++ {
				den := new(big.Int).Set(new(big.Rat).Quo(h[j], h[3]).Denom())
				for den.Bit(0) == 0 {
					den.Rsh(den, 1)
				}
				g := new(big.Int).GCD(nil, nil, scale, den)
				scale.Mul(new(big.Int).Quo(scale, g), den)
			}
		}
	}
	t.Log("exact coordinate scale", scale, scale.BitLen())
	for i, p := range d.Poles {
		v := []float64{p.X, p.Y, p.Z}
		for j := range v {
			v[j], e = geom.RepresentedExactScalar(new(big.Rat).Mul(new(big.Rat).SetFloat64(v[j]), new(big.Rat).SetInt(scale)))
			if e != nil {
				t.Fatal(e)
			}
		}
		d.Poles[i] = geom.Vec3{X: v[0], Y: v[1], Z: v[2]}
	}
	m.Geometry, e = geom.NewBSplineSurface(2, 2, 3, 3, d.Poles, d.KnotsU, d.KnotsV, weights)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteCurvedOvalFace(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadCurvedOvalFace(&b, Limits{}, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
}

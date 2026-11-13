package step

import (
	"bytes"
	"math"
	"math/big"
	"os"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestRationalLensUnnormalizedWeights(t *testing.T) {
	raw, e := os.ReadFile("testdata/partitioned-spline-lens.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadRationalSplineLensBody(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Curves[0][0].Weights[2] != 5 {
		t.Fatal("source weights normalized")
	}
	var b bytes.Buffer
	if e = WriteRationalSplineLensBody(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	got, e := ReadRationalSplineLensBody(&b, Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(m.Curves, got.Curves) {
		t.Fatal("raw source basis changed")
	}
	if _, e = ReadRefinedSplineLensBody(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{}); e == nil {
		t.Fatal("normalizing API broadened")
	}
}
func rationalConcurrentLensFixture(t testing.TB) RationalSplineLens {
	t.Helper()
	m := splineLensFixture(t)
	r := 263340. / (1 << 22)
	m.Radii = geom.Vec2{X: r, Y: r}
	sq := []float64{.25, -.25, .25}
	for side, sign := range []float64{1, -1} {
		p := []geom.Vec3{}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				p = append(p, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: 2 + sign*(r*r-sq[i]-sq[j])/2})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		patch, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if side == 0 {
			patch, e = patch.InsertVKnot(.5+4*r/5, 1, geom.SurfacePatchOptions{})
		} else {
			patch, e = patch.InsertUKnot(.5+3*r/5, 1, geom.SurfacePatchOptions{})
		}
		if e != nil {
			t.Fatal(e)
		}
		s, e = patch.ToBSplineSurface()
		if e != nil {
			t.Fatal(e)
		}
		if side == 0 {
			m.Upper = s
		} else {
			m.Lower = s
		}
	}
	o, _ := (CurvedOvalOptions{}).defaults()
	pieces, e := lensPiecePlan(m, o)
	if e != nil {
		t.Fatal(e)
	}
	out := RationalSplineLens{PartitionedSplineLens: PartitionedSplineLens{Body: m}}
	for index, p := range pieces {
		var cs [3]RationalLensCurve
		for role := 0; role < 3; role++ {
			curve := p.UV
			if role == 0 {
				curve = p.Model
			}
			ps, ws, e := exactBezierFloats(curve, o.patch())
			if e != nil {
				t.Fatal(e)
			}
			h := make([][4]*big.Rat, len(ps))
			for i, p := range ps {
				w := new(big.Rat).SetFloat64(ws[i])
				h[i] = [4]*big.Rat{new(big.Rat).Mul(new(big.Rat).SetFloat64(p.X), w), new(big.Rat).Mul(new(big.Rat).SetFloat64(p.Y), w), new(big.Rat).Mul(new(big.Rat).SetFloat64(p.Z), w), w}
			}

			knots := []float64{0, 0, 0, 1, 1, 1}
			if role == index%3 {
				mix := func(a, b [4]*big.Rat) (v [4]*big.Rat) {
					for i := range v {
						v[i] = new(big.Rat).Quo(new(big.Rat).Add(a[i], b[i]), big.NewRat(2, 1))
					}
					return
				}
				h = [][4]*big.Rat{h[0], mix(h[0], h[1]), mix(h[1], h[2]), h[2]}
				knots = []float64{0, 0, 0, .5, 1, 1, 1}
			}
			c := RationalLensCurve{Degree: 2, Knots: knots}
			for _, v := range h {
				var xyz [3]float64
				for k := range xyz {
					xyz[k], e = geom.RepresentedExactScalar(new(big.Rat).Quo(v[k], v[3]))
					if e != nil {
						t.Fatal("fixture pole", index, role, e)
					}
				}
				w, e := geom.RepresentedExactScalar(v[3])
				if e != nil {
					t.Fatal("fixture weight", index, role, e)
				}
				c.Poles = append(c.Poles, geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]})
				c.Weights = append(c.Weights, w)
			}
			cs[role] = c
		}
		out.Curves = append(out.Curves, cs)
	}
	return out
}
func TestConcurrentRationalLens(t *testing.T) {
	m := rationalConcurrentLensFixture(t)
	if len(m.Curves) != 7 {
		t.Fatal(len(m.Curves))
	}
	var b bytes.Buffer
	if e := WriteRationalSplineLensBody(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	got, e := ReadRationalSplineLensBody(&b, Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(m.Curves, got.Curves) || !reflect.DeepEqual(m.Body.Upper.Data(), got.Body.Upper.Data()) || !reflect.DeepEqual(m.Body.Lower.Data(), got.Body.Lower.Data()) {
		t.Fatal("concurrent source bases changed")
	}
}
func TestRationalLensPoleChange(t *testing.T) {
	for role := 0; role < 3; role++ {
		m := rationalConcurrentLensFixture(t)
		m.Curves[0][role].Poles[1].X = math.Nextafter(m.Curves[0][role].Poles[1].X, math.Inf(1))
		var b bytes.Buffer
		if e := WriteRationalSplineLensBody(&b, m, CurvedOvalOptions{}); e == nil || b.Len() != 0 {
			t.Fatal("rounded curve accepted", role, e)
		}
	}
}
func TestRationalLensInvalidWeightsAndBudgets(t *testing.T) {
	for _, weight := range []float64{0, -1, math.Inf(1), math.NaN()} {
		m := rationalConcurrentLensFixture(t)
		m.Curves[0][0].Weights[0] = weight
		var b bytes.Buffer
		if e := WriteRationalSplineLensBody(&b, m, CurvedOvalOptions{}); e == nil || b.Len() != 0 {
			t.Fatal("invalid weight", weight, e)
		}
	}
	m := rationalConcurrentLensFixture(t)
	var b bytes.Buffer
	if e := WriteRationalSplineLensBody(&b, m, CurvedOvalOptions{MaxOperations: 64}); e == nil || b.Len() != 0 {
		t.Fatal("operation budget", e)
	}
	if e := WriteRationalSplineLensBody(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadRationalSplineLensBody(bytes.NewReader(b.Bytes()), Limits{MaxEdgeUses: 13}, CurvedOvalOptions{}); e == nil {
		t.Fatal("source edge budget")
	}
}

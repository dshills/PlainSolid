package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// Each original graph side is sewn continuously, including nonconstant
// rational weights. The first roof is (2+x²)/(1+x); the second is below
// it and fully inside the first box, so their retained boundaries are known
// without an approximate intersection curve.
func weightedIdentityRoofStock(t *testing.T, tool bool, degree int) Solid {
	return weightedIdentityRoofStockWithKnots(t, tool, degree, false)
}

func weightedIdentityRoofStockWithKnots(t *testing.T, tool bool, degree int, insertKnots bool) Solid {
	return weightedIdentityRoofStockMode(t, tool, degree, insertKnots, false)
}

func weightedIdentityRoofStockMode(t *testing.T, tool bool, degree int, insertKnots, crease bool) Solid {
	t.Helper()
	ctx := context.Background()
	r := big.NewRat
	x0, x1, y0, y1, floor := 0., 1., 0., 1., 0.
	if tool {
		x0, x1, y0, y1, floor = .125, .875, .125, .875, .25
	}
	h := func(x float64) *big.Rat {
		if crease && !tool {
			return r(2, 1) // original roof edge at x=0 or x=1
		}
		if tool {
			return new(big.Rat).Add(r(7, 8), new(big.Rat).Quo(ratInset(x), r(4, 1)))
		}
		z := ratInset(x)
		return new(big.Rat).Add(r(2, 1), new(big.Rat).Mul(z, z))
	}
	weight := func(x float64) *big.Rat {
		if tool || crease {
			return r(1, 1)
		}
		return new(big.Rat).Add(r(1, 1), ratInset(x))
	}
	parts := []SewFace{}
	makeFace := func(d geom.ParameterRectangle, reverse bool, point func(u, v int) [4]*big.Rat) {
		var controls [][4]*big.Rat
		for i := 0; i < 3; i++ {
			for j := 0; j < 2; j++ {
				controls = append(controls, point(i, j))
			}
		}
		degreeU, degreeV := 2, 1
		if degree >= 3 {
			// Exact degree elevation preserves the original rational
			// walls and floor while allowing an independent bicubic roof.
			raised := make([][4]*big.Rat, 16)
			for i := 0; i < 4; i++ {
				for axis := 0; axis < 4; axis++ {
					l, h := new(big.Rat), new(big.Rat)
					if i > 0 {
						l.Mul(r(int64(i), 3), controls[(i-1)*2][axis])
						h.Mul(r(int64(i), 3), controls[(i-1)*2+1][axis])
					}
					if i < 3 {
						l.Add(l, new(big.Rat).Mul(r(int64(3-i), 3), controls[i*2][axis]))
						h.Add(h, new(big.Rat).Mul(r(int64(3-i), 3), controls[i*2+1][axis]))
					}
					for j := 0; j < 4; j++ {
						raised[i*4+j][axis] = new(big.Rat).Add(new(big.Rat).Mul(r(int64(3-j), 3), l), new(big.Rat).Mul(r(int64(j), 3), h))
					}
				}
			}
			// This Bernstein term contributes only in the roof interior:
			// (1/64)*u²(1-u)*v²(1-v), with no change to the four edges.
			if len(parts) == 1 && !tool {
				raised[2*4+2][2].Add(raised[2*4+2][2], r(1, 576))
			}
			controls = raised
			degreeU, degreeV = 3, 3
		}
		if degree == 4 && len(parts) == 1 && !tool {
			raised := make([][4]*big.Rat, 25)
			for i := 0; i < 5; i++ {
				for j := 0; j < 5; j++ {
					for v := 0; v < 4; v++ {
						z := new(big.Rat)
						for a := max(0, i-1); a <= min(3, i); a++ {
							for b := max(0, j-1); b <= min(3, j); b++ {
								// Cubic-to-quartic Bernstein degree elevation in each axis.
								wu, wv := 4-i, 4-j
								if a != i {
									wu = i
								}
								if b != j {
									wv = j
								}
								z.Add(z, new(big.Rat).Mul(r(int64(wu*wv), 16), controls[a*4+b][v]))
							}
						}
						raised[i*5+j][v] = z
					}
				}
			}
			// Add u³(1-u)v³(1-v)/256 without changing the four sewn edges.
			raised[3*5+3][2].Add(raised[3*5+3][2], r(1, 4096))
			controls, degreeU, degreeV = raised, 4, 4
		}
		p, err := geom.NewExactHomogeneousBezierSurface(ctx, degreeU, degreeV, controls, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		p, err = p.Reparameterize(d, false, false, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		var s geom.Surface
		if insertKnots {
			p, err = p.InsertUKnot(d.UMin+.25*(d.UMax-d.UMin), 1, geom.SurfacePatchOptions{})
			if err != nil {
				t.Fatal(err)
			}
			p, err = p.InsertVKnot(d.VMin+.75*(d.VMax-d.VMin), 1, geom.SurfacePatchOptions{})
			if err != nil {
				t.Fatal(err)
			}
			s, err = geom.NewExactTensorSurface(ctx, p, geom.SurfacePatchOptions{})
		} else {
			s, err = geom.NewExactBezierSurface(p)
		}
		if crease && !tool && len(parts) != 0 {
			// The horizontal roof slopes +1 then -1 at x=1/2. Only
			// the roof and the two y-side top curves need a knot.
			id := len(parts)
			if id == 1 || id >= 4 {
				var poles []geom.Vec3
				for i, x := range []float64{0, .5, 1} {
					for j := range 2 {
						z := []float64{2, 2.5, 2}[i]
						v := geom.Vec3{X: x, Y: float64(j), Z: z}
						if id >= 4 {
							v.Y, v.Z = float64(id-4), float64(j)*z
						}
						poles = append(poles, v)
					}
				}
				bs, e := geom.NewBSplineSurface(1, 1, 3, 2, poles, []float64{0, 0, .5, 1, 1}, []float64{0, 0, 1, 1}, nil)
				if e != nil {
					t.Fatal(e)
				}
				p, e = geom.NewSurfacePatch(bs, geom.SurfacePatchOptions{})
				if e != nil {
					t.Fatal(e)
				}
				s = bs
			}
		}
		if err != nil {
			t.Fatal(err)
		}
		f := SewFace{Face: Face{Surface: s, Reversed: reverse}}
		uv := [4]geom.Vec2{{X: d.UMin, Y: d.VMin}, {X: d.UMax, Y: d.VMin}, {X: d.UMax, Y: d.VMax}, {X: d.UMin, Y: d.VMax}}
		for i, a := range uv {
			b := uv[(i+1)%4]
			j, err := p.EvaluateRational(ctx, ratInset(a.X), ratInset(a.Y), geom.SurfacePatchEvaluationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			v, err := geom.RepresentedExactPoint(j.Point)
			if err != nil {
				t.Fatal(err)
			}
			f.Vertices = append(f.Vertices, v)
			f.Edges = append(f.Edges, parametric.Edge{Start: i, End: (i + 1) % 4, Curve: surfaceTrimCurve{s, a, b, nil}, Range: parametric.Interval{Max: 1}})
			f.Face.Outer = append(f.Face.Outer, Use{Edge: i, UVStart: a, UVEnd: b})
		}
		parts = append(parts, f)
	}
	// For weighted entries, homogeneous x=(1+x)x and w=1+x.
	// Convert each quadratic by its physical endpoint and midpoint samples.
	quad := func(a, b float64, i int, f func(float64) *big.Rat) *big.Rat {
		if i == 0 {
			return f(a)
		}
		if i == 2 {
			return f(b)
		}
		return new(big.Rat).Sub(new(big.Rat).Mul(r(2, 1), f((a+b)/2)), new(big.Rat).Mul(r(1, 2), new(big.Rat).Add(f(a), f(b))))
	}
	span := geom.ParameterRectangle{UMin: x0, UMax: x1, VMin: y0, VMax: y1}
	y := []float64{y0, y1}
	for _, reverse := range []bool{true, false} {
		makeFace(span, reverse, func(i, j int) [4]*big.Rat {
			w := quad(x0, x1, i, weight)
			x := quad(x0, x1, i, func(x float64) *big.Rat { return new(big.Rat).Mul(ratInset(x), weight(x)) })
			z := new(big.Rat).Mul(ratInset(floor), w)
			if !reverse {
				z = quad(x0, x1, i, h)
			}
			return [4]*big.Rat{x, new(big.Rat).Mul(ratInset(y[j]), w), z, w}
		})
	}
	for k, x := range []float64{x0, x1} {
		makeFace(geom.ParameterRectangle{UMin: y0, UMax: y1, VMax: 1}, k == 0, func(i, j int) [4]*big.Rat {
			w := weight(x)
			z := new(big.Rat).Mul(ratInset(floor), w)
			if j == 1 {
				z = h(x)
			}
			return [4]*big.Rat{new(big.Rat).Mul(ratInset(x), w), new(big.Rat).Mul(ratInset(y0+(y1-y0)*float64(i)/2), w), z, w}
		})
	}
	for k, y := range []float64{y0, y1} {
		makeFace(geom.ParameterRectangle{UMin: x0, UMax: x1, VMax: 1}, k == 1, func(i, j int) [4]*big.Rat {
			w := quad(x0, x1, i, weight)
			x := quad(x0, x1, i, func(x float64) *big.Rat { return new(big.Rat).Mul(ratInset(x), weight(x)) })
			z := new(big.Rat).Mul(ratInset(floor), w)
			if j == 1 {
				z = quad(x0, x1, i, h)
			}
			return [4]*big.Rat{x, new(big.Rat).Mul(ratInset(y), w), z, w}
		})
	}
	s, _, err := Sew(ctx, parts, SewOptions{Tolerance: 1e-10})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestOriginalBicubicWeightedRoofMaterialLine(t *testing.T) {
	ctx := context.Background()
	a, b := weightedIdentityRoofStock(t, false, 3), weightedIdentityRoofStock(t, true, 3)
	sec, err := SectionSplineShellsAlgebraic(ctx, a, b, AlgebraicShellSectionOptions{})
	if err != nil || !sec.Complete {
		t.Fatal("complete bicubic source section", sec.Coverage, err)
	}
	m, err := sec.CombineAlgebraic(ctx, parametric.Difference, AlgebraicShellBooleanOptions{})
	if err != nil {
		t.Fatal("bicubic cavity", err)
	}
	q, err := m.PrepareRoofMaterialQuery(ctx, MixedRoofQueryOptions{})
	if err != nil {
		t.Fatal("prepare bicubic source", err)
	}
	if limited, err := m.PrepareRoofMaterialQuery(ctx, MixedRoofQueryOptions{MaxPreparationOperations: q.PreparationOperations() - 1}); !errors.Is(err, geom.ErrConvergence) || limited != nil {
		t.Fatal("bicubic coefficient conversion escaped the preparation limit", limited, err)
	}
	r := big.NewRat
	origin, direction := [3]*big.Rat{r(0, 1), r(0, 1), r(3, 2)}, [3]*big.Rat{r(1, 1), r(1, 1), r(1, 4)}
	section, err := q.IntersectLineRational(ctx, origin, direction, r(1, 8), r(7, 8), MixedRoofLineOptions{})
	if err != nil || len(section.Events) != 1 || len(section.Spans) != 1 || section.Events[0].Before != parametric.Inside || section.Events[0].After != parametric.Outside ||
		len(section.Events[0].Owners) != 1 || section.Events[0].Owners[0] != (MixedRoofFaceOwner{0, 1}) {
		t.Fatal("bicubic line sweep", section, err)
	}
	// Multiply the actual original rational height equation by 64(1+t).
	// 32 -112 t +48t² +t⁴-2t⁵+t⁶ = 0.
	coeff := []*big.Rat{r(32, 1), r(-112, 1), r(48, 1), r(0, 1), r(1, 1), r(-2, 1), r(1, 1)}
	sign, err := section.Events[0].Parameter.SignPolynomial(ctx, coeff, geom.ExactCurvePlaneEventOptions{})
	if err != nil || sign.Sign != 0 {
		t.Fatal("sextic restriction lost its original identity", sign, err)
	}
	old, _, err := geom.NewExactRationalRoot(ctx, r(1, 3), geom.ExactCurvePlaneEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	order, err := section.Events[0].Parameter.Compare(ctx, old, geom.ExactCurvePlaneEventOptions{})
	if err != nil || !order.Proven || order.Order != 1 {
		t.Fatal("bicubic contact rounded into old quadratic", order, err)
	}
}

func TestOriginalBiquarticWeightedRoofMaterialLine(t *testing.T) {
	ctx := context.Background()
	a, b := weightedIdentityRoofStock(t, false, 4), weightedIdentityRoofStock(t, true, 2)
	sec, err := SectionSplineShellsAlgebraic(ctx, a, b, AlgebraicShellSectionOptions{})
	if err != nil || !sec.Complete {
		t.Fatal("complete biquartic original section", sec.Coverage, err)
	}
	m, err := sec.CombineAlgebraic(ctx, parametric.Difference, AlgebraicShellBooleanOptions{})
	if err != nil {
		t.Fatal("biquartic original cavity", err)
	}
	q, err := m.PrepareRoofMaterialQuery(ctx, MixedRoofQueryOptions{})
	if err != nil {
		t.Fatal("prepare biquartic material", err)
	}
	r := big.NewRat
	origin, direction := [3]*big.Rat{r(0, 1), r(0, 1), r(3, 2)}, [3]*big.Rat{r(1, 1), r(1, 1), r(1, 4)}
	line, err := q.IntersectLineRational(ctx, origin, direction, r(1, 8), r(7, 8), MixedRoofLineOptions{})
	if err != nil || len(line.Events) != 1 || len(line.Spans) != 1 || line.Events[0].Before != parametric.Inside || line.Events[0].After != parametric.Outside ||
		len(line.Events[0].Owners) != 1 || line.Events[0].Owners[0] != (MixedRoofFaceOwner{0, 1}) {
		t.Fatal("original biquartic line", line, err)
	}
	// Multiplication by 256(1+t) gives four times the sextic plus t⁶(1-t)².
	coeff := []*big.Rat{r(128, 1), r(-448, 1), r(192, 1), r(0, 1), r(4, 1), r(-8, 1), r(5, 1), r(-2, 1), r(1, 1)}
	sign, err := line.Events[0].Parameter.SignPolynomial(ctx, coeff, geom.ExactCurvePlaneEventOptions{})
	if err != nil || sign.Sign != 0 {
		t.Fatal("degree-eight roof station lost the original identity", sign, err)
	}
	old, err := line.Events[0].Parameter.SignPolynomial(ctx, []*big.Rat{r(32, 1), r(-112, 1), r(48, 1), r(0, 1), r(1, 1), r(-2, 1), r(1, 1)}, geom.ExactCurvePlaneEventOptions{})
	if err != nil || old.Sign != -1 {
		t.Fatal("degree-eight contact rounded to the former sextic", old, err)
	}
	volume, err := q.BoundVolume(ctx, MixedRoofVolumeOptions{AbsoluteGap: r(1, 20)})
	if err != nil || !volume.Converged || volume.Volume.Lower.Cmp(r(115, 100)) > 0 || volume.Volume.Upper.Cmp(r(116, 100)) < 0 {
		t.Fatal("biquartic volume did not cover the independently bracketed cavity", volume, err)
	}
}

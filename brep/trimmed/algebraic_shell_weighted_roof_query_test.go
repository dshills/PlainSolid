package trimmed

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// Each original graph side is sewn continuously, including nonconstant
// rational weights. The first roof is (2+x²)/(1+x); the second is below
// it and fully inside the first box, so their retained boundaries are known
// without an approximate intersection curve.
func weightedIdentityRoofStock(t *testing.T, tool, cubic bool) Solid {
	t.Helper()
	ctx := context.Background()
	r := big.NewRat
	x0, x1, y0, y1, floor := 0., 1., 0., 1., 0.
	if tool {
		x0, x1, y0, y1, floor = .125, .875, .125, .875, .25
	}
	h := func(x float64) *big.Rat {
		if tool {
			return new(big.Rat).Add(r(7, 8), new(big.Rat).Quo(ratInset(x), r(4, 1)))
		}
		z := ratInset(x)
		return new(big.Rat).Add(r(2, 1), new(big.Rat).Mul(z, z))
	}
	weight := func(x float64) *big.Rat {
		if tool {
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
		if cubic {
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
		p, err := geom.NewExactHomogeneousBezierSurface(ctx, degreeU, degreeV, controls, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		p, err = p.Reparameterize(d, false, false, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		s, err := geom.NewExactBezierSurface(p)
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

func TestOriginalWeightedRoofMaterialQueries(t *testing.T) {
	ctx := context.Background()
	a, b := weightedIdentityRoofStock(t, false, false), weightedIdentityRoofStock(t, true, false)
	// Test the complete original section and regularized result; a caller
	// cannot construct a query by asserting its own solid envelopes.
	sec, err := SectionSplineShellsAlgebraic(ctx, a, b, AlgebraicShellSectionOptions{})
	if err != nil || !sec.Complete {
		t.Fatal("weighted original source section", sec.Coverage, err)
	}
	m, err := sec.CombineAlgebraic(ctx, parametric.Difference, AlgebraicShellBooleanOptions{})
	if err != nil {
		t.Fatal("weighted cavity", err)
	}
	q, err := m.PrepareRoofMaterialQuery(ctx, MixedRoofQueryOptions{})
	if err != nil {
		t.Fatal("prepare weighted roof", err)
	}
	if q.box[0].minWeight == nil {
		t.Fatal("nonconstant original weight was discarded")
	}
	r := big.NewRat
	delta := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	p := [3]*big.Rat{r(1, 2), r(1, 2), new(big.Rat).Add(r(3, 2), delta)}
	if loc, err := q.LocateRational(ctx, p, geom.SurfacePatchEvaluationOptions{}); err != nil || loc.Location != parametric.Outside {
		t.Fatal("sub-binary weighted roof membership", loc, err)
	}
	// z=3/2+X/4 gives 3X²-7X+2=0, retaining X=1/3 inside
	// the footprint. The denominator remains strictly positive.
	line, err := q.IntersectLineRational(ctx, [3]*big.Rat{r(0, 1), r(1, 2), r(3, 2)}, [3]*big.Rat{r(1, 1), r(0, 1), r(1, 4)}, r(1, 8), r(7, 8), MixedRoofLineOptions{})
	if err != nil || len(line.Events) != 1 || len(line.Spans) != 1 || len(line.Events[0].Owners) != 1 || line.Events[0].Owners[0] != (MixedRoofFaceOwner{0, 1}) {
		t.Fatal("weighted roof line", line, err)
	}
	k, err := line.Events[0].Parameter.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
	if err != nil || k.Kind != geom.ExactRootKindRational || k.Rational.Cmp(r(1, 3)) != 0 {
		t.Fatal(k, err)
	}
	if program := os.Getenv("PLAINSOLID_OCCT_WEIGHTED_ROOF"); program != "" {
		out, err := exec.CommandContext(ctx, program, "weighted-line").Output()
		var ref struct{ Parameter float64 }
		if err != nil {
			t.Fatal("independent weighted roof oracle", err)
		}
		want, _ := k.Rational.Float64()
		if err := json.Unmarshal(out, &ref); err != nil || !finite(ref.Parameter) || math.Abs(ref.Parameter-want) > 1e-10 {
			t.Fatal("weighted line disagrees with OCCT's original sewn surfaces", ref, want, err)
		}
	}
	p[2].SetFrac64(3, 2)
	n, err := q.BoundaryNormalsRational(ctx, p, geom.SurfacePatchEvaluationOptions{})
	if err != nil || len(n.Normals) != 1 || n.Normals[0].Direction[0].Cmp(r(1, 3)) != 0 {
		t.Fatal("weighted quotient derivative lost", n, err)
	}
	dist, err := q.BoundBoundaryDistanceRational(ctx, [3]*big.Rat{r(1, 2), r(1, 2), r(-1, 8)}, MixedRoofDistanceOptions{})
	if err != nil || !dist.Converged || dist.Squared.Lower.Cmp(r(1, 64)) != 0 || dist.Squared.Upper.Cmp(r(1, 64)) != 0 ||
		len(dist.Owners) != 1 || dist.Owners[0] != (MixedRoofFaceOwner{0, 0}) {
		t.Fatal("weighted stock distance lost the original floor witness", dist, err)
	}
	if _, err := proveMixedRoofBox(ctx, a, 1, 1000000); !errors.Is(err, geom.ErrUnrepresentable) {
		t.Fatal("polynomial-only mesh proof accepted rational height", err)
	}
}

func TestOriginalBicubicWeightedRoofMaterialLine(t *testing.T) {
	ctx := context.Background()
	a, b := weightedIdentityRoofStock(t, false, true), weightedIdentityRoofStock(t, true, true)
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

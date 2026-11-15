package geom

import (
	"context"
	"math/big"
)

// SplineGraphPolynomialNode proves the complete contact set consists of two
// cubic or quartic graphs crossing once at an interior rational point. Power
// and Controls are ordered lower then upper at the low free-coordinate end.
// Terminals 2*k and 2*k+1 are the low and high free-coordinate ends of branch k.
// Tangents use original UV and point toward increasing free coordinate. The
// crossing is transverse in UV and in the one-sided projected charts, including
// at source knots. Continuity of the lifted model tangents is not assumed.
type SplineGraphPolynomialNode struct {
	Proven          bool
	Domain          ParameterRectangle
	FreeAxis        int
	Power           [2][]*big.Rat
	Controls        [2][][2]*big.Rat
	TerminalMasks   [2]uint8
	Parameters      [2]*big.Rat
	Point           [3]*big.Rat
	Terminals       [4][2]*big.Rat
	Tangents        [2][2]*big.Rat
	RayCycle        [4]int // counterclockwise in model XY, indexed by terminal
	Sectors         [4]GraphSaddleSector
	CertifiedCells  []ParameterRectangle
	Graph           SurfaceInjectivity
	ExactOperations int
}

// DiscoverSplineGraphPolynomialNode verifies both complete polynomial factors
// on every original union-knot cell. Exact root isolation and rational recovery
// prove the sole branch intersection is interior and simple. Endpoint, tangent,
// irrational and additional intersections are unresolved. Failure is atomic.
func DiscoverSplineGraphPolynomialNode(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphPolynomialNode, error) {
	var node SplineGraphPolynomialNode
	r, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, &node, nil)
	if e != nil {
		return SplineGraphPolynomialNode{ExactOperations: r.ExactOperations}, e
	}
	node.ExactOperations = r.ExactOperations
	node.Proven = true
	return node, nil
}

func discoverGraphPolynomialCrossings(ctx context.Context, delta rationalPolynomial, count, maxCells int, visits *int, b *polynomialBudget) (roots [2]*big.Rat, order int, err error) {
	if len(delta) < 2 || *visits >= maxCells {
		return roots, 0, ErrConvergence
	}
	zero, one := new(big.Rat), big.NewRat(1, 1)
	end, e := polynomialValue(delta, one, b)
	if e != nil {
		return roots, 0, e
	}
	order = delta[0].Sign()
	if count == 1 && order*end.Sign() != -1 {
		return roots, 0, ErrConvergence
	}
	if order == 0 || end.Sign() == 0 {
		return roots, 0, ErrDomain
	}
	var minimumWidth *big.Rat
	found := 0
	degenerate, irrational := ErrConvergence, ErrConvergence
	if count == 2 {
		degenerate, irrational = ErrDegenerate, ErrUnrepresentable
	}
	splits, e := isolatePolynomialRoots(delta, zero, one, b, (maxCells-*visits-1)/2, func(lo, hi *big.Rat, multiplicity int) (bool, error) {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		if multiplicity != 1 {
			return false, degenerate
		}
		if found == count {
			return false, ErrConvergence
		}
		x, e := recoverRationalPolynomialRoot(delta, lo, hi, b)
		if e != nil {
			return false, e
		}
		if x == nil {
			if len(delta) <= 3 {
				return false, irrational
			}
			if minimumWidth == nil {
				minimumWidth, e = graphRationalRootWidth(delta, b)
				if e != nil {
					return false, e
				}
			}
			width := new(big.Rat).Sub(hi, lo)
			if e := b.check(width); e != nil {
				return false, e
			}
			if width.Cmp(minimumWidth) < 0 {
				return false, irrational
			}
			return false, nil
		}
		if x.Sign() <= 0 || x.Cmp(one) >= 0 || found > 0 && x.Cmp(roots[found-1]) <= 0 {
			return false, ErrConvergence
		}
		roots[found] = x
		found++
		return true, nil
	})
	*visits += 1 + 2*splits
	if e != nil {
		return [2]*big.Rat{}, 0, e
	}
	if found != count {
		return [2]*big.Rat{}, 0, ErrConvergence
	}
	return roots, order, nil
}

// Below this width the midpoint's convergents include any rational root in the
// interval. By the rational root theorem the denominator divides the leading
// integer coefficient after clearing denominators. The midpoint error is less
// than half the inverse square of that coefficient.
func graphRationalRootWidth(p rationalPolynomial, b *polynomialBudget) (*big.Rat, error) {
	common := big.NewInt(1)
	for _, x := range p {
		d := x.Denom()
		gcd := new(big.Int).GCD(nil, nil, common, d)
		common.Quo(common, gcd).Mul(common, d)
		if e := b.check(new(big.Rat).SetInt(common)); e != nil {
			return nil, e
		}
	}
	lead := p[len(p)-1]
	common.Quo(common, lead.Denom()).Mul(common, lead.Num())
	width := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Mul(common, common))
	return width, b.check(width)
}

func (n *SplineGraphPolynomialNode) certifySectors(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, free *big.Rat, d ParameterRectangle, b *polynomialBudget) error {
	n.Domain, n.FreeAxis = pair.Domain, pair.FreeAxis
	n.Power, n.Controls = pair.Power, pair.Controls
	n.TerminalMasks, n.CertifiedCells, n.Graph = pair.TerminalMasks, pair.CertifiedCells, pair.Graph
	uv := func(free, dependent *big.Rat) [2]*big.Rat {
		if n.FreeAxis == 1 {
			return [2]*big.Rat{dependent, free}
		}
		return [2]*big.Rat{free, dependent}
	}
	dependent, e := polynomialValue(n.Power[0], free, b)
	if e != nil {
		return e
	}
	n.Parameters = uv(free, dependent)
	for k, power := range n.Power {
		slope, e := polynomialValue(polynomialDerivative(power), free, b)
		if e != nil {
			return e
		}
		n.Tangents[k] = uv(big.NewRat(1, 1), slope)
		controls := n.Controls[k]
		for j, end := range [2][2]*big.Rat{controls[0], controls[len(controls)-1]} {
			n.Terminals[2*k+j] = [2]*big.Rat{new(big.Rat).Set(end[0]), new(big.Rat).Set(end[1])}
		}
	}
	remaining := b.maxOperations - b.operations
	if remaining < 1 {
		return ErrConvergence
	}
	point, e := a.EvaluateRational(ctx, n.Parameters[0], n.Parameters[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: b.maxBits})
	b.operations += point.ExactOperations
	if e != nil {
		return e
	}
	n.Point = point.Point
	return n.buildSectorBoundaries(d, pair.StripSigns[0], b)
}

func (n *SplineGraphPolynomialNode) buildSectorBoundaries(d ParameterRectangle, sign int, b *polynomialBudget) error {
	// The branch ordering reverses at the node. In (free,dependent) order the
	// sectors are the right wedge, above both, the left wedge and below both.
	orientation := n.Graph.ProjectionOrientation
	if n.FreeAxis == 1 {
		orientation = -orientation
	}
	n.RayCycle = [4]int{3, 1, 2, 0}
	if orientation < 0 {
		n.RayCycle = [4]int{3, 0, 2, 1}
	}
	lo, hi := new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.UMax)
	bottom, top := new(big.Rat).SetFloat64(d.VMin), new(big.Rat).SetFloat64(d.VMax)
	x, y := n.Parameters[n.FreeAxis], n.Parameters[1-n.FreeAxis]
	between := func(a, c *big.Rat) *big.Rat {
		return new(big.Rat).Quo(new(big.Rat).Add(a, c), big.NewRat(2, 1))
	}
	witnesses := [4][2]*big.Rat{{between(x, hi), nil}, {x, between(y, top)}, {between(lo, x), nil}, {x, between(bottom, y)}}
	for _, i := range [...]int{0, 2} {
		a, e := polynomialValue(n.Power[0], witnesses[i][0], b)
		if e != nil {
			return e
		}
		c, e := polynomialValue(n.Power[1], witnesses[i][0], b)
		if e != nil {
			return e
		}
		witnesses[i][1] = between(a, c)
	}
	signs := [4]int{-sign, sign, -sign, sign}
	for i := range n.Sectors {
		j := i
		if orientation < 0 {
			j = 3 - i
		}
		w := witnesses[j]
		if n.FreeAxis == 1 {
			w[0], w[1] = w[1], w[0]
		}
		for k, q := range w {
			if e := b.check(q); e != nil {
				return e
			}
			w[k] = new(big.Rat).Set(q)
		}
		start, end := n.RayCycle[i], n.RayCycle[(i+1)%4]
		o := n.Graph.ProjectionOrientation
		n.Sectors[i] = GraphSaddleSector{
			HeightSide:       signs[j],
			Witness:          w,
			PositiveBoundary: SaddleSectorBoundary{start, end, o},
			NegativeBoundary: SaddleSectorBoundary{end, start, -o},
		}
	}
	return nil
}

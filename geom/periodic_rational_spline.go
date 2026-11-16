package geom

import (
	"context"
	"math/big"
	"sort"
)

// ExactPeriodicSplineCurve is one closed cyclic rational B-spline net with
// exact, potentially nonuniform knot intervals. It owns its input. Knot
// multiplicity may reach degree; seam repetitions are stored at the start
// of the fundamental interval, with just one closing endpoint.
type ExactPeriodicSplineCurve struct {
	exactCyclicAxis
	net []exactHomogeneous
}

type exactCyclicAxis struct {
	degree int
	knots  []*big.Rat
	period *big.Rat
}

// NewExactPeriodicSplineCurve constructs one cyclic control net. knots
// contains len(controls)+1 physical parameters from one seam to the next.
// For control i the supporting knot indices run from i-degree to i+1
// on the infinite periodic continuation of knots.
func NewExactPeriodicSplineCurve(ctx context.Context, degree int, knots []*big.Rat, controls [][4]*big.Rat,
	o SurfacePatchOptions) (*ExactPeriodicSplineCurve, error) {
	if ctx == nil || degree < 1 || degree > 64 || len(controls) <= degree || len(knots) != len(controls)+1 {
		return nil, ErrInvalid
	}
	o, err := patchOptions(o)
	if err != nil {
		return nil, err
	}
	if len(controls) > o.MaxPoles || len(controls) > o.MaxOperations/5 {
		return nil, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	axis, err := newExactCyclicAxis(ctx, degree, knots, b)
	if err != nil {
		return nil, err
	}
	net, err := ownedHomogeneousNet(ctx, controls, b)
	if err != nil {
		return nil, err
	}
	return &ExactPeriodicSplineCurve{axis, net}, nil
}

func newExactCyclicAxis(ctx context.Context, degree int, knots []*big.Rat, b *polynomialBudget) (exactCyclicAxis, error) {
	result := exactCyclicAxis{degree: degree, knots: make([]*big.Rat, len(knots))}
	mult := 1
	for i, x := range knots {
		if err := ctx.Err(); err != nil {
			return exactCyclicAxis{}, err
		}
		if x == nil {
			return exactCyclicAxis{}, ErrInvalid
		}
		if err := b.check(x); err != nil {
			return exactCyclicAxis{}, err
		}
		if i > 0 {
			switch x.Cmp(knots[i-1]) {
			case -1:
				return exactCyclicAxis{}, ErrInvalid
			case 0:
				mult++
				if i == len(knots)-1 || mult > degree {
					return exactCyclicAxis{}, ErrInvalid
				}
			default:
				mult = 1
			}
		}
		result.knots[i] = new(big.Rat).Set(x)
	}
	result.period = new(big.Rat).Sub(knots[len(knots)-1], knots[0])
	if err := b.check(result.period); err != nil {
		return exactCyclicAxis{}, err
	}
	return result, nil
}

func ownedHomogeneousNet(ctx context.Context, controls [][4]*big.Rat, b *polynomialBudget) ([]exactHomogeneous, error) {
	result := make([]exactHomogeneous, len(controls))
	for i, h := range controls {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if h[3] == nil || h[3].Sign() <= 0 {
			return nil, ErrInvalid
		}
		for axis, v := range h {
			if v == nil {
				return nil, ErrInvalid
			}
			if err := b.check(v); err != nil {
				return nil, err
			}
			result[i][axis] = new(big.Rat).Set(v)
		}
	}
	return result, nil
}

// ExactCyclicSpline converts the original uniform curve without rounding
// its physical knot lattice or homogeneous weights.
func (c *PeriodicBSpline) ExactCyclicSpline(ctx context.Context, o SurfacePatchOptions) (*ExactPeriodicSplineCurve, error) {
	if ctx == nil || c == nil || c.curve == nil {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	o, err := patchOptions(o)
	if err != nil {
		return nil, err
	}
	if len(c.poles) > o.MaxPoles || len(c.poles) > o.MaxOperations/5 {
		return nil, ErrConvergence
	}
	origin, step := exactUniformPeriodicAxis(len(c.poles), c.start, c.period)
	knots := make([]*big.Rat, len(c.poles)+1)
	control := make([][4]*big.Rat, len(c.poles))
	for i := range knots {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		knots[i] = new(big.Rat).Add(origin, new(big.Rat).Mul(step, big.NewRat(int64(i), 1)))
	}
	for i, p := range c.poles {
		w := new(big.Rat).SetFloat64(c.weights[i])
		control[i][3] = w
		for j := range 3 {
			control[i][j] = new(big.Rat).Mul(new(big.Rat).SetFloat64(coordinate(p, j)), w)
		}
	}
	return NewExactPeriodicSplineCurve(ctx, c.degree, knots, control, o)
}

// Knots returns the exact physical knot vector of one cycle, including both
// seam ends. Modifying the returned values does not change the curve.
func (c *ExactPeriodicSplineCurve) Knots() []*big.Rat {
	if c == nil {
		return nil
	}
	out := make([]*big.Rat, len(c.knots))
	for i, k := range c.knots {
		out[i] = new(big.Rat).Set(k)
	}
	return out
}

// Degree reports the degree of the original cyclic basis.
func (c *ExactPeriodicSplineCurve) Degree() int {
	if c == nil {
		return 0
	}
	return c.degree
}

// HomogeneousControls returns one cycle of caller-owned (wx, wy, wz, w)
// control coordinates in the original cyclic order.
func (c *ExactPeriodicSplineCurve) HomogeneousControls() [][4]*big.Rat {
	if c == nil {
		return nil
	}
	out := make([][4]*big.Rat, len(c.net))
	for i, h := range c.net {
		for axis, v := range h {
			out[i][axis] = new(big.Rat).Set(v)
		}
	}
	return out
}

// InsertKnot inserts t modulo the original period throughout the cyclic net.
// This is exact homogeneous knot insertion, including in a nonuniform span
// or at a repeated interior knot or seam. Introducing multiplicity greater
// than the degree is outside the supported domain.
func (c *ExactPeriodicSplineCurve) InsertKnot(ctx context.Context, t *big.Rat, o SurfacePatchOptions) (*ExactPeriodicSplineCurve, error) {
	if ctx == nil || c == nil || len(c.net) == 0 || t == nil {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	o, err := patchOptions(o)
	if err != nil {
		return nil, err
	}
	n := len(c.net)
	if n+1 > o.MaxPoles || n+1 > o.MaxOperations/5 {
		return nil, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	axis, blends, err := c.insertion(ctx, t, b)
	if err != nil {
		return nil, err
	}
	controls := make([][4]*big.Rat, n+1)
	for i, q := range blends {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		controls[i] = q.apply(c.net[q.left], c.net[q.right])
	}
	net, err := ownedHomogeneousNet(ctx, controls, b)
	if err != nil {
		return nil, err
	}
	return &ExactPeriodicSplineCurve{axis, net}, nil
}

type cyclicKnotBlend struct {
	left, right int
	alpha, beta *big.Rat
}

func (q cyclicKnotBlend) apply(left, right exactHomogeneous) [4]*big.Rat {
	if q.alpha == nil {
		return left
	}
	var out [4]*big.Rat
	for axis := range out {
		out[axis] = new(big.Rat).Add(new(big.Rat).Mul(q.beta, left[axis]), new(big.Rat).Mul(q.alpha, right[axis]))
	}
	return out
}

func (c exactCyclicAxis) insertion(ctx context.Context, t *big.Rat, b *polynomialBudget) (exactCyclicAxis, []cyclicKnotBlend, error) {
	n := len(c.knots) - 1
	x, _, _, s, err := c.locate(t, b)
	if err != nil {
		return exactCyclicAxis{}, nil, err
	}
	m := 0
	for j := s; j >= 0 && c.knots[j].Cmp(x) == 0; j-- {
		m++
	}
	if m >= c.degree {
		return exactCyclicAxis{}, nil, ErrDomain
	}
	knots := make([]*big.Rat, n+2)
	copy(knots, c.knots[:s+1])
	knots[s+1] = x
	copy(knots[s+2:], c.knots[s+1:])
	controls := make([]cyclicKnotBlend, n+1)
	// The unrolled Boehm updates start after s and end just before the
	// next orbit's updates. Because n > degree those update bands never
	// overlap. The old and new lengths have distinct cyclic indices.
	for i := s + 1; i <= s+n+1; i++ {
		if err := ctx.Err(); err != nil {
			return exactCyclicAxis{}, nil, err
		}
		q := &controls[i%(n+1)]
		q.left, q.right = (i-1)%n, i%n
		if i > s+c.degree-m {
			continue
		}
		k0, k1 := c.knot(i-c.degree), c.knot(i)
		alpha := new(big.Rat).Quo(new(big.Rat).Sub(x, k0), new(big.Rat).Sub(k1, k0))
		beta := new(big.Rat).Sub(big.NewRat(1, 1), alpha)
		for _, v := range []*big.Rat{alpha, beta} {
			if err := b.check(v); err != nil {
				return exactCyclicAxis{}, nil, err
			}
		}
		q.alpha, q.beta = alpha, beta
	}
	axis, err := newExactCyclicAxis(ctx, c.degree, knots, b)
	if err != nil {
		return exactCyclicAxis{}, nil, err
	}
	return axis, controls, nil
}

// EvaluateRational evaluates the exact nonuniform cyclic law. Seam and
// repeated interior knots select the original right-hand piece.
func (c *ExactPeriodicSplineCurve) EvaluateRational(ctx context.Context, t *big.Rat,
	o SurfacePatchOptions) (result RationalPeriodicCurveJet, err error) {
	if ctx == nil || c == nil || len(c.net) == 0 || t == nil {
		return result, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	o, err = patchOptions(o)
	if err != nil {
		return result, err
	}
	if c.degree+1 > o.MaxPoles {
		return result, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.ExactOperations = b.operations }()
	owner, local, err := c.owner(t, b)
	if err != nil {
		return result, err
	}
	source := make([]exactHomogeneous, c.degree+1)
	for i := range source {
		source[i] = c.net[(owner.Span+i)%len(c.net)]
	}
	h, d, err := differentiatePeriodicNet(ctx, source, rationalSplineBlend(c.degree, owner.Span, local, c.knot), b)
	if err != nil {
		return result, err
	}
	return exactPeriodicCurveResult(ctx, h, d, big.NewRat(1, 1), owner, b)
}

func rationalSplineBlend(degree, span int, t *big.Rat, knot func(int) *big.Rat) func(j, r int) (a, slope *big.Rat) {
	return func(j, r int) (a, slope *big.Rat) {
		left := knot(span + j - degree)
		right := knot(span + j + 1 - r)
		slope = new(big.Rat).Inv(new(big.Rat).Sub(right, left))
		a = new(big.Rat).Mul(new(big.Rat).Sub(t, left), slope)
		return a, slope
	}
}

func (c exactCyclicAxis) owner(t *big.Rat, b *polynomialBudget) (ExactSurfaceKnotSpan, *big.Rat, error) {
	local, shift, cycle, span, err := c.locate(t, b)
	if err != nil {
		return ExactSurfaceKnotSpan{}, nil, err
	}
	start := new(big.Rat).Add(c.knots[span], shift)
	end := new(big.Rat).Add(c.knots[span+1], shift)
	for _, q := range []*big.Rat{start, end} {
		if err := b.check(q); err != nil {
			return ExactSurfaceKnotSpan{}, nil, err
		}
	}
	return ExactSurfaceKnotSpan{Cycle: cycle, Span: span, Domain: ExactParameterInterval{Lower: start, Upper: end}}, local, nil
}

func (c exactCyclicAxis) locate(t *big.Rat, b *polynomialBudget) (local, shift *big.Rat, cycle *big.Int, span int, err error) {
	if err := b.check(t); err != nil {
		return nil, nil, nil, 0, err
	}
	q := new(big.Rat).Quo(new(big.Rat).Sub(t, c.knots[0]), c.period)
	if err := b.check(q); err != nil {
		return nil, nil, nil, 0, err
	}
	cycle = new(big.Int).Div(q.Num(), q.Denom())
	shift = new(big.Rat).Mul(new(big.Rat).SetInt(cycle), c.period)
	local = new(big.Rat).Sub(t, shift)
	if err := b.check(local); err != nil {
		return nil, nil, nil, 0, err
	}
	span = sort.Search(len(c.knots), func(i int) bool { return c.knots[i].Cmp(local) > 0 }) - 1
	return local, shift, cycle, span, nil
}

func (c exactCyclicAxis) knot(index int) *big.Rat {
	n := len(c.knots) - 1
	cycle, offset := index/n, index%n
	if offset < 0 {
		offset += n
		cycle--
	}
	return new(big.Rat).Add(c.knots[offset], new(big.Rat).Mul(big.NewRat(int64(cycle), 1), c.period))
}

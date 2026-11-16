package geom

import (
	"context"
	"math/big"
	"sort"
)

// ExactTensorSplineAxis specifies a rational tensor spline basis. A
// periodic axis has Count+1 knot entries, including both simple seam
// ends. An ordinary axis has the full Count+Degree+1 knot vector and
// the usual active domain [Knots[Degree], Knots[Count]].
type ExactTensorSplineAxis struct {
	Degree, Count int
	Periodic      bool
	Knots         []*big.Rat
}

// ExactPeriodicSplineSurface is an immutable exact homogeneous tensor
// with one or two potentially nonuniform periodic axes. An ordinary
// axis, when present, may retain discontinuous original knot stations.
type ExactPeriodicSplineSurface struct {
	axis [2]exactTensorSplineAxis
	net  []exactHomogeneous
}

type exactTensorSplineAxis struct {
	exactCyclicAxis
	count int
}

// NewExactPeriodicSplineSurface constructs a tensor with at least one
// cyclic direction. Controls are caller-supplied (wx,wy,wz,w) values
// in u-major order, with strictly positive weights. All inputs are copied.
func NewExactPeriodicSplineSurface(ctx context.Context, u, v ExactTensorSplineAxis, controls [][4]*big.Rat,
	o SurfacePatchOptions) (*ExactPeriodicSplineSurface, error) {
	if ctx == nil || !u.Periodic && !v.Periodic || u.Count <= 0 || v.Count <= 0 ||
		u.Count > len(controls)/v.Count || u.Count*v.Count != len(controls) {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	o, err := patchOptions(o)
	if err != nil {
		return nil, err
	}
	if len(controls) > o.MaxPoles || len(controls) > o.MaxOperations/4 {
		return nil, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	var axes [2]exactTensorSplineAxis
	for i, original := range [2]ExactTensorSplineAxis{u, v} {
		var err error
		axes[i], err = newExactTensorSplineAxis(ctx, original, b)
		if err != nil {
			return nil, err
		}
	}
	net, err := ownedHomogeneousNet(ctx, controls, b)
	if err != nil {
		return nil, err
	}
	return &ExactPeriodicSplineSurface{axes, net}, nil
}

func newExactTensorSplineAxis(ctx context.Context, a ExactTensorSplineAxis, b *polynomialBudget) (exactTensorSplineAxis, error) {
	extra := a.Degree + 1
	if a.Periodic {
		extra = 1
	}
	if a.Degree < 1 || a.Degree > 64 || a.Count <= a.Degree || len(a.Knots)-extra != a.Count {
		return exactTensorSplineAxis{}, ErrInvalid
	}
	if a.Periodic {
		cyclic, err := newExactCyclicAxis(ctx, a.Degree, a.Knots, b)
		return exactTensorSplineAxis{cyclic, a.Count}, err
	}
	r := exactTensorSplineAxis{exactCyclicAxis{degree: a.Degree, knots: make([]*big.Rat, len(a.Knots))}, a.Count}
	multiplicity := 1
	for i, k := range a.Knots {
		if err := ctx.Err(); err != nil {
			return exactTensorSplineAxis{}, err
		}
		if k == nil {
			return exactTensorSplineAxis{}, ErrInvalid
		}
		if err := b.check(k); err != nil {
			return exactTensorSplineAxis{}, err
		}
		if i > 0 {
			switch k.Cmp(a.Knots[i-1]) {
			case -1:
				return exactTensorSplineAxis{}, ErrInvalid
			case 0:
				multiplicity++
				if multiplicity > a.Degree+1 {
					return exactTensorSplineAxis{}, ErrInvalid
				}
			default:
				multiplicity = 1
			}
		}
		r.knots[i] = new(big.Rat).Set(k)
	}
	if r.knots[r.degree].Cmp(r.knots[r.count]) >= 0 {
		return exactTensorSplineAxis{}, ErrInvalid
	}
	return r, nil
}

// ExactCyclicSurface converts the exact geometry represented by the
// original uniform surface without rounding its physical knot lattice.
func (s *PeriodicBSplineSurface) ExactCyclicSurface(ctx context.Context, options SurfacePatchOptions) (*ExactPeriodicSplineSurface, error) {
	if ctx == nil || s == nil || s.surface == nil {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	o, err := patchOptions(options)
	if err != nil {
		return nil, err
	}
	n := s.count[0] * s.count[1]
	if n > o.MaxPoles || n > o.MaxOperations/4 {
		return nil, ErrConvergence
	}
	var defs [2]ExactTensorSplineAxis
	for dir, a := range s.axis {
		degree, _, knots := s.tensorSplineAxis(dir)
		defs[dir] = ExactTensorSplineAxis{Degree: degree, Count: s.count[dir], Periodic: a.Period != 0}
		if a.Period != 0 {
			origin, step := exactUniformPeriodicAxis(s.count[dir], a.Start, a.Period)
			defs[dir].Knots = make([]*big.Rat, s.count[dir]+1)
			for i := range defs[dir].Knots {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				defs[dir].Knots[i] = new(big.Rat).Add(origin, new(big.Rat).Mul(step, big.NewRat(int64(i), 1)))
			}
		} else {
			defs[dir].Knots = make([]*big.Rat, len(knots))
			for i, k := range knots {
				defs[dir].Knots[i] = new(big.Rat).SetFloat64(k)
			}
		}
	}
	controls := make([][4]*big.Rat, n)
	for i := range controls {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		j := i/s.count[1]*s.surface.countV + i%s.count[1]
		w := new(big.Rat).SetFloat64(s.surface.weights[j])
		controls[i][3] = w
		for d := 0; d < 3; d++ {
			controls[i][d] = new(big.Rat).Mul(w, new(big.Rat).SetFloat64(coordinate(s.surface.poles[j], d)))
		}
	}
	return NewExactPeriodicSplineSurface(ctx, defs[0], defs[1], controls, o)
}

// Data returns caller-owned axes and homogeneous controls.
func (s *ExactPeriodicSplineSurface) Data() (u, v ExactTensorSplineAxis, controls [][4]*big.Rat) {
	if s == nil || len(s.net) == 0 {
		return
	}
	var axes [2]ExactTensorSplineAxis
	for i, a := range s.axis {
		axes[i] = ExactTensorSplineAxis{Degree: a.degree, Count: a.count, Periodic: a.period != nil, Knots: make([]*big.Rat, len(a.knots))}
		for j, k := range a.knots {
			axes[i].Knots[j] = new(big.Rat).Set(k)
		}
	}
	controls = make([][4]*big.Rat, len(s.net))
	for i, h := range s.net {
		for d, value := range h {
			controls[i][d] = new(big.Rat).Set(value)
		}
	}
	return axes[0], axes[1], controls
}

// InsertPeriodicKnot inserts t once per cycle on direction 0 (U) or
// 1 (V). Insertion uses the same exact homogeneous operation on each
// isoparametric control row and leaves the other basis unchanged.
func (s *ExactPeriodicSplineSurface) InsertPeriodicKnot(ctx context.Context, direction int, t *big.Rat,
	o SurfacePatchOptions) (*ExactPeriodicSplineSurface, error) {
	if ctx == nil || s == nil || len(s.net) == 0 || direction < 0 || direction > 1 || t == nil {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	a, other := s.axis[direction], s.axis[1-direction]
	if a.period == nil {
		return nil, ErrDomain
	}
	o, err := patchOptions(o)
	if err != nil {
		return nil, err
	}
	if a.count+1 > o.MaxPoles/other.count || a.count+1 > o.MaxOperations/4/other.count {
		return nil, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	axis, blend, err := a.insertion(ctx, t, b)
	if err != nil {
		return nil, err
	}
	axes := s.axis
	axes[direction] = exactTensorSplineAxis{axis, a.count + 1}
	controls := make([][4]*big.Rat, axes[0].count*axes[1].count)
	for i := range controls {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		u, v := i/axes[1].count, i%axes[1].count
		if direction == 0 {
			q := blend[u]
			controls[i] = q.apply(s.net[q.left*s.axis[1].count+v], s.net[q.right*s.axis[1].count+v])
		} else {
			q := blend[v]
			row := u * s.axis[1].count
			controls[i] = q.apply(s.net[row+q.left], s.net[row+q.right])
		}
	}
	net, err := ownedHomogeneousNet(ctx, controls, b)
	if err != nil {
		return nil, err
	}
	return &ExactPeriodicSplineSurface{axes, net}, nil
}

// EvaluateRational evaluates the exact point and both physical partials.
// Interior knots and periodic seams are owned by their right-hand spans,
// while an ordinary upper endpoint is evaluated from the left.
func (s *ExactPeriodicSplineSurface) EvaluateRational(ctx context.Context, u, v *big.Rat,
	o SurfacePatchOptions) (result RationalPeriodicSurfaceJet, err error) {
	if ctx == nil || s == nil || len(s.net) == 0 || u == nil || v == nil {
		return result, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	o, err = patchOptions(o)
	if err != nil {
		return result, err
	}
	a, c := s.axis[0], s.axis[1]
	if (a.degree+1)*(c.degree+1) > o.MaxPoles {
		return result, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.ExactOperations = b.operations }()
	uOwner, uBlend, err := a.window(u, b)
	if err != nil {
		return result, err
	}
	vOwner, vBlend, err := c.window(v, b)
	if err != nil {
		return result, err
	}
	row := make([]exactHomogeneous, c.degree+1)
	uNet, dvNet := make([]exactHomogeneous, a.degree+1), make([]exactHomogeneous, a.degree+1)
	for i := range uNet {
		for j := range row {
			row[j] = s.net[((uOwner.Span+i)%a.count)*c.count+(vOwner.Span+j)%c.count]
		}
		uNet[i], dvNet[i], err = differentiatePeriodicNet(ctx, row, vBlend, b)
		if err != nil {
			return result, err
		}
	}
	h, du, err := differentiatePeriodicNet(ctx, uNet, uBlend, b)
	if err != nil {
		return result, err
	}
	dv, _, err := evaluateSplineNet(ctx, dvNet, uBlend, false, b)
	if err != nil {
		return result, err
	}
	uJet, err := exactPeriodicCurveResult(ctx, h, du, big.NewRat(1, 1), uOwner, b)
	if err != nil {
		return result, err
	}
	vJet, err := exactPeriodicCurveResult(ctx, h, dv, big.NewRat(1, 1), vOwner, b)
	if err != nil {
		return result, err
	}
	result.U, result.V = uOwner, vOwner
	result.Point, result.Du, result.Dv = uJet.Point, uJet.Derivative, vJet.Derivative
	result.PointBounds, result.DuBounds, result.DvBounds = uJet.PointBounds, uJet.DerivativeBounds, vJet.DerivativeBounds
	return result, nil
}

func (a exactTensorSplineAxis) window(t *big.Rat, b *polynomialBudget) (ExactSurfaceKnotSpan, func(j, r int) (*big.Rat, *big.Rat), error) {
	if a.period != nil {
		owner, local, err := a.owner(t, b)
		if err != nil {
			return ExactSurfaceKnotSpan{}, nil, err
		}
		return owner, rationalSplineBlend(a.degree, owner.Span, local, a.knot), nil
	}
	var owner ExactSurfaceKnotSpan
	if err := b.check(t); err != nil {
		return owner, nil, err
	}
	relation := t.Cmp(a.knots[a.count])
	if t.Cmp(a.knots[a.degree]) < 0 || relation > 0 {
		return owner, nil, ErrDomain
	}
	span := sort.Search(a.count-a.degree, func(i int) bool {
		cmp := a.knots[i+a.degree+1].Cmp(t)
		return cmp > 0 || cmp == 0 && relation == 0
	})
	for _, k := range a.knots[span : span+2*a.degree+1] {
		if err := b.check(k); err != nil {
			return owner, nil, err
		}
	}
	owner = ExactSurfaceKnotSpan{Span: span, Domain: ExactParameterInterval{
		Lower: new(big.Rat).Set(a.knots[span+a.degree]), Upper: new(big.Rat).Set(a.knots[span+a.degree+1]),
	}}
	knot := func(i int) *big.Rat { return a.knots[i+a.degree] }
	return owner, rationalSplineBlend(a.degree, span, t, knot), nil
}

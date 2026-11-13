package geom

import (
	"fmt"
	"math"
	"math/big"
)

// LineOpenConicHit includes the conic's native parameter and tangent root
// multiplicity. ApproximateCoplanarity marks tolerance-qualified local planes.
type LineOpenConicHit struct {
	LineSurfaceHit
	ConicParameter         float64
	Multiplicity           int
	ApproximateCoplanarity bool
}
type LineOpenConicResult struct {
	Hits                []LineOpenConicHit
	Precision           uint
	CoplanarityResidual float64
}

// IntersectLineParabola solves the coplanar specialization with exact-sign
// quadratic classification of the stored float64 local line. Noncoplanar lines
// outside supplied tolerances return ErrInvalid. Root enclosures and optional
// signed line clipping use LineSurfaceOptions; each hit checks a full 3D residual.
func IntersectLineParabola(line Line, p Parabola, options PlanarLineOptions) (LineOpenConicResult, error) {
	if _, e := p.Eval(0); e != nil {
		return LineOpenConicResult{}, e
	}
	return intersectLineOpenConic(line, p.frame, p.focal, 0, 0, p, options)
}

// IntersectLineHyperbola uses the same coplanar exact-sign solver and filters the
// requested hyperbola branch. Its ConicParameter is rational t in (-1,1), not a
// hyperbolic angle. Asymptotic roots at infinity are not finite intersections.
func IntersectLineHyperbola(line Line, h Hyperbola, options PlanarLineOptions) (LineOpenConicResult, error) {
	if e := h.check(0); e != nil {
		return LineOpenConicResult{}, e
	}
	return intersectLineOpenConic(line, h.frame, h.a, h.b, h.branch, h, options)
}
func intersectLineOpenConic(line Line, frame Frame, a, b float64, branch int, curve Curve, options PlanarLineOptions) (result LineOpenConicResult, err error) {
	o, e := lineSurfaceOptions(options.LineSurfaceOptions)
	if e != nil {
		return result, e
	}
	angular := options.AngularTolerance
	if angular == 0 {
		angular = 1e-12
	}
	if !finite(angular) || angular < 0 || angular >= 1 {
		return result, ErrInvalid
	}
	origin, direction, e := localLine(line, frame)
	if e != nil {
		return result, e
	}
	result.CoplanarityResidual = math.Abs(origin.Z)
	if math.Abs(origin.Z) > o.DistanceTolerance || math.Abs(direction.Z) > angular {
		return result, fmt.Errorf("%w: line is not coplanar with conic", ErrInvalid)
	}
	x, y, dx, dy := new(big.Rat).SetFloat64(origin.X), new(big.Rat).SetFloat64(origin.Y), new(big.Rat).SetFloat64(direction.X), new(big.Rat).SetFloat64(direction.Y)
	q := exactQuadratic{new(big.Rat), new(big.Rat), new(big.Rat)}
	if branch == 0 {
		fourF := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).SetFloat64(a))
		q.a.Mul(dy, dy)
		q.b.Sub(new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(y, dy)), new(big.Rat).Mul(fourF, dx))
		q.c.Sub(new(big.Rat).Mul(y, y), new(big.Rat).Mul(fourF, x))
	} else {
		aa, bb := new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64(b)
		aa.Mul(aa, aa)
		bb.Mul(bb, bb)
		q.a.Sub(new(big.Rat).Mul(bb, new(big.Rat).Mul(dx, dx)), new(big.Rat).Mul(aa, new(big.Rat).Mul(dy, dy)))
		q.b.Sub(new(big.Rat).Mul(bb, new(big.Rat).Mul(x, dx)), new(big.Rat).Mul(aa, new(big.Rat).Mul(y, dy)))
		q.b.Mul(q.b, big.NewRat(2, 1))
		q.c.Sub(new(big.Rat).Mul(bb, new(big.Rat).Mul(x, x)), new(big.Rat).Mul(aa, new(big.Rat).Mul(y, y)))
		q.c.Sub(q.c, new(big.Rat).Mul(aa, bb))
	}
	roots, coincident, tangent, precision, e := solveExactQuadratic(q, o.MaxPrecision)
	result.Precision = precision
	if e != nil {
		return result, e
	}
	if coincident {
		return result, ErrDegenerate
	}
	for _, root := range roots {
		root, inside := inLineRange(root, q, o)
		if !inside {
			continue
		}
		point, e := line.Eval(root.parameter)
		if e != nil {
			return result, e
		}
		local := frame.ToLocal(point)
		if !local.IsFinite() {
			return result, ErrInvalid
		}
		parameter := local.Y / a / 2
		if branch != 0 {
			left := new(big.Rat).Add(x, new(big.Rat).Mul(dx, new(big.Rat).SetFloat64(root.lower)))
			right := new(big.Rat).Add(x, new(big.Rat).Mul(dx, new(big.Rat).SetFloat64(root.upper)))
			if left.Sign() == 0 || right.Sign() == 0 || left.Sign() != right.Sign() {
				return result, ErrConvergence
			}
			if left.Sign() != branch {
				continue
			}
			xx, yy := float64(branch)*local.X/a, local.Y/b
			parameter = yy / (xx + 1)
			if !finite(parameter) || parameter <= -1 || parameter >= 1 {
				return result, ErrConvergence
			}
		}
		actual, e := curve.Eval(parameter)
		if e != nil {
			return result, e
		}
		residual := actual.Distance(point)
		if !finite(residual) {
			return result, ErrInvalid
		}
		if residual > o.DistanceTolerance {
			return result, ErrConvergence
		}
		multiplicity := 1
		if tangent {
			multiplicity = 2
		}
		result.Hits = append(result.Hits, LineOpenConicHit{LineSurfaceHit{root.parameter, root.lower, root.upper, point, residual, tangent, false}, parameter, multiplicity, origin.Z != 0 || direction.Z != 0})
	}
	return result, nil
}

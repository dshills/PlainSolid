package geom

import (
	"context"
	"errors"
	"math/big"
)

type TubeTrimIntersectionOptions struct {
	SurfaceCurveAgreementOptions
	Tolerance float64
	// CoincidentSpans also clips positive-length trim spans with strictly
	// monotone free coordinate and proves their strict dependent-wall clearance.
	CoincidentSpans bool
}
type TubeTrimContact struct {
	// SideSigns gives source height minus opposite graph height before/after
	// the original trim parameter. Zero means absent or unproved on that side.
	SideSigns                         [2]int
	TrimEvent                         *ExactComposedCurveEvent
	Parameters                        [4]*ExactAlgebraicRoot
	ParametersA, ParametersB          UVBox
	Bounds                            Box
	Multiplicity                      int
	Transverse, Tangent, SingularTrim bool
	// CoincidentEndpoint marks the end of a trim that lies on the branch.
	// Its multiplicity is zero because the height residual vanishes identically.
	CoincidentEndpoint bool
}
type TubeTrimIntersection struct {
	Complete        bool
	Hits            []TubeTrimContact
	Pending         []*ExactComposedCurveEvent
	Coincident      []ComposedCurvePlaneIntersection
	Spans           []TubeCoincidentTrimSpan
	ExactOperations int
}

// TubeCoincidentTrimSpan carries both chart parameter roots at each end of an
// exact trim interval lying on the branch. Endpoints follow trim traversal;
// FreeOrder is +1 for increasing tube free coordinate and -1 for decreasing.
type TubeCoincidentTrimSpan struct {
	Ends      [2]TubeTrimContact
	FreeOrder int
	source    *tubeCoincidentTrimSource
}

// IntersectQuadraticTrim composes a positive-weight quadratic UV Bezier on
// [0,1] with the selected original support. Original supports are independently
// compared with the immutable local tube supports. The opposite full chart must
// be invertible affine XY, and the trim's full inverse-projected UV path must fit its rectangle.
// Isolated exact contacts are accepted only inside the tube's closed free range
// and strict dependent ranges. Coincident spans and boundary-dependent contacts
// remain pending unless excluded or explicitly certified by CoincidentSpans.
// Hits and spans retain per-knot ancestry.
func (t VerifiedSurfaceTube) IntersectQuadraticTrim(ctx context.Context, originalA, originalB *SurfacePatch, chart int, trim *ExactSplineCurve, o TubeTrimIntersectionOptions) (r TubeTrimIntersection, err error) {
	return t.intersectRationalBezierTrim(ctx, originalA, originalB, chart, trim, o, true)
}

// IntersectRationalBezierTrim proves the same contact relation for one exact
// positive-weight UV Bézier of degree one through eight. The curve's original
// normalized parameter, contact signs and multiplicities are retained.
func (t VerifiedSurfaceTube) IntersectRationalBezierTrim(ctx context.Context, originalA, originalB *SurfacePatch, chart int, trim *ExactSplineCurve, o TubeTrimIntersectionOptions) (TubeTrimIntersection, error) {
	return t.intersectRationalBezierTrim(ctx, originalA, originalB, chart, trim, o, false)
}

func (t VerifiedSurfaceTube) intersectRationalBezierTrim(ctx context.Context, originalA, originalB *SurfacePatch, chart int, trim *ExactSplineCurve, o TubeTrimIntersectionOptions, quadraticOnly bool) (r TubeTrimIntersection, err error) {
	if ctx == nil || t.a == nil || t.b == nil || t.tangentSign == 0 || originalA == nil || originalB == nil || chart < 0 || chart > 1 || trim == nil || trim.patch == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	options, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return r, e
	}
	o.SurfacePatchOptions = options
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if o.Tolerance == 0 {
		o.Tolerance = 1e-9
	}
	if o.MaxSubdivisions < 1 || !finite(o.Tolerance) || o.Tolerance <= 0 {
		return r, ErrInvalid
	}
	d := trim.patch.degreeU
	if trim.patch.countU != d+1 || d < 1 || d > 8 || (quadraticOnly && d != 2) {
		return r, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	remaining := func() (int, error) {
		n := options.MaxOperations - b.operations
		if n < 1 {
			return 0, ErrConvergence
		}
		return n, nil
	}
	originals := [2]*SurfacePatch{originalA, originalB}
	locals := [2]*SurfacePatch{t.a, t.b}
	for i, p := range originals {
		restricted, e := trimPatchBudget(p, locals[i].Domain(), b)
		if e != nil {
			return r, e
		}
		n, e := remaining()
		if e != nil {
			return r, e
		}
		identity, e := CompareExactSurfaces(ctx, restricted, locals[i], ExactSurfaceComparisonOptions{MaxOperations: n, MaxCoefficientBits: options.MaxCoefficientBits})
		b.operations += identity.ExactOperations
		if e != nil {
			return r, e
		}
		if !identity.Complete || !identity.Equal {
			return r, ErrDomain
		}
	}

	start, end := trim.Domain()
	if start != 0 || end != 1 {
		return r, ErrDomain
	}
	uv := &ExactComposedCurveCell{endpoints: [2]*ExactGraphLineRoot{{polynomial: rationalPolynomial{new(big.Rat), big.NewRat(1, 1)}, bounds: [2]*big.Rat{new(big.Rat), new(big.Rat)}}, {polynomial: rationalPolynomial{big.NewRat(-1, 1), big.NewRat(1, 1)}, bounds: [2]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1)}}}}
	for axis := 0; axis < 4; axis++ {
		values := make([]*big.Rat, trim.patch.countU)
		for i := range values {
			values[i] = trim.patch.net[i*trim.patch.countV][axis]
		}
		uv.power[axis], e = bernsteinToPower(values, b)
		if e != nil {
			return r, e
		}
	}
	child := o.SurfaceCurveAgreementOptions
	child.MaxOperations, e = remaining()
	if e != nil {
		return r, e
	}
	collection, e := originals[chart].BuildSurfaceCurveCompositionCells(ctx, trim, child)
	b.operations += collection.ExactOperations
	if e != nil {
		return r, e
	}
	da, db := t.a.Domain(), t.b.Domain()
	domains := [4][2]float64{{da.UMin, da.UMax}, {da.VMin, da.VMax}, {db.UMin, db.UMax}, {db.VMin, db.VMax}}
	for _, cell := range collection.Cells {
		child.MaxOperations, e = remaining()
		if e != nil {
			return r, e
		}
		contact, e := IntersectExactComposedCurveAffineGraph(ctx, cell, originals[1-chart], child)
		b.operations += contact.ExactOperations
		if e != nil {
			return r, e
		}
		for _, piece := range contact.Cells {

			if piece.Intersection.Coincident {
				model := piece.Intersection.Boundary[0].cell
				projected, e := transformComposedXY(model, contact.InverseXY, b)
				if e != nil {
					return r, e
				}
				models := [2]*ExactComposedCurveCell{projected, projected}
				models[chart] = uv
				parameter := exactInterval{lo: piece.Intersection.Domain[0].Lower, hi: piece.Intersection.Domain[1].Upper}
				outside, e := coincidentTrimOutsideTube(models, parameter, domains, b)
				if e != nil {
					return r, e
				}
				if !outside {
					if o.CoincidentSpans {
						span, proven, e := certifyCoincidentTubeSpan(ctx, piece.Intersection, models, parameter, domains, t.axis, o, b)
						if e != nil {
							return r, e
						}
						if proven {
							if span.FreeOrder != 0 {
								r.Spans = append(r.Spans, span)
							} else if span.Ends[0].TrimEvent != nil {
								r.Hits = append(r.Hits, span.Ends[0])
							}
							continue
						}
					}
					r.Coincident = append(r.Coincident, piece.Intersection)
				}
				continue
			}
			for eventIndex, event := range piece.Intersection.Events {
				hit := TubeTrimContact{TrimEvent: event, Multiplicity: event.Multiplicity()}
				if eventIndex < len(piece.EventSigns) {
					hit.SideSigns = piece.EventSigns[eventIndex]
				}
				projected, e := transformComposedXY(event.cell, contact.InverseXY, b)
				if e != nil {
					return r, e
				}

				pending, outside := false, false
				for owner := 0; owner < 2; owner++ {
					model := projected
					if owner == chart {
						model = uv
					}
					for axis := 0; axis < 2; axis++ {
						n, e := remaining()
						if e != nil {
							return r, e
						}
						root, ops, e := model.CoordinateRoot(ctx, event.Root(), axis, ExactCurvePlaneEventOptions{MaxOperations: n, MaxCoefficientBits: options.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions})
						b.operations += ops
						if e != nil {
							if errors.Is(e, ErrConvergence) {
								pending = true
								break
							}
							return r, e
						}
						hit.Parameters[2*owner+axis] = root
					}
					if pending {
						break
					}
				}
				if pending {
					r.Pending = append(r.Pending, event)
					continue
				}
				var boxes [4][2]float64
				for axis, root := range hit.Parameters {
					for side := 0; side < 2; side++ {
						q := new(big.Rat).SetFloat64(domains[axis][side])
						bound := &ExactGraphLineRoot{polynomial: rationalPolynomial{new(big.Rat).Neg(q), big.NewRat(1, 1)}, bounds: [2]*big.Rat{q, q}}
						comparison, e := compareRootBudget(ctx, root.root, bound, b, o.MaxSubdivisions)
						if e != nil {
							return r, e
						}
						if !comparison.Proven {
							return r, ErrConvergence
						}
						if side == 0 && comparison.Order < 0 || side == 1 && comparison.Order > 0 {
							outside = true
						}
						if comparison.Order == 0 && axis != t.axis {
							pending = true
						}
					}
					boxes[axis] = [2]float64{max(domains[axis][0], rationalFloatBound(root.root.bounds[0], true)), min(domains[axis][1], rationalFloatBound(root.root.bounds[1], false))}
				}
				if outside {
					continue
				}
				if pending {
					atAnchor, e := t.matchesAnchorRoots(ctx, hit.Parameters, b, o.MaxSubdivisions)
					if e != nil {
						return r, e
					}
					pending = !atAnchor
				}
				if pending {
					r.Pending = append(r.Pending, event)
					continue
				}
				moving := false
				for axis := 0; axis < 2; axis++ {
					derivative, e := rationalDerivativeNumerator(uv.power[axis], uv.power[3], b)
					if e != nil {
						return r, e
					}
					if len(derivative) > 0 {
						sign, _, e := composedPolynomialRootSign(ctx, event.root, derivative, b, o.MaxSubdivisions)
						if e != nil {
							return r, e
						}
						moving = moving || sign != 0
					}
				}
				hit.SingularTrim = !moving
				hit.Transverse = moving && hit.Multiplicity == 1
				hit.Tangent = moving && hit.Multiplicity > 1
				n, e := remaining()
				if e != nil {
					return r, e
				}
				point, e := event.Enclose(ctx, ComposedCurveEndpointOptions{Tolerance: o.Tolerance, ExactCurvePlaneEventOptions: ExactCurvePlaneEventOptions{MaxOperations: n, MaxCoefficientBits: options.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}})
				b.operations += point.ExactOperations
				if e != nil {
					return r, e
				}
				hit.ParametersA = UVBox{Min: Vec2{X: boxes[0][0], Y: boxes[1][0]}, Max: Vec2{X: boxes[0][1], Y: boxes[1][1]}}
				hit.ParametersB = UVBox{Min: Vec2{X: boxes[2][0], Y: boxes[3][0]}, Max: Vec2{X: boxes[2][1], Y: boxes[3][1]}}
				hit.Bounds = point.Bounds
				r.Hits = append(r.Hits, hit)
			}
		}
	}
	r.Complete = len(r.Pending) == 0 && len(r.Coincident) == 0
	if !r.Complete {
		return r, ErrConvergence
	}
	return r, nil
}

func rationalDerivativeNumerator(numerator, denominator rationalPolynomial, b *polynomialBudget) (rationalPolynomial, error) {
	first, e := polynomialProduct(polynomialDerivative(numerator), denominator, b)
	if e != nil {
		return nil, e
	}
	second, e := polynomialProduct(numerator, polynomialDerivative(denominator), b)
	if e != nil {
		return nil, e
	}
	return subtractScaledPolynomial(first, second, big.NewRat(1, 1), b)
}

// Exact equality was proved on the composition cell by the original affine
// graph comparison. The local tube supplies uniqueness at each free value.
// Bernstein signs certify containment and injectivity over the entire cell.
func certifyCoincidentTubeSpan(ctx context.Context, piece ComposedCurvePlaneIntersection, charts [2]*ExactComposedCurveCell,
	parameter exactInterval, domains [4][2]float64, free int, o TubeTrimIntersectionOptions, b *polynomialBudget) (r TubeCoincidentTrimSpan, proven bool, err error) {
	options := func() ExactCurvePlaneEventOptions {
		return ExactCurvePlaneEventOptions{MaxOperations: b.maxOperations - b.operations, MaxCoefficientBits: b.maxBits, MaxSubdivisions: o.MaxSubdivisions}
	}
	freeChart := *charts[free/2]
	for i, end := range piece.Boundary {
		freeChart.endpoints[i] = end.root
	}
	p, w := freeChart.power[free%2], freeChart.power[3]
	weight, e := polynomialBernsteinInterval(w, parameter, b)
	if e != nil || weight.lo.Sign() <= 0 {
		return r, false, e
	}
	n, e := rationalDerivativeNumerator(p, w, b)
	if e != nil {
		return r, false, e
	}
	derivative, e := polynomialBernsteinInterval(n, parameter, b)
	if e != nil {
		return r, false, e
	}
	switch {
	case derivative.lo.Sign() > 0:
		r.FreeOrder = 1
	case derivative.hi.Sign() < 0:
		r.FreeOrder = -1
	default:
		return r, false, nil
	}
	pointOnly := false
	for side, bound := range domains[free] {
		residual, e := subtractScaledPolynomial(p, w, new(big.Rat).SetFloat64(bound), b)
		if e != nil {
			return r, false, e
		}
		var signs [2]int
		for i, end := range piece.Boundary {
			signs[i], _, e = composedPolynomialRootSign(ctx, end.root, residual, b, o.MaxSubdivisions)
			if e != nil {
				return r, false, e
			}
			if side == 1 {
				signs[i] = -signs[i]
			}
		}
		if signs[0] < 0 && signs[1] < 0 {
			return TubeCoincidentTrimSpan{}, true, nil
		}
		if signs[0] >= 0 && signs[1] >= 0 {
			continue
		}
		if signs[0] == 0 || signs[1] == 0 {
			end := piece.Boundary[0]
			if signs[1] == 0 {
				end = piece.Boundary[1]
			}
			piece.Boundary = [2]*ExactComposedCurveEvent{end, end}
			pointOnly = true
			continue
		}
		cut, e := freeChart.intersectComposedPolynomial(ctx, residual, options(), b)
		if e != nil {
			return r, false, e
		}
		if !cut.Complete || len(cut.Events) != 1 {
			return r, false, nil
		}
		i := 0
		if signs[1] < 0 {
			i = 1
		}
		root := cut.Events[0].root
		opts := options()
		if opts.MaxOperations < 1 {
			return r, false, ErrConvergence
		}
		refined, e := (&ExactAlgebraicRoot{root: root}).Enclose(ctx, big.NewRat(1, 1<<40), opts)
		b.operations += refined.Operations
		if e != nil || !refined.Converged {
			return r, false, e
		}
		piece.Boundary[i] = &ExactComposedCurveEvent{root: &ExactGraphLineRoot{
			polynomial: root.polynomial, bounds: [2]*big.Rat{refined.Interval.Lower, refined.Interval.Upper},
		}, cell: piece.Boundary[i].cell}
	}
	parameter = exactInterval{lo: piece.Boundary[0].root.bounds[0], hi: piece.Boundary[1].root.bounds[1]}
	for owner, chart := range charts {
		weight := chart.power[3]
		w, e := polynomialBernsteinInterval(weight, parameter, b)
		if e != nil || w.lo.Sign() <= 0 {
			return r, false, e
		}
		for coordinate := 0; coordinate < 2; coordinate++ {
			axis := 2*owner + coordinate
			if axis == free {
				continue
			}
			p := chart.power[coordinate]
			for side, bound := range domains[axis] {
				residual, e := subtractScaledPolynomial(p, weight, new(big.Rat).SetFloat64(bound), b)
				if e != nil {
					return r, false, e
				}
				hull, e := polynomialBernsteinInterval(residual, parameter, b)
				if e != nil {
					return r, false, e
				}
				sign := hull.lo.Sign()
				if side == 1 {
					sign = -hull.hi.Sign()
				}
				if sign <= 0 {
					return r, false, nil
				}
			}
		}
	}
	for i, event := range piece.Boundary {
		if pointOnly && i == 1 {
			r.FreeOrder = 0
			break
		}
		end := &r.Ends[i]
		end.TrimEvent = event
		end.CoincidentEndpoint = true
		var boxes [4][2]float64
		for owner, chart := range charts {
			for coordinate := 0; coordinate < 2; coordinate++ {
				axis := 2*owner + coordinate
				opts := options()
				if opts.MaxOperations < 1 {
					return r, false, ErrConvergence
				}
				root, used, e := chart.CoordinateRoot(ctx, event.Root(), coordinate, opts)
				b.operations += used
				if e != nil {
					return r, false, e
				}
				end.Parameters[axis] = root
				boxes[axis] = [2]float64{max(domains[axis][0], rationalFloatBound(root.root.bounds[0], true)), min(domains[axis][1], rationalFloatBound(root.root.bounds[1], false))}
			}
		}
		opts := options()
		if opts.MaxOperations < 1 {
			return r, false, ErrConvergence
		}
		point, e := event.Enclose(ctx, ComposedCurveEndpointOptions{Tolerance: o.Tolerance, ExactCurvePlaneEventOptions: opts})
		b.operations += point.ExactOperations
		if e != nil {
			return r, false, e
		}
		end.ParametersA = UVBox{Min: Vec2{boxes[0][0], boxes[1][0]}, Max: Vec2{boxes[0][1], boxes[1][1]}}
		end.ParametersB = UVBox{Min: Vec2{boxes[2][0], boxes[3][0]}, Max: Vec2{boxes[2][1], boxes[3][1]}}
		end.Bounds = point.Bounds
	}
	if !pointOnly {
		for i, end := range piece.Boundary {
			freeChart.endpoints[i] = end.root
		}
		r.source = &tubeCoincidentTrimSource{chart: &freeChart, coordinate: free % 2}
	}
	return r, true, nil
}

// A strict outward coordinate hull separation proves the entire coincident
// span misses this local tube. Failure to separate retains the unresolved span.
func coincidentTrimOutsideTube(models [2]*ExactComposedCurveCell, parameter exactInterval, domains [4][2]float64, b *polynomialBudget) (bool, error) {
	for owner, model := range models {
		denominator, e := polynomialBernsteinInterval(model.power[3], parameter, b)
		if e != nil {
			return false, e
		}
		if denominator.lo.Sign() <= 0 {
			continue
		}
		for axis := 0; axis < 2; axis++ {
			numerator, e := polynomialBernsteinInterval(model.power[axis], parameter, b)
			if e != nil {
				return false, e
			}
			value, e := intervalDivide(numerator, denominator, b)
			if e != nil {
				return false, e
			}
			if value.hi.Cmp(new(big.Rat).SetFloat64(domains[2*owner+axis][0])) < 0 || value.lo.Cmp(new(big.Rat).SetFloat64(domains[2*owner+axis][1])) > 0 {
				return true, nil
			}
		}
	}
	return false, nil
}

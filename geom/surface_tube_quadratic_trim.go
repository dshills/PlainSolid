package geom

import (
	"context"
	"errors"
	"math/big"
)

type TubeTrimIntersectionOptions struct {
	SurfaceCurveAgreementOptions
	Tolerance float64
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
}
type TubeTrimIntersection struct {
	Complete        bool
	Hits            []TubeTrimContact
	Pending         []*ExactComposedCurveEvent
	Coincident      []ComposedCurvePlaneIntersection
	ExactOperations int
}

// IntersectQuadraticTrim composes a positive-weight quadratic UV Bezier on
// [0,1] with the selected original support. Original supports are independently
// compared with the immutable local tube supports. The opposite full chart must
// be invertible affine XY, and the trim's full inverse-projected UV path must fit its rectangle.
// Isolated exact contacts are accepted only inside the tube's closed free range
// and strict dependent ranges. Coincident spans and boundary-dependent contacts
// remain pending unless a strict outward hull proves the whole span outside
// this tube. Hits retain per-knot ancestry; no rounded event merging occurs.
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
					first, e := polynomialProduct(polynomialDerivative(uv.power[axis]), uv.power[3], b)
					if e != nil {
						return r, e
					}
					second, e := polynomialProduct(uv.power[axis], polynomialDerivative(uv.power[3]), b)
					if e != nil {
						return r, e
					}
					derivative, e := subtractScaledPolynomial(first, second, big.NewRat(1, 1), b)
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

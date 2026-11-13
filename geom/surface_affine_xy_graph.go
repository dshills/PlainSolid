package geom

import (
	"context"
	"math/big"
)

// AffineGraphCurveIntersection retains original model events. InverseXY gives
// exact target U/V as coefficients of model X,Y,1 (owned diagnostic copies).
// Unlike identity-XY reports, model X/Y bounds are not target UV bounds.
type AffineGraphCurveIntersection struct {
	IdentityGraphCurveIntersection
	InverseXY [2][3]*big.Rat
}

// ProjectedAffineGraphCurveIntersection retains exact original model events
// and the target chart's inverse map in ProjectionAxes order.
type ProjectedAffineGraphCurveIntersection struct {
	IdentityGraphCurveIntersection
	ProjectionAxes [2]int
	Inverse        [2][3]*big.Rat
}

// IntersectExactComposedCurveProjectedGraph proves curve contacts against a
// target graph with an invertible affine map in the chosen model plane.
// The coordinate permutation is exact; returned events evaluate in the
// original XYZ coordinates and use the original curve parameter.
func IntersectExactComposedCurveProjectedGraph(ctx context.Context, cell *ExactComposedCurveCell, target *SurfacePatch,
	axes [2]int, o SurfaceCurveAgreementOptions) (out ProjectedAffineGraphCurveIntersection, err error) {
	order, e := modelProjectionPermutation(axes)
	if e != nil || ctx == nil || cell == nil || target == nil || len(target.net) == 0 {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	out.ProjectionAxes = axes
	if axes == ([2]int{0, 1}) {
		r, e := IntersectExactComposedCurveAffineGraph(ctx, cell, target, o)
		out.IdentityGraphCurveIntersection, out.Inverse = r.IdentityGraphCurveIntersection, r.InverseXY
		return out, e
	}
	opts, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return out, e
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	defer func() { out.ExactOperations += b.operations }()
	p, e := permuteGraphPatch(target, order, b)
	if e != nil {
		return out, e
	}
	opts.MaxOperations -= b.operations
	if opts.MaxOperations < 1 {
		return out, ErrConvergence
	}
	o.SurfacePatchOptions = opts
	r, e := IntersectExactComposedCurveAffineGraph(ctx, permuteGraphCell(cell, order), p, o)
	out.IdentityGraphCurveIntersection, out.Inverse = r.IdentityGraphCurveIntersection, r.InverseXY
	for _, q := range out.Cells {
		restore := func(event *ExactComposedCurveEvent) {
			if event != nil {
				model := *event.cell
				model.power = cell.power
				event.cell = &model
			}
		}
		for _, event := range q.Intersection.Events {
			restore(event)
		}
		for _, event := range q.Intersection.Boundary {
			restore(event)
		}
	}
	return out, e
}

// IntersectExactComposedCurveAffineGraph proves an invertible affine XY map on
// the entire target and reduces contact to an exact identity-XY graph. Z may be
// any supported rational spline. Original curve parameter/model provenance is
// restored in every returned event, including coincident interval boundaries.
// Exact knot/domain clipping retains only closed target-chart contacts; an
// isolated outside-to-outside boundary touch has multiplicity zero and no side
// signs because no open target-chart interval is present.
func IntersectExactComposedCurveAffineGraph(ctx context.Context, cell *ExactComposedCurveCell, target *SurfacePatch, o SurfaceCurveAgreementOptions) (r AffineGraphCurveIntersection, err error) {
	if ctx == nil || cell == nil || target == nil || cell.endpoints[0] == nil || cell.endpoints[1] == nil || len(cell.power[3]) == 0 {
		return r, ErrInvalid
	}
	options, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { r.ExactOperations += b.operations }()
	normalized, inverse, e := normalizeAffineXYGraph(ctx, target, b)
	if e != nil {
		return r, e
	}
	r.InverseXY = inverse
	source, e := transformComposedXY(cell, inverse, b)
	if e != nil {
		return r, e
	}

	o.SurfacePatchOptions = options
	contact, e := clipComposedIdentityGraph(ctx, source, normalized, o, b)
	r.IdentityGraphCurveIntersection = contact
	restore := func(event *ExactComposedCurveEvent) {
		if event == nil {
			return
		}
		model := *cell
		model.endpoints = event.cell.endpoints
		model.cell = event.cell.cell
		event.cell = &model
	}
	for i := range r.Cells {
		for _, event := range r.Cells[i].Intersection.Events {
			restore(event)
		}
		for _, event := range r.Cells[i].Intersection.Boundary {
			restore(event)
		}
	}
	return r, e
}

func transformComposedXY(cell *ExactComposedCurveCell, inverse [2][3]*big.Rat, b *polynomialBudget) (*ExactComposedCurveCell, error) {
	result := *cell
	for axis := 0; axis < 2; axis++ {
		degree := max(len(cell.power[0]), len(cell.power[1]), len(cell.power[3]))
		p := make(rationalPolynomial, degree)
		for i := range p {
			p[i] = new(big.Rat)
		}
		for j, field := range []int{0, 1, 3} {
			for i, x := range cell.power[field] {
				p[i].Add(p[i], new(big.Rat).Mul(inverse[axis][j], x))
				if e := b.check(p[i]); e != nil {
					return nil, e
				}
			}
		}
		result.power[axis] = trimPolynomial(p)
	}
	return &result, nil
}

func normalizeAffineXYGraph(ctx context.Context, p *SurfacePatch, b *polynomialBudget) (*SurfacePatch, [2][3]*big.Rat, error) {
	var inverse [2][3]*big.Rat
	if ctx == nil || p == nil || len(p.net) == 0 {
		return nil, inverse, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, inverse, e
	}
	d := p.Domain()
	remaining := b.maxOperations - b.operations
	if remaining < 1 {
		return nil, inverse, ErrConvergence
	}
	jet, e := p.EvaluateRational(ctx, new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin), SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: b.maxBits})
	b.operations += jet.ExactOperations
	if e != nil {
		return nil, inverse, e
	}
	var affine [2][3]*big.Rat // constant,u,v
	for axis := 0; axis < 2; axis++ {
		affine[axis] = [3]*big.Rat{new(big.Rat).Sub(new(big.Rat).Sub(jet.Point[axis], new(big.Rat).Mul(jet.Du[axis], new(big.Rat).SetFloat64(d.UMin))), new(big.Rat).Mul(jet.Dv[axis], new(big.Rat).SetFloat64(d.VMin))), jet.Du[axis], jet.Dv[axis]}
	}
	determinant := new(big.Rat).Sub(new(big.Rat).Mul(affine[0][1], affine[1][2]), new(big.Rat).Mul(affine[0][2], affine[1][1]))
	if e := b.check(determinant); e != nil {
		return nil, inverse, e
	}
	if determinant.Sign() == 0 {
		return nil, inverse, ErrDegenerate
	}
	cells, e := graphContactBezierCells(ctx, p, 4096, b)
	if e != nil {
		return nil, inverse, e
	}
	for _, cell := range cells {
		w, e := tensorPowerCoefficients(cell.net, cell.countU, cell.countV, 3, b)
		if e != nil {
			return nil, inverse, e
		}
		domain := cell.Domain()
		for axis := 0; axis < 2; axis++ {
			coord, e := tensorPowerCoefficients(cell.net, cell.countU, cell.countV, axis, b)
			if e != nil {
				return nil, inverse, e
			}
			offset := new(big.Rat).Add(affine[axis][0], new(big.Rat).Add(new(big.Rat).Mul(affine[axis][1], new(big.Rat).SetFloat64(domain.UMin)), new(big.Rat).Mul(affine[axis][2], new(big.Rat).SetFloat64(domain.VMin))))
			du := new(big.Rat).Mul(affine[axis][1], new(big.Rat).Sub(new(big.Rat).SetFloat64(domain.UMax), new(big.Rat).SetFloat64(domain.UMin)))
			dv := new(big.Rat).Mul(affine[axis][2], new(big.Rat).Sub(new(big.Rat).SetFloat64(domain.VMax), new(big.Rat).SetFloat64(domain.VMin)))
			for i := 0; i <= cell.countU; i++ {
				for j := 0; j <= cell.countV; j++ {
					value := new(big.Rat)
					if i < cell.countU && j < cell.countV {
						value.Sub(coord[i*cell.countV+j], new(big.Rat).Mul(offset, w[i*cell.countV+j]))
					}
					if i > 0 && j < cell.countV {
						value.Sub(value, new(big.Rat).Mul(du, w[(i-1)*cell.countV+j]))
					}
					if j > 0 && i < cell.countU {
						value.Sub(value, new(big.Rat).Mul(dv, w[i*cell.countV+j-1]))
					}
					if e := b.check(value); e != nil {
						return nil, inverse, e
					}
					if value.Sign() != 0 {
						return nil, inverse, ErrConvergence
					}
				}
			}
		}
	}
	inverse[0][0] = new(big.Rat).Quo(affine[1][2], determinant)
	inverse[0][1] = new(big.Rat).Neg(new(big.Rat).Quo(affine[0][2], determinant))
	inverse[1][0] = new(big.Rat).Neg(new(big.Rat).Quo(affine[1][1], determinant))
	inverse[1][1] = new(big.Rat).Quo(affine[0][1], determinant)
	for axis := 0; axis < 2; axis++ {
		inverse[axis][2] = new(big.Rat).Neg(new(big.Rat).Add(new(big.Rat).Mul(inverse[axis][0], affine[0][0]), new(big.Rat).Mul(inverse[axis][1], affine[1][0])))
	}
	result := *p
	result.net = make([]exactHomogeneous, len(p.net))
	for i, h := range p.net {
		for field := 0; field < 4; field++ {
			result.net[i][field] = new(big.Rat).Set(h[field])
		}
		for axis := 0; axis < 2; axis++ {
			value := new(big.Rat)
			for j, field := range []int{0, 1, 3} {
				value.Add(value, new(big.Rat).Mul(inverse[axis][j], h[field]))
				if e := b.check(value); e != nil {
					return nil, inverse, e
				}
			}
			result.net[i][axis] = value
		}
	}
	if e := result.computeBounds(b); e != nil {
		return nil, inverse, e
	}
	return &result, inverse, nil
}

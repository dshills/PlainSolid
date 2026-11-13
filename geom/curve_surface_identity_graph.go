package geom

import (
	"context"
	"math/big"
)

// IdentityGraphCurveCellContact retains original surface-cell ancestry and all
// exact scalar height events in that interval. Multiplicity is the order of
// curve.Z-surface.Z; even orders include tangencies. Coincident means the entire
// interval lies on the graph. Adjacent cells may both report the same endpoint;
// CompareParameter proves identity without merging different nearby roots.
// Event.Enclose supplies the model box; its X/Y bounds are also chart U/V bounds.
type IdentityGraphCurveCellContact struct {
	SurfaceCell  ParameterRectangle
	Intersection ComposedCurvePlaneIntersection
	// EventSigns aligns with Intersection.Events. Signs are curve.Z-graph.Z
	// on the incident open cell sides; zero denotes no side at a cell boundary.
	EventSigns [][2]int
}

type IdentityGraphCurveIntersection struct {
	Complete                      bool
	Cells                         []IdentityGraphCurveCellContact
	ExactOperations, Subdivisions int
}

// IntersectExactCurveIdentityGraph proves global X=u,Y=v and isolates contacts
// of one positive-weight exact Bezier on [0,1]. The full projected curve must
// stay inside the surface rectangle; unsupported paths remain unresolved.
// All knot crossings and contact parameters retain algebraic identities.
func IntersectExactCurveIdentityGraph(ctx context.Context, curve *ExactSplineCurve, surface *SurfacePatch, o SurfaceCurveAgreementOptions) (IdentityGraphCurveIntersection, error) {
	return intersectExactCurveIdentityGraph(ctx, curve, surface, o, nil)
}
func intersectExactCurveIdentityGraph(ctx context.Context, curve *ExactSplineCurve, surface *SurfacePatch, o SurfaceCurveAgreementOptions, restriction *ExactComposedCurveCell) (r IdentityGraphCurveIntersection, err error) {
	if ctx == nil || curve == nil || curve.patch == nil || surface == nil || len(surface.net) == 0 {
		return r, ErrInvalid
	}
	options, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return r, e
	}
	o.SurfacePatchOptions = options
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if o.MaxEvents == 0 {
		o.MaxEvents = 256
	}
	if o.MaxSubdivisions < 1 || o.MaxEvents < 2 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	cells, e := graphContactBezierCells(ctx, surface, o.MaxEvents, b)
	if e != nil {
		return r, e
	}
	for _, cell := range cells {
		if e := proveIdentityGraphCell(cell, b); e != nil {
			return r, e
		}
	}
	remaining := func() SurfacePatchOptions { x := options; x.MaxOperations -= b.operations; return x }
	if remaining().MaxOperations < 1 {
		return r, ErrConvergence
	}
	net, ops, e := curve.BezierControlNet(ctx, remaining())
	b.operations += ops
	if e != nil {
		return r, e
	}
	if net.Start != 0 || net.End != 1 {
		return r, ErrDomain
	}

	if len(net.Control) > 65 {
		return r, ErrConvergence
	}
	original := &ExactComposedCurveCell{}
	for axis := 0; axis < 4; axis++ {
		values := make([]*big.Rat, len(net.Control))
		for i := range values {
			values[i] = net.Control[i][axis]
		}
		original.power[axis], e = bernsteinToPower(values, b)
		if e != nil {
			return r, e
		}
	}
	for i := range net.Control {
		net.Control[i][2] = new(big.Rat)
	}
	homogeneous := make([]exactHomogeneous, len(net.Control))
	for i := range homogeneous {
		homogeneous[i] = exactHomogeneous(net.Control[i])
	}
	uv, e := exactCurveFromHomogeneousBezier(homogeneous, TubeAffineLineOptions{MaxPoles: options.MaxPoles, MaxOperations: options.MaxOperations, MaxCoefficientBits: options.MaxCoefficientBits}, b)
	if e != nil {
		return r, e
	}
	if remaining().MaxOperations < 1 {
		return r, ErrConvergence
	}
	o.SurfacePatchOptions = remaining()
	composed, e := surface.BuildSurfaceCurveCompositionCells(ctx, uv, o)
	b.operations += composed.ExactOperations
	r.Subdivisions += composed.Subdivisions
	if e != nil {
		return r, e
	}

	if restriction != nil {
		remainingOps := options.MaxOperations - b.operations
		remainingSubs := o.MaxSubdivisions - r.Subdivisions
		if remainingOps < 1 || remainingSubs < 1 {
			return r, ErrConvergence
		}
		lo, e := restriction.EndpointRoot(0)
		if e != nil {
			return r, e
		}
		hi, e := restriction.EndpointRoot(1)
		if e != nil {
			return r, e
		}
		trimmed, e := composed.RestrictAlgebraic(ctx, lo, hi, ExactCurvePlaneEventOptions{MaxOperations: remainingOps, MaxCoefficientBits: options.MaxCoefficientBits, MaxSubdivisions: remainingSubs})
		b.operations += trimmed.ExactOperations
		r.Subdivisions += trimmed.Subdivisions
		if e != nil {
			return r, e
		}
		composed = trimmed
	}
	for _, cell := range composed.Cells {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		left, e := polynomialProduct(original.power[2], cell.power[3], b)
		if e != nil {
			return r, e
		}
		right, e := polynomialProduct(cell.power[2], original.power[3], b)
		if e != nil {
			return r, e
		}
		residual, e := subtractScaledPolynomial(left, right, big.NewRat(1, 1), b)
		if e != nil {
			return r, e
		}
		model := *original
		model.endpoints = cell.endpoints
		model.cell = cell.cell
		if r.Subdivisions >= o.MaxSubdivisions && len(residual) > 0 {
			return r, ErrConvergence
		}
		contact, e := model.intersectComposedPolynomial(ctx, residual, ExactCurvePlaneEventOptions{MaxOperations: options.MaxOperations, MaxCoefficientBits: options.MaxCoefficientBits, MaxSubdivisions: max(1, o.MaxSubdivisions-r.Subdivisions)}, b)
		r.Subdivisions += contact.Subdivisions

		report := IdentityGraphCurveCellContact{SurfaceCell: cell.cell, Intersection: contact}
		r.Cells = append(r.Cells, report)
		if e != nil {
			return r, e
		}
		for _, event := range contact.Events {
			derivative := residual
			for order := 0; order < event.multiplicity; order++ {
				derivative = polynomialDerivative(derivative)
				for _, x := range derivative {
					if e := b.check(x); e != nil {
						return r, e
					}
				}
			}
			remaining := o.MaxSubdivisions - r.Subdivisions
			if remaining < 1 {
				return r, ErrConvergence
			}
			sign, steps, e := composedPolynomialRootSign(ctx, event.root, derivative, b, remaining)
			r.Subdivisions += steps
			if e != nil {
				return r, e
			}
			if sign == 0 {
				return r, ErrConvergence
			}
			signs := [2]int{sign, sign}
			if event.multiplicity%2 == 1 {
				signs[0] = -sign
			}
			if event.endpointMask&1 != 0 {
				signs[0] = 0
			}
			if event.endpointMask&2 != 0 {
				signs[1] = 0
			}
			r.Cells[len(r.Cells)-1].EventSigns = append(r.Cells[len(r.Cells)-1].EventSigns, signs)
		}
	}
	r.Complete = composed.Complete
	return r, nil
}

func proveIdentityGraphCell(p *SurfacePatch, b *polynomialBudget) error {
	w, e := tensorPowerCoefficients(p.net, p.countU, p.countV, 3, b)
	if e != nil {
		return e
	}
	d := p.Domain()
	for axis := 0; axis < 2; axis++ {
		coord, e := tensorPowerCoefficients(p.net, p.countU, p.countV, axis, b)
		if e != nil {
			return e
		}
		lo, hi := d.UMin, d.UMax
		if axis == 1 {
			lo, hi = d.VMin, d.VMax
		}
		origin := new(big.Rat).SetFloat64(lo)
		width := new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), origin)
		for i := 0; i <= p.countU; i++ {
			for j := 0; j <= p.countV; j++ {
				value := new(big.Rat)
				if i < p.countU && j < p.countV {
					value.Sub(coord[i*p.countV+j], new(big.Rat).Mul(origin, w[i*p.countV+j]))
				}
				ii, jj := i, j
				if axis == 0 {
					ii--
				} else {
					jj--
				}
				if ii >= 0 && ii < p.countU && jj >= 0 && jj < p.countV {
					value.Sub(value, new(big.Rat).Mul(width, w[ii*p.countV+jj]))
				}
				if e := b.check(value); e != nil {
					return e
				}
				if value.Sign() != 0 {
					return ErrConvergence
				}
			}
		}
	}
	return nil
}

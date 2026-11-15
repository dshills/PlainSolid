package geom

import (
	"context"
	"math/big"
)

// PeriodicSurfaceParameterMap gives the exact correspondence
// physical = Offset + Scale*working on one axis. A periodic working axis
// has one unit per original span. An ordinary axis retains its source units.
type PeriodicSurfaceParameterMap struct {
	Offset, Scale *big.Rat
}

// PeriodicSurfaceFundamental is a certified wrapped representation of one
// original fundamental rectangle. Maps are independent of the immutable
// patch and preserve physical parameters even when a knot is nonbinary.
type PeriodicSurfaceFundamental struct {
	Wrapped         *WrappedSurfacePatch
	Maps            [2]PeriodicSurfaceParameterMap
	Seam            SurfaceSeamCertificate
	ExactOperations int
}

// PeriodicSurfaceFundamentalOptions shares limits between exact extraction,
// clamped tensor assembly and seam verification. ContinuityOrder 0 proves
// position closure. Order 1 also verifies matching first partials.
type PeriodicSurfaceFundamentalOptions struct {
	PeriodicSurfaceBezierOptions
	ContinuityOrder int
}

// ExactFundamentalPatch reassembles exact original polynomial pieces into
// a clamped tensor and validates its periodic seams. An ordinary axis must
// be continuous in homogeneous coordinates at each original knot.
func (s *PeriodicBSplineSurface) ExactFundamentalPatch(ctx context.Context, options PeriodicSurfaceFundamentalOptions) (result PeriodicSurfaceFundamental, err error) {
	if ctx == nil || s == nil || s.surface == nil || options.ContinuityOrder < 0 || options.ContinuityOrder > 1 {
		return result, ErrInvalid
	}
	if options.MaxOperations == 0 {
		options.MaxOperations = 4000000
	}
	limits, err := patchOptions(options.SurfacePatchOptions)
	if err != nil {
		return result, err
	}
	wrap := [2]bool{s.axis[0].Period != 0, s.axis[1].Period != 0}
	var spans [2]ExactParameterInterval
	var maps [2]PeriodicSurfaceParameterMap
	for direction, axis := range s.axis {
		if wrap[direction] {
			offset, step := exactUniformPeriodicAxis(s.count[direction], axis.Start, axis.Period)
			maps[direction] = PeriodicSurfaceParameterMap{Offset: offset, Scale: step}
			spans[direction] = ExactParameterInterval{Lower: offset, Upper: new(big.Rat).Add(offset, new(big.Rat).SetFloat64(axis.Period))}
		} else {
			degree, count, knots := s.tensorSplineAxis(direction)
			maps[direction] = PeriodicSurfaceParameterMap{Offset: new(big.Rat), Scale: big.NewRat(1, 1)}
			spans[direction] = ExactParameterInterval{Lower: new(big.Rat).SetFloat64(knots[degree]), Upper: new(big.Rat).SetFloat64(knots[count])}
		}
	}
	cells, work, err := s.ExactBezierCells(ctx, spans[0], spans[1], options.PeriodicSurfaceBezierOptions)
	budget := &polynomialBudget{operations: work, maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { result.ExactOperations = budget.operations }()
	if err != nil {
		return result, err
	}
	patch, err := assemblePeriodicFundamental(ctx, cells, wrap, limits, budget)
	if err != nil {
		return result, err
	}
	remaining := limits.MaxOperations - budget.operations
	if remaining < 1 {
		return result, ErrConvergence
	}
	w, seam, err := NewWrappedSurfacePatch(ctx, patch, WrappedSurfaceOptions{
		WrapU: wrap[0], WrapV: wrap[1], ContinuityOrder: options.ContinuityOrder,
		MaxPoles: limits.MaxPoles, MaxOperations: remaining, MaxCoefficientBits: limits.MaxCoefficientBits,
	})
	budget.operations += seam.ExactOperations
	if err != nil {
		return result, err
	}
	w.options.MaxOperations = limits.MaxOperations
	result.Wrapped, result.Maps, result.Seam = w, maps, seam
	return result, nil
}

func assemblePeriodicFundamental(ctx context.Context, cells []ExactPeriodicSurfaceBezierCell, wrap [2]bool, limits SurfacePatchOptions, budget *polynomialBudget) (*SurfacePatch, error) {
	work := budget.operations
	nv := 1
	for nv < len(cells) && cells[nv].U.Domain.Lower.Cmp(cells[0].U.Domain.Lower) == 0 {
		nv++
	}
	nu := len(cells) / nv
	p, q := cells[0].DegreeU, cells[0].DegreeV
	countU, countV := nu*p+1, nv*q+1
	if countU > limits.MaxPoles/countV {
		return nil, ErrConvergence
	}
	patch := &SurfacePatch{
		degreeU: p, degreeV: q, countU: countU, countV: countV,
		knotsU: fundamentalPeriodicKnots(cells, nu, nv, p, true, wrap[0]),
		knotsV: fundamentalPeriodicKnots(cells, nv, 1, q, false, wrap[1]),
		net:    make([]exactHomogeneous, countU*countV), options: limits,
	}
	for i, cell := range cells {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		u, v := i/nv*p, i%nv*q
		for a := 0; a <= p; a++ {
			for z := 0; z <= q; z++ {
				src := cell.Control[a*(q+1)+z]
				dst := &patch.net[(u+a)*countV+v+z]
				if dst[3] == nil {
					*dst = src
					continue
				}
				for axis, value := range src {
					if err := budget.check(value); err != nil {
						return nil, err
					}
					if value.Cmp(dst[axis]) != 0 {
						return nil, ErrDomain
					}
				}
			}
		}
	}
	if err := patch.computeBounds(budget); err != nil {
		return nil, err
	}
	patch.operations = budget.operations - work
	return patch, nil
}

func fundamentalPeriodicKnots(cells []ExactPeriodicSurfaceBezierCell, count, stride, degree int, u, wrapped bool) []float64 {
	knots := make([]float64, 0, count*degree+degree+2)
	coordinate := func(i int) float64 {
		if wrapped {
			return float64(i)
		}
		cell := cells[min(i, count-1)*stride]
		span := cell.V.Domain
		if u {
			span = cell.U.Domain
		}
		value := span.Lower
		if i == count {
			value = span.Upper
		}
		out, _ := value.Float64()
		return out
	}
	knots = append(knots, coordinate(0))
	for i := 0; i <= count; i++ {
		x := coordinate(i)
		for j := 0; j < degree; j++ {
			knots = append(knots, x)
		}
	}
	return append(knots, coordinate(count))
}

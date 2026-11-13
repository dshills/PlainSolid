package geom

import (
	"context"
	"math/big"
	"sort"
)

type graphAxisTrace struct {
	span       ParameterSpan
	polynomial rationalPolynomial
}

func certifyMultispanSeparableContact(ctx context.Context, a, p *SurfacePatch, uv [2]*big.Rat, o SeparableGraphContactOptions, b *polynomialBudget) (r SeparableSplineGraphContact, err error) {
	d := p.Domain()
	limits := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	for axis, x := range uv {
		if x.Cmp(new(big.Rat).SetFloat64(limits[axis][0])) < 0 || x.Cmp(new(big.Rat).SetFloat64(limits[axis][1])) > 0 {
			return r, ErrDomain
		}
	}
	cells, e := graphContactBezierCells(ctx, p, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	_, e = graphContactC1(ctx, cells, b)
	if e != nil {
		return r, e
	}
	var traces [2][]graphAxisTrace
	seen := [2]map[ParameterSpan]bool{{}, {}}
	for _, cell := range cells {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		if ok, e := separableGraphCell(cell, b); e != nil {
			return r, e
		} else if !ok {
			return r, ErrConvergence
		}
		cd := cell.Domain()
		local := [2][2]float64{{cd.UMin, cd.UMax}, {cd.VMin, cd.VMax}}
		for axis := 0; axis < 2; axis++ {
			fixed := 1 - axis
			lo, hi := new(big.Rat).SetFloat64(local[fixed][0]), new(big.Rat).SetFloat64(local[fixed][1])
			if uv[fixed].Cmp(lo) < 0 || uv[fixed].Cmp(hi) > 0 {
				continue
			}
			span := ParameterSpan{Start: local[axis][0], End: local[axis][1]}
			if seen[axis][span] {
				continue
			}
			seen[axis][span] = true
			t := new(big.Rat).Quo(new(big.Rat).Sub(uv[fixed], lo), new(big.Rat).Sub(hi, lo))
			count, varying := cell.countU, cell.countV
			if axis == 1 {
				count, varying = varying, count
			}
			values := make([]*big.Rat, count)
			for i := 0; i < count; i++ {
				line := make([]*big.Rat, varying)
				for j := range line {
					index := i*cell.countV + j
					if axis == 1 {
						index = j*cell.countV + i
					}
					line[j] = cell.net[index][2]
				}
				values[i], e = scalarBernsteinValue(line, t, b)
				if e != nil {
					return r, e
				}
			}
			poly, e := bernsteinToPower(values, b)
			if e != nil {
				return r, e
			}
			traces[axis] = append(traces[axis], graphAxisTrace{span, poly})
		}
	}
	// Local additive identities and exact C1 (in particular C0) compatibility
	// imply one additive decomposition on the entire tensor rectangle. Traces
	// through the witness then sum to F, provided F(witness)=0.
	remaining := o.MaxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	jet, e := p.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += jet.ExactOperations
	if e != nil {
		return r, e
	}
	if jet.Point[2].Sign() != 0 {
		return r, ErrConvergence
	}
	commonSide := 0
	for axis, list := range traces {
		sort.Slice(list, func(i, j int) bool { return list[i].span.Start < list[j].span.Start })
		if len(list) == 0 || list[0].span.Start != limits[axis][0] || list[len(list)-1].span.End != limits[axis][1] {
			return r, ErrConvergence
		}
		foundRoot := false
		for index, trace := range list {
			if index > 0 && list[index-1].span.End != trace.span.Start {
				return r, ErrConvergence
			}
			if len(trace.polynomial) == 0 {
				return r, ErrConvergence
			}
			lo, hi := new(big.Rat).SetFloat64(trace.span.Start), new(big.Rat).SetFloat64(trace.span.End)
			inside := uv[axis].Cmp(lo) >= 0 && uv[axis].Cmp(hi) <= 0
			target := new(big.Rat).Quo(new(big.Rat).Sub(uv[axis], lo), new(big.Rat).Sub(hi, lo))
			if inside {
				value, e := polynomialValue(trace.polynomial, target, b)
				if e != nil {
					return r, e
				}
				if value.Sign() != 0 {
					return r, ErrConvergence
				}
			}
			count, order := 0, 0
			remaining := o.MaxRootSubdivisions - r.RootSubdivisions
			if remaining < 1 {
				return r, ErrConvergence
			}
			n, e := isolatePolynomialRoots(trace.polynomial, new(big.Rat), big.NewRat(1, 1), b, remaining, func(_, _ *big.Rat, m int) (bool, error) {
				if e := ctx.Err(); e != nil {
					return false, e
				}
				count++
				order = m
				return true, nil
			})
			r.RootSubdivisions += n
			if e != nil {
				return r, e
			}
			if (inside && count != 1) || (!inside && count != 0) {
				return r, ErrConvergence
			}
			if inside {
				foundRoot = true
				if r.Orders[axis] == 0 || order < r.Orders[axis] {
					r.Orders[axis] = order
				}
			}
			var samples []*big.Rat
			if inside {
				if target.Sign() > 0 {
					samples = append(samples, new(big.Rat).Mul(target, big.NewRat(1, 2)))
				}
				if target.Cmp(big.NewRat(1, 1)) < 0 {
					samples = append(samples, new(big.Rat).Mul(new(big.Rat).Add(target, big.NewRat(1, 1)), big.NewRat(1, 2)))
				}
			} else {
				samples = append(samples, big.NewRat(1, 2))
			}
			for _, sample := range samples {
				value, e := polynomialValue(trace.polynomial, sample, b)
				if e != nil {
					return r, e
				}
				if value.Sign() == 0 || (commonSide != 0 && commonSide != value.Sign()) {
					return r, ErrConvergence
				}
				commonSide = value.Sign()
			}
		}
		if !foundRoot {
			return r, ErrConvergence
		}
	}
	remaining = o.MaxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	r.Graph, e = CertifySurfaceInjectivity(a, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxDepth: o.MaxDepth, MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += r.Graph.ExactOperations
	if e != nil {
		return r, e
	}
	if !r.Graph.ProvenInjective {
		return r, ErrConvergence
	}
	remaining = o.MaxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	point, e := a.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += point.ExactOperations
	if e != nil {
		return r, e
	}
	r.Point = point.Point
	r.Parameters = [2]*big.Rat{new(big.Rat).Set(uv[0]), new(big.Rat).Set(uv[1])}
	r.Side = commonSide
	r.Proven = true
	return r, nil
}
func separableGraphCell(p *SurfacePatch, b *polynomialBudget) (bool, error) {
	rows := make([]rationalPolynomial, p.countU)
	for i := range rows {
		values := make([]*big.Rat, p.countV)
		for j := range values {
			values[j] = p.net[i*p.countV+j][2]
		}
		var e error
		rows[i], e = bernsteinToPower(values, b)
		if e != nil {
			return false, e
		}
	}
	for j := 1; j < p.countV; j++ {
		values := make([]*big.Rat, p.countU)
		for i := range values {
			values[i] = new(big.Rat)
			if j < len(rows[i]) {
				values[i].Set(rows[i][j])
			}
		}
		poly, e := bernsteinToPower(values, b)
		if e != nil {
			return false, e
		}
		for i := 1; i < len(poly); i++ {
			if poly[i].Sign() != 0 {
				return false, nil
			}
		}
	}
	return true, nil
}

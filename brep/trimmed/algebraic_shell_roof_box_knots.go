package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

var errMixedRoofPiecewise = fmt.Errorf("distinct homogeneous roof spans: %w", geom.ErrUnrepresentable)

// Recover one global homogeneous polynomial from an exactly knot-refined
// tensor patch. Each independent span is pulled back to whole-chart units
// and must give precisely the same homogeneous coefficients. A genuine
// piecewise chart is not eligible for the current whole-column line sweep.
func mixedRoofWholePower(ctx context.Context, p *geom.SurfacePatch, d geom.ParameterRectangle, limit int) ([][][4]*big.Rat, *big.Rat, int, error) {
	u, v := p.KnotBreaks()
	if len(u) < 2 || len(v) < 2 || (len(u)-1)*(len(v)-1) > 64 {
		return nil, nil, 0, geom.ErrUnrepresentable
	}
	w := mixedRoofLineWork{o: MixedRoofLineOptions{SurfacePatchEvaluationOptions: geom.SurfacePatchEvaluationOptions{MaxOperations: limit, MaxCoefficientBits: 4096}}}
	var first [][][4]*big.Rat
	var minimum *big.Rat
	for a := 1; a < len(u); a++ {
		for b := 1; b < len(v); b++ {
			if err := ctx.Err(); err != nil {
				return nil, nil, w.operations, err
			}
			if w.operations >= limit {
				return nil, nil, w.operations, geom.ErrConvergence
			}
			span := geom.ParameterRectangle{UMin: u[a-1], UMax: u[a], VMin: v[b-1], VMax: v[b]}
			cell := p
			if span != d {
				var work int
				var err error
				cell, work, err = p.TrimWithWorkLimit(span, limit-w.operations)
				w.operations += work
				if err != nil {
					return nil, nil, w.operations, err
				}
			}
			if w.operations >= limit {
				return nil, nil, w.operations, geom.ErrConvergence
			}
			coeff, mw, work, ok := cell.RationalBezierCoefficientsWithWork(mixedRoofMaxDegree, limit-w.operations)
			w.operations += work
			if !ok {
				if w.operations >= limit {
					return nil, nil, w.operations, geom.ErrConvergence
				}
				return nil, nil, w.operations, geom.ErrUnrepresentable
			}
			if span == d {
				return coeff, mw, w.operations, nil
			}
			global, err := mixedRoofGlobalPower(coeff, span, d, &w)
			if err != nil {
				return nil, nil, w.operations, err
			}
			if first == nil {
				first, minimum = global, mw
				continue
			}
			minimum = minRat(minimum, mw)
			for i, row := range global {
				for j, c := range row {
					for axis, x := range c {
						if x.Cmp(first[i][j][axis]) != 0 {
							return nil, nil, w.operations, errMixedRoofPiecewise
						}
					}
				}
			}
		}
	}
	return first, minimum, w.operations, nil
}

func mixedRoofSpans(ctx context.Context, p *geom.SurfacePatch, limit int) ([]*mixedRoofBox, int, error) {
	u, v := p.KnotBreaks()
	var spans []*mixedRoofBox
	work := 0
	for a := 1; a < len(u); a++ {
		for b := 1; b < len(v); b++ {
			if err := ctx.Err(); err != nil {
				return nil, work, err
			}
			if work >= limit {
				return nil, work, geom.ErrConvergence
			}
			d := geom.ParameterRectangle{UMin: u[a-1], UMax: u[a], VMin: v[b-1], VMax: v[b]}
			cell, n, err := p.TrimWithWorkLimit(d, limit-work)
			work += n
			if err != nil {
				return nil, work, err
			}
			if work >= limit {
				return nil, work, geom.ErrConvergence
			}
			coeff, w, n, ok := cell.RationalBezierCoefficientsWithWork(mixedRoofMaxDegree, limit-work)
			work += n
			if !ok {
				if work >= limit {
					return nil, work, geom.ErrConvergence
				}
				return nil, work, geom.ErrUnrepresentable
			}
			span := &mixedRoofBox{domain: d, roof: cell}
			err = span.setRoofPower(coeff, w, limit-work)
			work += span.operations
			if err != nil {
				return nil, work, err
			}
			spans = append(spans, span)
		}
	}
	return spans, work, nil
}

func mixedRoofGlobalPower(poly [][][4]*big.Rat, local, whole geom.ParameterRectangle, w *mixedRoofLineWork) ([][][4]*big.Rat, error) {
	var powers [2][][]*big.Rat
	for i, pairs := range [2][4]float64{
		{local.UMin, local.UMax, whole.UMin, whole.UMax}, {local.VMin, local.VMax, whole.VMin, whole.VMax},
	} {
		span, err := w.rat(new(big.Rat).Sub(ratInset(pairs[1]), ratInset(pairs[0])))
		if err != nil {
			return nil, err
		}
		offset, err := w.rat(new(big.Rat).Sub(ratInset(pairs[2]), ratInset(pairs[0])))
		if err != nil {
			return nil, err
		}
		if _, err = w.rat(offset.Quo(offset, span)); err != nil {
			return nil, err
		}
		scale, err := w.rat(new(big.Rat).Sub(ratInset(pairs[3]), ratInset(pairs[2])))
		if err != nil {
			return nil, err
		}
		if _, err = w.rat(scale.Quo(scale, span)); err != nil {
			return nil, err
		}
		m := len(poly)
		if i == 1 {
			m = len(poly[0])
		}
		powers[i] = make([][]*big.Rat, m)
		powers[i][0] = []*big.Rat{big.NewRat(1, 1)}
		for j := 1; j < m; j++ {
			row := make([]*big.Rat, j+1)
			for k := range row {
				row[k] = new(big.Rat)
			}
			for k, x := range powers[i][j-1] {
				for z, coefficient := range [2]*big.Rat{offset, scale} {
					v, err := w.rat(new(big.Rat).Mul(x, coefficient))
					if err != nil {
						return nil, err
					}
					if _, err = w.rat(row[k+z].Add(row[k+z], v)); err != nil {
						return nil, err
					}
				}
			}
			powers[i][j] = row
		}
	}
	out := make([][][4]*big.Rat, len(poly))
	for i, row := range poly {
		out[i] = make([][4]*big.Rat, len(row))
		for j := range row {
			for axis := range out[i][j] {
				out[i][j][axis] = new(big.Rat)
			}
		}
	}
	for i, row := range poly {
		for j, term := range row {
			for a, x := range powers[0][i] {
				for b, y := range powers[1][j] {
					if x.Sign() == 0 || y.Sign() == 0 {
						continue
					}
					xy, err := w.rat(new(big.Rat).Mul(x, y))
					if err != nil {
						return nil, err
					}
					for k, c := range term {
						if c.Sign() == 0 {
							continue
						}
						v, err := w.rat(new(big.Rat).Mul(xy, c))
						if err != nil {
							return nil, err
						}
						z := out[a][b][k]
						if _, err = w.rat(z.Add(z, v)); err != nil {
							return nil, err
						}
					}
				}
			}
		}
	}
	return out, nil
}

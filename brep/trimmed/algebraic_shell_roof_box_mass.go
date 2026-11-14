package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// MixedRoofMassOptions bounds horizontal subdivision for original roof box
// moments. MaxDepth defaults to eight and MaxCells to 32768. Cells with a
// proved order and polynomial thickness need no approximation for volume.
type MixedRoofMassOptions struct {
	geom.SurfacePatchEvaluationOptions
	MaxDepth, MaxCells int
}

// MixedRoofMass encloses unit-density measures in model coordinates. Second
// moments are integral(x_i*x_j); inertia is about the model origin. Centroid
// is available only when the lower volume bound is positive.
type MixedRoofMass struct {
	Volume                                       geom.ExactParameterInterval
	FirstMoment, Centroid                        [3]geom.ExactParameterInterval
	SecondMoment, Inertia                        [3][3]geom.ExactParameterInterval
	VisitedCells, EnclosedCells, ExactOperations int
}

var mixedRoofMomentAxes = [10][3]int{
	{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 0, 1},
	{2, 0, 0}, {0, 2, 0}, {0, 0, 2}, {1, 1, 0}, {1, 0, 1}, {0, 1, 1},
}

func mixedRoofZeroInterval() geom.ExactParameterInterval {
	return geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)}
}

// BoundMass integrates columns on the original supports. On a proved cell
// it integrates polynomial primitives exactly. Otherwise an inner and outer
// material column enclose all contributions, including signed products over
// coordinates crossing zero. Exhaustion or cancellation publishes no output.
func (q *MixedRoofMaterialQuery) BoundMass(ctx context.Context, o MixedRoofMassOptions) (MixedRoofMass, error) {
	var zero MixedRoofMass
	var err error
	o.SurfacePatchEvaluationOptions, err = q.evaluationOptions(ctx, o.SurfacePatchEvaluationOptions)
	if err != nil {
		return zero, err
	}
	if o.MaxDepth < 0 || o.MaxDepth > 32 || o.MaxCells < 0 || o.MaxCells > 262144 {
		return zero, geom.ErrInvalid
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 8
	}
	if o.MaxCells == 0 {
		o.MaxCells = 32768
	}
	w := mixedRoofLineWork{o: MixedRoofLineOptions{SurfacePatchEvaluationOptions: o.SurfacePatchEvaluationOptions}}
	var sum [10]geom.ExactParameterInterval
	for i := range sum {
		sum[i] = mixedRoofZeroInterval()
	}
	var out MixedRoofMass
	var roofPowers [2][3]roofPowerGrid
	stack := mixedRoofVolumeFootprint(q)
	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		if out.VisitedCells == o.MaxCells {
			return zero, geom.ErrConvergence
		}
		out.VisitedCells++
		c := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		var uv [2][2]geom.ExactParameterInterval
		var h [2]geom.ExactParameterInterval
		var floor [2]*big.Rat
		for i, b := range q.box {
			floor[i] = b.floor
			h[i] = geom.ExactParameterInterval{Lower: b.floor, Upper: b.floor}
			if !c.chart[i] {
				continue
			}
			uv[i], err = b.volumeRoofDomain(c.xy, &w)
			if err != nil {
				return zero, err
			}
			h[i], err = b.roofHeightRange(uv[i], &w)
			if err != nil {
				return zero, err
			}
			h[i].Lower = maxRat(h[i].Lower, b.floor)
		}
		roof, base, proved := q.fixedColumnPrimitives(c.chart, h)
		for i, coefficient := range roof {
			if coefficient != 0 && (q.box[i].weight != nil || len(q.box[i].roofSpans) != 0) {
				proved = false
			}
		}
		var measure [10]geom.ExactParameterInterval
		if proved {
			measure, err = q.exactRoofCellMoments(c.xy, uv, roof, base, &roofPowers, &w)
		} else if c.depth < o.MaxDepth {
			axis := c.depth % 2
			m, e := w.rat(new(big.Rat).Add(c.xy[axis].Lower, c.xy[axis].Upper))
			if e != nil {
				return zero, e
			}
			if _, e := w.rat(m.Quo(m, big.NewRat(2, 1))); e != nil {
				return zero, e
			}
			c.depth++
			other := c
			c.xy[axis].Upper, other.xy[axis].Lower = m, m
			stack = append(stack, c, other)
			continue
		} else {
			out.EnclosedCells++
			lo, hi := mixedRoofColumnSandwich(c.chart, q.op, h, floor)
			measure, err = enclosedRoofCellMoments(c.xy, lo, hi, &w)
		}
		if err != nil {
			return zero, err
		}
		for i, m := range measure {
			if _, err = w.rat(sum[i].Lower.Add(sum[i].Lower, m.Lower)); err != nil {
				return zero, err
			}
			if _, err = w.rat(sum[i].Upper.Add(sum[i].Upper, m.Upper)); err != nil {
				return zero, err
			}
		}
	}
	out.Volume = sum[0]
	copy(out.FirstMoment[:], sum[1:4])
	for i := range 3 {
		out.SecondMoment[i][i] = sum[4+i]
		if out.Volume.Lower.Sign() > 0 {
			for _, n := range []*big.Rat{out.FirstMoment[i].Lower, out.FirstMoment[i].Upper} {
				for _, d := range []*big.Rat{out.Volume.Lower, out.Volume.Upper} {
					v, err := w.rat(new(big.Rat).Quo(n, d))
					if err != nil {
						return zero, err
					}
					mixedRoofInclude(&out.Centroid[i], v)
				}
			}
		}
	}
	for i, pair := range [3][2]int{{0, 1}, {0, 2}, {1, 2}} {
		a, b := pair[0], pair[1]
		out.SecondMoment[a][b], out.SecondMoment[b][a] = sum[7+i], mixedRoofCopyInterval(sum[7+i])
		lo, err := w.rat(new(big.Rat).Neg(sum[7+i].Upper))
		if err != nil {
			return zero, err
		}
		hi, err := w.rat(new(big.Rat).Neg(sum[7+i].Lower))
		if err != nil {
			return zero, err
		}
		inv := geom.ExactParameterInterval{Lower: lo, Upper: hi}
		out.Inertia[a][b], out.Inertia[b][a] = inv, mixedRoofCopyInterval(inv)
	}
	for i := range 3 {
		a, b := sum[4+(i+1)%3], sum[4+(i+2)%3]
		lo, err := w.rat(new(big.Rat).Add(a.Lower, b.Lower))
		if err != nil {
			return zero, err
		}
		hi, err := w.rat(new(big.Rat).Add(a.Upper, b.Upper))
		if err != nil {
			return zero, err
		}
		out.Inertia[i][i] = geom.ExactParameterInterval{Lower: lo, Upper: hi}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	out.ExactOperations = w.operations
	return out, nil
}

func mixedRoofInclude(r *geom.ExactParameterInterval, v *big.Rat) {
	if r.Lower == nil || v.Cmp(r.Lower) < 0 {
		r.Lower = v
	}
	if r.Upper == nil || v.Cmp(r.Upper) > 0 {
		r.Upper = v
	}
}

func mixedRoofCopyInterval(r geom.ExactParameterInterval) geom.ExactParameterInterval {
	return geom.ExactParameterInterval{Lower: new(big.Rat).Set(r.Lower), Upper: new(big.Rat).Set(r.Upper)}
}

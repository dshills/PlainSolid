package trimmed

import (
	"math/big"

	"plainsolid/geom"
)

// On a cell with proved column order, the material thickness is an affine
// combination of the two original heights and floors. Integrate each active
// polynomial exactly; a rational height that cancels does not prevent this.
// A nil average leaves the conservative pointwise enclosure in force.
func (q *MixedRoofMaterialQuery) exactPolynomialColumnAverage(chart [2]bool, uv [2][2]geom.ExactParameterInterval, height [2]geom.ExactParameterInterval, w *mixedRoofLineWork) (*big.Rat, error) {
	roof, floor, proven := q.fixedColumnPrimitives(chart, height)
	if !proven {
		return nil, nil
	}
	for i, c := range roof {
		if c != 0 && (q.box[i].weight != nil || len(q.box[i].roofSpans) != 0) {
			return nil, nil
		}
	}
	avg := new(big.Rat)
	accumulate := func(c int, r *big.Rat) error {
		if c == 0 {
			return nil
		}
		if c > 0 {
			avg.Add(avg, r)
		} else {
			avg.Sub(avg, r)
		}
		_, err := w.rat(avg)
		return err
	}
	for i, b := range q.box {
		if err := accumulate(floor[i], b.floor); err != nil {
			return nil, err
		}
		if roof[i] != 0 {
			r, err := mixedRoofAveragePower(b.linePower, uv[i], w)
			if err != nil {
				return nil, err
			}
			if err = accumulate(roof[i], r); err != nil {
				return nil, err
			}
		}
	}
	return avg, nil
}

// Average powers over [a,b] using (a^k + a^(k-1)b + … + b^k)/(k+1).
// This form also avoids dividing by an extremely narrow cell width.
func mixedRoofAveragePower(power [][]*big.Rat, uv [2]geom.ExactParameterInterval, w *mixedRoofLineWork) (*big.Rat, error) {
	degree := [2]int{}
	for i, row := range power {
		for j, c := range row {
			if c.Sign() != 0 {
				degree[0], degree[1] = max(degree[0], i), max(degree[1], j)
			}
		}
	}
	var avg [2][mixedRoofMaxDegree + 1]*big.Rat
	for i, s := range uv {
		avg[i][0] = big.NewRat(1, 1)
		p, sum := big.NewRat(1, 1), big.NewRat(1, 1)
		for k := 1; k <= degree[i]; k++ {
			if _, err := w.rat(p.Mul(p, s.Upper)); err != nil {
				return nil, err
			}
			if _, err := w.rat(sum.Mul(sum, s.Lower)); err != nil {
				return nil, err
			}
			if _, err := w.rat(sum.Add(sum, p)); err != nil {
				return nil, err
			}
			r, err := w.rat(new(big.Rat).Quo(sum, big.NewRat(int64(k+1), 1)))
			if err != nil {
				return nil, err
			}
			avg[i][k] = r
		}
	}
	out := new(big.Rat)
	for i, row := range power {
		for j, c := range row {
			if c.Sign() == 0 {
				continue
			}
			p, err := w.rat(new(big.Rat).Mul(avg[0][i], avg[1][j]))
			if err != nil {
				return nil, err
			}
			if _, err = w.rat(p.Mul(p, c)); err != nil {
				return nil, err
			}
			if _, err = w.rat(out.Add(out, p)); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

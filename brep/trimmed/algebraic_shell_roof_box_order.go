package trimmed

import (
	"math/big"

	"plainsolid/geom"
)

// roofOrderChart retains the signed homogeneous difference of two roofs on
// their exact common chart, (A_z B_w - B_z A_w). Original positive weights
// make its sign the height order. Combining the coefficients before bounding
// cancels correlations that separate absolute height intervals cannot see.
type roofOrderChart struct {
	power roofPowerGrid
	chart mixedRoofBox
}

type roofOrderCache map[[2]*mixedRoofBox]roofOrderChart

func (c roofOrderCache) order(boxes [2]*mixedRoofBox, xy [2]geom.ExactParameterInterval, w *mixedRoofLineWork) (geom.ExactParameterInterval, error) {
	var zero geom.ExactParameterInterval
	if boxes[0] == nil || boxes[1] == nil {
		return zero, geom.ErrInvalid
	}
	d, ok := c[boxes]
	if !ok {
		a, b := boxes[0].domain, boxes[1].domain
		d.chart.domain = geom.ParameterRectangle{
			UMin: max(a.UMin, b.UMin), UMax: min(a.UMax, b.UMax),
			VMin: max(a.VMin, b.VMin), VMax: min(a.VMax, b.VMax),
		}
		if d.chart.domain.UMin >= d.chart.domain.UMax || d.chart.domain.VMin >= d.chart.domain.VMax {
			return zero, geom.ErrUnrepresentable
		}
		var numerator, weight [2]roofPowerGrid
		for i, s := range boxes {
			p := make([][][4]*big.Rat, len(s.linePower))
			for j, row := range s.linePower {
				p[j] = make([][4]*big.Rat, len(row))
				for k, n := range row {
					p[j][k][2] = n
					if s.weight != nil {
						p[j][k][3] = s.weight[j][k]
					} else if j == 0 && k == 0 {
						p[j][k][3] = big.NewRat(1, 1)
					}
					for axis, v := range p[j][k] {
						if v == nil {
							p[j][k][axis] = new(big.Rat)
						}
					}
				}
			}
			p, err := mixedRoofGlobalPower(p, s.domain, d.chart.domain, w)
			if err != nil {
				return zero, err
			}
			numerator[i], weight[i] = make(roofPowerGrid, len(p)), make(roofPowerGrid, len(p))
			for u, row := range p {
				numerator[i][u], weight[i][u] = make([]*big.Rat, len(row)), make([]*big.Rat, len(row))
				for v, q := range row {
					numerator[i][u][v], weight[i][u][v] = q[2], q[3]
				}
			}
			numerator[i], weight[i] = trimRoofPower(numerator[i]), trimRoofPower(weight[i])
		}
		l, err := roofMultiplyPower(numerator[0], weight[1], w)
		if err != nil {
			return zero, err
		}
		r, err := roofMultiplyPower(numerator[1], weight[0], w)
		if err != nil {
			return zero, err
		}
		d.power = make(roofPowerGrid, max(len(l), len(r)))
		for i := range d.power {
			d.power[i] = make([]*big.Rat, max(len(l[0]), len(r[0])))
			for j := range d.power[i] {
				v := new(big.Rat)
				if i < len(l) && j < len(l[0]) {
					v.Set(l[i][j])
				}
				if i < len(r) && j < len(r[0]) {
					if _, err := w.rat(v.Sub(v, r[i][j])); err != nil {
						return zero, err
					}
				}
				d.power[i][j] = v
			}
		}
		d.power = trimRoofPower(d.power)
		c[boxes] = d
	}
	uv, err := d.chart.volumeRoofDomain(xy, w)
	if err != nil {
		return zero, err
	}
	return mixedRoofPowerRange(d.power, uv, w)
}

func trimRoofPower(g roofPowerGrid) roofPowerGrid {
	width, height := 1, 1
	for i, row := range g {
		for j, v := range row {
			if v.Sign() != 0 {
				width, height = max(width, i+1), max(height, j+1)
			}
		}
	}
	for i := 0; i < width; i++ {
		g[i] = g[i][:height]
	}
	return g[:width]
}

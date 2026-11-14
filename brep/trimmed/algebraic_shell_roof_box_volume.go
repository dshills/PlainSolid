package trimmed

import (
	"container/heap"
	"context"
	"errors"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// MixedRoofVolumeOptions bounds a certified integral of the original roof
// heights. AbsoluteGap defaults to 1/100 cubic model units; MaxDepth and
// MaxCells default to 24 and 32768.
type MixedRoofVolumeOptions struct {
	geom.SurfacePatchEvaluationOptions
	AbsoluteGap        *big.Rat
	MaxDepth, MaxCells int
}

// MixedRoofVolume is a positive-material volume enclosure in original units.
// Converged means the requested absolute width was reached. Cell or arithmetic
// limits may retain a wider valid interval; cancellation publishes nothing.
type MixedRoofVolume struct {
	Volume                        geom.ExactParameterInterval
	VisitedCells, ExactOperations int
	Converged                     bool
}
type mixedRoofVolumeCell struct {
	xy    [2]geom.ExactParameterInterval
	chart [2]bool
	value geom.ExactParameterInterval
	width *big.Rat
	depth int
}
type mixedRoofVolumeHeap []mixedRoofVolumeCell

func (h mixedRoofVolumeHeap) Len() int           { return len(h) }
func (h mixedRoofVolumeHeap) Less(i, j int) bool { return h[i].width.Cmp(h[j].width) > 0 }
func (h mixedRoofVolumeHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *mixedRoofVolumeHeap) Push(x any)        { *h = append(*h, x.(mixedRoofVolumeCell)) }
func (h *mixedRoofVolumeHeap) Pop() any {
	a := *h
	n := len(a) - 1
	x := a[n]
	a[n] = mixedRoofVolumeCell{}
	*h = a[:n]
	return x
}

func mixedRoofVolumeBound(chart [2]bool, op parametric.BooleanOp, a, b geom.ExactParameterInterval, fa, fb *big.Rat, w *mixedRoofLineWork) (geom.ExactParameterInterval, error) {
	var out geom.ExactParameterInterval
	for i, upper := range [2]bool{false, true} {
		h1, h2 := a.Lower, b.Lower
		if upper {
			h1, h2 = a.Upper, b.Upper
		}
		// Difference is nondecreasing in stock height and nonincreasing
		// in tool height. All other operations are increasing in both.
		if op == parametric.Difference {
			h2 = b.Upper
			if upper {
				h2 = b.Lower
			}
		}
		at := new(big.Rat)
		switch {
		case op == parametric.Intersection && chart[0] && chart[1]:
			at.Sub(minRat(h1, h2), maxRat(fa, fb))
		case op == parametric.Union && chart[0] && chart[1]:
			at.Sub(h1, fa)
			if _, err := w.rat(at); err != nil {
				return out, err
			}
			at.Add(at, new(big.Rat).Sub(h2, fb))
			if _, err := w.rat(at); err != nil {
				return out, err
			}
			overlap := new(big.Rat).Sub(minRat(h1, h2), maxRat(fa, fb))
			if overlap.Sign() > 0 {
				at.Sub(at, overlap)
			}
		case op == parametric.Difference && chart[0] && chart[1]:
			at.Sub(h1, fa)
			if _, err := w.rat(at); err != nil {
				return out, err
			}
			overlap := new(big.Rat).Sub(minRat(h1, h2), maxRat(fa, fb))
			if overlap.Sign() > 0 {
				at.Sub(at, overlap)
			}
		case op != parametric.Intersection && chart[0]:
			at.Sub(h1, fa)
		case op == parametric.Union && chart[1]:
			at.Sub(h2, fb)
		}
		if at.Sign() < 0 {
			at.SetInt64(0)
		}
		if _, err := w.rat(at); err != nil {
			return out, err
		}
		if i == 0 {
			out.Lower = at
		} else {
			out.Upper = at
		}
	}
	return out, nil
}

func (b *mixedRoofBox) volumeRoofDomain(xy [2]geom.ExactParameterInterval, w *mixedRoofLineWork) ([2]geom.ExactParameterInterval, error) {
	var u [2]geom.ExactParameterInterval
	d := b.domain
	for i, x := range [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}} {
		a, c := ratInset(x[0]), ratInset(x[1])
		delta, err := w.rat(new(big.Rat).Sub(c, a))
		if err != nil {
			return u, err
		}
		for j, p := range [2]*big.Rat{xy[i].Lower, xy[i].Upper} {
			q, err := w.rat(new(big.Rat).Sub(p, a))
			if err != nil {
				return u, err
			}
			if _, err = w.rat(q.Quo(q, delta)); err != nil {
				return u, err
			}
			if j == 0 {
				u[i].Lower = q
			} else {
				u[i].Upper = q
			}
		}
	}
	return u, nil
}

// BoundVolume integrates the material thickness over the original XY
// footprint. Exact footprint cuts precede integration, and pointwise roof
// intervals bound the remaining overlap including unresolved section cells.
func (q *MixedRoofMaterialQuery) BoundVolume(ctx context.Context, o MixedRoofVolumeOptions) (MixedRoofVolume, error) {
	var zero MixedRoofVolume
	if o.AbsoluteGap == nil {
		o.AbsoluteGap = big.NewRat(1, 100)
	}
	var err error
	o.SurfacePatchEvaluationOptions, err = q.evaluationOptions(ctx, o.SurfacePatchEvaluationOptions, o.AbsoluteGap)
	if err != nil {
		return zero, err
	}
	if o.AbsoluteGap.Sign() <= 0 || o.MaxDepth < 0 || o.MaxDepth > 40 || o.MaxCells < 0 || o.MaxCells > 262144 {
		return zero, geom.ErrInvalid
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 24
	}
	if o.MaxCells == 0 {
		o.MaxCells = 32768
	}
	w := mixedRoofLineWork{o: MixedRoofLineOptions{SurfacePatchEvaluationOptions: o.SurfacePatchEvaluationOptions}}
	qh := mixedRoofVolumeHeap{}
	total := geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)}
	calculate := func(c mixedRoofVolumeCell) (mixedRoofVolumeCell, error) {
		if err := ctx.Err(); err != nil {
			return c, err
		}
		heights := [2]geom.ExactParameterInterval{}
		var uv [2][2]geom.ExactParameterInterval
		for i, b := range q.box {
			heights[i] = geom.ExactParameterInterval{Lower: b.floor, Upper: b.floor}
			if !c.chart[i] {
				continue
			}
			var err error
			uv[i], err = b.volumeRoofDomain(c.xy, &w)
			if err != nil {
				return c, err
			}
			v, err := b.roofHeightRange(uv[i], &w)
			if err != nil {
				return c, err
			}
			// Strict roof-above-floor was independently proved at preparation.
			v.Lower = maxRat(v.Lower, b.floor)
			heights[i] = v
		}
		v, err := mixedRoofVolumeBound(c.chart, q.op, heights[0], heights[1], q.box[0].floor, q.box[1].floor, &w)
		if err != nil {
			return c, err
		}
		if v.Lower.Cmp(v.Upper) != 0 {
			avg, err := q.exactPolynomialColumnAverage(c.chart, uv, heights, &w)
			if err != nil {
				return c, err
			}
			if avg != nil {
				v = geom.ExactParameterInterval{Lower: avg, Upper: new(big.Rat).Set(avg)}
			}
		}
		area := big.NewRat(1, 1)
		for _, a := range c.xy {
			width, err := w.rat(new(big.Rat).Sub(a.Upper, a.Lower))
			if err != nil {
				return c, err
			}
			if _, err = w.rat(area.Mul(area, width)); err != nil {
				return c, err
			}
		}
		for _, x := range []*big.Rat{v.Lower, v.Upper} {
			if _, err := w.rat(x.Mul(x, area)); err != nil {
				return c, err
			}
		}
		c.value = v
		c.width, err = w.rat(new(big.Rat).Sub(v.Upper, v.Lower))
		return c, err
	}
	footprint := mixedRoofVolumeFootprint(q)
	if len(footprint) > o.MaxCells {
		return zero, geom.ErrConvergence
	}
	for _, x := range footprint {
		c, err := calculate(x)
		if err != nil {
			return zero, err
		}
		if _, err = w.rat(total.Lower.Add(total.Lower, c.value.Lower)); err != nil {
			return zero, err
		}
		if _, err = w.rat(total.Upper.Add(total.Upper, c.value.Upper)); err != nil {
			return zero, err
		}
		if c.width.Sign() > 0 {
			heap.Push(&qh, c)
		}
	}
	best := total
	out := MixedRoofVolume{VisitedCells: len(footprint)}
	var activeAxis [2]bool
	for _, b := range q.box {
		for _, grid := range [][][]*big.Rat{b.linePower, b.weight} {
			for a, row := range grid {
				for c, value := range row {
					if value.Sign() != 0 {
						activeAxis[0] = activeAxis[0] || a > 0
						activeAxis[1] = activeAxis[1] || c > 0
					}
				}
			}
		}
	}
	finish := func(cause error) (MixedRoofVolume, error) {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		if cause != nil && !errors.Is(cause, geom.ErrConvergence) {
			return zero, cause
		}
		out.Volume = best
		out.ExactOperations = w.operations
		out.Converged = new(big.Rat).Sub(best.Upper, best.Lower).Cmp(o.AbsoluteGap) <= 0
		if out.Converged {
			return out, nil
		}
		return out, geom.ErrConvergence
	}
	for len(qh) > 0 {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		if new(big.Rat).Sub(best.Upper, best.Lower).Cmp(o.AbsoluteGap) <= 0 {
			return finish(nil)
		}
		if o.MaxCells-out.VisitedCells < 2 {
			return finish(geom.ErrConvergence)
		}
		c := heap.Pop(&qh).(mixedRoofVolumeCell)
		if c.depth >= o.MaxDepth {
			continue
		} // fixed leaf still contributes to total
		var axis int
		if !activeAxis[0] || activeAxis[1] && new(big.Rat).Sub(c.xy[1].Upper, c.xy[1].Lower).Cmp(new(big.Rat).Sub(c.xy[0].Upper, c.xy[0].Lower)) > 0 {
			axis = 1
		}
		d := c.xy[axis]
		m, err := w.rat(new(big.Rat).Add(d.Lower, d.Upper))
		if err != nil {
			return finish(err)
		}
		if _, err = w.rat(m.Quo(m, big.NewRat(2, 1))); err != nil {
			return finish(err)
		}
		children := [2]mixedRoofVolumeCell{c, c}
		children[0].xy[axis].Upper, children[1].xy[axis].Lower = m, m
		for i, x := range children {
			x.depth++
			children[i], err = calculate(x)
			if err != nil {
				return finish(err)
			}
			out.VisitedCells++
		}
		// Retain each unchanged and previously certified parent interval
		// until BOTH child contributions and the aggregate are available.
		bounds := [2]*big.Rat{}
		for i, f := range [2][4]*big.Rat{{total.Lower, c.value.Lower, children[0].value.Lower, children[1].value.Lower}, {total.Upper, c.value.Upper, children[0].value.Upper, children[1].value.Upper}} {
			z, err := w.rat(new(big.Rat).Sub(f[0], f[1]))
			if err != nil {
				return finish(err)
			}
			if _, err = w.rat(z.Add(z, f[2])); err != nil {
				return finish(err)
			}
			bounds[i], err = w.rat(z.Add(z, f[3]))
			if err != nil {
				return finish(err)
			}
		}
		total = geom.ExactParameterInterval{Lower: bounds[0], Upper: bounds[1]}
		best = geom.ExactParameterInterval{Lower: maxRat(best.Lower, total.Lower), Upper: minRat(best.Upper, total.Upper)}
		for _, x := range children {
			if x.width.Sign() > 0 {
				heap.Push(&qh, x)
			}
		}
	}
	return finish(nil)
}

// BoundVolume preserves the exact enclosure under a proper rigid placement.
func (q PlacedRoofMaterialQuery) BoundVolume(ctx context.Context, o MixedRoofVolumeOptions) (MixedRoofVolume, error) {
	return q.source.BoundVolume(ctx, o)
}

func mixedRoofVolumeFootprint(q *MixedRoofMaterialQuery) []mixedRoofVolumeCell {
	var cuts [2][]float64
	for i, b := range [2][4]float64{
		{q.box[0].domain.UMin, q.box[0].domain.UMax, q.box[1].domain.UMin, q.box[1].domain.UMax},
		{q.box[0].domain.VMin, q.box[0].domain.VMax, q.box[1].domain.VMin, q.box[1].domain.VMax},
	} {
		sort.Float64s(b[:])
		for _, u := range b {
			if len(cuts[i]) == 0 || cuts[i][len(cuts[i])-1] != u {
				cuts[i] = append(cuts[i], u)
			}
		}
	}
	var out []mixedRoofVolumeCell
	for i := 1; i < len(cuts[0]); i++ {
		for j := 1; j < len(cuts[1]); j++ {
			x, y := [2]float64{cuts[0][i-1], cuts[0][i]}, [2]float64{cuts[1][j-1], cuts[1][j]}
			c := mixedRoofVolumeCell{xy: [2]geom.ExactParameterInterval{{Lower: ratInset(x[0]), Upper: ratInset(x[1])}, {Lower: ratInset(y[0]), Upper: ratInset(y[1])}}}
			for i, b := range q.box {
				d := b.domain
				c.chart[i] = x[0] >= d.UMin && x[1] <= d.UMax && y[0] >= d.VMin && y[1] <= d.VMax
			}
			if q.op == parametric.Intersection && c.chart[0] && c.chart[1] ||
				q.op == parametric.Difference && c.chart[0] || q.op == parametric.Union && (c.chart[0] || c.chart[1]) {
				out = append(out, c)
			}
		}
	}
	return out
}

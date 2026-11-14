package trimmed

import (
	"container/heap"
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// BoundCrossingSurfaceArea encloses the area of the retained subsets of the
// twelve original faces. The boxes must have distinct floor elevations and
// distinct parallel footprint boundary planes across the sources. The result
// identifies original faces with a possibly nonzero contribution. A reversed
// row contributes cutter material to a difference. Unresolved face cells
// contribute conservative intervals, never a sampled inside/outside vote.
// The default gap is 0.15 square model units. On exhaustion or cancellation
// the method returns no face ledger.
func (q *MixedRoofMaterialQuery) BoundCrossingSurfaceArea(ctx context.Context, o MixedRoofAreaOptions) (MixedRoofArea, error) {
	var zero MixedRoofArea
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	var err error
	o.SurfacePatchEvaluationOptions, err = q.evaluationOptions(ctx, o.SurfacePatchEvaluationOptions)
	if err != nil {
		return zero, err
	}
	if o.AbsoluteGap == 0 {
		o.AbsoluteGap = .15
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 24
	}
	if o.MaxCells == 0 {
		o.MaxCells = 32768
	}
	if !finite(o.AbsoluteGap) || o.AbsoluteGap <= 0 || o.MaxDepth < 1 || o.MaxDepth > 64 || o.MaxCells < 1 || o.MaxCells > 262144 {
		return zero, geom.ErrInvalid
	}
	switch q.op {
	case parametric.Intersection, parametric.Union, parametric.Difference:
	default:
		return zero, geom.ErrInvalid
	}
	a, b := q.box[0], q.box[1]
	if a.floor.Cmp(b.floor) == 0 || roofAreaCoincidentWalls(a.domain, b.domain) {
		return zero, geom.ErrUnrepresentable
	}
	w := mixedRoofLineWork{o: MixedRoofLineOptions{SurfacePatchEvaluationOptions: o.SurfacePatchEvaluationOptions}}
	asUnion := *q
	asUnion.op = parametric.Union
	footprint := mixedRoofVolumeFootprint(&asUnion)
	out := MixedRoofArea{Area: mixedRoofZeroInterval(), Faces: make([]MixedRoofAreaFace, 12)}
	for i := range out.Faces {
		out.Faces[i] = MixedRoofAreaFace{Owner: MixedRoofFaceOwner{i / 6, i % 6}, Area: mixedRoofZeroInterval()}
	}
	work := crossingRoofAreaWork{
		ctx: ctx, owner: q, arithmetic: &w, options: o, result: &out,
		charts: map[*mixedRoofBox]crossingRoofAreaDerivatives{},
	}
	var pending crossingRoofAreaHeap
	for _, cell := range footprint {
		for i, support := range cell.roof {
			if support == nil {
				continue
			}
			box := q.box[i]
			add := func(j int, kind crossingRoofAreaKind, axis int, xy [2]geom.ExactParameterInterval) error {
				item := crossingRoofAreaCell{footprint: cell.roof, xy: xy, owner: i, face: 6*i + j, kind: kind, axis: axis}
				if err := work.calculate(&item); err != nil {
					return err
				}
				if err := work.replace(item.face, [2]*big.Rat{}, item.area); err != nil {
					return err
				}
				if item.width.Sign() != 0 {
					heap.Push(&pending, item)
				}
				return nil
			}
			if err = add(box.floorFace, crossingAreaFloor, 0, cell.xy); err != nil {
				return zero, err
			}
			if err = add(box.roofFace, crossingAreaRoof, 0, cell.xy); err != nil {
				return zero, err
			}
			planes := [2][2]float64{{box.domain.UMin, box.domain.UMax}, {box.domain.VMin, box.domain.VMax}}
			for axis := range 2 {
				ends := [2]*big.Rat{cell.xy[axis].Lower, cell.xy[axis].Upper}
				for edge, x := range ends {
					if x.Cmp(ratInset(planes[axis][edge])) == 0 {
						wall := cell.xy
						wall[axis] = geom.ExactParameterInterval{Lower: x, Upper: x}
						if err = add(box.sideFaces[2*axis+edge], crossingAreaWall, axis, wall); err != nil {
							return zero, err
						}
					}
				}
			}
		}
	}
	gap := ratInset(o.AbsoluteGap)
	for new(big.Rat).Sub(out.Area.Upper, out.Area.Lower).Cmp(gap) > 0 {
		if len(pending) == 0 {
			return zero, geom.ErrConvergence
		}
		cell := heap.Pop(&pending).(crossingRoofAreaCell)
		if cell.depth >= o.MaxDepth {
			continue
		}
		axis := 1 - cell.axis
		if cell.kind != crossingAreaWall {
			axis = cell.depth % 2
		}
		m, err := w.rat(new(big.Rat).Add(cell.xy[axis].Upper, cell.xy[axis].Lower))
		if err != nil {
			return zero, err
		}
		if _, err = w.rat(m.Quo(m, big.NewRat(2, 1))); err != nil {
			return zero, err
		}
		pair := [2]crossingRoofAreaCell{cell, cell}
		pair[0].xy[axis].Upper, pair[1].xy[axis].Lower = m, m
		for i := range pair {
			child := &pair[i]
			child.depth++
			if err := work.calculate(child); err != nil {
				return zero, err
			}
			if child.width.Sign() != 0 {
				heap.Push(&pending, *child)
			}
		}
		lo, err := w.rat(new(big.Rat).Add(pair[0].area.Lower, pair[1].area.Lower))
		if err != nil {
			return zero, err
		}
		hi, err := w.rat(new(big.Rat).Add(pair[0].area.Upper, pair[1].area.Upper))
		if err != nil {
			return zero, err
		}
		if err := work.replace(cell.face, [2]*big.Rat{cell.area.Lower, cell.area.Upper}, geom.ExactParameterInterval{Lower: lo, Upper: hi}); err != nil {
			return zero, err
		}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	out.ExactOperations = w.operations
	for i := range out.Faces {
		row := &out.Faces[i]
		row.Retained = row.Area.Upper.Sign() > 0
		row.Reversed = row.Retained && row.Owner.Operand == 1 && q.op == parametric.Difference
	}
	return out, nil
}

// BoundCrossingSurfaceArea is invariant under an exact proper rigid pose.
func (q PlacedRoofMaterialQuery) BoundCrossingSurfaceArea(ctx context.Context, o MixedRoofAreaOptions) (MixedRoofArea, error) {
	return q.source.BoundCrossingSurfaceArea(ctx, o)
}

func roofAreaCoincidentWalls(a, b geom.ParameterRectangle) bool {
	for _, axis := range [2][4]float64{{a.UMin, a.UMax, b.UMin, b.UMax}, {a.VMin, a.VMax, b.VMin, b.VMax}} {
		if axis[0] == axis[2] || axis[0] == axis[3] || axis[1] == axis[2] || axis[1] == axis[3] {
			return true
		}
	}
	return false
}

type crossingRoofAreaKind uint8

const (
	crossingAreaFloor crossingRoofAreaKind = iota
	crossingAreaRoof
	crossingAreaWall
)

type crossingRoofAreaCell struct {
	footprint   [2]*mixedRoofBox
	xy          [2]geom.ExactParameterInterval
	owner, face int
	kind        crossingRoofAreaKind
	axis, depth int // axis is fixed for walls
	area        geom.ExactParameterInterval
	width       *big.Rat
}

type crossingRoofAreaHeap []crossingRoofAreaCell

func (h crossingRoofAreaHeap) Len() int           { return len(h) }
func (h crossingRoofAreaHeap) Less(i, j int) bool { return h[i].width.Cmp(h[j].width) > 0 }
func (h crossingRoofAreaHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *crossingRoofAreaHeap) Push(x any)        { *h = append(*h, x.(crossingRoofAreaCell)) }
func (h *crossingRoofAreaHeap) Pop() any {
	rows := *h
	i := len(rows) - 1
	x := rows[i]
	rows[i] = crossingRoofAreaCell{}
	*h = rows[:i]
	return x
}

type crossingRoofAreaDerivatives struct {
	dz, dw [2]roofPowerGrid
	scale  [2]*big.Rat
}
type crossingRoofAreaWork struct {
	ctx        context.Context
	owner      *MixedRoofMaterialQuery
	arithmetic *mixedRoofLineWork
	options    MixedRoofAreaOptions
	result     *MixedRoofArea
	charts     map[*mixedRoofBox]crossingRoofAreaDerivatives
}

func (s *crossingRoofAreaWork) replace(index int, old [2]*big.Rat, added geom.ExactParameterInterval) error {
	row := &s.result.Faces[index].Area
	for i, pair := range [2][3]*big.Rat{{s.result.Area.Lower, row.Lower, added.Lower}, {s.result.Area.Upper, row.Upper, added.Upper}} {
		d := new(big.Rat).Set(pair[2])
		if old[i] != nil {
			if _, err := s.arithmetic.rat(d.Sub(d, old[i])); err != nil {
				return err
			}
		}
		for _, a := range pair[:2] {
			if _, err := s.arithmetic.rat(a.Add(a, d)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *crossingRoofAreaWork) calculate(c *crossingRoofAreaCell) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if s.result.Cells >= s.options.MaxCells {
		return geom.ErrConvergence
	}
	s.result.Cells++
	w := s.arithmetic
	a, b := c.owner, 1-c.owner
	insideWanted := s.owner.op == parametric.Intersection || (s.owner.op == parametric.Difference && a == 1)
	var density geom.ExactParameterInterval
	var uv [2]geom.ExactParameterInterval
	var heights [2]geom.ExactParameterInterval
	floors := [2]*big.Rat{s.owner.box[0].floor, s.owner.box[1].floor}
	for k, support := range c.footprint {
		if support == nil || (k == a && c.kind == crossingAreaFloor) {
			continue
		}
		p, err := support.volumeRoofDomain(c.xy, w)
		if err != nil {
			return err
		}
		if k == a {
			uv = p
		}
		h, err := support.roofHeightRange(p, w)
		if err != nil {
			return err
		}
		h.Lower = maxRat(h.Lower, floors[k])
		heights[k] = h
	}
	c.area = mixedRoofZeroInterval()
	maybe, guaranteed := true, true
	if c.kind != crossingAreaWall {
		height := heights[a]
		if c.kind == crossingAreaFloor {
			height = geom.ExactParameterInterval{Lower: floors[a], Upper: floors[a]}
		}
		strictlyIn := c.footprint[b] != nil && height.Lower.Cmp(floors[b]) > 0 && height.Upper.Cmp(heights[b].Lower) < 0
		strictlyOut := c.footprint[b] == nil || height.Upper.Cmp(floors[b]) < 0 || height.Lower.Cmp(heights[b].Upper) > 0
		guaranteed, maybe = strictlyIn, !strictlyOut
		if !insideWanted {
			guaranteed, maybe = strictlyOut, !strictlyIn
		}
		if !maybe {
			c.width = new(big.Rat)
			return nil
		}
		density = geom.ExactParameterInterval{Lower: big.NewRat(1, 1), Upper: big.NewRat(1, 1)}
		if c.kind == crossingAreaRoof {
			support := c.footprint[a]
			d, ok := s.charts[support]
			if !ok {
				var err error
				d.dz, err = roofAreaDerivativeGrids(support.linePower, w)
				if err != nil {
					return err
				}
				if support.weight != nil {
					d.dw, err = roofAreaDerivativeGrids(support.weight, w)
					if err != nil {
						return err
					}
				}
				domain := support.domain
				for i, axis := range [2][2]float64{{domain.UMin, domain.UMax}, {domain.VMin, domain.VMax}} {
					d.scale[i], err = w.rat(new(big.Rat).Sub(ratInset(axis[1]), ratInset(axis[0])))
					if err != nil {
						return err
					}
				}
				s.charts[support] = d
			}
			squared, err := support.roofAreaDensity(uv, d.scale, d.dz, d.dw, w)
			if err != nil {
				return err
			}
			lo, err := roofAreaSqrt(squared.Lower, w)
			if err != nil {
				return err
			}
			hi, err := roofAreaSqrt(squared.Upper, w)
			if err != nil {
				return err
			}
			density = geom.ExactParameterInterval{Lower: lo.Lower, Upper: hi.Upper}
		}
		if !guaranteed {
			density.Lower = new(big.Rat)
		}
	} else {
		var err error
		density, err = crossingRoofAreaWall(heights, floors, a, c.footprint[b] != nil, insideWanted, w)
		if err != nil {
			return err
		}
	}
	weight := big.NewRat(1, 1)
	for i, p := range c.xy {
		if c.kind == crossingAreaWall && i == c.axis {
			continue
		}
		d, err := w.rat(new(big.Rat).Sub(p.Upper, p.Lower))
		if err != nil {
			return err
		}
		if _, err := w.rat(weight.Mul(weight, d)); err != nil {
			return err
		}
	}
	var err error
	c.area.Lower, err = w.rat(new(big.Rat).Mul(weight, density.Lower))
	if err != nil {
		return err
	}
	c.area.Upper, err = w.rat(new(big.Rat).Mul(weight, density.Upper))
	if err != nil {
		return err
	}
	c.width, err = w.rat(new(big.Rat).Sub(c.area.Upper, c.area.Lower))
	return err
}

// Wall strip height is monotone in stock height. The inside strip is
// nondecreasing in the opposing height, and the exterior strip is
// nonincreasing. Each endpoint can therefore be evaluated without assuming
// independence or correlation between the two source functions.
func crossingRoofAreaWall(h [2]geom.ExactParameterInterval, f [2]*big.Rat, a int, cover, inside bool, w *mixedRoofLineWork) (geom.ExactParameterInterval, error) {
	b := 1 - a
	row := mixedRoofZeroInterval()
	for i, upper := range []bool{false, true} {
		ha, hb := h[a].Lower, h[b].Lower
		if upper {
			ha, hb = h[a].Upper, h[b].Upper
		}
		if !inside {
			hb = h[b].Upper
			if upper {
				hb = h[b].Lower
			}
		}
		value := new(big.Rat)
		if cover {
			if z := minRat(ha, hb); z.Cmp(maxRat(f[a], f[b])) > 0 {
				if _, err := w.rat(value.Sub(z, maxRat(f[a], f[b]))); err != nil {
					return row, err
				}
			}
		}
		if !inside {
			base, err := w.rat(new(big.Rat).Sub(ha, f[a]))
			if err != nil {
				return row, err
			}
			if _, err = w.rat(value.Sub(base, value)); err != nil {
				return row, err
			}
		}
		[2]*big.Rat{row.Lower, row.Upper}[i].Set(value)
	}
	return row, nil
}

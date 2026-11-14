package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AlgebraicGraphClippedWindowAggregate encloses material above Requested,
// including its portion outside the exact common chart rectangle. Inside.Leaves
// cover only Clipped. OutsideArea has zero material volume; zero-area contact
// with the chart boundary is not a pointwise empty-set classification.
// Complete concerns area/volume classification, not boundary point location.
type AlgebraicGraphClippedWindowAggregate struct {
	HasBounds, Complete, HasAreaOverlap                                 bool
	Requested, CommonDomain, Clipped                                    AlgebraicGraphProjectionWindow
	ProjectionAxes                                                      [2]int
	ResidualAxis                                                        int
	RequestedArea, OutsideArea, MaterialArea, EmptyArea, UnresolvedArea *big.Rat
	Volume                                                              [2]*big.Rat
	Inside                                                              AlgebraicGraphWindowAggregate
	ExactOperations                                                     int
}

// RefineClippedWindowDepth accepts windows outside original domains only after
// proving both original trims fill their complete rectangular charts and both
// projected affine inverses are signed/scaled coordinate permutations. The
// common projected rectangle and clipping are exact rational operations. It
// then reuses RefineWindowDepth with unchanged per-cell limits. Additional
// bounded rectangle/area arithmetic reserves256 operations and respects the
// same coefficient limit. Unsupported shears or nonrectangular trims fail.
func (q *AlgebraicGraphMaterialQuery) RefineClippedWindowDepth(ctx context.Context, w AlgebraicGraphProjectionWindow, o AlgebraicGraphWindowRefineOptions) (out AlgebraicGraphClippedWindowAggregate, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicGraphClippedWindowAggregate{ExactOperations: out.ExactOperations}
		}
	}()
	if ctx == nil || q == nil || q.graphs[0] == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	o.CellOptions.Exact, err = graphMaterialLimits(o.CellOptions.Exact)
	if err != nil {
		return out, err
	}
	if o.MaxCells < 0 || o.MaxCells > 4096 || o.MaxDepth < 0 || o.MaxDepth > 12 || o.CellOptions.MaxCells < 0 || o.CellOptions.MaxCells > 65536 {
		return out, geom.ErrInvalid
	}
	if o.CellOptions.Exact.MaxOperations < 256 {
		return out, geom.ErrConvergence
	}
	out.ExactOperations = 256
	check := func(x *big.Rat) error {
		if x == nil {
			return geom.ErrInvalid
		}
		if x.Num().BitLen() > o.CellOptions.Exact.MaxCoefficientBits || x.Denom().BitLen() > o.CellOptions.Exact.MaxCoefficientBits {
			return geom.ErrConvergence
		}
		return nil
	}
	copyWindow := func(a AlgebraicGraphProjectionWindow) AlgebraicGraphProjectionWindow {
		return AlgebraicGraphProjectionWindow{Min: [2]*big.Rat{new(big.Rat).Set(a.Min[0]), new(big.Rat).Set(a.Min[1])}, Max: [2]*big.Rat{new(big.Rat).Set(a.Max[0]), new(big.Rat).Set(a.Max[1])}}
	}
	area := func(a AlgebraicGraphProjectionWindow) (*big.Rat, error) {
		d := [2]*big.Rat{}
		for i := range d {
			d[i] = new(big.Rat).Sub(a.Max[i], a.Min[i])
			if e := check(d[i]); e != nil {
				return nil, e
			}
		}
		r := new(big.Rat).Mul(d[0], d[1])
		return r, check(r)
	}
	for i := 0; i < 2; i++ {
		if e := check(w.Min[i]); e != nil {
			return out, e
		}
		if e := check(w.Max[i]); e != nil {
			return out, e
		}
		if w.Min[i].Cmp(w.Max[i]) >= 0 {
			return out, geom.ErrInvalid
		}
	}
	var domains [2]AlgebraicGraphProjectionWindow
	for chart, g := range q.graphs {
		d := g.Domain()
		if !exactFullGraphRectangle(q.faces[chart], d) {
			return out, geom.ErrConvergence
		}
		inverse, e := g.InverseCoefficients()
		if e != nil {
			return out, e
		}
		seen := [2]bool{}
		for uv, row := range inverse {
			for _, x := range row {
				if e := check(x); e != nil {
					return out, e
				}
			}
			axis := -1
			for i := 0; i < 2; i++ {
				if row[i].Sign() != 0 {
					if axis != -1 {
						return out, geom.ErrConvergence
					}
					axis = i
				}
			}
			if axis < 0 || seen[axis] {
				return out, geom.ErrConvergence
			}
			seen[axis] = true
			limits := [2]float64{d.UMin, d.UMax}
			if uv == 1 {
				limits = [2]float64{d.VMin, d.VMax}
			}
			points := [2]*big.Rat{}
			for end, f := range limits {
				v := ratInset(f)
				if e := check(v); e != nil {
					return out, e
				}
				v.Sub(v, row[2])
				if e := check(v); e != nil {
					return out, e
				}
				v.Quo(v, row[axis])
				if e := check(v); e != nil {
					return out, e
				}
				points[end] = v
			}
			if points[0].Cmp(points[1]) > 0 {
				points[0], points[1] = points[1], points[0]
			}
			domains[chart].Min[axis], domains[chart].Max[axis] = points[0], points[1]
		}
	}
	common := copyWindow(domains[0])
	for i := 0; i < 2; i++ {
		if domains[1].Min[i].Cmp(common.Min[i]) > 0 {
			common.Min[i].Set(domains[1].Min[i])
		}
		if domains[1].Max[i].Cmp(common.Max[i]) < 0 {
			common.Max[i].Set(domains[1].Max[i])
		}
		if common.Min[i].Cmp(common.Max[i]) >= 0 {
			return out, geom.ErrConvergence
		}
	}
	out.Requested = copyWindow(w)
	out.CommonDomain = common
	out.ProjectionAxes = q.axes
	out.ResidualAxis = q.residual
	out.RequestedArea, err = area(w)
	if err != nil {
		return out, err
	}
	out.OutsideArea = new(big.Rat).Set(out.RequestedArea)
	out.MaterialArea = new(big.Rat)
	out.EmptyArea = new(big.Rat).Set(out.RequestedArea)
	out.UnresolvedArea = new(big.Rat)
	out.Volume = [2]*big.Rat{new(big.Rat), new(big.Rat)}
	clipped := copyWindow(w)
	hasArea := true
	for i := 0; i < 2; i++ {
		if common.Min[i].Cmp(clipped.Min[i]) > 0 {
			clipped.Min[i].Set(common.Min[i])
		}
		if common.Max[i].Cmp(clipped.Max[i]) < 0 {
			clipped.Max[i].Set(common.Max[i])
		}
		if clipped.Min[i].Cmp(clipped.Max[i]) >= 0 {
			hasArea = false
		}
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if !hasArea {
		out.HasBounds = true
		out.Complete = true
		return out, nil
	}
	out.Clipped = clipped
	out.HasAreaOverlap = true
	insideArea, e := area(clipped)
	if e != nil {
		return out, e
	}
	out.OutsideArea.Sub(out.RequestedArea, insideArea)
	if e := check(out.OutsideArea); e != nil {
		return out, e
	}
	inside, e := q.RefineWindowDepth(ctx, clipped, o)
	if inside.ExactOperations > int(^uint(0)>>1)-out.ExactOperations {
		return out, geom.ErrConvergence
	}
	out.ExactOperations += inside.ExactOperations
	if e != nil {
		return out, e
	}
	out.Inside = inside
	out.MaterialArea.Set(inside.MaterialArea)
	out.EmptyArea.Add(out.OutsideArea, inside.EmptyArea)
	out.UnresolvedArea.Set(inside.UnresolvedArea)
	for i := 0; i < 2; i++ {
		out.Volume[i].Set(inside.Volume[i])
	}
	if e := check(out.EmptyArea); e != nil {
		return out, e
	}
	out.Complete = inside.Complete
	out.HasBounds = inside.HasBounds
	return out, nil
}

// Four exact corner-to-corner edges, without holes, prove the entire original
// rectangular chart; no bounding-box approximation of a polygon is accepted.
func exactFullGraphRectangle(f Face, d geom.ParameterRectangle) bool {
	if len(f.Outer) != 4 || len(f.Holes) != 0 {
		return false
	}
	seen := 0
	corner := func(p geom.Vec2) int {
		x, y := -1, -1
		if p.X == d.UMin {
			x = 0
		} else if p.X == d.UMax {
			x = 1
		}
		if p.Y == d.VMin {
			y = 0
		} else if p.Y == d.VMax {
			y = 1
		}
		if x < 0 || y < 0 {
			return -1
		}
		return x + 2*y
	}
	for i, u := range f.Outer {
		a, b := corner(u.UVStart), corner(u.UVEnd)
		if u.Curve != nil || a < 0 || b < 0 || seen&(1<<a) != 0 || u.UVEnd != f.Outer[(i+1)%4].UVStart {
			return false
		}
		if (a^b) != 1 && (a^b) != 2 {
			return false
		}
		seen |= 1 << a
	}
	return seen == 15
}

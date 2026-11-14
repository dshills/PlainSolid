package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
)

// AlgebraicGraphWindowRefineOptions limits an exact quadtree partition. MaxCells
// counts attempted strict certificates (default85, maximum4096); MaxDepth is
// the maximum subdivision depth (default3, maximum12). CellOptions bounds each
// independent certificate, so total exact work is bounded by MaxCells times
// its operation limit, plus eight inversions and two source restrictions during domain preflight.
// Rational aggregation is linear in the bounded leaf count and checked against
// the coefficient limit.
// The immutable tube exclusion boxes are not refined: unresolved area need not
// tend to zero even with larger limits.
type AlgebraicGraphWindowRefineOptions struct {
	CellOptions        AlgebraicGraphWindowOptions
	MaxCells, MaxDepth int
}

// AlgebraicGraphWindowLeaf retains exact partition geometry and native face
// ancestry when classified. Unresolved leaves have depth [0, global support
// height gap over the window]; no inside/outside assertion is made for those leaves.
type AlgebraicGraphWindowLeaf struct {
	Window                                AlgebraicGraphProjectionWindow
	Depth                                 [2]*big.Rat
	Faces                                 [2]int
	FullMaterial, NoFootprint, Unresolved bool
	Level                                 int
}

// AlgebraicGraphWindowAggregate encloses volume of retained material above the
// requested projection window, in world coordinate units. Complete means all
// leaves were classified. HasBounds also holds with unresolved leaves. Areas
// and volumes are exact rational enclosures, not estimates or witnesses.
type AlgebraicGraphWindowAggregate struct {
	HasBounds, Complete                                    bool
	ProjectionAxes                                         [2]int
	ResidualAxis                                           int
	Leaves                                                 []AlgebraicGraphWindowLeaf
	ProjectedArea, MaterialArea, EmptyArea, UnresolvedArea *big.Rat
	Volume                                                 [2]*big.Rat
	CellAttempts, ExactOperations                          int
}

// RefineWindowDepth retains every unresolved cell in the result. Domain,
// cancellation, invalid-option and coefficient-limit failures are atomic;
// strict-cell convergence failures may instead yield conservative unresolved
// leaves. Boundary cells are never silently declared empty or full.
func (q *AlgebraicGraphMaterialQuery) RefineWindowDepth(ctx context.Context, w AlgebraicGraphProjectionWindow, o AlgebraicGraphWindowRefineOptions) (out AlgebraicGraphWindowAggregate, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicGraphWindowAggregate{CellAttempts: out.CellAttempts, ExactOperations: out.ExactOperations}
		}
	}()
	addWork := func(n int) error {
		if n < 0 || n > int(^uint(0)>>1)-out.ExactOperations {
			return geom.ErrConvergence
		}
		out.ExactOperations += n
		return nil
	}
	if ctx == nil || q == nil || q.graphs[0] == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.MaxCells == 0 {
		o.MaxCells = 85
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 3
	}
	if o.MaxCells < 1 || o.MaxCells > 4096 || o.MaxDepth < 1 || o.MaxDepth > 12 {
		return out, geom.ErrInvalid
	}
	o.CellOptions.Exact, err = graphMaterialLimits(o.CellOptions.Exact)
	if err != nil {
		return out, err
	}
	if o.CellOptions.MaxCells < 0 || o.CellOptions.MaxCells > 65536 {
		return out, geom.ErrInvalid
	}
	check := func(x *big.Rat) error {
		if x == nil {
			return geom.ErrInvalid
		}
		if x.Num().BitLen() > o.CellOptions.Exact.MaxCoefficientBits || x.Denom().BitLen() > o.CellOptions.Exact.MaxCoefficientBits {
			return geom.ErrConvergence
		}
		return nil
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
	// Convex original charts plus affine inverse: the four corners prove the
	// entire window belongs to both support domains, even when cells are unknown.
	var uvBounds [2][2][2]*big.Rat
	for chart, g := range q.graphs {
		for corner := 0; corner < 4; corner++ {
			xy := [2]*big.Rat{w.Min[0], w.Min[1]}
			if corner&1 != 0 {
				xy[0] = w.Max[0]
			}
			if corner&2 != 0 {
				xy[1] = w.Max[1]
			}
			v, e := g.InvertRational(ctx, xy, geom.SurfacePatchEvaluationOptions{MaxOperations: o.CellOptions.Exact.MaxOperations, MaxCoefficientBits: o.CellOptions.Exact.MaxCoefficientBits})
			if e := addWork(v.Operations); e != nil {
				return out, e
			}
			if e != nil {
				return out, e
			}
			if !v.Inside || !insideConvexTubeFace(q.faces[chart], v.UV) {
				return out, geom.ErrConvergence
			}
			for axis, x := range v.UV {
				b := &uvBounds[chart][axis]
				if b[0] == nil || x.Cmp(b[0]) < 0 {
					b[0] = new(big.Rat).Set(x)
				}
				if b[1] == nil || x.Cmp(b[1]) > 0 {
					b[1] = new(big.Rat).Set(x)
				}
			}
		}
	}
	// Restriction encloses a possibly enlarged UV rectangle; its height hull
	// bounds every material fiber even where trim classification is unresolved.
	var heights [2][2]*big.Rat
	for chart, p := range q.patches {
		var d [4]float64
		for axis := 0; axis < 2; axis++ {
			for end := 0; end < 2; end++ {
				x := uvBounds[chart][axis][end]
				f, _ := x.Float64()
				if !finite(f) {
					return out, geom.ErrConvergence
				}
				cmp := ratInset(f).Cmp(x)
				if end == 0 && cmp > 0 {
					f = math.Nextafter(f, math.Inf(-1))
				}
				if end == 1 && cmp < 0 {
					f = math.Nextafter(f, math.Inf(1))
				}
				if !finite(f) {
					return out, geom.ErrConvergence
				}
				d[2*axis+end] = f
			}
		}
		limited, e := p.WithLimits(geom.SurfacePatchOptions{MaxOperations: o.CellOptions.Exact.MaxOperations, MaxCoefficientBits: o.CellOptions.Exact.MaxCoefficientBits})
		if e != nil {
			return out, e
		}
		if e := addWork(limited.ExactOperations()); e != nil {
			return out, e
		}
		remain := o.CellOptions.Exact.MaxOperations - limited.ExactOperations()
		if remain < 1 {
			return out, geom.ErrConvergence
		}
		clipped, used, e := limited.TrimWithWorkLimit(geom.ParameterRectangle{UMin: d[0], UMax: d[1], VMin: d[2], VMax: d[3]}, remain)
		if e := addWork(used); e != nil {
			return out, e
		}
		if e != nil {
			return out, e
		}
		box, e := clipped.Bounds()
		if e != nil {
			return out, e
		}
		lo := [3]float64{box.Min.X, box.Min.Y, box.Min.Z}
		hi := [3]float64{box.Max.X, box.Max.Y, box.Max.Z}
		if !finite(lo[q.residual]) || !finite(hi[q.residual]) {
			return out, geom.ErrConvergence
		}
		heights[chart] = [2]*big.Rat{ratInset(lo[q.residual]), ratInset(hi[q.residual])}
		for _, h := range heights[chart] {
			if e := check(h); e != nil {
				return out, e
			}
		}
	}
	maxDepth := new(big.Rat)
	for _, r := range q.regions {
		d := new(big.Rat).Sub(heights[r.upper][1], heights[r.upper^1][0])
		if e := check(d); e != nil {
			return out, e
		}
		if d.Cmp(maxDepth) > 0 {
			maxDepth = d
		}
	}
	out.ProjectedArea = new(big.Rat)
	out.MaterialArea = new(big.Rat)
	out.EmptyArea = new(big.Rat)
	out.UnresolvedArea = new(big.Rat)
	out.Volume = [2]*big.Rat{new(big.Rat), new(big.Rat)}
	type cell struct {
		w     AlgebraicGraphProjectionWindow
		level int
	}
	queue := []cell{{w, 0}}
	for len(queue) > 0 {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		c := queue[0]
		queue = queue[1:]
		leaf := AlgebraicGraphWindowLeaf{Faces: [2]int{-1, -1}, Level: c.level, Depth: [2]*big.Rat{new(big.Rat), new(big.Rat).Set(maxDepth)}, Unresolved: true}
		for i := 0; i < 2; i++ {
			leaf.Window.Min[i] = new(big.Rat).Set(c.w.Min[i])
			leaf.Window.Max[i] = new(big.Rat).Set(c.w.Max[i])
		}
		if out.CellAttempts < o.MaxCells {
			r, e := q.BoundWindowDepth(ctx, c.w, o.CellOptions)
			out.CellAttempts++
			if e := addWork(r.ExactOperations); e != nil {
				return out, e
			}
			if e != nil && !errors.Is(e, geom.ErrConvergence) {
				return out, e
			}
			if e == nil {
				leaf.Unresolved = false
				leaf.FullMaterial = r.FullMaterial
				leaf.NoFootprint = r.NoFootprint
				leaf.Faces = r.Faces
				if r.FullMaterial {
					leaf.Depth = r.Depth
				} else {
					leaf.Depth = [2]*big.Rat{new(big.Rat), new(big.Rat)}
				}
			}
		}
		if leaf.Unresolved && c.level < o.MaxDepth && out.CellAttempts+len(queue)+4 <= o.MaxCells {
			mid := [2]*big.Rat{}
			for i := 0; i < 2; i++ {
				mid[i] = new(big.Rat).Add(c.w.Min[i], c.w.Max[i])
				if e := check(mid[i]); e != nil {
					return out, e
				}
				mid[i].Quo(mid[i], big.NewRat(2, 1))
				if e := check(mid[i]); e != nil {
					return out, e
				}
			}
			for k := 0; k < 4; k++ {
				child := c.w
				for i := 0; i < 2; i++ {
					if k&(1<<i) == 0 {
						child.Max[i] = mid[i]
					} else {
						child.Min[i] = mid[i]
					}
				}
				queue = append(queue, cell{child, c.level + 1})
			}
			continue
		}
		area := new(big.Rat).Mul(new(big.Rat).Sub(c.w.Max[0], c.w.Min[0]), new(big.Rat).Sub(c.w.Max[1], c.w.Min[1]))
		if e := check(area); e != nil {
			return out, e
		}
		out.ProjectedArea.Add(out.ProjectedArea, area)
		if leaf.Unresolved {
			out.UnresolvedArea.Add(out.UnresolvedArea, area)
		} else if leaf.FullMaterial {
			out.MaterialArea.Add(out.MaterialArea, area)
		} else {
			out.EmptyArea.Add(out.EmptyArea, area)
		}
		for i := 0; i < 2; i++ {
			v := new(big.Rat).Mul(area, leaf.Depth[i])
			if e := check(v); e != nil {
				return out, e
			}
			out.Volume[i].Add(out.Volume[i], v)
			if e := check(out.Volume[i]); e != nil {
				return out, e
			}
		}
		for _, a := range []*big.Rat{out.ProjectedArea, out.MaterialArea, out.EmptyArea, out.UnresolvedArea} {
			if e := check(a); e != nil {
				return out, e
			}
		}
		out.Leaves = append(out.Leaves, leaf)
	}
	out.ProjectionAxes = q.axes
	out.ResidualAxis = q.residual
	out.HasBounds = true
	out.Complete = out.UnresolvedArea.Sign() == 0
	return out, nil
}

package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// AlgebraicGraphProjectionWindow specifies closed limits in ProjectionAxes order.
// Both intervals must have positive width. Inputs are copied into successful results.
type AlgebraicGraphProjectionWindow struct{ Min, Max [2]*big.Rat }

// AlgebraicGraphWindowOptions bounds query work independently of preparation.
// Exact limits cover inversion, center classification and support restriction;
// MaxCells separately bounds immutable recipe decoding and exclusion checks.
type AlgebraicGraphWindowOptions struct {
	Exact    geom.ExactCurvePlaneEventOptions
	MaxCells int // Decoded recipe cells plus tested tubes; default8192, maximum65536.
}

// AlgebraicGraphWindowDepth is a uniform enclosure, not an extrema witness.
// NoFootprint means no retained material above any point of the window; it
// includes nested holes. FullMaterial retains original native face ownership.
type AlgebraicGraphWindowDepth struct {
	Complete, FullMaterial, NoFootprint     bool
	Faces                                   [2]int // Original Boolean bracketing faces, in operand order.
	ProjectionAxes                          [2]int
	ResidualAxis                            int
	Window                                  AlgebraicGraphProjectionWindow // Owned original rational limits.
	EnclosureDomains                        [2]geom.ParameterRectangle     // Outward enlarged original UV hulls.
	LowerSupport, UpperSupport              [2]*big.Rat                    // Conservative residual height ranges, by physical height.
	Depth                                   [2]*big.Rat                    // Uniform lower/upper material depth bounds, not extrema witnesses.
	ExactOperations, RecipeCells, TubeTests int
}

// BoundWindowDepth proves that an entire closed projected rectangle lies in
// one retained fragment or in its excluded footprint. It requires the window
// inside both original convex charts and strict separation from every verified
// native tube cell. Touching/crossing or overly broad tube boxes fail closed.
// Connectedness and complete native loop coverage then transport the exact
// center classification throughout the window; corners alone never establish
// material inclusion. Exact source restrictions on outward-rounded UV hulls
// bound both support heights. Output rational endpoints enclose real depths,
// including hull/display conversion error; they need not be attained extrema.
func (q *AlgebraicGraphMaterialQuery) BoundWindowDepth(ctx context.Context, w AlgebraicGraphProjectionWindow, o AlgebraicGraphWindowOptions) (out AlgebraicGraphWindowDepth, err error) {
	out.Faces = [2]int{-1, -1}
	defer func() {
		if err != nil {
			out = AlgebraicGraphWindowDepth{Faces: [2]int{-1, -1}, ExactOperations: out.ExactOperations, RecipeCells: out.RecipeCells, TubeTests: out.TubeTests}
		}
	}()
	if ctx == nil || q == nil || q.graphs[0] == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	o.Exact, err = graphMaterialLimits(o.Exact)
	if err != nil {
		return out, err
	}
	if o.MaxCells == 0 {
		o.MaxCells = 8192
	}
	if o.MaxCells < 1 || o.MaxCells > 65536 {
		return out, geom.ErrInvalid
	}
	check := func(x *big.Rat) error {
		if x == nil {
			return geom.ErrInvalid
		}
		if x.Num().BitLen() > o.Exact.MaxCoefficientBits || x.Denom().BitLen() > o.Exact.MaxCoefficientBits {
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
	// Bounded interval comparisons, rational midpoints and final hull arithmetic.
	if o.Exact.MaxOperations <= 128 {
		return out, geom.ErrConvergence
	}
	out.ExactOperations = 128
	remaining := func() geom.ExactCurvePlaneEventOptions {
		p := o.Exact
		p.MaxOperations -= out.ExactOperations
		return p
	}
	var uvBounds [2][2][2]*big.Rat // chart, axis, min/max
	for chart, g := range q.graphs {
		for corner := 0; corner < 4; corner++ {
			xy := [2]*big.Rat{w.Min[0], w.Min[1]}
			if corner&1 != 0 {
				xy[0] = w.Max[0]
			}
			if corner&2 != 0 {
				xy[1] = w.Max[1]
			}
			p := remaining()
			if p.MaxOperations < 1 {
				return out, geom.ErrConvergence
			}
			inverse, e := g.InvertRational(ctx, xy, geom.SurfacePatchEvaluationOptions{MaxOperations: p.MaxOperations, MaxCoefficientBits: p.MaxCoefficientBits})
			out.ExactOperations += inverse.Operations
			if e != nil {
				return out, e
			}
			if !inverse.Inside || !insideConvexTubeFace(q.faces[chart], inverse.UV) {
				return out, geom.ErrConvergence
			}
			for axis, x := range inverse.UV {
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
	// A TubeSegment owns a reverified immutable recipe and its cell index.
	// RebuildTubeCurves trims the original supports to precisely these domains
	// before verifying its branch. Raw caller recipes cannot enter this query.
	cache := map[intersect.TubeRecipe]intersect.TubeRecipeData{}
	for _, loop := range q.loops {
		for _, piece := range loop {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			if out.RecipeCells+out.TubeTests >= o.MaxCells {
				return out, geom.ErrConvergence
			}
			recipe, index, second := piece.Segment.TrimA().Recipe()
			if second {
				return out, geom.ErrConvergence
			}
			data, ok := cache[recipe]
			if !ok {
				data = recipe.Data()
				out.RecipeCells += len(data.Cells)
				if out.RecipeCells+out.TubeTests >= o.MaxCells {
					return out, geom.ErrConvergence
				}
				cache[recipe] = data
			}
			if index < 0 || index >= len(data.Cells) {
				return out, geom.ErrConvergence
			}
			out.TubeTests++
			if o.Exact.MaxOperations-out.ExactOperations < 4 {
				return out, geom.ErrConvergence
			}
			out.ExactOperations += 4
			cell := data.Cells[index].DomainA
			b := uvBounds[0]
			if !(b[0][1].Cmp(ratInset(cell.UMin)) < 0 || b[0][0].Cmp(ratInset(cell.UMax)) > 0 || b[1][1].Cmp(ratInset(cell.VMin)) < 0 || b[1][0].Cmp(ratInset(cell.VMax)) > 0) {
				return out, geom.ErrConvergence
			}
		}
	}
	center := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	for i, axis := range q.axes {
		center[axis] = new(big.Rat).Add(w.Min[i], w.Max[i])
		if e := check(center[axis]); e != nil {
			return out, e
		}
		center[axis].Quo(center[axis], big.NewRat(2, 1))
		if e := check(center[axis]); e != nil {
			return out, e
		}
	}
	p := remaining()
	if p.MaxOperations < 1 {
		return out, geom.ErrConvergence
	}
	fiber, e := q.locateRationalGraphMaterial(ctx, center, p, true)
	out.ExactOperations += fiber.ExactOperations
	if e != nil {
		return out, e
	}
	if !fiber.Complete {
		return out, geom.ErrConvergence
	}
	out.ProjectionAxes = q.axes
	out.ResidualAxis = q.residual
	for i := 0; i < 2; i++ {
		out.Window.Min[i] = new(big.Rat).Set(w.Min[i])
		out.Window.Max[i] = new(big.Rat).Set(w.Max[i])
	}
	if fiber.Lower == nil {
		out.Complete, out.NoFootprint = true, true
		return out, nil
	}
	if fiber.Lower.Cmp(fiber.Upper) >= 0 || len(fiber.BoundaryFaces) != 2 {
		return out, geom.ErrConvergence
	}
	var heights [2][2]*big.Rat
	round := func(x *big.Rat, up bool) (float64, error) {
		f, _ := x.Float64()
		if !finite(f) {
			return 0, geom.ErrConvergence
		}
		c := ratInset(f).Cmp(x)
		if up && c < 0 {
			f = math.Nextafter(f, math.Inf(1))
		}
		if !up && c > 0 {
			f = math.Nextafter(f, math.Inf(-1))
		}
		if !finite(f) {
			return 0, geom.ErrConvergence
		}
		return f, nil
	}
	for chart, patch := range q.patches {
		var bounds [4]float64
		for axis := 0; axis < 2; axis++ {
			for end := 0; end < 2; end++ {
				value, e := round(uvBounds[chart][axis][end], end == 1)
				if e != nil {
					return out, e
				}
				bounds[2*axis+end] = value
			}
		}
		domain := geom.ParameterRectangle{UMin: bounds[0], UMax: bounds[1], VMin: bounds[2], VMax: bounds[3]}
		out.EnclosureDomains[chart] = domain
		p := remaining()
		if p.MaxOperations < 1 {
			return out, geom.ErrConvergence
		}
		if e := ctx.Err(); e != nil {
			return out, e
		}
		limited, e := patch.WithLimits(geom.SurfacePatchOptions{MaxOperations: p.MaxOperations, MaxCoefficientBits: p.MaxCoefficientBits})
		if e != nil {
			return out, e
		}
		out.ExactOperations += limited.ExactOperations()
		p = remaining()
		if p.MaxOperations < 1 {
			return out, geom.ErrConvergence
		}
		support, used, e := limited.TrimWithWorkLimit(domain, p.MaxOperations)
		out.ExactOperations += used
		if e != nil {
			return out, e
		}
		b, e := support.Bounds()
		if e != nil {
			return out, e
		}
		lo := [3]float64{b.Min.X, b.Min.Y, b.Min.Z}
		hi := [3]float64{b.Max.X, b.Max.Y, b.Max.Z}
		if !finite(lo[q.residual]) || !finite(hi[q.residual]) {
			return out, geom.ErrConvergence
		}
		heights[chart] = [2]*big.Rat{ratInset(lo[q.residual]), ratInset(hi[q.residual])}
		for _, x := range heights[chart] {
			if e := check(x); e != nil {
				return out, e
			}
		}
	}
	lower := 0
	if fiber.BoundaryFaces[0] == fiber.Faces[1] {
		lower = 1
	} else if fiber.BoundaryFaces[0] != fiber.Faces[0] {
		return out, geom.ErrConvergence
	}
	out.Faces = fiber.Faces
	out.LowerSupport = heights[lower]
	out.UpperSupport = heights[lower^1]
	out.Depth = [2]*big.Rat{new(big.Rat).Sub(out.UpperSupport[0], out.LowerSupport[1]), new(big.Rat).Sub(out.UpperSupport[1], out.LowerSupport[0])}
	for _, x := range out.Depth {
		if e := check(x); e != nil {
			return out, e
		}
	}
	if out.Depth[1].Sign() < 0 {
		return out, geom.ErrConvergence
	}
	if out.Depth[0].Sign() < 0 {
		out.Depth[0].SetInt64(0)
	}
	out.Complete, out.FullMaterial = true, true
	return out, nil
}

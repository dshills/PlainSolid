//go:build research

package trimmed

import (
	"container/heap"
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// AlgebraicGraphOccurrenceDistance encloses WORLD squared distance to the
// retained native boundary. SourceWitness/WorldWitness enclose the same verified
// boundary point; Face/RimEdge retain original native IDs. They identify an
// upper-bound witness, not a unique minimizer. Cell/depth exhaustion can return
// HasBounds with Converged=false. Arithmetic or cancellation errors are atomic.
type AlgebraicGraphOccurrenceDistance struct {
	Squared                       geom.ExactParameterInterval
	SourceWitness, WorldWitness   [3]geom.ExactParameterInterval
	Face, RimEdge                 int
	HasBounds, Converged          bool
	ExactOperations, VisitedCells int
}

// BoundBoundaryDistance reuses original retained trim/rim exclusion and exact
// support subdivision, but evaluates each lower bound and witness in the actual
// rational affine world metric. No scalar source-distance transformation is
// used. Original support hulls yield conservative image boxes; retained exact
// graph samples and certified rim points yield upper witnesses. MaxCells and
// MaxDepth bound refinement while preserving all pending lower bounds.
func (o *AlgebraicGraphOccurrence) BoundBoundaryDistance(ctx context.Context, point [3]*big.Rat, options AlgebraicTubeLensDistanceOptions) (out AlgebraicGraphOccurrenceDistance, err error) {
	out.Face, out.RimEdge = -1, -1
	var a *graphOccurrenceArithmetic
	defer func() {
		if a != nil {
			out.ExactOperations = a.used
		}
		if err != nil {
			out = AlgebraicGraphOccurrenceDistance{Face: -1, RimEdge: -1, ExactOperations: out.ExactOperations, VisitedCells: out.VisitedCells}
		}
	}()
	if ctx == nil || o == nil || o.query == nil || options.MaxDepth < 0 || options.MaxDepth > 40 || options.MaxCells < 0 || options.MaxCells > 262144 {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if options.MaxDepth == 0 {
		options.MaxDepth = 24
	}
	if options.MaxCells == 0 {
		options.MaxCells = 16384
	}
	if options.Patch.MaxOperations == 0 {
		options.Patch.MaxOperations = 64000000
	}
	if options.SquaredGap == nil {
		options.SquaredGap = big.NewRat(1, 1000000)
	}
	if options.SquaredGap.Sign() <= 0 {
		return out, geom.ErrInvalid
	}
	exact, e := graphMaterialLimits(geom.ExactCurvePlaneEventOptions{MaxOperations: options.Patch.MaxOperations, MaxCoefficientBits: options.Patch.MaxCoefficientBits})
	if e != nil {
		return out, e
	}
	a = &graphOccurrenceArithmetic{ctx: ctx, options: exact}
	for _, v := range [4]*big.Rat{point[0], point[1], point[2], options.SquaredGap} {
		a.calc('+', v, new(big.Rat))
	}
	if a.err != nil {
		return out, a.err
	}
	q := o.query
	remaining := func() geom.ExactCurvePlaneEventOptions { p := exact; p.MaxOperations -= a.used; return p }
	charge := func(work int) error {
		if work < 0 || work > exact.MaxOperations-a.used {
			return geom.ErrConvergence
		}
		a.used += work
		return ctx.Err()
	}
	singleton := func(p [3]*big.Rat) (r [3]geom.ExactParameterInterval) {
		for i := range r {
			r[i] = geom.ExactParameterInterval{Lower: new(big.Rat).Set(p[i]), Upper: new(big.Rat).Set(p[i])}
		}
		return r
	}
	imageBox := func(b geom.Box) (source, world [3]geom.ExactParameterInterval, e error) {
		values := [3][2]float64{{b.Min.X, b.Max.X}, {b.Min.Y, b.Max.Y}, {b.Min.Z, b.Max.Z}}
		for i := 0; i < 3; i++ {
			if !finite(values[i][0]) || !finite(values[i][1]) || values[i][0] > values[i][1] {
				return source, world, geom.ErrConvergence
			}
			source[i] = geom.ExactParameterInterval{Lower: ratInset(values[i][0]), Upper: ratInset(values[i][1])}
		}
		for i := 0; i < 3; i++ {
			lo, hi := new(big.Rat).Set(o.pose[i][3]), new(big.Rat).Set(o.pose[i][3])
			for j := 0; j < 3; j++ {
				x := a.calc('*', o.pose[i][j], source[j].Lower)
				y := a.calc('*', o.pose[i][j], source[j].Upper)
				if x.Cmp(y) > 0 {
					x, y = y, x
				}
				lo = a.calc('+', lo, x)
				hi = a.calc('+', hi, y)
			}
			world[i] = geom.ExactParameterInterval{Lower: lo, Upper: hi}
		}
		return source, world, a.err
	}
	boxDistance := func(b [3]geom.ExactParameterInterval) (lo, hi *big.Rat) {
		lo, hi = new(big.Rat), new(big.Rat)
		for i, rangei := range b {
			d := new(big.Rat)
			if point[i].Cmp(rangei.Lower) < 0 {
				d = a.calc('-', rangei.Lower, point[i])
			} else if point[i].Cmp(rangei.Upper) > 0 {
				d = a.calc('-', point[i], rangei.Upper)
			}
			lo = a.calc('+', lo, a.calc('*', d, d))
			x := a.calc('-', point[i], rangei.Lower)
			y := a.calc('-', point[i], rangei.Upper)
			x = a.calc('*', x, x)
			y = a.calc('*', y, y)
			if x.Cmp(y) < 0 {
				x = y
			}
			hi = a.calc('+', hi, x)
		}
		return lo, hi
	}
	accept := func(value *big.Rat, source, world [3]geom.ExactParameterInterval, face, rim int) {
		if out.Squared.Upper == nil || value.Cmp(out.Squared.Upper) < 0 {
			out.Squared.Upper = value
			out.SourceWitness = source
			out.WorldWitness = world
			out.Face, out.RimEdge = face, rim
		}
	}
	acceptPoint := func(p [3]*big.Rat, face, rim int) {
		world := o.imagePoint(a, p)
		value := new(big.Rat)
		for i := 0; i < 3; i++ {
			d := a.calc('-', world[i], point[i])
			value = a.calc('+', value, a.calc('*', d, d))
		}
		accept(value, singleton(p), singleton(world), face, rim)
	}
	queue := tubeLensDistanceHeap{}
	for owner, patch := range q.patches {
		b, e := patch.Bounds()
		if e != nil {
			return out, e
		}
		_, world, e := imageBox(b)
		if e != nil {
			return out, e
		}
		lo, _ := boxDistance(world)
		if a.err != nil {
			return out, a.err
		}
		heap.Push(&queue, tubeLensDistanceCell{patch: patch, chart: owner, lower: lo})
	}
	var pending *big.Rat
	finish := func() (AlgebraicGraphOccurrenceDistance, error) {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		lo := pending
		if len(queue) > 0 {
			lo = minLensDistance(lo, queue[0].lower)
		}
		if out.Squared.Upper != nil {
			lo = minLensDistance(lo, out.Squared.Upper)
		}
		if lo == nil || out.Squared.Upper == nil {
			return out, geom.ErrConvergence
		}
		out.Squared.Lower = new(big.Rat).Set(lo)
		out.HasBounds = true
		out.Converged = a.calc('-', out.Squared.Upper, lo).Cmp(options.SquaredGap) <= 0
		if a.err != nil {
			return out, a.err
		}
		return out, nil
	}
	sourcePoint, _ := o.pullbackLine(a, point, [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)})
	if a.err != nil {
		return out, a.err
	}
	if remaining().MaxOperations < 1 {
		return out, geom.ErrConvergence
	}
	fiber, e := q.locateRationalGraphMaterial(ctx, sourcePoint, remaining(), true)
	if ce := charge(fiber.ExactOperations); ce != nil {
		return out, ce
	}
	if e != nil {
		return out, e
	}
	if fiber.Lower != nil && fiber.Upper != nil {
		faces := fiber.Faces
		for _, region := range q.regions {
			if region.faces == faces && region.upper == 0 && fiber.Lower.Cmp(fiber.Upper) != 0 {
				faces[0], faces[1] = faces[1], faces[0]
				break
			}
		}
		for i, z := range []*big.Rat{fiber.Lower, fiber.Upper} {
			p := sourcePoint
			p[q.residual] = z
			face, rim := faces[i], -1
			if len(fiber.RimEdges) > 0 {
				face, rim = -1, fiber.RimEdges[0]
			}
			acceptPoint(p, face, rim)
		}
		if a.err != nil {
			return out, a.err
		}
		if a.calc('-', out.Squared.Upper, queue[0].lower).Cmp(options.SquaredGap) <= 0 {
			return finish()
		}
		if a.err != nil {
			return out, a.err
		}
	}
	var rims [2][]geom.ParameterRectangle
	for i, ring := range q.loops {
		for j, piece := range ring {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			p, e := piece.Segment.Evaluate(0)
			if e != nil {
				return out, e
			}
			if !p.Enclosed {
				return out, geom.ErrConvergence
			}
			if e := charge(p.ExactOperations); e != nil {
				return out, e
			}
			source, world, e := imageBox(p.Bounds)
			if e != nil {
				return out, e
			}
			_, hi := boxDistance(world)
			if a.err != nil {
				return out, a.err
			}
			accept(hi, source, world, -1, q.edges[i][j])
			for chart, trim := range [2]intersect.TubeTrim{piece.Segment.TrimA(), piece.Segment.TrimB()} {
				if chart == 0 && len(q.windowTubeBoxes) > 0 {
					rims[chart] = append(rims[chart], q.windowTubeBoxes[i][j]...)
					continue
				}
				b, e := trim.BoundsUV()
				if e != nil {
					return out, e
				}
				rims[chart] = append(rims[chart], geom.ParameterRectangle{UMin: b.Min.X, UMax: b.Max.X, VMin: b.Min.Y, VMax: b.Max.Y})
			}
		}
	}
	for len(queue) > 0 {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if out.VisitedCells >= options.MaxCells {
			return finish()
		}
		if remaining().MaxOperations < 1 {
			return out, geom.ErrConvergence
		}
		lo := minLensDistance(pending, queue[0].lower)
		if out.Squared.Upper != nil && a.calc('-', out.Squared.Upper, lo).Cmp(options.SquaredGap) <= 0 {
			return finish()
		}
		if a.err != nil {
			return out, a.err
		}
		x := heap.Pop(&queue).(tubeLensDistanceCell)
		out.VisitedCells++
		if out.Squared.Upper != nil && x.lower.Cmp(out.Squared.Upper) >= 0 {
			continue
		}
		d := x.patch.Domain()
		mid := func(lo, hi float64) *big.Rat {
			return a.calc('/', a.calc('+', ratInset(lo), ratInset(hi)), big.NewRat(2, 1))
		}
		u, v := mid(d.UMin, d.UMax), mid(d.VMin, d.VMax)
		if a.err != nil {
			return out, a.err
		}
		if remaining().MaxOperations < 1 {
			return out, geom.ErrConvergence
		}
		jet, e := x.patch.EvaluateRational(ctx, u, v, geom.SurfacePatchEvaluationOptions{MaxOperations: remaining().MaxOperations, MaxCoefficientBits: exact.MaxCoefficientBits})
		if ce := charge(jet.ExactOperations); ce != nil {
			return out, ce
		}
		if e != nil {
			return out, e
		}
		if remaining().MaxOperations < 1 {
			return out, geom.ErrConvergence
		}
		location, e := q.locateRationalGraphMaterial(ctx, jet.Point, remaining(), true)
		if ce := charge(location.ExactOperations); ce != nil {
			return out, ce
		}
		if e != nil {
			return out, e
		}
		if location.Complete && location.Faces[x.chart] >= 0 {
			face, rim := location.Faces[x.chart], -1
			if len(location.RimEdges) > 0 {
				face, rim = -1, location.RimEdges[0]
			}
			acceptPoint(jet.Point, face, rim)
		} else if location.Complete {
			hit := false
			for _, box := range rims[x.chart] {
				if box.UMin <= d.UMax && box.UMax >= d.UMin && box.VMin <= d.VMax && box.VMax >= d.VMin {
					hit = true
					break
				}
			}
			if !hit {
				continue
			}
		}
		if a.err != nil {
			return out, a.err
		}
		if x.depth >= options.MaxDepth {
			pending = minLensDistance(pending, x.lower)
			continue
		}
		left, right := d, d
		if d.UMax-d.UMin >= d.VMax-d.VMin {
			m := d.UMin/2 + d.UMax/2
			left.UMax, right.UMin = m, m
		} else {
			m := d.VMin/2 + d.VMax/2
			left.VMax, right.VMin = m, m
		}
		for _, domain := range [2]geom.ParameterRectangle{left, right} {
			if !domain.Valid() || remaining().MaxOperations < 1 {
				return out, geom.ErrConvergence
			}
			patch, work, e := x.patch.TrimWithWorkLimit(domain, remaining().MaxOperations)
			if ce := charge(work); ce != nil {
				return out, ce
			}
			if e != nil {
				return out, e
			}
			box, e := patch.Bounds()
			if e != nil {
				return out, e
			}
			_, world, e := imageBox(box)
			if e != nil {
				return out, e
			}
			lower, _ := boxDistance(world)
			if a.err != nil {
				return out, a.err
			}
			if lower.Cmp(x.lower) < 0 {
				lower = x.lower
			}
			if out.Squared.Upper == nil || lower.Cmp(out.Squared.Upper) < 0 {
				heap.Push(&queue, tubeLensDistanceCell{patch: patch, chart: x.chart, depth: x.depth + 1, lower: lower})
			}
		}
	}
	return finish()
}

package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type AlgebraicTubeRegionOptions struct {
	AlgebraicTubeLensOptions
	MaxLoops int
	Exact    geom.ExactCurvePlaneEventOptions // total work for containment
}

type AlgebraicTubeRegionLoop struct {
	Branch, Parent     int // Parent is -1 at the root of a containment tree
	Winding            [2]int
	InteriorHeightSign int // sign of ZA-ZB just inside this loop
}

type AlgebraicTubeRegion struct {
	Outer      int
	Holes      []int
	UpperChart int
}

type AlgebraicTubeRegionTopology struct {
	Vertices, Edges, Faces, Components, InnerWires, EulerCharacteristic int
}

// AlgebraicTubeRegions describes all bounded regions between the graphs,
// with one closed boundary shell per region. Adjacent nested regions can
// meet at a zero-thickness rim; this collection does not regularize their
// union into one solid or claim that their closed boundaries are disjoint.
type AlgebraicTubeRegions struct {
	loops           []AlgebraicTubeRegionLoop
	regions         []AlgebraicTubeRegion
	partitions      []AlgebraicTubePairedPartition
	graphs          [2]*geom.AffineXYGraph
	support         *geom.SurfacePatch
	section         AlgebraicTrimmedPairSection
	ExactOperations int
}

func (r AlgebraicTubeRegions) Loops() []AlgebraicTubeRegionLoop {
	return append([]AlgebraicTubeRegionLoop(nil), r.loops...)
}

func (r AlgebraicTubeRegions) Regions() []AlgebraicTubeRegion {
	out := append([]AlgebraicTubeRegion(nil), r.regions...)
	for i := range out {
		out[i].Holes = append([]int(nil), out[i].Holes...)
	}
	return out
}

func (r AlgebraicTubeRegions) Boundary(loop int) []AlgebraicTubeFaceCutPiece {
	if loop < 0 || loop >= len(r.partitions) {
		return nil
	}
	return append([]AlgebraicTubeFaceCutPiece(nil), r.partitions[loop].Faces[0].Cut...)
}

func (r AlgebraicTubeRegions) Topology(region int) AlgebraicTubeRegionTopology {
	if region < 0 || region >= len(r.regions) {
		return AlgebraicTubeRegionTopology{}
	}
	x := r.regions[region]
	n := len(r.partitions[x.Outer].Faces[0].Cut)
	for _, i := range x.Holes {
		n += len(r.partitions[i].Faces[0].Cut)
	}
	return AlgebraicTubeRegionTopology{n, n, 2, 1, 2 * len(x.Holes), 2 - 2*len(x.Holes)}
}

// SealAlgebraicTubeRegions establishes containment among every disjoint
// closed section on convex affine-XY faces. The exact boundary point of
// one loop is classified against all others by a certified generic ray.
// Each loop yields its bounded region with immediate children as holes.
func (s AlgebraicTrimmedPairSection) SealAlgebraicTubeRegions(ctx context.Context, o AlgebraicTubeRegionOptions) (out AlgebraicTubeRegions, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicTubeRegions{ExactOperations: out.ExactOperations}
		}
	}()
	if ctx == nil || s.runSource() == nil || o.MaxLoops < 0 || o.MaxLoops > 64 {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if s.runSource().projection != [2]int{0, 1} {
		return out, geom.ErrConvergence
	}
	if o.MaxLoops == 0 {
		o.MaxLoops = 16
	}
	if o.Exact.MaxOperations == 0 {
		o.Exact.MaxOperations = 128000000
	}
	if o.Exact.MaxCoefficientBits == 0 {
		o.Exact.MaxCoefficientBits = 16384
	}
	if o.Exact.MaxOperations < 1 || o.Exact.MaxCoefficientBits < 64 || o.Exact.MaxSubdivisions < 0 {
		return out, geom.ErrInvalid
	}
	for _, f := range s.runSource().faces {
		if !convexStraightTubeLensFace(f) {
			return out, geom.ErrConvergence
		}
	}
	exact := func() geom.ExactCurvePlaneEventOptions {
		p := o.Exact
		p.MaxOperations -= out.ExactOperations
		return p
	}
	for i, f := range s.runSource().faces {
		p := o.Graph
		p.MaxOperations = capPositiveBudget(p.MaxOperations, exact().MaxOperations)
		if p.MaxOperations < 1 {
			return out, geom.ErrConvergence
		}
		q, e := exactSplineSupportPatch(f.Surface, p)
		if e != nil {
			return out, e
		}
		out.ExactOperations += q.ExactOperations()
		p.MaxOperations = capPositiveBudget(o.Graph.MaxOperations, exact().MaxOperations)
		if p.MaxOperations < 1 {
			return out, geom.ErrConvergence
		}
		out.graphs[i], e = geom.NewAffineXYGraph(ctx, q, p)
		if e != nil {
			return out, e
		}
		out.ExactOperations += out.graphs[i].ExactOperations()
		if i == 0 {
			out.support = q
		}
	}
	for i, b := range s.runSource().branches {
		for _, n := range b.Nodes {
			if n.Isolated {
				return out, geom.ErrConvergence
			}
		}
		if len(b.Runs) == 0 {
			continue
		}
		if len(b.Runs) != 1 || !b.Runs[0].Closed || len(out.loops) >= o.MaxLoops || exact().MaxOperations < 1 {
			return out, geom.ErrConvergence
		}
		p := o.Partition
		p.MaxOperations = capPositiveBudget(p.MaxOperations, exact().MaxOperations)
		q, e := s.PartitionBothFacesByTubeRun(ctx, i, 0, p)
		out.ExactOperations += q.ExactOperations
		if e != nil {
			return out, e
		}
		sign := q.Faces[0].FragmentHeightSigns[0]
		if sign == 0 || sign != -q.Faces[1].FragmentHeightSigns[0] {
			return out, geom.ErrConvergence
		}
		out.loops = append(out.loops, AlgebraicTubeRegionLoop{i, -1, [2]int{q.Faces[0].CutWinding, q.Faces[1].CutWinding}, sign})
		out.partitions = append(out.partitions, q)
	}
	if len(out.loops) == 0 {
		return out, geom.ErrConvergence
	}
	contains := make([][]bool, len(out.loops))
	for i := range contains {
		contains[i] = make([]bool, len(out.loops))
	}
	for i, x := range out.partitions {
		inner := x.Faces[0].Cut
		for j, z := range out.partitions {
			if i == j {
				continue
			}
			inside, used, e := tubeLoopContainment(ctx, out.support, z.Faces[0].Cut, inner, exact())
			out.ExactOperations += used
			if e != nil {
				return out, e
			}
			contains[j][i] = inside
		}
	}
	out.regions, err = assembleTubeRegionHierarchy(out.loops, contains)
	if err != nil {
		return out, err
	}
	out.section = s
	return out, nil
}

func assembleTubeRegionHierarchy(loops []AlgebraicTubeRegionLoop, contains [][]bool) ([]AlgebraicTubeRegion, error) {
	for i, x := range contains {
		for j, y := range x {
			if !y {
				continue
			}
			if contains[j][i] {
				return nil, geom.ErrConvergence
			}
			for k, z := range contains[j] {
				if z && !contains[i][k] {
					return nil, geom.ErrConvergence
				}
			}
		}
	}
	regions := make([]AlgebraicTubeRegion, len(loops))
	for i, l := range loops {
		regions[i].Outer = i
		if l.InteriorHeightSign < 0 {
			regions[i].UpperChart = 1
		}
		p := sortedTubeParents(contains, i)
		if len(p) == 0 {
			continue
		}
		parent := p[0].index
		for _, ancestor := range p[1:] {
			if !contains[ancestor.index][parent] {
				return nil, geom.ErrConvergence
			}
		}
		if l.InteriorHeightSign == loops[parent].InteriorHeightSign {
			return nil, geom.ErrConvergence
		}
		loops[i].Parent = parent
		regions[parent].Holes = append(regions[parent].Holes, i)
	}
	return regions, nil
}

// RegionFaces returns the two original graph supports with outward physical
// orientation, and exact original-UV traversal of their outer and hole cuts.
type AlgebraicTubeRegionFace struct {
	SourceSurface           geom.Surface // Original exact adapter or float support; Support is nil for adapters.
	Support                 *geom.BSplineSurface
	Reversed, OuterReversed bool
	HolesReversed           []bool
}

func (r AlgebraicTubeRegions) RegionFaces(i int) (out [2]AlgebraicTubeRegionFace) {
	if i < 0 || i >= len(r.regions) {
		return out
	}
	x := r.regions[i]
	for chart := range out {
		o := &out[chart]
		o.Support = r.partitions[x.Outer].Faces[chart].Support
		o.SourceSurface = tubeLensPartitionSurface(r.partitions[x.Outer].Faces[chart])
		o.Reversed = (chart != x.UpperChart) == (r.graphs[chart].Orientation() > 0)
		o.OuterReversed = r.loops[x.Outer].Winding[chart] < 0
		for _, h := range x.Holes {
			o.HolesReversed = append(o.HolesReversed, r.loops[h].Winding[chart] > 0)
		}
	}
	return out
}

type AlgebraicTubeRegionVerticalInterval struct {
	AlgebraicTubeLensVerticalInterval
	// Region is the selected open material component, or -1 for exclusion
	// or a shared rim. RimLoop is the exact section owner, or -1.
	Region, RimLoop int
}

func (r AlgebraicTubeRegions) LocateRational(ctx context.Context, p [3]*big.Rat,
	o geom.ExactCurvePlaneEventOptions) (parametric.Location, error) {
	if p[2] == nil {
		return parametric.Outside, geom.ErrInvalid
	}
	x, e := r.VerticalInterval(ctx, [2]*big.Rat{p[0], p[1]}, o)
	if e != nil {
		return parametric.Outside, e
	}
	return locateTubeVerticalInterval(p[2], x.AlgebraicTubeLensVerticalInterval)
}

// VerticalInterval classifies exact rational model XY in the arrangement of
// all section loops. A generic horizontal UV ray locates the deepest
// containing loop; an exact zero height identifies the unique rim owner.
func (r AlgebraicTubeRegions) VerticalInterval(ctx context.Context, xy [2]*big.Rat,
	o geom.ExactCurvePlaneEventOptions) (out AlgebraicTubeRegionVerticalInterval, err error) {
	out.Region, out.RimLoop = -1, -1
	if ctx == nil || r.graphs[0] == nil || xy[0] == nil || xy[1] == nil {
		return out, geom.ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 32000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxSubdivisions < 0 {
		return out, geom.ErrInvalid
	}
	exact := func() geom.ExactCurvePlaneEventOptions {
		p := o
		p.MaxOperations -= out.ExactOperations
		return p
	}
	eval := func() geom.SurfacePatchEvaluationOptions {
		p := exact()
		return geom.SurfacePatchEvaluationOptions{MaxOperations: p.MaxOperations, MaxCoefficientBits: p.MaxCoefficientBits}
	}
	for chart, g := range r.graphs {
		p := eval()
		if p.MaxOperations < 1 {
			return out, geom.ErrConvergence
		}
		x, e := g.InvertRational(ctx, xy, p)
		out.ExactOperations += x.Operations
		if e != nil {
			return out, e
		}
		if !x.Inside || !insideConvexTubeFace(r.section.runSource().faces[chart], x.UV) {
			out.Complete = true
			return out, nil
		}
		out.Parameters[chart] = x.UV
	}
	p := eval()
	if p.MaxOperations < 1 {
		return out, geom.ErrConvergence
	}
	h, e := r.graphs[0].CompareHeightAtRational(ctx, r.graphs[1], out.Parameters[0], p)
	out.ExactOperations += h.Operations
	if e != nil {
		return out, e
	}
	if !h.Complete || h.TargetOutside {
		return out, geom.ErrConvergence
	}
	uv := out.Parameters[0]
	if h.Sign == 0 {
		for i, q := range r.partitions {
			ok, used, e := pointOnTubeLoop(ctx, q.Faces[0].Cut, out.Parameters, exact())
			out.ExactOperations += used
			if e != nil {
				return out, e
			}
			if ok {
				if out.RimLoop >= 0 {
					return out, geom.ErrConvergence
				}
				out.RimLoop = i
			}
		}
		if out.RimLoop < 0 {
			return out, geom.ErrConvergence
		}
		out.Rim, out.Present, out.Complete = true, true, true
		out.Lower, out.Upper = h.SourcePoint[2], new(big.Rat).Set(h.SourcePoint[2])
		return out, nil
	}
	if exact().MaxOperations < 1 {
		return out, geom.ErrConvergence
	}
	root, used, e := geom.NewExactRationalRoot(ctx, uv[0], exact())
	out.ExactOperations += used
	if e != nil {
		return out, e
	}
	depth := -1
	for i, q := range r.partitions {
		inside, used, e := insideClosedTubeAtRoot(ctx, 0, r.support, q.Faces[0].Cut, root, uv[1], exact())
		out.ExactOperations += used
		if errors.Is(e, geom.ErrConvergence) {
			p := exact()
			if p.MaxOperations < 1 {
				return out, geom.ErrConvergence
			}
			v, used, e := geom.NewExactRationalRoot(ctx, uv[1], p)
			out.ExactOperations += used
			if e != nil {
				return out, e
			}
			inside, used, e = insideClosedTubeAtAxisRoot(ctx, 0, r.support, q.Faces[0].Cut, v, uv[0], 1, exact())
			out.ExactOperations += used
			if e != nil {
				return out, e
			}
		} else if e != nil {
			return out, e
		}
		if !inside {
			continue
		}
		n := 0
		for j := r.loops[i].Parent; j >= 0; j = r.loops[j].Parent {
			n++
		}
		if n == depth {
			return out, geom.ErrConvergence
		}
		if n > depth {
			depth, out.Region = n, i
		}
	}
	if out.Region >= 0 {
		x := r.regions[out.Region]
		if h.Sign != r.loops[x.Outer].InteriorHeightSign {
			return out, geom.ErrConvergence
		}
		z := [2]*big.Rat{h.SourcePoint[2], h.TargetPoint[2]}
		out.Lower, out.Upper = z[x.UpperChart^1], z[x.UpperChart]
		out.Present = true
	}
	out.Complete = true
	return out, nil
}

func insideConvexTubeFace(f Face, uv [2]*big.Rat) bool {
	for _, u := range f.Outer {
		x, y := ratInset(u.UVStart.X), ratInset(u.UVStart.Y)
		a, b := new(big.Rat).Sub(ratInset(u.UVEnd.X), x), new(big.Rat).Sub(ratInset(u.UVEnd.Y), y)
		d := new(big.Rat).Sub(new(big.Rat).Mul(a, new(big.Rat).Sub(uv[1], y)), new(big.Rat).Mul(b, new(big.Rat).Sub(uv[0], x)))
		if d.Sign() < 0 {
			return false
		}
	}
	return true
}

func pointOnTubeLoop(ctx context.Context, cut []AlgebraicTubeFaceCutPiece,
	uv [2][2]*big.Rat, o geom.ExactCurvePlaneEventOptions) (bool, int, error) {
	used := 0
	exact := func() geom.ExactCurvePlaneEventOptions {
		p := o
		p.MaxOperations -= used
		return p
	}
	makeRoot := func(v *big.Rat) (*geom.ExactAlgebraicRoot, error) {
		p := exact()
		if p.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		r, ops, e := geom.NewExactRationalRoot(ctx, v, p)
		used += ops
		return r, e
	}
	compare := func(a, z *geom.ExactAlgebraicRoot) (int, error) {
		p := exact()
		if p.MaxOperations < 1 {
			return 0, geom.ErrConvergence
		}
		r, e := a.Compare(ctx, z, p)
		used += r.Operations
		if e == nil && !r.Proven {
			e = geom.ErrConvergence
		}
		return r.Order, e
	}
	var roots [4]*geom.ExactAlgebraicRoot
	for i := range roots {
		x, e := makeRoot(uv[i/2][i%2])
		if e != nil {
			return false, used, e
		}
		roots[i] = x
	}
	for _, p := range cut {
		if e := ctx.Err(); e != nil {
			return false, used, e
		}
		axis, _, reverse, e := p.Segment.ExactFreeDomain()
		if e != nil {
			return false, used, e
		}
		bounds := [2]*geom.ExactAlgebraicRoot{p.FreeStart, p.FreeEnd}
		if reverse {
			bounds[0], bounds[1] = bounds[1], bounds[0]
		}
		a, e := compare(roots[axis], bounds[0])
		if e != nil {
			return false, used, e
		}
		z, e := compare(roots[axis], bounds[1])
		if e != nil {
			return false, used, e
		}
		if a < 0 || z > 0 {
			continue
		}
		x := exact()
		if x.MaxOperations < 1 {
			return false, used, geom.ErrConvergence
		}
		r, e := p.Segment.EvaluateRationalGraph(ctx, uv[axis/2][axis%2], geom.TubeAlgebraicGraphEndpointOptions{
			SurfaceCurveAgreementOptions: geom.SurfaceCurveAgreementOptions{
				SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: x.MaxOperations, MaxCoefficientBits: x.MaxCoefficientBits},
				MaxSubdivisions:     x.MaxSubdivisions}})
		used += r.ExactOperations
		if e != nil {
			return false, used, e
		}
		if !r.Proven {
			return false, used, geom.ErrConvergence
		}
		match := true
		for i, z := range roots {
			c, e := compare(z, r.Parameters[i])
			if e != nil {
				return false, used, e
			}
			match = match && c == 0
		}
		if match {
			return true, used, nil
		}
	}
	return false, used, nil
}

func capPositiveBudget(limit, remaining int) int {
	if limit == 0 || limit > remaining {
		return remaining
	}
	return limit
}

func tubeLoopContainment(ctx context.Context, support *geom.SurfacePatch, outer, inner []AlgebraicTubeFaceCutPiece,
	o geom.ExactCurvePlaneEventOptions) (inside bool, operations int, err error) {
	for _, c := range inner {
		if e := ctx.Err(); e != nil {
			return false, operations, e
		}
		free, d, _, e := c.Segment.ExactFreeDomain()
		if e != nil {
			return false, operations, e
		}
		// A V-free owner gives a rational horizontal-ray level directly.
		// The other chart's V is usable only after proving exact rationality.
		if free%2 == 0 {
			continue
		}
		width := new(big.Rat).Sub(d.Upper, d.Lower)
		for _, q := range [][2]int64{{1, 2}, {1, 3}, {2, 3}, {2, 5}, {3, 5}} {
			p := o
			p.MaxOperations -= operations
			if p.MaxOperations < 1 {
				return false, operations, geom.ErrConvergence
			}
			t := new(big.Rat).Add(d.Lower, new(big.Rat).Mul(width, big.NewRat(q[0], q[1])))
			ev, e := c.Segment.EvaluateRationalGraph(ctx, t, geom.TubeAlgebraicGraphEndpointOptions{
				SurfaceCurveAgreementOptions: geom.SurfaceCurveAgreementOptions{
					SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: p.MaxOperations, MaxCoefficientBits: p.MaxCoefficientBits},
					MaxSubdivisions:     p.MaxSubdivisions}})
			operations += ev.ExactOperations
			if errors.Is(e, geom.ErrConvergence) {
				continue
			}
			if e != nil {
				return false, operations, e
			}
			if !ev.Proven {
				continue
			}
			p.MaxOperations = o.MaxOperations - operations
			if p.MaxOperations < 1 {
				return false, operations, geom.ErrConvergence
			}
			r, e := ev.Parameters[1].Classify(ctx, p)
			operations += r.Operations
			if e != nil {
				return false, operations, e
			}
			if r.Kind != geom.ExactRootKindRational {
				continue
			}
			p.MaxOperations = o.MaxOperations - operations
			if p.MaxOperations < 1 {
				return false, operations, geom.ErrConvergence
			}
			b, used, e := insideClosedTubeAtRoot(ctx, 0, support, outer, ev.Parameters[0], r.Rational, p)
			operations += used
			if errors.Is(e, geom.ErrConvergence) {
				continue
			}
			return b, operations, e
		}
	}
	return false, operations, geom.ErrConvergence
}

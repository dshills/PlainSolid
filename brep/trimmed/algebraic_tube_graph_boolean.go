package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// AlgebraicTubeGraphBooleanOptions bounds the common-domain and strict
// base separation proofs. The sign cover is a sufficient Bernstein test:
// an inconclusive ordering rejects the operation.
type AlgebraicTubeGraphBooleanOptions struct {
	Patch              geom.SurfacePatchOptions
	MaxDepth, MaxCells int
	Regions            AlgebraicTubeRegionOptions
}

// AlgebraicTubeGraphDifference is a regularized subtraction of two graph
// solids over the same projected rectangle and common base. Every component
// retains an exact pair of original graph faces and original tube edges.
type AlgebraicTubeGraphDifference struct {
	regions                     AlgebraicTubeRegions
	selected                    []int
	verifiedBase                *geom.BSplineSurface
	verifiedMinuend             int
	exterior                    bool
	ProofCells, ExactOperations int
}

func (d AlgebraicTubeGraphDifference) Regions() AlgebraicTubeRegions { return d.regions }
func (d AlgebraicTubeGraphDifference) Base() *geom.BSplineSurface    { return d.verifiedBase }
func (d AlgebraicTubeGraphDifference) MinuendChart() int             { return d.verifiedMinuend }
func (d AlgebraicTubeGraphDifference) IncludesExterior() bool        { return d.exterior }

// SelectedRegions returns material region IDs in component order. The
// exterior component is represented by -1.
func (d AlgebraicTubeGraphDifference) SelectedRegions() []int {
	return append([]int(nil), d.selected...)
}

// SubtractAlgebraicTubeGraphSolids subtracts the other chart's material
// from minuend. Both input graph solids share a strictly lower base over
// an exactly matched projected rectangle. Only bounded positive-difference
// regions enclosed by the certified section are supported here; use
// SubtractAlgebraicTubeGraphSolidsWithExterior for an exterior shell.
func (s AlgebraicTrimmedPairSection) SubtractAlgebraicTubeGraphSolids(ctx context.Context,
	base *geom.BSplineSurface, minuend int, o AlgebraicTubeGraphBooleanOptions) (out AlgebraicTubeGraphDifference, err error) {
	return s.subtractAlgebraicTubeGraphSolids(ctx, base, minuend, o, false)
}

// SubtractAlgebraicTubeGraphSolidsWithExterior additionally permits positive
// height difference along the entire projected boundary. Its exterior
// component retains the four original domain walls.
func (s AlgebraicTrimmedPairSection) SubtractAlgebraicTubeGraphSolidsWithExterior(ctx context.Context,
	base *geom.BSplineSurface, minuend int, o AlgebraicTubeGraphBooleanOptions) (AlgebraicTubeGraphDifference, error) {
	return s.subtractAlgebraicTubeGraphSolids(ctx, base, minuend, o, true)
}

func (s AlgebraicTrimmedPairSection) subtractAlgebraicTubeGraphSolids(ctx context.Context,
	base *geom.BSplineSurface, minuend int, o AlgebraicTubeGraphBooleanOptions, allowExterior bool) (out AlgebraicTubeGraphDifference, err error) {
	if ctx == nil || base == nil || s.partitionSource == nil || minuend < 0 || minuend > 1 ||
		o.MaxDepth < 0 || o.MaxDepth > 20 || o.MaxCells < 0 || o.MaxCells > 262144 {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 12
	}
	if o.MaxCells == 0 {
		o.MaxCells = 8192
	}
	if o.Patch.MaxOperations == 0 {
		o.Patch.MaxOperations = 64000000
	}
	if o.Patch.MaxCoefficientBits == 0 {
		o.Patch.MaxCoefficientBits = 16384
	}
	if o.Patch.MaxOperations < 1 || o.Patch.MaxCoefficientBits < 64 {
		return out, geom.ErrInvalid
	}
	defer func() {
		if err != nil {
			out = AlgebraicTubeGraphDifference{ProofCells: out.ProofCells, ExactOperations: out.ExactOperations}
		}
	}()
	patchOptions := func() geom.SurfacePatchOptions {
		p := o.Patch
		p.MaxOperations -= out.ExactOperations
		return p
	}
	makePatch := func(s *geom.BSplineSurface) (*geom.SurfacePatch, error) {
		p := patchOptions()
		if p.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		r, e := geom.NewSurfacePatch(s, p)
		if e == nil {
			out.ExactOperations += r.ExactOperations()
		}
		return r, e
	}
	var tops [2]*geom.SurfacePatch
	for i, f := range s.partitionSource.faces {
		d, e := faceUVBounds(f)
		if e != nil {
			return out, e
		}
		if !splineDistanceRectangle(f, d) {
			return out, geom.ErrConvergence
		}
		p, e := makePatch(facespline(f))
		if e != nil {
			return out, e
		}
		tops[i], e = p.Trim(d)
		if e != nil {
			return out, e
		}
		out.ExactOperations += tops[i].ExactOperations()
	}
	floor, e := makePatch(base)
	if e != nil {
		return out, e
	}
	difference := func(a, b *geom.SurfacePatch) (*geom.SurfacePatch, error) {
		p := patchOptions()
		if p.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		m, e := geom.AlignAffineXYCharts(ctx, a, b, p)
		if e != nil {
			return nil, e
		}
		out.ExactOperations += m.ExactOperations()
		p = patchOptions()
		if p.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		r, e := geom.NewGeneralSurfaceHeightDifferencePatch(ctx, a, m.TargetOnSource(), geom.GeneralSurfaceHeightOptions{
			MaxPoles: p.MaxPoles, MaxOperations: p.MaxOperations, MaxCoefficientBits: p.MaxCoefficientBits})
		if e == nil {
			out.ExactOperations += r.ExactOperations()
		}
		return r, e
	}
	for _, p := range tops {
		q, e := difference(p, floor)
		if e != nil {
			return out, e
		}
		visited, used, e := proveStrictGraphHeight(ctx, q, o.MaxDepth, o.MaxCells-out.ProofCells, patchOptions().MaxOperations)
		out.ProofCells += visited
		out.ExactOperations += used
		if e != nil {
			return out, e
		}
	}
	d, e := difference(tops[minuend], tops[minuend^1])
	if e != nil {
		return out, e
	}
	p := patchOptions()
	if p.MaxOperations < 1 {
		return out, geom.ErrConvergence
	}
	zero, _ := geom.NewPlane(geom.Vec3{}, geom.Vec3{Z: 1})
	sign, e := geom.ClassifyPlaneBoundarySigns(d, zero, geom.PlaneBoundarySignOptions{
		MaxOperations: p.MaxOperations, MaxCoefficientBits: p.MaxCoefficientBits})
	out.ExactOperations += sign.ExactOperations
	if e != nil {
		return out, e
	}
	if !sign.Complete || sign.StrictSign == 0 || sign.StrictSign > 0 && !allowExterior {
		return out, geom.ErrConvergence
	}
	r, e := s.SealAlgebraicTubeRegions(ctx, o.Regions)
	if e != nil {
		return out, e
	}
	out.regions = r
	out.verifiedBase, out.verifiedMinuend = base, minuend
	if sign.StrictSign > 0 {
		unit := geom.ParameterRectangle{UMax: 1, VMax: 1}
		for _, f := range s.partitionSource.faces {
			if !splineDistanceRectangle(f, unit) {
				return out, geom.ErrConvergence
			}
			if e := requireIdentitySplineChart(ctx, facespline(f), o.Patch); e != nil {
				return out, e
			}
		}
		out.exterior = true
		out.selected = append(out.selected, -1)
	}
	for i, x := range r.regions {
		if x.UpperChart == minuend {
			out.selected = append(out.selected, i)
		}
	}
	return out, nil
}

func proveStrictGraphHeight(ctx context.Context, p *geom.SurfacePatch, maxDepth, maxCells, maxOps int) (visited, used int, err error) {
	type item struct {
		p     *geom.SurfacePatch
		depth int
	}
	q := []item{{p, 0}}
	for len(q) > 0 {
		if e := ctx.Err(); e != nil {
			return visited, used, e
		}
		if visited >= maxCells || used >= maxOps {
			return visited, used, geom.ErrConvergence
		}
		x := q[len(q)-1]
		q = q[:len(q)-1]
		visited++
		b, e := x.p.Bounds()
		if e != nil {
			return visited, used, e
		}
		if b.Min.Z > 0 {
			continue
		}
		if b.Max.Z <= 0 || x.depth >= maxDepth {
			return visited, used, geom.ErrConvergence
		}
		a, z := x.p.Domain(), x.p.Domain()
		if a.UMax-a.UMin >= a.VMax-a.VMin {
			m := a.UMin/2 + a.UMax/2
			a.UMax, z.UMin = m, m
		} else {
			m := a.VMin/2 + a.VMax/2
			a.VMax, z.VMin = m, m
		}
		for _, d := range [2]geom.ParameterRectangle{a, z} {
			y, e := x.p.Trim(d)
			if e != nil {
				return visited, used, e
			}
			used += y.ExactOperations()
			if used > maxOps {
				return visited, used, geom.ErrConvergence
			}
			q = append(q, item{y, x.depth + 1})
		}
	}
	return visited, used, nil
}

// Solid constructs a selected native component when all exact section
// edges have the complete-tube representation supported by Solid.
func (d AlgebraicTubeGraphDifference) Solid(ctx context.Context, component int, tolerance float64, o edgecheck.Options) (Solid, error) {
	if component < 0 || component >= len(d.selected) {
		return Solid{}, geom.ErrInvalid
	}
	i := d.selected[component]
	if i < 0 {
		return d.exteriorSolid(ctx, tolerance, o)
	}
	return d.regions.Solid(ctx, i, tolerance, o)
}

// Operation identifies the regularized operation represented by this result.
func (d AlgebraicTubeGraphDifference) Operation() parametric.BooleanOp { return parametric.Difference }

// LocateRational classifies against the retained subtraction components.
// An unselected component's rim is retained only when it also bounds a
// selected component, so removed zero-thickness contacts cannot reappear.
func (d AlgebraicTubeGraphDifference) LocateRational(ctx context.Context, p [3]*big.Rat,
	o geom.ExactCurvePlaneEventOptions) (parametric.Location, error) {
	if p[2] == nil || d.verifiedBase == nil {
		return parametric.Outside, geom.ErrInvalid
	}
	v, e := d.regions.VerticalInterval(ctx, [2]*big.Rat{p[0], p[1]}, o)
	if e != nil {
		return parametric.Outside, e
	}
	if !v.Complete {
		return parametric.Outside, geom.ErrConvergence
	}
	if !v.Present {
		if !d.exterior || v.Parameters[0][0] == nil || v.Parameters[1][0] == nil {
			return parametric.Outside, nil
		}
		if o.MaxOperations == 0 {
			o.MaxOperations = 32000000
		}
		if o.MaxCoefficientBits == 0 {
			o.MaxCoefficientBits = 16384
		}
		remaining := o.MaxOperations - v.ExactOperations
		if remaining < 1 {
			return parametric.Outside, geom.ErrConvergence
		}
		h, e := d.regions.graphs[0].CompareHeightAtRational(ctx, d.regions.graphs[1], v.Parameters[0], geom.SurfacePatchEvaluationOptions{
			MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return parametric.Outside, e
		}
		if !h.Complete || h.TargetOutside {
			return parametric.Outside, geom.ErrConvergence
		}
		z := [2]*big.Rat{h.SourcePoint[2], h.TargetPoint[2]}
		i := d.verifiedMinuend
		if z[i].Cmp(z[i^1]) <= 0 {
			return parametric.Outside, geom.ErrConvergence
		}
		result, e := locateTubeVerticalInterval(p[2], AlgebraicTubeLensVerticalInterval{Complete: true, Present: true, Lower: z[i^1], Upper: z[i]})
		if e == nil && result == parametric.Inside {
			for _, u := range v.Parameters[0] {
				if u.Sign() == 0 || u.Cmp(big.NewRat(1, 1)) == 0 {
					return parametric.Boundary, nil
				}
			}
		}
		return result, e
	}
	for _, i := range d.selected {
		if i < 0 {
			continue
		}
		x := d.regions.regions[i]
		if i == v.Region || v.Rim && (v.RimLoop == x.Outer || d.regions.loops[v.RimLoop].Parent == x.Outer) {
			return locateTubeVerticalInterval(p[2], v.AlgebraicTubeLensVerticalInterval)
		}
	}
	if d.exterior && v.Rim && v.RimLoop >= 0 && d.regions.loops[v.RimLoop].Parent < 0 {
		return locateTubeVerticalInterval(p[2], v.AlgebraicTubeLensVerticalInterval)
	}
	return parametric.Outside, nil
}

func (d AlgebraicTubeGraphDifference) exteriorSolid(ctx context.Context, tolerance float64, o edgecheck.Options) (Solid, error) {
	if !d.exterior || ctx == nil || tolerance <= 0 || !finite(tolerance) {
		return Solid{}, geom.ErrInvalid
	}
	i := d.verifiedMinuend
	src := d.regions.section.partitionSource.faces
	sides := make([]openSectionPiece, 4)
	for i := range sides {
		sides[i] = openSectionPiece{a: unitSquare[i], b: unitSquare[(i+1)%4], side: i}
	}
	stock, h, e := assembleOpenSplineGraphPair(facespline(src[i]), facespline(src[i^1]), 0, true, tolerance, sides)
	if e != nil {
		return Solid{}, e
	}
	v, edges, faces := stock.Vertices(), stock.Edges(), stock.Faces()
	holes := 0
	for j, x := range d.regions.loops {
		if x.Parent >= 0 {
			continue
		}
		q := d.regions.partitions[j]
		var local [2]Face
		for k := range local {
			// Temporary shell represents the opposite order inside this
			// disk. Its complete tubes can then supply the exterior hole.
			local[k] = Face{Surface: q.Faces[k].Support, Reversed: k == i}
		}
		ring := tubeRegionSolidRing{cut: q.Faces[0].Cut, reversed: [2]bool{
			q.Faces[0].Fragments[0].CutReversed, q.Faces[1].Fragments[0].CutReversed}}
		disk, e := assembleTubeRegionSolid(ctx, local, []tubeRegionSolidRing{ring}, tolerance, o)
		if e != nil {
			return Solid{}, e
		}
		vo, eo := len(v), len(edges)
		v = append(v, disk.vertices...)
		for _, p := range disk.edges {
			p.Start += vo
			p.End += vo
			edges = append(edges, p)
		}
		for chart, fid := range [2]int{h.SplineFace, h.PlaneFace} {
			original := i ^ chart
			var hole []Use
			u := disk.faces[original].Outer
			for k := len(u) - 1; k >= 0; k-- {
				z := u[k]
				z.Edge += eo
				z.Reversed = !z.Reversed
				z.UVStart, z.UVEnd = z.UVEnd, z.UVStart
				z.Curve = reversePCurve(z.Curve)
				hole = append(hole, z)
			}
			faces[fid].Holes = append(faces[fid].Holes, hole)
		}
		holes++
	}
	s, e := NewSolidContext(ctx, v, edges, faces, tolerance)
	if e != nil {
		return Solid{}, e
	}
	p, e := s.ValidateEdgeAgreement(ctx, o)
	if e != nil {
		return Solid{}, e
	}
	if !p.Complete || s.Topology().EulerCharacteristic != 2-2*holes {
		return Solid{}, geom.ErrConvergence
	}
	return s, nil
}

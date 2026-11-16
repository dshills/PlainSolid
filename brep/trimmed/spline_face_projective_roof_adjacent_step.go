package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// BuildProjectiveSplineRoofAdjacentStepFuse joins two original stocks
// along one complete adjacent projected side whose original lifted rims
// are strictly ordered. The sole separating wall lies between the rims;
// the two adjacent exterior walls retain the exact lower-height corners.
func BuildProjectiveSplineRoofAdjacentStepFuse(ctx context.Context, a, b Face, axes [2]int, baseHeight float64,
	o ProjectiveRoofSolidOptions) (r ProjectiveRoofSolid, err error) {
	p, err := proveProjectiveRoofFootprints(ctx, a, b, axes, baseHeight, o)
	if err != nil {
		return r, err
	}
	return buildProjectiveRoofAdjacentStepFuse(ctx, p, axes, baseHeight, o)
}

func buildProjectiveRoofAdjacentStepFuse(ctx context.Context, p projectiveRoofFootprintProof, axes [2]int, baseHeight float64,
	o ProjectiveRoofSolidOptions) (r ProjectiveRoofSolid, err error) {
	if !p.overlap.NoArea {
		return r, geom.ErrDomain
	}
	source := o.Envelope.Imprint.Contacts.Patches
	order, err := geom.CertifyProjectiveGraphBoundaryHeightOrder(ctx, p.supports[0], p.supports[1], axes,
		geom.SurfaceAffineCurveOptions{TubeAffineLineOptions: geom.TubeAffineLineOptions{
			MaxPoles: source.MaxPoles, MaxOperations: source.MaxOperations, MaxCoefficientBits: source.MaxCoefficientBits,
		}})
	if err != nil {
		return r, err
	}
	if !order.Complete || order.Sign == 0 {
		return r, geom.ErrConvergence
	}
	low, high := 0, 1
	if order.Sign > 0 {
		low, high = 1, 0
	}
	var parts []SewFace
	var external []projectiveRoofRim
	var shared [2]projectiveRoofRim
	var sourceEdges [2]int
	for owner := range 2 {
		f, rims, err := buildOriginalProjectiveRoofRim(ctx, p.owners[owner], p.supports[owner], owner, o.Build)
		if err != nil {
			return r, err
		}
		found := false
		for _, rim := range rims {
			u := f.Face.Outer[rim.use.OriginalUse]
			if !projectiveOriginalBoundaryMatch(u, owner, order.Boundary) {
				external = append(external, rim)
				continue
			}
			if found {
				return r, geom.ErrConvergence
			}
			found, shared[owner], sourceEdges[owner] = true, rim, u.Edge
		}
		if !found {
			return r, geom.ErrConvergence // Partial original sides are not a step roof.
		}
		parts = append(parts, f)
	}
	if len(external) != 6 || !orderProjectiveSteppedPerimeter(external, axes) {
		return r, geom.ErrConvergence
	}
	height := 3 - axes[0] - axes[1]
	parity := 1
	if (axes[0]+1)%3 != axes[1] {
		parity = -1
	}
	var mins, maxs [2]float64
	for i, axis := range axes {
		mins[i] = projectiveVectorAxis(external[0].ends[0], axis)
		maxs[i] = mins[i]
		for _, side := range external {
			for _, p := range side.ends {
				x := projectiveVectorAxis(p, axis)
				mins[i], maxs[i] = min(mins[i], x), max(maxs[i], x)
			}
		}
	}
	var walls []ProjectiveRoofWall
	var bottoms []projectiveRoofBottom
	splits := 0
	for _, side := range external {
		wall, bottom, err := makeProjectiveRoofWallWithBaseChart(ctx, side, axes, height, baseHeight, mins, maxs, true, o.Build)
		if err != nil {
			return r, err
		}
		var at [2]*geom.Vec3
		if side.owner == high {
			for i, end := range side.ends {
				for _, p := range shared[low].ends {
					if sameProjectiveRoofProjection(end, p, axes) {
						p := p
						at[i] = &p
						splits++
					}
				}
			}
		}
		if at != [2]*geom.Vec3{} {
			lower := projectiveRoofRim{model: bottom.model, ends: bottom.ends}
			wall, _, err = makeProjectiveRoofPhysicalWall(ctx, lower, side, at, axes, height, o.Build)
			if err != nil {
				return r, err
			}
		}
		walls = append(walls, ProjectiveRoofWall{Owner: side.owner, Face: len(parts), Source: side.use})
		bottoms = append(bottoms, bottom)
		parts = append(parts, wall)
	}
	if splits != 2 {
		return r, geom.ErrConvergence
	}
	stepID := len(parts)
	step, local, err := makeProjectiveRoofPhysicalWall(ctx, shared[low], shared[high], [2]*geom.Vec3{}, axes, height, o.Build)
	if err != nil {
		return r, err
	}
	parts = append(parts, step)
	baseID := len(parts)
	base, err := makeProjectiveRoofBaseWithChart(ctx, bottoms, axes, height, baseHeight, mins, maxs, parity, true, o.Build)
	if err != nil {
		return r, err
	}
	parts = append(parts, base)
	pairs := [][2]projectiveRoofEdgeID{{{low, sourceEdges[low]}, {stepID, local[0]}}, {{high, sourceEdges[high]}, {stepID, local[1]}}}
	solid, sections, err := sewProjectiveRoofPairedEdges(ctx, parts, pairs, o.Sew)
	if err != nil {
		return r, err
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	stepSource := ProjectiveRoofStep{Face: stepID, Owners: [2]int{low, high}, Edges: [2]int(sections),
		Uses: [2]ProjectiveSplineImprintUse{shared[low].use, shared[high].use}}
	return ProjectiveRoofSolid{Solid: solid, Roofs: [2]int{0, 1}, Walls: walls, Steps: []ProjectiveRoofStep{stepSource},
		BaseFace: baseID, Section: sections[0], SectionEdges: sections, Complete: true}, nil
}

func projectiveOriginalBoundaryMatch(u Use, owner int, b geom.ProjectiveGraphBoundaryContact) bool {
	for rev := range 2 {
		good := true
		for i, p := range [2]geom.Vec2{u.UVStart, u.UVEnd} {
			q := b.Endpoints[i^rev].OriginalUV[owner]
			good = good && q[0].Cmp(new(big.Rat).SetFloat64(p.X)) == 0 && q[1].Cmp(new(big.Rat).SetFloat64(p.Y)) == 0
		}
		if good {
			return true
		}
	}
	return false
}

func sameProjectiveRoofProjection(a, b geom.Vec3, axes [2]int) bool {
	return projectiveVectorAxis(a, axes[0]) == projectiveVectorAxis(b, axes[0]) &&
		projectiveVectorAxis(a, axes[1]) == projectiveVectorAxis(b, axes[1])
}

func orderProjectiveSteppedPerimeter(rims []projectiveRoofRim, axes [2]int) bool {
	for i := 1; i < len(rims); i++ {
		found := -1
		for j := i; j < len(rims); j++ {
			if sameProjectiveRoofProjection(rims[i-1].ends[1], rims[j].ends[0], axes) {
				if found >= 0 {
					return false
				}
				found = j
			}
		}
		if found < 0 {
			return false
		}
		rims[i], rims[found] = rims[found], rims[i]
	}
	return sameProjectiveRoofProjection(rims[len(rims)-1].ends[1], rims[0].ends[0], axes)
}

// makeProjectiveRoofPhysicalWall uses actual free coordinate and height.
// A normalized ruled wall may not represent the exact intermediate corner.
func makeProjectiveRoofPhysicalWall(ctx context.Context, bottom, top projectiveRoofRim, split [2]*geom.Vec3,
	axes [2]int, height int, o geom.SurfacePatchOptions) (out SewFace, bands [2]int, err error) {
	fixed, free := axes[0], axes[1]
	if projectiveVectorAxis(top.ends[0], fixed) != projectiveVectorAxis(top.ends[1], fixed) {
		fixed, free = free, fixed
	}
	z := func(p geom.Vec3) float64 { return projectiveVectorAxis(p, height) }
	x := func(p geom.Vec3) float64 { return projectiveVectorAxis(p, free) }
	uv := func(p geom.Vec3) geom.Vec2 { return geom.Vec2{X: x(p), Y: z(p)} }
	baseReverse := !sameProjectiveRoofProjection(top.ends[0], bottom.ends[0], axes)
	if baseReverse {
		bottom.ends[0], bottom.ends[1] = bottom.ends[1], bottom.ends[0]
	}
	for i := range 2 {
		if !sameProjectiveRoofProjection(top.ends[i], bottom.ends[i], axes) || z(top.ends[i]) <= z(bottom.ends[i]) {
			return out, bands, geom.ErrConvergence
		}
		if p := split[i]; p != nil && (!sameProjectiveRoofProjection(top.ends[i], *p, axes) || z(*p) <= z(bottom.ends[i]) || z(*p) >= z(top.ends[i])) {
			return out, bands, geom.ErrConvergence
		}
	}
	wallAt := projectiveVectorAxis(top.ends[0], fixed)
	if x(top.ends[0]) == x(top.ends[1]) || wallAt != projectiveVectorAxis(top.ends[1], fixed) {
		return out, bands, geom.ErrConvergence
	}
	zlo, zhi := min(z(bottom.ends[0]), z(bottom.ends[1])), max(z(top.ends[0]), z(top.ends[1]))
	for _, r := range []projectiveRoofRim{bottom, top} {
		net, _, err := r.model.BezierControlNet(ctx, o)
		if err != nil {
			return out, bands, err
		}
		for _, h := range net.Control {
			if new(big.Rat).Mul(new(big.Rat).SetFloat64(wallAt), h[3]).Cmp(h[fixed]) != 0 {
				return out, bands, geom.ErrConvergence
			}
		}
		bounds, err := r.model.ProjectionBounds()
		if err != nil {
			return out, bands, err
		}
		zlo, zhi = min(zlo, z(bounds.Min)), max(zhi, z(bounds.Max))
	}
	xlo, xhi := min(x(top.ends[0]), x(top.ends[1])), max(x(top.ends[0]), x(top.ends[1]))
	var poles []geom.Vec3
	for _, a := range [2]float64{xlo, xhi} {
		for _, b := range [2]float64{zlo, zhi} {
			var p [3]float64
			p[free], p[height], p[fixed] = a, b, wallAt
			poles = append(poles, geom.Vec3{X: p[0], Y: p[1], Z: p[2]})
		}
	}
	surface, err := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{xlo, xlo, xhi, xhi}, []float64{zlo, zlo, zhi, zhi}, nil)
	if err != nil {
		return out, bands, err
	}
	out.Face.Surface = surface
	out.Vertices = append(out.Vertices, bottom.ends[0], bottom.ends[1])
	if split[1] != nil {
		out.Vertices = append(out.Vertices, *split[1])
	}
	upper := len(out.Vertices)
	out.Vertices = append(out.Vertices, top.ends[1], top.ends[0])
	if split[0] != nil {
		out.Vertices = append(out.Vertices, *split[0])
	}
	bands = [2]int{0, upper}
	for i, p := range out.Vertices {
		j := (i + 1) % len(out.Vertices)
		q := out.Vertices[j]
		var model *geom.ExactSplineCurve
		var use Use
		use.Edge, use.UVStart, use.UVEnd = i, uv(p), uv(q)
		switch i {
		case 0:
			model, use.Reversed = bottom.model, baseReverse
		case upper:
			model, use.Reversed = top.model, true
		default:
			model, err = projectiveRoofVertical(ctx, p, q, o)
			if err != nil {
				return SewFace{}, bands, err
			}
		}
		edge := parametric.Edge{Start: i, End: j, Curve: model, Range: parametric.Interval{Max: 1}}
		if use.Reversed {
			edge.Start, edge.End = edge.End, edge.Start
		}
		if i == 0 || i == upper {
			net, _, err := model.BezierControlNet(ctx, o)
			if err != nil {
				return SewFace{}, bands, err
			}
			curve, err := projectiveRoofBaseUV(net.Control, [2]int{free, height}, [2]float64{}, [2]float64{}, true)
			if err != nil {
				return SewFace{}, bands, err
			}
			if use.Reversed {
				curve = curve.reversed()
			}
			use.Curve = curve
		}
		out.Edges, out.Face.Outer = append(out.Edges, edge), append(out.Face.Outer, use)
	}
	if x(top.ends[0]) > x(top.ends[1]) {
		reverseGraphUses(out.Face.Outer)
		out.Face.Reversed = true
	}
	return out, bands, nil
}

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ProjectiveRoofSolidOptions bounds source material selection and each
// exact curve/surface construction and sewn shell verification.
type ProjectiveRoofSolidOptions struct {
	Envelope ProjectiveSplineEnvelopeOptions
	Build    geom.SurfacePatchOptions
	Sew      ExactSewOptions
	// MatchExteriorPartitions lets a contour Cut split exterior uses at
	// the exact common atlas stations of either original roof.
	MatchExteriorPartitions bool
}

// ProjectiveRoofWall retains the original roof owner, its original
// boundary use and the resulting wall face.
type ProjectiveRoofWall struct {
	Owner, Face int
	Source      ProjectiveSplineImprintUse
}

// ProjectiveRoofSolid is a regularized Boolean of two solids defined
// between the same constant-height base and the original faces.
// Roofs and Walls retain their original face/edge sources.
type ProjectiveRoofSolid struct {
	Solid Solid
	Roofs [2]int // -1 for an absent owner in a completed result.
	Walls []ProjectiveRoofWall
	// FloorSources is ordered like Walls for Cut; it retains original B uses.
	FloorSources []ProjectiveSplineImprintUse
	BaseFace     int
	Section      int // First section edge, or -1 when there is no section.
	SectionEdges []int
	Complete     bool
	Empty        bool // A proved empty result has no Solid.
}

type projectiveRoofRim struct {
	model *geom.ExactSplineCurve
	ends  [2]geom.Vec3
	owner int
	use   ProjectiveSplineImprintUse
}

type projectiveRoofBottom struct {
	model *geom.ExactSplineCurve
	ends  [2]geom.Vec3
	uv    ExactQuadraticUV
}

// BuildProjectiveSplineRoofSolid closes the exact lower or upper envelope
// over its full rectangular physical projection. Roofs must lie strictly
// above baseHeight. Each exterior border is a single exact linear or
// quadratic rational curve whose projection stays on one rectangle side.
func BuildProjectiveSplineRoofSolid(ctx context.Context, a, b Face, axes [2]int, baseHeight float64, operation parametric.BooleanOp,
	o ProjectiveRoofSolidOptions) (r ProjectiveRoofSolid, err error) {
	return buildProjectiveSplineRoofSolid(ctx, a, b, axes, baseHeight, operation, o, true)
}

// BuildProjectiveSplineRoofContourSolid closes an original Common or Fuse
// with one open analytic contour, preserving all its section edges and
// the original subdivided exterior boundary above the common base.
func BuildProjectiveSplineRoofContourSolid(ctx context.Context, a, b Face, axes [2]int, baseHeight float64,
	operation parametric.BooleanOp, o ProjectiveRoofSolidOptions) (r ProjectiveRoofSolid, err error) {
	return buildProjectiveSplineRoofSolid(ctx, a, b, axes, baseHeight, operation, o, false)
}

func buildProjectiveSplineRoofSolid(ctx context.Context, a, b Face, axes [2]int, baseHeight float64, operation parametric.BooleanOp,
	o ProjectiveRoofSolidOptions, single bool) (r ProjectiveRoofSolid, err error) {
	if ctx == nil || !finite(baseHeight) || axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 ||
		axes[0] == axes[1] || o.Sew.RepairOrientation {
		return r, geom.ErrInvalid
	}
	envelope, err := selectProjectiveSplineFaceEnvelope(ctx, a, b, axes, operation, o.Envelope, single)
	if err != nil {
		return r, err
	}
	height := 3 - axes[0] - axes[1]
	parity := 1
	if (axes[0]+1)%3 != axes[1] {
		parity = -1
	}
	if err := projectiveRoofsAboveBase(a, b, height, baseHeight, o.Build); err != nil {
		return r, err
	}
	var rim []projectiveRoofRim
	var seams [2][]int
	arcs := make(map[int]int, len(envelope.Sections))
	for i, section := range envelope.Sections {
		arcs[section.Arc] = i
	}
	for owner, f := range envelope.Faces {
		seams[owner] = make([]int, len(arcs))
		for i := range seams[owner] {
			seams[owner][i] = -1
		}
		n := len(f.Face.Outer)
		for j := range n {
			i := j
			if f.Face.Reversed {
				i = n - 1 - j
			}
			u, source := f.Face.Outer[i], envelope.Sources[owner][i]
			if source.Section {
				if k, ok := arcs[source.SectionArc]; ok && seams[owner][k] < 0 {
					seams[owner][k] = u.Edge
				} else {
					return r, geom.ErrConvergence
				}
				continue
			}
			m := f.Edges[u.Edge].Curve.(*geom.ExactSplineCurve)
			net, _, err := m.BezierControlNet(ctx, o.Build)
			if err != nil {
				return r, err
			}
			if len(net.Control) < 2 || len(net.Control) > 3 || net.Start != 0 || net.End != 1 {
				return r, geom.ErrConvergence
			}
			if u.Reversed != f.Face.Reversed {
				for p, q := 0, len(net.Control)-1; p < q; p, q = p+1, q-1 {
					net.Control[p], net.Control[q] = net.Control[q], net.Control[p]
				}
				m, err = geom.NewExactHomogeneousBezier(ctx, net.Control, o.Build)
				if err != nil {
					return r, err
				}
			}
			var ends [2]geom.Vec3
			for j := range ends {
				ends[j], err = m.ExactPoint(float64(j))
				if err != nil {
					return r, err
				}
			}
			rim = append(rim, projectiveRoofRim{m, ends, owner, source})
		}
	}
	if len(rim) < 6 || single && len(rim) != 6 {
		return r, geom.ErrConvergence
	}
	for i := 1; i < len(rim); i++ {
		found := -1
		for j := i; j < len(rim); j++ {
			if rim[i-1].ends[1] == rim[j].ends[0] {
				if found >= 0 {
					return r, geom.ErrConvergence
				}
				found = j
			}
		}
		if found < 0 {
			return r, geom.ErrConvergence
		}
		rim[i], rim[found] = rim[found], rim[i]
	}
	if rim[len(rim)-1].ends[1] != rim[0].ends[0] {
		return r, geom.ErrConvergence
	}
	p := [2]float64{projectiveVectorAxis(rim[0].ends[0], axes[0]), projectiveVectorAxis(rim[0].ends[0], axes[1])}
	lower, upper := p, p
	for _, edge := range rim {
		for _, end := range edge.ends {
			for i, axis := range axes {
				x := projectiveVectorAxis(end, axis)
				lower[i], upper[i] = min(lower[i], x), max(upper[i], x)
			}
		}
	}
	if lower[0] >= upper[0] || lower[1] >= upper[1] {
		return r, geom.ErrConvergence
	}
	// The outward boundary of the roofs must have the positive projected
	// order relative to the physical upward normal, independently of X/Y.
	var area *big.Rat = new(big.Rat)
	for _, edge := range rim {
		x0, x1 := projectiveVectorRat(edge.ends[0], axes[0]), projectiveVectorRat(edge.ends[1], axes[0])
		y0, y1 := projectiveVectorRat(edge.ends[0], axes[1]), projectiveVectorRat(edge.ends[1], axes[1])
		area.Add(area, new(big.Rat).Sub(new(big.Rat).Mul(x0, y1), new(big.Rat).Mul(x1, y0)))
	}
	if area.Sign() != parity {
		return r, geom.ErrConvergence
	}

	parts := []SewFace{envelope.Faces[0], envelope.Faces[1]}
	var walls []ProjectiveRoofWall
	bottoms := make([]projectiveRoofBottom, 0, len(rim))
	for _, top := range rim {
		wall, bottom, err := makeProjectiveRoofWall(ctx, top, axes, height, baseHeight, lower, upper, o.Build)
		if err != nil {
			return r, err
		}
		bottoms = append(bottoms, bottom)
		walls = append(walls, ProjectiveRoofWall{Owner: top.owner, Face: len(parts), Source: top.use})
		parts = append(parts, wall)
	}
	base, err := makeProjectiveRoofBase(ctx, bottoms, axes, height, baseHeight, lower, upper, parity, o.Build)
	if err != nil {
		return r, err
	}
	baseFace := len(parts)
	parts = append(parts, base)

	solid, edges, err := sewProjectiveRoofSectionParts(ctx, parts, seams, o.Sew)
	if err != nil {
		return r, err
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.Solid, r.Roofs, r.Walls, r.BaseFace, r.Section, r.SectionEdges, r.Complete = solid, [2]int{0, 1}, walls, baseFace, edges[0], edges, true
	return r, nil
}

func projectiveRoofsAboveBase(a, b Face, height int, baseHeight float64, limits geom.SurfacePatchOptions) error {
	for _, owner := range [2]Face{a, b} {
		d, err := projectiveSplineFaceRectangle(owner.Outer, false)
		if err != nil {
			return err
		}
		raw, pose := parametric.SurfaceMapping(owner.Surface)
		patch, err := exactSplineSupportPatch(raw, limits)
		if err != nil {
			return err
		}
		if patch.Domain() != d {
			patch, err = patch.Trim(d)
			if err != nil {
				return err
			}
		}
		if pose != geom.Identity() {
			patch, err = patch.Transformed(pose)
			if err != nil {
				return err
			}
		}
		box, err := patch.Bounds()
		if err != nil {
			return err
		}
		if projectiveVectorAxis(box.Min, height) <= baseHeight {
			return geom.ErrConvergence
		}
	}
	return nil
}

func sewProjectiveRoofParts(ctx context.Context, parts []SewFace, seams [2]int, o ExactSewOptions) (Solid, int, error) {
	var section [2][]int
	if seams != [2]int{-1, -1} {
		section = [2][]int{{seams[0]}, {seams[1]}}
	}
	s, edges, err := sewProjectiveRoofSectionParts(ctx, parts, section, o)
	if err != nil || len(edges) == 0 {
		return s, -1, err
	}
	return s, edges[0], nil
}

func sewProjectiveRoofSectionParts(ctx context.Context, parts []SewFace, seams [2][]int, o ExactSewOptions) (Solid, []int, error) {
	var allEdges []parametric.Edge
	var allVertices []geom.Vec3
	var allFaces []Face
	var offsets [2]int
	if len(parts) < 2 || len(seams[0]) != len(seams[1]) {
		return Solid{}, nil, geom.ErrConvergence
	}
	for i, f := range parts {
		vo, eo := len(allVertices), len(allEdges)
		if i < 2 {
			offsets[i] = eo
			for _, id := range seams[i] {
				if id < 0 || id >= len(f.Edges) {
					return Solid{}, nil, geom.ErrConvergence
				}
			}
		}
		allVertices = append(allVertices, f.Vertices...)
		for _, e := range f.Edges {
			e.Start, e.End = e.Start+vo, e.End+vo
			allEdges = append(allEdges, e)
		}
		face := cloneFace(f.Face)
		for j := range face.Outer {
			face.Outer[j].Edge += eo
		}
		allFaces = append(allFaces, face)
	}
	solid, proof, err := SewExactTopology(ctx, allVertices, allEdges, allFaces, o)
	if err != nil {
		return Solid{}, nil, err
	}
	if len(proof.FlippedFaces) != 0 {
		return Solid{}, nil, geom.ErrConvergence
	}
	var edges []int
	seen := make(map[int]bool, len(seams[0]))
	for i, local := range seams[0] {
		edge := proof.Edges[offsets[0]+local].Edge
		if seen[edge] || edge != proof.Edges[offsets[1]+seams[1][i]].Edge {
			return Solid{}, nil, geom.ErrConvergence
		}
		seen[edge] = true
		edges = append(edges, edge)
	}
	return solid, edges, nil
}

func makeProjectiveRoofWall(ctx context.Context, top projectiveRoofRim, axes [2]int, height int, baseHeight float64,
	mins, maxs [2]float64, o geom.SurfacePatchOptions) (SewFace, projectiveRoofBottom, error) {
	var bottom projectiveRoofBottom
	q, _, err := top.model.BezierControlNet(ctx, o)
	if err != nil {
		return SewFace{}, bottom, err
	}
	fixed := 0
	if projectiveVectorAxis(top.ends[0], axes[0]) != projectiveVectorAxis(top.ends[1], axes[0]) {
		fixed = 1
	}
	start, end := projectiveVectorAxis(top.ends[0], axes[fixed]), projectiveVectorAxis(top.ends[1], axes[fixed])
	if start != end || start != mins[fixed] && start != maxs[fixed] ||
		projectiveVectorAxis(top.ends[0], axes[1-fixed]) == projectiveVectorAxis(top.ends[1], axes[1-fixed]) {
		return SewFace{}, bottom, geom.ErrConvergence
	}
	z := new(big.Rat).SetFloat64(baseHeight)
	x := new(big.Rat).SetFloat64(start)
	var wallNet, baseNet [][4]*big.Rat
	for _, h := range q.Control {
		if new(big.Rat).Mul(x, h[3]).Cmp(h[axes[fixed]]) != 0 {
			return SewFace{}, bottom, geom.ErrConvergence
		}
		p := h
		p[height] = new(big.Rat).Mul(z, h[3])
		wallNet = append(wallNet, p, h)
		baseNet = append(baseNet, p)
	}
	bottom.model, err = geom.NewExactHomogeneousBezier(ctx, baseNet, o)
	if err != nil {
		return SewFace{}, bottom, err
	}
	for i, v := range top.ends {
		p := [3]float64{v.X, v.Y, v.Z}
		p[height] = baseHeight
		bottom.ends[i] = geom.Vec3{X: p[0], Y: p[1], Z: p[2]}
	}
	patch, err := geom.NewExactHomogeneousBezierSurface(ctx, len(baseNet)-1, 1, wallNet, o)
	if err != nil {
		return SewFace{}, bottom, err
	}
	surface, err := geom.NewExactBezierSurface(patch)
	if err != nil {
		return SewFace{}, bottom, err
	}
	bottom.uv, err = projectiveRoofBaseUV(baseNet, axes, mins, maxs)
	if err != nil {
		return SewFace{}, bottom, err
	}
	ps := [4]geom.Vec3{bottom.ends[0], bottom.ends[1], top.ends[1], top.ends[0]}
	m1, err := projectiveRoofVertical(ctx, ps[1], ps[2], o)
	if err != nil {
		return SewFace{}, bottom, err
	}
	m0, err := projectiveRoofVertical(ctx, ps[0], ps[3], o)
	if err != nil {
		return SewFace{}, bottom, err
	}
	unit := parametric.Interval{Max: 1}
	edges := []parametric.Edge{{Start: 0, End: 1, Curve: bottom.model, Range: unit},
		{Start: 1, End: 2, Curve: m1, Range: unit}, {Start: 3, End: 2, Curve: top.model, Range: unit},
		{Start: 0, End: 3, Curve: m0, Range: unit}}
	corners := [4]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
	outer := make([]Use, 4)
	for i := range outer {
		outer[i] = Use{Edge: i, Reversed: i >= 2, UVStart: corners[i], UVEnd: corners[(i+1)%4]}
	}
	return SewFace{Vertices: ps[:], Edges: edges, Face: Face{Surface: surface, Outer: outer}}, bottom, nil
}

func projectiveRoofBaseUV(net [][4]*big.Rat, axes [2]int, mins, maxs [2]float64) (ExactQuadraticUV, error) {
	param := make([][3]*big.Rat, len(net))
	for i, p := range net {
		for j, a := range axes {
			base, limit := new(big.Rat).SetFloat64(mins[j]), new(big.Rat).SetFloat64(maxs[j])
			param[i][j] = new(big.Rat).Quo(new(big.Rat).Sub(p[a], new(big.Rat).Mul(base, p[3])), limit.Sub(limit, base))
		}
		param[i][2] = p[3]
	}
	h := [3][3]*big.Rat{param[0], param[1], param[len(param)-1]}
	if len(param) == 2 {
		for i := range 3 {
			h[1][i] = new(big.Rat).Quo(new(big.Rat).Add(h[0][i], h[2][i]), big.NewRat(2, 1))
		}
	}
	return NewExactQuadraticUV(h)
}

func makeProjectiveRoofBase(ctx context.Context, bottom []projectiveRoofBottom, axes [2]int, height int, z float64,
	mins, maxs [2]float64, parity int, o geom.SurfacePatchOptions) (SewFace, error) {
	corners := make([][4]*big.Rat, 0, 4)
	for i := range 2 {
		for j := range 2 {
			h := [4]*big.Rat{}
			h[axes[0]], h[axes[1]] = new(big.Rat).SetFloat64([2]float64{mins[0], maxs[0]}[i]), new(big.Rat).SetFloat64([2]float64{mins[1], maxs[1]}[j])
			h[height], h[3] = new(big.Rat).SetFloat64(z), big.NewRat(1, 1)
			corners = append(corners, h)
		}
	}
	p, err := geom.NewExactHomogeneousBezierSurface(ctx, 1, 1, corners, o)
	if err != nil {
		return SewFace{}, err
	}
	surface, err := geom.NewExactBezierSurface(p)
	if err != nil {
		return SewFace{}, err
	}
	out := SewFace{Face: Face{Surface: surface, Reversed: parity > 0}}
	n := len(bottom)
	for i := range n {
		j := i
		if parity < 0 {
			j = n - 1 - i
		}
		e := bottom[j]
		if parity < 0 {
			out.Vertices = append(out.Vertices, e.ends[1])
		} else {
			out.Vertices = append(out.Vertices, e.ends[0])
		}
		m := parametric.Edge{Start: i, End: (i + 1) % n, Curve: e.model, Range: parametric.Interval{Max: 1}}
		if parity < 0 {
			m.Start, m.End = m.End, m.Start
			e.uv = e.uv.reversed()
		}
		out.Edges = append(out.Edges, m)
		out.Face.Outer = append(out.Face.Outer, Use{Edge: i, Reversed: parity < 0, UVStart: e.uv.ends[0], UVEnd: e.uv.ends[1], Curve: e.uv})
	}
	return out, nil
}

func projectiveRoofVertical(ctx context.Context, a, b geom.Vec3, o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, error) {
	net := [][4]*big.Rat{
		{new(big.Rat).SetFloat64(a.X), new(big.Rat).SetFloat64(a.Y), new(big.Rat).SetFloat64(a.Z), big.NewRat(1, 1)},
		{new(big.Rat).SetFloat64(b.X), new(big.Rat).SetFloat64(b.Y), new(big.Rat).SetFloat64(b.Z), big.NewRat(1, 1)},
	}
	return geom.NewExactHomogeneousBezier(ctx, net, o)
}

func projectiveVectorAxis(p geom.Vec3, axis int) float64 {
	return [3]float64{p.X, p.Y, p.Z}[axis]
}

func projectiveVectorRat(p geom.Vec3, axis int) *big.Rat {
	return new(big.Rat).SetFloat64(projectiveVectorAxis(p, axis))
}

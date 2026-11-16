package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// BuildProjectiveSplineRoofDifference returns the regularized A-minus-B
// difference of the two subgraphs above the same base plane. It preserves
// the upper A region and the corresponding reversed original B region;
// a seam terminal shared by both roofs needs no zero-length wall edge.
// The result retains BaseFace=-1 because the common base cancels.
func BuildProjectiveSplineRoofDifference(ctx context.Context, a, b Face, axes [2]int, baseHeight float64,
	o ProjectiveRoofSolidOptions) (r ProjectiveRoofSolid, err error) {
	return buildProjectiveSplineRoofDifference(ctx, a, b, axes, baseHeight, o, true)
}

// BuildProjectiveSplineRoofContourDifference closes the A-minus-B material
// along every edge of one open analytic contour. Each retained exterior
// top and floor segment must have the same projected endpoints so the
// original exterior partitions mate exactly.
func BuildProjectiveSplineRoofContourDifference(ctx context.Context, a, b Face, axes [2]int, baseHeight float64,
	o ProjectiveRoofSolidOptions) (r ProjectiveRoofSolid, err error) {
	return buildProjectiveSplineRoofDifference(ctx, a, b, axes, baseHeight, o, false)
}

func buildProjectiveSplineRoofDifference(ctx context.Context, a, b Face, axes [2]int, baseHeight float64,
	o ProjectiveRoofSolidOptions, single bool) (r ProjectiveRoofSolid, err error) {
	if ctx == nil || !finite(baseHeight) || axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 ||
		axes[0] == axes[1] || o.Sew.RepairOrientation {
		return r, geom.ErrInvalid
	}
	height := 3 - axes[0] - axes[1]
	if err := projectiveRoofsAboveBase(a, b, height, baseHeight, o.Build); err != nil {
		return r, err
	}
	selected, err := selectProjectiveSplineFaceContourFragments(ctx, a, b, axes, [2]int{1, 1}, o.Envelope, single)
	if err != nil {
		return r, fmt.Errorf("projective Cut roof pair: %w", err)
	}
	parts := []SewFace{selected.Faces[0], selected.Faces[1]}
	top, topSeam, err := projectiveDifferenceRim(ctx, parts[0], selected.Sources[0], 0, o.Build)
	if err != nil {
		return r, err
	}
	bottom, bottomSeam, err := projectiveDifferenceRim(ctx, parts[1], selected.Sources[1], 1, o.Build)
	if err != nil {
		return r, err
	}
	if len(top) != len(bottom) || len(top) < 2 || single && len(top) > 4 {
		return r, fmt.Errorf("projective Cut border count: %w", geom.ErrConvergence)
	}
	parity := 1
	if (axes[0]+1)%3 != axes[1] {
		parity = -1
	}
	// The upper roof uses the projected outward orientation; the other
	// original roof becomes the material floor and reverses exactly once.
	area := new(big.Rat)
	for _, edge := range top {
		area.Add(area, new(big.Rat).Sub(
			new(big.Rat).Mul(projectiveVectorRat(edge.ends[0], axes[0]), projectiveVectorRat(edge.ends[1], axes[1])),
			new(big.Rat).Mul(projectiveVectorRat(edge.ends[1], axes[0]), projectiveVectorRat(edge.ends[0], axes[1]))))
	}
	var sections [2][]int
	for _, piece := range selected.Sections {
		up, upOK := topSeam[piece.Arc]
		lo, loOK := bottomSeam[piece.Arc]
		if !upOK || !loOK {
			return r, geom.ErrConvergence
		}
		sections[0], sections[1] = append(sections[0], up.Edge), append(sections[1], lo.Edge)
		seamEdge := parts[0].Edges[up.Edge]
		x, y := parts[0].Vertices[seamEdge.Start], parts[0].Vertices[seamEdge.End]
		if up.Reversed != parts[0].Face.Reversed {
			x, y = y, x
		}
		area.Add(area, new(big.Rat).Sub(new(big.Rat).Mul(projectiveVectorRat(x, axes[0]), projectiveVectorRat(y, axes[1])),
			new(big.Rat).Mul(projectiveVectorRat(y, axes[0]), projectiveVectorRat(x, axes[1]))))
	}
	if area.Sign() != parity {
		return r, fmt.Errorf("projective Cut upper roof orientation: %w", geom.ErrConvergence)
	}
	parts[1].Face.Reversed = !parts[1].Face.Reversed
	used := make([]bool, len(bottom))
	var walls []ProjectiveRoofWall
	var floorSources []ProjectiveSplineImprintUse
	for _, upperBorder := range top {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		id := -1
		for i, lowerBorder := range bottom {
			if !used[i] && projectiveSameBorderProjection(upperBorder, lowerBorder, axes) {
				if id >= 0 {
					return r, geom.ErrConvergence
				}
				id = i
			}
		}
		if id < 0 {
			return r, fmt.Errorf("projective Cut paired border: %w", geom.ErrConvergence)
		}
		used[id] = true
		wall, err := makeProjectiveDifferenceWall(ctx, upperBorder, bottom[id], axes, height, o.Build)
		if err != nil {
			return r, fmt.Errorf("projective Cut side %d: %w", len(walls), err)
		}
		walls = append(walls, ProjectiveRoofWall{Owner: 0, Face: len(parts), Source: upperBorder.use})
		floorSources = append(floorSources, bottom[id].use)
		parts = append(parts, wall)
	}

	solid, edges, err := sewProjectiveRoofSectionParts(ctx, parts, sections, o.Sew)
	if err != nil {
		return r, fmt.Errorf("projective Cut sewing: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.Solid, r.Roofs, r.Walls, r.FloorSources = solid, [2]int{0, 1}, walls, floorSources
	r.BaseFace, r.Section, r.SectionEdges, r.Complete = -1, edges[0], edges, true
	return r, nil
}

func projectiveDifferenceRim(ctx context.Context, f SewFace, sources []ProjectiveSplineImprintUse, owner int,
	o geom.SurfacePatchOptions) ([]projectiveRoofRim, map[int]Use, error) {
	var rim []projectiveRoofRim
	seams := make(map[int]Use)
	n := len(f.Face.Outer)
	for j := range n {
		i := j
		if f.Face.Reversed {
			i = n - j - 1
		}
		u, source := f.Face.Outer[i], sources[i]
		if source.Section {
			if _, repeated := seams[source.SectionArc]; repeated {
				return nil, nil, geom.ErrConvergence
			}
			seams[source.SectionArc] = u
			continue
		}
		m := f.Edges[u.Edge].Curve.(*geom.ExactSplineCurve)
		net, _, err := m.BezierControlNet(ctx, o)
		if err != nil {
			return nil, nil, err
		}
		if len(net.Control) < 2 || len(net.Control) > 3 || net.Start != 0 || net.End != 1 {
			return nil, nil, geom.ErrConvergence
		}
		if u.Reversed != f.Face.Reversed {
			for i, j := 0, len(net.Control)-1; i < j; i, j = i+1, j-1 {
				net.Control[i], net.Control[j] = net.Control[j], net.Control[i]
			}
			m, err = geom.NewExactHomogeneousBezier(ctx, net.Control, o)
			if err != nil {
				return nil, nil, err
			}
		}
		var ends [2]geom.Vec3
		for i := range ends {
			ends[i], err = m.ExactPoint(float64(i))
			if err != nil {
				return nil, nil, err
			}
		}
		rim = append(rim, projectiveRoofRim{model: m, ends: ends, owner: owner, use: source})
	}
	if len(seams) == 0 {
		return nil, nil, geom.ErrConvergence
	}
	return rim, seams, nil
}

func makeProjectiveDifferenceWall(ctx context.Context, top, bottom projectiveRoofRim, axes [2]int, height int,
	o geom.SurfacePatchOptions) (SewFace, error) {
	fixed := axes[0]
	free := axes[1]
	if projectiveVectorAxis(top.ends[0], fixed) != projectiveVectorAxis(top.ends[1], fixed) {
		fixed, free = free, fixed
	}
	value := projectiveVectorAxis(top.ends[0], fixed)
	a, b := projectiveVectorAxis(top.ends[0], free), projectiveVectorAxis(top.ends[1], free)
	if projectiveVectorAxis(top.ends[1], fixed) != value || a == b {
		return SewFace{}, geom.ErrConvergence
	}
	sign := 1.
	if a > b {
		sign = -1
	}
	a, b = sign*a, sign*b
	var curve [2]ExactQuadraticUV
	zlo, zhi := projectiveVectorAxis(bottom.ends[0], height), projectiveVectorAxis(top.ends[0], height)
	for owner, edge := range [2]projectiveRoofRim{bottom, top} {
		net, _, err := edge.model.BezierControlNet(ctx, o)
		if err != nil {
			return SewFace{}, err
		}
		plane := new(big.Rat).SetFloat64(value)
		var h [3][3]*big.Rat
		for i, p := range net.Control {
			if new(big.Rat).Mul(plane, p[3]).Cmp(p[fixed]) != 0 {
				return SewFace{}, geom.ErrConvergence
			}
			j := i
			if len(net.Control) == 2 && j == 1 {
				j = 2
			}
			h[j] = [3]*big.Rat{new(big.Rat).Mul(new(big.Rat).SetFloat64(sign), p[free]), p[height], p[3]}
		}
		if len(net.Control) == 2 {
			for k := range 3 {
				h[1][k] = new(big.Rat).Quo(new(big.Rat).Add(h[0][k], h[2][k]), big.NewRat(2, 1))
			}
		}
		curve[owner], err = NewExactQuadraticUV(h)
		if err != nil {
			return SewFace{}, err
		}
		box, err := edge.model.ProjectionBounds()
		if err != nil {
			return SewFace{}, err
		}
		zlo, zhi = min(zlo, projectiveVectorAxis(box.Min, height)), max(zhi, projectiveVectorAxis(box.Max, height))
	}
	if zhi <= zlo || !finite(zhi) || !finite(zlo) || top.ends[0] == bottom.ends[0] && top.ends[1] == bottom.ends[1] {
		return SewFace{}, geom.ErrConvergence
	}
	var net []geom.Vec3
	for _, u := range [2]float64{a, b} {
		for _, z := range [2]float64{zlo, zhi} {
			var v [3]float64
			v[fixed], v[free], v[height] = value, sign*u, z
			net = append(net, geom.Vec3{X: v[0], Y: v[1], Z: v[2]})
		}
	}
	s, err := geom.NewBSplineSurface(1, 1, 2, 2, net, []float64{a, a, b, b}, []float64{zlo, zlo, zhi, zhi}, nil)
	if err != nil {
		return SewFace{}, err
	}
	p := [4]geom.Vec3{bottom.ends[0], bottom.ends[1], top.ends[1], top.ends[0]}
	u := [4]geom.Vec2{curve[0].ends[0], curve[0].ends[1], curve[1].ends[1], curve[1].ends[0]}
	m := [4]*geom.ExactSplineCurve{bottom.model, nil, top.model, nil}
	if p[1] != p[2] {
		m[1], err = projectiveRoofVertical(ctx, p[1], p[2], o)
		if err != nil {
			return SewFace{}, err
		}
	}
	if p[3] != p[0] {
		m[3], err = projectiveRoofVertical(ctx, p[3], p[0], o)
		if err != nil {
			return SewFace{}, err
		}
	}
	q := SewFace{Face: Face{Surface: s}}
	for i, model := range m {
		if model == nil {
			continue
		}
		start := len(q.Vertices)
		q.Vertices = append(q.Vertices, p[i])
		e := parametric.Edge{Start: start, End: start + 1, Curve: model, Range: parametric.Interval{Max: 1}}
		coedge := Use{Edge: start, UVStart: u[i], UVEnd: u[(i+1)%4]}
		switch i {
		case 0:
			coedge.Curve = curve[0]
		case 2:
			coedge.Curve = curve[1].reversed()
			e.Start, e.End = e.End, e.Start
			coedge.Reversed = true
		}
		q.Edges = append(q.Edges, e)
		q.Face.Outer = append(q.Face.Outer, coedge)
	}
	n := len(q.Vertices)
	for i, e := range q.Edges {
		q.Edges[i].Start, q.Edges[i].End = e.Start%n, e.End%n
	}
	return q, nil
}

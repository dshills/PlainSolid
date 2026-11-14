package trimmed

import (
	"context"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// ThreadedRectangularPrism replaces the circular exterior of a qualified
// internal groove with a rectangular outer boundary. The exact threaded wall
// and its end radii remain shared with the two original cap planes.
func ThreadedRectangularPrism(ctx context.Context, r HelicalGrooveRecipe, rect ExtrusionRectangle,
	reliefRadius, depth, chartError float64) (HelicalOverlapRelief, mesh.Mesh, error) {
	fail := func(err error) (HelicalOverlapRelief, mesh.Mesh, error) {
		return HelicalOverlapRelief{}, mesh.Mesh{}, err
	}
	if r.BoreRadius <= 0 || !uvFinite(rect.Min) || !uvFinite(rect.Max) ||
		-r.OuterRadius <= rect.Min.X || -r.OuterRadius <= rect.Min.Y ||
		r.OuterRadius >= rect.Max.X || r.OuterRadius >= rect.Max.Y {
		return fail(geom.ErrDomain)
	}
	groove, err := grooveWithOverlappingEntryRelief(ctx, r, reliefRadius, depth, chartError, false, false)
	if err != nil {
		return fail(err)
	}
	s := groove.Solid
	var capFaces [2]int
	capFaces[0], capFaces[1] = -1, -1
	outerWall := -1
	for i, face := range s.faces {
		if wall, ok := face.Surface.(geom.CylinderSurface); ok {
			f, radius := wall.Parameters()
			if radius == r.OuterRadius && f == r.Frame {
				if outerWall >= 0 {
					return fail(geom.ErrDomain)
				}
				outerWall = i
			}
		}
		if cap, ok := face.Surface.(geom.TwistedAnnularSurface); ok {
			f, _, outer, height, _ := cap.Parameters()
			level := r.Frame.ToLocal(f.Origin()).Z + height
			if outer == r.OuterRadius && (level == 0 || level == r.Height) {
				k := 0
				if level == r.Height {
					k = 1
				}
				if capFaces[k] >= 0 {
					return fail(geom.ErrDomain)
				}
				capFaces[k] = i
			}
		}
	}
	if outerWall < 0 || capFaces[0] < 0 || capFaces[1] < 0 {
		return fail(geom.ErrDomain)
	}
	capIDs := [2][]int{}
	for k, fi := range capFaces {
		face := s.faces[fi]
		_, radius, _, _, _ := face.Surface.(geom.TwistedAnnularSurface).Parameters()
		inner := -1
		for _, use := range face.Outer {
			edge := s.edges[use.Edge]
			if edge.Start == edge.End && use.UVStart.Y == radius && use.UVEnd.Y == radius {
				if inner >= 0 {
					return fail(geom.ErrDomain)
				}
				inner = use.Edge
			}
		}
		if inner < 0 {
			return fail(geom.ErrDomain)
		}
		for i := 1; i < 4; i++ {
			var split EdgeSplit
			s, split, err = s.splitEdgeDraft(inner, float64(i)/4)
			if err != nil {
				return fail(err)
			}
			capIDs[k] = append(capIDs[k], split.First)
			inner = split.Second
		}
		capIDs[k] = append(capIDs[k], inner)
	}
	builder := cornerCylinderBuilder{ctx: ctx, frame: r.Frame, edgesByEnds: map[[2]int]int{}, verticesByPoint: map[geom.Vec3]int{}}
	outline := []geom.Vec2{rect.Min, {X: rect.Max.X, Y: rect.Min.Y}, rect.Max, {X: rect.Min.X, Y: rect.Max.Y}}
	for i, p := range outline {
		if err := builder.wall(p, outline[(i+1)%4], 0, r.Height); err != nil {
			return fail(err)
		}
	}
	for _, z := range []float64{0, r.Height} {
		if err := builder.horizontal(outline, nil, z, z == 0, 0); err != nil {
			return fail(err)
		}
	}
	vertices, edges := s.Vertices(), s.Edges()
	vo, eo := len(vertices), len(edges)
	vertices = append(vertices, builder.vertices...)
	for _, edge := range builder.edges {
		edge.Start += vo
		edge.End += vo
		edges = append(edges, edge)
	}
	var faces []Face
	for i, face := range s.Faces() {
		if i != outerWall && i != capFaces[0] && i != capFaces[1] {
			faces = append(faces, face)
		}
	}
	for i, face := range builder.faces {
		for j := range face.Outer {
			face.Outer[j].Edge += eo
		}
		if i >= 4 {
			k := i - 4
			plane := face.Surface.(geom.PlaneSurface).Frame()
			oldCap := s.faces[capFaces[k]].Surface.(geom.TwistedAnnularSurface)
			_, radius, _, _, _ := oldCap.Parameters()
			hole, err := helicalCapHole(s, capIDs[k], plane, radius)
			if err != nil {
				return fail(err)
			}
			face.Holes = append(face.Holes, hole)
		}
		faces = append(faces, face)
	}
	groove.Solid, err = compactSteppedTopologyWithValidation(ctx, vertices, edges, faces, s.tolerance, false)
	if err != nil {
		return fail(err)
	}
	display, err := groove.Solid.TessellateHelical(ctx, parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 256, MaxTriangles: 100000})
	if err != nil {
		return fail(err)
	}
	return groove, display, nil
}

func helicalCapHole(s Solid, ids []int, plane geom.Frame, radius float64) ([]Use, error) {
	first := s.edges[ids[0]]
	p, err := first.Curve.Eval((first.Range.Min + first.Range.Max) / 2)
	if err != nil {
		return nil, err
	}
	a := plane.ToLocal(s.vertices[first.Start])
	mid := plane.ToLocal(p)
	forward := a.X*mid.Y-a.Y*mid.X > 0
	var hole []Use
	for j := range ids {
		i := j
		if forward {
			i = len(ids) - 1 - j
		}
		edge := s.edges[ids[i]]
		start, end := edge.Start, edge.End
		if forward {
			start, end = end, start
		}
		x, y := plane.ToLocal(s.vertices[start]), plane.ToLocal(s.vertices[end])
		angle := math.Atan2(x.Y, x.X)
		// Start and end are authoritative points on the retained rim. The
		// phase is recovered from the first endpoint, then matched against
		// its shared model edge during final boundary validation.
		arc := EllipseUV{Cos: geom.Vec2{X: radius}, Sin: geom.Vec2{Y: radius}, From: angle, To: angle - math.Pi/2}
		uvStart, uvEnd := geom.Vec2{X: x.X, Y: x.Y}, geom.Vec2{X: y.X, Y: y.Y}
		hole = append(hole, Use{Edge: ids[i], Reversed: forward, UVStart: uvStart, UVEnd: uvEnd,
			Curve: anchoredCircularUV{EllipseUV: arc, Start: uvStart, End: uvEnd}})
	}
	return hole, nil
}

package trimmed

import (
	"context"
	"fmt"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

type openGraphRegion struct {
	loop  []openSectionPiece
	holes [][]openSectionPiece
	sign  int
}

// openGraphRegions builds a directed trim arrangement of regular section
// contours. Open contours have two boundary endpoints each. Pairwise
// disjoint outward boxes across different contours justify containment tests
// on the chord loops. Shared endpoints and ambiguous bounds stay unresolved.
func openGraphRegions(ctx context.Context, contours []intersect.CertifiedContour, boundarySign int) ([]openGraphRegion, error) {
	fail := func(reason string) ([]openGraphRegion, error) {
		return nil, fmt.Errorf("trimmed: open graph arrangement: %s", reason)
	}
	if len(contours) < 1 || len(contours) > 16 {
		return fail("unsupported contour count")
	}
	type directed struct {
		steps []openSectionPiece
		sign  int
	}
	var paths []directed
	var boxes []geom.UVBox
	segments := 0
	pointOnSide := func(p geom.Vec2) (int, float64) {
		switch {
		case p.Y == 0 && p.X > 0 && p.X < 1:
			return 0, p.X
		case p.X == 1 && p.Y > 0 && p.Y < 1:
			return 1, p.Y
		case p.Y == 1 && p.X > 0 && p.X < 1:
			return 2, -p.X
		case p.X == 0 && p.Y > 0 && p.Y < 1:
			return 3, -p.Y
		}
		return -1, 0
	}
	type boundaryPoint struct {
		uv       geom.Vec2
		side     int
		order    float64
		outgoing int // section path index; -1 at corners
	}
	boundary := make([]boundaryPoint, 0, 4+2*len(contours))
	for i, p := range unitSquare {
		order := 0.
		if i >= 2 {
			order = -1
		}
		boundary = append(boundary, boundaryPoint{p, i, order, -1})
	}
	seen := map[geom.Vec2]bool{}
	var closed []intersect.CertifiedContour
	for _, c := range contours {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if len(c.Segments) == 0 || len(c.Segments) > 128 {
			return fail("expected a regular open contour")
		}
		segments += len(c.Segments)
		if segments > 1024 {
			return nil, parametric.ErrBudget
		}
		right := c.Segments[0].PlaneSignOnRight()
		if right == 0 {
			return fail("unproved contour orientation")
		}
		old := len(boxes)
		path := make([]openSectionPiece, len(c.Segments))
		for i, seg := range c.Segments {
			if seg.PlaneSignOnRight() != right {
				return fail("inconsistent contour orientation")
			}
			p := seg.SplineTrim()
			b, e := p.BoundsUV()
			if e != nil {
				return nil, e
			}
			for _, x := range boxes[:old] {
				if x.Min.X <= b.Max.X && b.Min.X <= x.Max.X && x.Min.Y <= b.Max.Y && b.Min.Y <= x.Max.Y {
					return fail("overlapping contour enclosures")
				}
			}
			boxes = append(boxes, b)
			a, e := p.EvalUV(0)
			if e != nil {
				return nil, e
			}
			z, e := p.EvalUV(1)
			if e != nil {
				return nil, e
			}
			path[i] = openSectionPiece{a, z, p, -1}
		}
		if c.Closed {
			for _, p := range path {
				if p.a.X <= 0 || p.a.X >= 1 || p.a.Y <= 0 || p.a.Y >= 1 {
					return fail("closed contour meets chart boundary")
				}
			}
			closed = append(closed, c)
			continue
		}
		back := make([]openSectionPiece, len(path))
		for i, p := range path {
			back[len(path)-1-i] = openSectionPiece{p.b, p.a, reversePCurve(p.c), -1}
		}
		for _, q := range []directed{{path, -right}, {back, right}} {
			p := q.steps[0].a
			side, order := pointOnSide(p)
			if side < 0 || seen[p] {
				return fail("contours must meet open domain sides at distinct points")
			}
			seen[p] = true
			boundary = append(boundary, boundaryPoint{p, side, order, len(paths)})
			paths = append(paths, q)
		}
	}
	sort.Slice(boundary, func(i, j int) bool {
		a, b := boundary[i], boundary[j]
		if a.side != b.side {
			return a.side < b.side
		}
		return a.order < b.order
	})
	indices := make(map[geom.Vec2]int, len(boundary))
	for i, p := range boundary {
		indices[p.uv] = i
	}
	var out []openGraphRegion
	used := make([]bool, len(boundary))
	usedPaths := make([]bool, len(paths))
	for start := range boundary {
		if used[start] {
			continue
		}
		var r openGraphRegion
		if len(paths) == 0 {
			r.sign = boundarySign
		}
		for i := start; !used[i]; {
			used[i] = true
			j := (i + 1) % len(boundary)
			r.loop = append(r.loop, openSectionPiece{a: boundary[i].uv, b: boundary[j].uv, side: boundary[i].side})
			path := boundary[j].outgoing
			i = j
			if path >= 0 {
				if usedPaths[path] {
					return fail("repeated section use")
				}
				usedPaths[path] = true
				d := paths[path]
				if r.sign != 0 && r.sign != d.sign {
					return fail("inconsistent material sign around region")
				}
				r.sign = d.sign
				r.loop = append(r.loop, d.steps...)
				i = indices[d.steps[len(d.steps)-1].b]
			}
			if used[i] && i != start {
				return fail("unclosed region")
			}
		}
		if r.sign == 0 || len(r.loop) < 3 {
			return fail("degenerate or unsigned region")
		}
		out = append(out, r)
	}
	if len(out) != len(contours)-len(closed)+1 {
		return fail("unexpected arrangement region count")
	}
	for _, used := range usedPaths {
		if !used {
			return fail("missing section use")
		}
	}
	if len(closed) != 0 {
		return insertInteriorGraphLoops(ctx, out, closed)
	}
	return out, nil
}

func insertInteriorGraphLoops(ctx context.Context, regions []openGraphRegion, contours []intersect.CertifiedContour) ([]openGraphRegion, error) {
	type ring struct {
		path   []openSectionPiece
		points []geom.Vec2
		sign   int
		depth  int
	}
	rings := make([]ring, len(contours))
	reverse := func(path []openSectionPiece) []openSectionPiece {
		out := make([]openSectionPiece, len(path))
		for i, p := range path {
			out[len(path)-1-i] = openSectionPiece{p.b, p.a, reversePCurve(p.c), p.side}
		}
		return out
	}
	points := func(path []openSectionPiece) []geom.Vec2 {
		out := make([]geom.Vec2, len(path))
		for i, p := range path {
			out[i] = p.a
		}
		return out
	}
	for i, c := range contours {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		r := &rings[i]
		for _, s := range c.Segments {
			p := s.SplineTrim()
			a, e := p.EvalUV(0)
			if e != nil {
				return nil, e
			}
			b, e := p.EvalUV(1)
			if e != nil {
				return nil, e
			}
			r.path = append(r.path, openSectionPiece{a, b, p, -1})
		}
		r.points = points(r.path)
		if len(r.points) < 3 {
			return nil, fmt.Errorf("trimmed: insufficient closed contour segments")
		}
		k := 0
		for j, p := range r.points {
			if q := r.points[k]; p.X < q.X || p.X == q.X && p.Y < q.Y {
				k = j
			}
		}
		a, b, z := r.points[(k+len(r.points)-1)%len(r.points)], r.points[k], r.points[(k+1)%len(r.points)]
		sign, e := geom.Orient2D(a, b, z)
		if e != nil || sign == 0 {
			return nil, fmt.Errorf("trimmed: unresolved closed contour winding")
		}
		if sign < 0 {
			r.path = reverse(r.path)
			r.points = points(r.path)
		}
		r.sign = -sign * c.Segments[0].PlaneSignOnRight()
	}
	for i, r := range rings {
		for j, q := range rings {
			if i != j && winding(r.points[0], q.points) != 0 {
				rings[i].depth++
			}
		}
	}
	sort.SliceStable(rings, func(i, j int) bool { return rings[i].depth < rings[j].depth })
	for _, r := range rings {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		parent := -1
		for i := len(regions) - 1; i >= 0; i-- {
			x := regions[i]
			if winding(r.points[0], points(x.loop)) == 0 {
				continue
			}
			insideHole := false
			for _, h := range x.holes {
				insideHole = insideHole || winding(r.points[0], points(h)) != 0
			}
			if !insideHole {
				parent = i
				break
			}
		}
		if parent < 0 || regions[parent].sign != -r.sign {
			return nil, fmt.Errorf("trimmed: unresolved mixed contour nesting")
		}
		regions[parent].holes = append(regions[parent].holes, reverse(r.path))
		regions = append(regions, openGraphRegion{loop: r.path, sign: r.sign})
	}
	return regions, nil
}

func assembleGraphRegionDifference(a, b *geom.BSplineSurface, r openGraphRegion, tolerance float64) (Solid, OpenSectionHistory, error) {
	s, h, e := assembleOpenSplineGraphPair(a, b, 0, true, tolerance, r.loop)
	if e != nil || len(r.holes) == 0 {
		return s, h, e
	}
	v, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	for _, hole := range r.holes {
		offset := len(v)
		var uses []Use
		for i, p := range hole {
			point, e := graphTrimStartDisplay(context.Background(), a, p)
			if e != nil {
				return Solid{}, OpenSectionHistory{}, e
			}
			v = append(v, point)
			id := len(edges)
			edges = append(edges, parametric.Edge{Start: offset + i, End: offset + (i+1)%len(hole),
				Curve: surfaceTrimCurve{a, p.a, p.b, p.c}, Range: parametric.Interval{Max: 1}})
			h.ContourEdges = append(h.ContourEdges, id)
			uses = append(uses, Use{Edge: id, UVStart: p.a, UVEnd: p.b, Curve: p.c})
		}
		faces[h.SplineFace].Holes = append(faces[h.SplineFace].Holes, uses)
		faces[h.PlaneFace].Holes = append(faces[h.PlaneFace].Holes, uses)
	}
	s, e = NewSolid(v, edges, faces, tolerance)
	return s, h, e
}

package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// OpenSectionHistory records a graph face, its section-plane face, shared
// intersection edges, and the original domain sides retained as planar walls.
// WallFaces are ordered along the retained boundary; DomainSides use CCW
// unit-square indexing starting from the bottom side.
type OpenSectionHistory struct {
	SplineFace, PlaneFace                int
	ContourEdges, WallFaces, DomainSides []int
}

type openSectionPiece struct {
	a, b geom.Vec2
	c    PCurve
	side int
}

var unitSquare = [4]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}

// SealOpenSplinePlaneSection forms the part between an identity unit-square
// Bézier graph and a horizontal plane on one side of a single regular open
// contour. above selects graph heights greater than height; false selects
// lesser heights. The contour endpoints must lie in distinct domain sides.
// The boundary includes exact graph/plane section curves, original graph
// boundary segments, and vertical planar walls along retained domain sides.
func SealOpenSplinePlaneSection(surface *geom.BSplineSurface, height float64, above bool, tolerance float64, o intersect.SplinePlaneOptions) (Solid, OpenSectionHistory, error) {
	fail := func(reason string) (Solid, OpenSectionHistory, error) {
		return Solid{}, OpenSectionHistory{}, fmt.Errorf("trimmed: open spline plane seal: %s", reason)
	}
	e := requireIdentitySplineChart(context.Background(), surface, o.Patch)
	if e != nil {
		return Solid{}, OpenSectionHistory{}, e
	}
	if !finite(height) || !finite(tolerance) || tolerance <= 0 || o.Domain != (geom.ParameterRectangle{}) {
		return fail("invalid plane, tolerance or domain override")
	}
	p, e := geom.NewSurfacePatch(surface, o.Patch)
	if e != nil {
		return Solid{}, OpenSectionHistory{}, e
	}
	return sealOpenSplineGraphPair(surface, nil, p, height, above, tolerance, o)
}

// OpenSplineGraphHistory identifies the original graph faces, their section
// edges, and vertical planar walls along retained square domain sides.
type OpenSplineGraphHistory struct {
	FaceA, FaceB                         int
	ContourEdges, WallFaces, DomainSides []int
}

// FillSplineGraphPair forms the entire solid between two strictly ordered
// unit-square spline graphs with the same identity XY chart.
// It certifies that their difference has no zero on the chart before using
// the two graphs as caps and constructing four vertical planar walls.
// The returned history has no ContourEdges.
func FillSplineGraphPair(a, b *geom.BSplineSurface, tolerance float64, o intersect.SplinePlaneOptions) (Solid, OpenSplineGraphHistory, error) {
	fail := func(e error) (Solid, OpenSplineGraphHistory, error) { return Solid{}, OpenSplineGraphHistory{}, e }
	if a == nil || b == nil || !finite(tolerance) || tolerance <= 0 || o.Domain != (geom.ParameterRectangle{}) {
		return fail(geom.ErrInvalid)
	}
	if e := requireIdentitySplineChart(context.Background(), a, o.Patch); e != nil {
		return fail(e)
	}
	p, _, _, e := exactGraphPatches(a, b, o.Patch, o.Domain)
	if e != nil {
		return fail(e)
	}
	pl, _ := geom.NewPlane(geom.Vec3{}, geom.Vec3{Z: 1})
	trace, e := geom.TraceSurfacePlane(p, pl, o.Trace)
	if e != nil {
		return fail(e)
	}
	if !trace.Complete || len(trace.Contours) > 0 || len(trace.Isolation.Coplanar) > 0 || len(trace.Isolation.Tangencies) > 0 {
		return fail(fmt.Errorf("trimmed: graph pair does not have a proved strict height order"))
	}
	sign, e := geom.ClassifySurfacePointPlane(p, pl, .5, .5, geom.SurfacePointSignOptions{})
	if e != nil {
		return fail(e)
	}
	if sign.Sign == 0 {
		return fail(geom.ErrConvergence)
	}
	pieces := make([]openSectionPiece, 4)
	for i := range pieces {
		pieces[i] = openSectionPiece{a: unitSquare[i], b: unitSquare[(i+1)%4], side: i}
	}
	out, h, e := assembleOpenSplineGraphPair(a, b, 0, sign.Sign > 0, tolerance, pieces)
	return out, OpenSplineGraphHistory{h.SplineFace, h.PlaneFace, h.ContourEdges, h.WallFaces, h.DomainSides}, e
}

// SealOpenSplineGraphPair forms the solid between two unit-square
// spline graphs on one side of a regular open contour. The original graphs
// must share their XY chart (x=u,y=v). Their height differences are exact.
// above selects where A is above B.
// The contour must meet two distinct domain sides away from the corners.
func SealOpenSplineGraphPair(a, b *geom.BSplineSurface, above bool, tolerance float64, o intersect.SplinePlaneOptions) (Solid, OpenSplineGraphHistory, error) {
	if a == nil || b == nil || !finite(tolerance) || tolerance <= 0 || o.Domain != (geom.ParameterRectangle{}) {
		return Solid{}, OpenSplineGraphHistory{}, geom.ErrInvalid
	}
	e := requireIdentitySplineChart(context.Background(), a, o.Patch)
	if e != nil {
		return Solid{}, OpenSplineGraphHistory{}, e
	}
	p, _, _, e := exactGraphPatches(a, b, o.Patch, o.Domain)
	if e != nil {
		return Solid{}, OpenSplineGraphHistory{}, e
	}
	out, h, e := sealOpenSplineGraphPair(a, b, p, 0, above, tolerance, o)
	return out, OpenSplineGraphHistory{h.SplineFace, h.PlaneFace, h.ContourEdges, h.WallFaces, h.DomainSides}, e
}

func sealOpenSplineGraphPair(surface, bottom *geom.BSplineSurface, difference *geom.SurfacePatch, height float64, above bool, tolerance float64, o intersect.SplinePlaneOptions) (Solid, OpenSectionHistory, error) {
	fail := func(reason string) (Solid, OpenSectionHistory, error) {
		return Solid{}, OpenSectionHistory{}, fmt.Errorf("trimmed: open spline graph seal: %s", reason)
	}
	fr, _ := geom.FrameFromNormal(geom.Vec3{Z: height}, geom.Vec3{Z: 1})
	if o.ResidualTolerance == 0 {
		o.ResidualTolerance = tolerance
	}
	var cs []intersect.CertifiedContour
	var trace geom.SurfacePlaneTrace
	var e error
	if bottom == nil {
		cs, trace, e = intersect.TraceSplinePlane(surface, fr, o)
	} else {
		cs, trace, e = intersect.TraceSplineHeightDifference(surface, bottom, o)
	}
	if e != nil {
		return Solid{}, OpenSectionHistory{}, e
	}
	if len(cs) != 1 || cs[0].Closed || len(cs[0].Segments) == 0 || len(cs[0].Segments) > 128 || len(trace.Isolation.Tangencies) > 0 {
		return fail("expected one open transverse contour")
	}
	pl, _ := geom.NewPlane(fr.Origin(), fr.ZAxis())
	pieces, _, e := openGraphLoop(cs[0], difference, pl, above)
	if e != nil {
		return Solid{}, OpenSectionHistory{}, e
	}
	return assembleOpenSplineGraphPair(surface, bottom, height, above, tolerance, pieces)
}

func openGraphLoop(c intersect.CertifiedContour, difference *geom.SurfacePatch, pl geom.Plane, above bool) ([]openSectionPiece, int, error) {
	fail := func(reason string) ([]openSectionPiece, int, error) {
		return nil, 0, fmt.Errorf("trimmed: open spline graph loop: %s", reason)
	}
	var contour []openSectionPiece
	for _, x := range c.Segments {
		c := x.SplineTrim()
		a, e := c.EvalUV(0)
		if e != nil {
			return nil, 0, e
		}
		b, e := c.EvalUV(1)
		if e != nil {
			return nil, 0, e
		}
		contour = append(contour, openSectionPiece{a, b, c, -1})
	}
	a, b := contour[0].a, contour[len(contour)-1].b
	which := func(p geom.Vec2) int {
		switch {
		case p.Y == 0 && p.X > 0 && p.X < 1:
			return 0
		case p.X == 1 && p.Y > 0 && p.Y < 1:
			return 1
		case p.Y == 1 && p.X > 0 && p.X < 1:
			return 2
		case p.X == 0 && p.Y > 0 && p.Y < 1:
			return 3
		}
		return -1
	}
	ia, ib := which(a), which(b)
	if ia < 0 || ib < 0 || ia == ib {
		return fail("endpoints must be in distinct open domain sides")
	}
	walk := func(from, to geom.Vec2, i, j int) []openSectionPiece {
		var r []openSectionPiece
		p := from
		for i != j {
			q := unitSquare[(i+1)%4]
			r = append(r, openSectionPiece{a: p, b: q, side: i})
			p = q
			i = (i + 1) % 4
		}
		return append(r, openSectionPiece{a: p, b: to, side: j})
	}
	at := func(p geom.Vec2) (int, error) {
		h, e := geom.ClassifySurfacePointPlane(difference, pl, p.X, p.Y, geom.SurfacePointSignOptions{})
		return h.Sign, e
	}
	path := walk(b, a, ib, ia)
	h, e := at(unitSquare[(ib+1)%4])
	if e != nil {
		return nil, 0, e
	}
	if h == 0 {
		return fail("unresolved domain-corner contact")
	}
	right := c.Segments[0].PlaneSignOnRight()
	if right == 0 {
		return fail("unresolved contour side")
	}
	// The square walk is counterclockwise, so the enclosed graph
	// region lies left of the directed cut. Check the corner sign
	// against the independently certified implicit derivative.
	if (h > 0) != (right < 0) {
		return fail("domain winding disagrees with certified contour orientation")
	}
	if (right < 0) != above {
		reversed := make([]openSectionPiece, len(contour))
		for i, x := range contour {
			reversed[len(contour)-1-i] = openSectionPiece{x.b, x.a, reverseTrim{x.c.(differentiableTrim)}, -1}
		}
		contour = reversed
		path = walk(a, b, ia, ib)
	}
	for _, p := range path {
		if p.a != a && p.a != b {
			h, e := at(p.a)
			if e != nil {
				return nil, 0, e
			}
			if h == 0 || (h > 0) != above {
				return fail("domain path does not have consistent sign")
			}
		}
	}
	return append(contour, path...), len(contour), nil
}

func assembleOpenSplineGraphPair(surface, bottom *geom.BSplineSurface, height float64, above bool, tolerance float64, pieces []openSectionPiece, evaluate ...graphVertexEvaluator) (Solid, OpenSectionHistory, error) {
	return assembleOpenSplineGraphPairOnCharts(context.Background(), surface, bottom, height, above, tolerance, pieces, [2]graphEnvelopeChart{}, evaluate...)
}

func assembleOpenSplineGraphPairOnCharts(ctx context.Context, surface, bottom *geom.BSplineSurface, height float64, above bool,
	tolerance float64, pieces []openSectionPiece, charts [2]graphEnvelopeChart, evaluate ...graphVertexEvaluator) (Solid, OpenSectionHistory, error) {
	fail := func(reason string) (Solid, OpenSectionHistory, error) {
		return Solid{}, OpenSectionHistory{}, fmt.Errorf("trimmed: spline wall assembly: %s", reason)
	}
	fr, _ := geom.FrameFromNormal(geom.Vec3{Z: height}, geom.Vec3{Z: 1})
	n := len(pieces)
	ownerPieces := [2][]openSectionPiece{pieces, pieces}
	exactVertices := charts[0].transfer != nil || charts[1].transfer != nil
	if exactVertices && (len(evaluate) == 0 || bottom == nil) {
		return fail("independent charts require two roofs and an exact vertex evaluator")
	}
	for i, chart := range charts {
		if chart.transfer == nil {
			continue
		}
		ownerPieces[i] = make([]openSectionPiece, n)
		for j, p := range pieces {
			owned, e := chart.piece(p)
			if e != nil {
				return Solid{}, OpenSectionHistory{}, e
			}
			ownerPieces[i][j] = owned
		}
	}
	vertices := make([]geom.Vec3, n)
	topIDs, baseIDs := make([]int, n), make([]int, n)
	basePoint := func(p geom.Vec2) (geom.Vec3, error) {
		if bottom == nil {
			return geom.Vec3{X: p.X, Y: p.Y, Z: height}, nil
		}
		if len(evaluate) > 0 {
			return evaluate[0](bottom, openSectionPiece{a: p})
		}
		return bottom.Eval(p.X, p.Y)
	}
	for i, x := range ownerPieces[0] {
		if e := ctx.Err(); e != nil {
			return Solid{}, OpenSectionHistory{}, e
		}
		var p geom.Vec3
		var e error
		if len(evaluate) > 0 {
			p, e = evaluate[0](surface, x)
		} else {
			p, e = surface.Eval(x.a.X, x.a.Y)
		}
		if e != nil {
			return Solid{}, OpenSectionHistory{}, e
		}
		topIDs[i] = i
		vertices[i] = p
		q, e := basePoint(ownerPieces[1][i].a)
		if e != nil {
			return Solid{}, OpenSectionHistory{}, e
		}
		// A vertex incident to any section edge belongs to both original charts.
		if x.side < 0 || pieces[(i+n-1)%n].side < 0 {
			baseIDs[i] = i
			if exactVertices && p != q {
				return Solid{}, OpenSectionHistory{}, fmt.Errorf("trimmed: original layer contour vertices differ exactly: %w", geom.ErrConvergence)
			}
			if !closePoint(p, q, tolerance) {
				return fail("contour vertex exceeds tolerance")
			}
			continue
		}
		if (p.Z > q.Z) != above || p.Z == q.Z {
			return fail("domain path crosses the section")
		}
		baseIDs[i] = len(vertices)
		vertices = append(vertices, q)
	}
	plane, _ := geom.NewPlaneSurface(fr)
	var baseSurface geom.Surface = plane
	if bottom != nil {
		baseSurface = bottom
	}
	faces := []Face{{Surface: surface, Reversed: !above != charts[0].reversed}, {Surface: baseSurface, Reversed: above != charts[1].reversed}}
	var edges []parametric.Edge
	add := func(a, b int, c geom.Curve, span float64) int {
		id := len(edges)
		edges = append(edges, parametric.Edge{Start: a, End: b, Curve: c, Range: parametric.Interval{Max: span}})
		return id
	}
	line := func(a, b int) (int, error) {
		p := vertices[a]
		d := vertices[b].Sub(p)
		c, e := geom.NewLine(p, d)
		if e != nil {
			return 0, e
		}
		return add(a, b, c, d.Length()), nil
	}
	makeUse := func(id int, rev bool, a, b geom.Vec2, c PCurve) Use {
		return Use{Edge: id, Reversed: rev, UVStart: a, UVEnd: b, Curve: c}
	}
	topEdges, baseEdges, verticals := make([]int, n), make([]int, n), make([]int, n)
	report := OpenSectionHistory{SplineFace: 0, PlaneFace: 1}
	for i, x := range ownerPieces[0] {
		j := (i + 1) % n
		low := ownerPieces[1][i]
		c := surfaceTrimCurve{surface: surface, a: x.a, b: x.b, curve: x.c}
		id := add(topIDs[i], topIDs[j], c, 1)
		topEdges[i] = id
		faces[0].Outer = append(faces[0].Outer, makeUse(id, false, x.a, x.b, x.c))
		if x.side < 0 {
			baseEdges[i] = id
			report.ContourEdges = append(report.ContourEdges, id)
			faces[1].Outer = append(faces[1].Outer, makeUse(id, false, low.a, low.b, low.c))
			continue
		}
		if bottom == nil {
			newID, e := line(baseIDs[i], baseIDs[j])
			if e != nil {
				return Solid{}, OpenSectionHistory{}, e
			}
			id = newID
		} else {
			id = add(baseIDs[i], baseIDs[j], surfaceTrimCurve{bottom, low.a, low.b, nil}, 1)
		}
		baseEdges[i] = id
		faces[1].Outer = append(faces[1].Outer, makeUse(id, false, low.a, low.b, nil))
	}
	for i, chart := range charts {
		if chart.reversed {
			reverseGraphUses(faces[i].Outer)
		}
	}
	for i := range pieces {
		verticals[i] = -1
		if topIDs[i] != baseIDs[i] {
			id, e := line(baseIDs[i], topIDs[i])
			if e != nil {
				return Solid{}, OpenSectionHistory{}, e
			}
			verticals[i] = id
		}
	}
	for i, x := range pieces {
		if x.side < 0 {
			continue
		}
		j := (i + 1) % n
		// The wall chart runs from a to b along its horizontal axis and
		// measures distance from the section plane along +Z.
		dir := geom.Vec3{X: x.b.X - x.a.X, Y: x.b.Y - x.a.Y}
		norm := dir.Cross(geom.Vec3{Z: 1})
		wf, e := geom.NewFrame(geom.Vec3{X: x.a.X, Y: x.a.Y, Z: height}, dir, norm)
		if e != nil {
			return Solid{}, OpenSectionHistory{}, e
		}
		ws, _ := geom.NewPlaneSurface(wf)
		length := dir.Length()
		aa, bb := geom.Vec2{Y: vertices[baseIDs[i]].Z - height}, geom.Vec2{X: length, Y: vertices[baseIDs[j]].Z - height}
		c, d := geom.Vec2{X: length, Y: vertices[j].Z - height}, geom.Vec2{Y: vertices[i].Z - height}
		if verticals[j] < 0 {
			c = bb
		}
		if verticals[i] < 0 {
			d = aa
		}
		// Evaluators preserve the same graph-edge normalized parameter.
		top, low := ownerPieces[0][i], ownerPieces[1][i]
		arc := charts[0].wall(wallTrim{surface: surface, a: top.a, b: top.b, length: length, height: height, start: d.Y, end: c.Y})
		var baseArc PCurve
		if bottom != nil {
			baseArc = charts[1].wall(wallTrim{surface: bottom, a: low.a, b: low.b, length: length, height: height, start: aa.Y, end: bb.Y})
		}
		l := []Use{makeUse(baseEdges[i], false, aa, bb, baseArc)}
		if verticals[j] >= 0 {
			l = append(l, makeUse(verticals[j], false, bb, c, nil))
		}
		l = append(l, makeUse(topEdges[i], true, c, d, reverseTrim{arc}))
		if verticals[i] >= 0 {
			l = append(l, makeUse(verticals[i], true, d, aa, nil))
		}
		if !above {
			reverseGraphUses(l)
		}
		report.WallFaces = append(report.WallFaces, len(faces))
		report.DomainSides = append(report.DomainSides, x.side)
		faces = append(faces, Face{Surface: ws, Outer: l})
	}
	out, e := NewSolidContext(ctx, vertices, edges, faces, tolerance)
	if e != nil {
		return Solid{}, OpenSectionHistory{}, e
	}
	return out, report, nil
}

type wallTrim struct {
	surface                    *geom.BSplineSurface
	a, b                       geom.Vec2
	length, height, start, end float64
}

// affineGraphWallTrim evaluates height on the original roof isocurve.
// Its horizontal station is in model wall coordinates, with a separately
// certified affine projection from the roof's own UV.
type affineGraphWallTrim struct{ wallTrim }

func (w wallTrim) EvalUV(t float64) (geom.Vec2, error) {
	if !finite(t) || t < 0 || t > 1 {
		return geom.Vec2{}, geom.ErrDomain
	}
	if t == 0 {
		return geom.Vec2{Y: w.start}, nil
	}
	if t == 1 {
		return geom.Vec2{X: w.length, Y: w.end}, nil
	}
	p := mix(w.a, w.b, t)
	v, e := w.surface.Eval(p.X, p.Y)
	return geom.Vec2{X: w.length * t, Y: v.Z - w.height}, e
}
func (w wallTrim) DerivativeUV(t float64) (geom.Vec2, error) {
	if !finite(t) || t < 0 || t > 1 {
		return geom.Vec2{}, geom.ErrDomain
	}
	p := mix(w.a, w.b, t)
	a, b, e := w.surface.Partials(p.X, p.Y)
	d := sub2(w.b, w.a)
	return geom.Vec2{X: w.length, Y: a.Z*d.X + b.Z*d.Y}, e
}
func (w wallTrim) BoundsUV() (geom.UVBox, error) {
	d := w.surface.Data()
	lo, hi := 0., 0.
	for _, p := range d.Poles {
		z := p.Z - w.height
		lo, hi = min(lo, z), max(hi, z)
	}
	return geom.UVBox{Min: geom.Vec2{Y: lo}, Max: geom.Vec2{X: w.length, Y: hi}}, nil
}

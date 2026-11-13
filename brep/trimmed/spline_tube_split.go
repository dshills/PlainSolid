package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// SplitSplineTubeAcrossEdges uses one certified spline-pair branch to split
// a hole-free spline face. The branch's chosen chart must be the face
// support. The caller supplies two original edge parameters, which must
// reconcile exactly with the branch's UV endpoints. The operation proves
// local curve identity but uses the bounded path validator for its embedding
// in this particular face.
func (s Solid) SplitSplineTubeAcrossEdges(ctx context.Context, face int, start, end EdgeAnchor,
	segments []intersect.TubeSegment, secondChart bool) (Solid, AnchoredFaceSplit, error) {
	return s.splitSplineTubeAcrossEdges(ctx, face, start, end, segments, secondChart, Solid.SplitEdge)
}

func (s Solid) splitSplineTubeAcrossEdges(ctx context.Context, face int, start, end EdgeAnchor,
	segments []intersect.TubeSegment, secondChart bool, split func(Solid, int, float64) (Solid, EdgeSplit, error)) (Solid, AnchoredFaceSplit, error) {
	bad := func(reason string) (Solid, AnchoredFaceSplit, error) {
		return Solid{}, AnchoredFaceSplit{}, fmt.Errorf("trimmed: spline tube split: %s", reason)
	}
	if ctx == nil || face < 0 || face >= len(s.faces) || len(segments) == 0 || len(segments) > 64 {
		return bad("invalid input")
	}
	base, placement := parametric.SurfaceMapping(s.faces[face].Surface)
	surface, ok := base.(*geom.BSplineSurface)
	if !ok || placement != geom.Identity() {
		return bad("requires an unmapped spline face")
	}
	p := make([]CutSegment, len(segments))
	var recipe intersect.TubeRecipe
	previous := -1
	for i, segment := range segments {
		if e := ctx.Err(); e != nil {
			return Solid{}, AnchoredFaceSplit{}, e
		}
		u := segment.TrimA()
		if secondChart {
			u = segment.TrimB()
		}
		key, j, _ := u.Recipe()
		if !geom.SameSplineSurface(surface, u.Support()) || i > 0 && (recipe != key || j != previous+1) {
			return bad("segments must follow one certified recipe on the selected chart")
		}
		recipe, previous = key, j
		a, e := u.EvalUV(0)
		if e != nil {
			return Solid{}, AnchoredFaceSplit{}, e
		}
		b, e := u.EvalUV(1)
		if e != nil {
			return Solid{}, AnchoredFaceSplit{}, e
		}
		p[i] = CutSegment{Start: a, End: b, Curve: u}
	}
	return s.splitFaceAcrossEdges(ctx, face, start, end, func(a, b geom.Vec2) ([]CutSegment, error) {
		if p[0].Start == a && p[len(p)-1].End == b {
			return p, nil
		}
		if p[0].Start != b || p[len(p)-1].End != a {
			return nil, fmt.Errorf("trimmed: tube endpoints do not match shared edges")
		}
		out := make([]CutSegment, len(p))
		for i, x := range p {
			out[len(p)-1-i] = CutSegment{x.End, x.Start, reversePCurve(x.Curve)}
		}
		return out, nil
	}, split)
}

// SplitSplineTubeLoop splits a hole-free spline face by a closed certified
// spline-pair branch. Both sides share every original tube segment. The
// selected chart must match the face support. Embedding in the original
// face uses SplitFaceCurvedLoop's bounded validation.
func (s Solid) SplitSplineTubeLoop(ctx context.Context, face int, segments []intersect.TubeSegment, secondChart bool) (Solid, FaceLoopSplit, error) {
	bad := func(reason string) (Solid, FaceLoopSplit, error) {
		return Solid{}, FaceLoopSplit{}, fmt.Errorf("trimmed: spline tube loop: %s", reason)
	}
	if ctx == nil || face < 0 || face >= len(s.faces) || len(segments) < 3 || len(segments) > 64 {
		return bad("invalid input")
	}
	base, placement := parametric.SurfaceMapping(s.faces[face].Surface)
	surface, ok := base.(*geom.BSplineSurface)
	if !ok || placement != geom.Identity() {
		return bad("requires an unmapped spline face")
	}
	curves := make([]PCurve, len(segments))
	points := make([]geom.Vec2, len(segments))
	var recipe intersect.TubeRecipe
	for i, segment := range segments {
		if e := ctx.Err(); e != nil {
			return Solid{}, FaceLoopSplit{}, e
		}
		u := segment.TrimA()
		if secondChart {
			u = segment.TrimB()
		}
		key, j, _ := u.Recipe()
		if i == 0 {
			recipe = key
			d := recipe.Data()
			if !d.Closed || len(d.Cells) != len(segments) {
				return bad("requires the complete closed tube recipe")
			}
		}
		if recipe != key || j != i || !geom.SameSplineSurface(surface, u.Support()) {
			return bad("segments do not follow one closed recipe on the selected chart")
		}
		p, e := u.EvalUV(0)
		if e != nil {
			return Solid{}, FaceLoopSplit{}, e
		}
		points[i], curves[i] = p, u
	}
	n := len(points)
	sign := loopOrientation(points)
	if sign == 0 {
		return bad("unresolved loop orientation")
	}
	if sign < 0 {
		ccw := make([]PCurve, n)
		for i, c := range curves {
			ccw[n-1-i] = reversePCurve(c)
		}
		curves = ccw
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, FaceLoopSplit{}, e
	}
	return s.SplitFaceCurvedLoopContext(ctx, face, curves)
}

// ReplaceSplineTubePatch replaces the inside of a closed spline-tube cut
// by the original patch in the other chart. It proves continuous section
// identity and checks the assembled boundary topology. The caller must
// establish that the replacement and retained faces have no other contacts
// and that the replacement has the desired solid orientation.
func (s Solid) ReplaceSplineTubePatch(ctx context.Context, face int, segments []intersect.TubeSegment, secondChart bool) (Solid, FaceLoopSplit, error) {
	r, h, e := s.SplitSplineTubeLoop(ctx, face, segments, secondChart)
	if e != nil {
		return Solid{}, FaceLoopSplit{}, e
	}
	faces := r.Faces()
	old := faces[h.Inner]
	replacement := Face{Reversed: old.Reversed}
	points := make([]geom.Vec2, len(old.Outer))
	for i, u := range old.Outer {
		if e := ctx.Err(); e != nil {
			return Solid{}, FaceLoopSplit{}, e
		}
		c, flip := u.Curve, false
		if rev, ok := c.(reverseTrim); ok {
			c, flip = rev.differentiableTrim, true
		}
		p, ok := c.(intersect.TubeTrim)
		if !ok {
			return Solid{}, FaceLoopSplit{}, geom.ErrInvalid
		}
		_, j, _ := p.Recipe()
		q := segments[j].TrimB()
		if secondChart {
			q = segments[j].TrimA()
		}
		replacement.Surface = q.Support()
		u.Curve = q
		if flip {
			u.Curve = reversePCurve(q)
		}
		u.UVStart, e = u.Curve.EvalUV(0)
		if e != nil {
			return Solid{}, FaceLoopSplit{}, e
		}
		u.UVEnd, e = u.Curve.EvalUV(1)
		if e != nil {
			return Solid{}, FaceLoopSplit{}, e
		}
		if !sameCertifiedSection(r.edges[u.Edge], replacement.Surface, u) {
			return Solid{}, FaceLoopSplit{}, fmt.Errorf("trimmed: replacement trim is not the shared tube branch")
		}
		points[i] = u.UVStart
		replacement.Outer = append(replacement.Outer, u)
	}
	orientation := loopOrientation(points)
	if orientation == 0 {
		return Solid{}, FaceLoopSplit{}, fmt.Errorf("trimmed: unresolved replacement chart orientation")
	}
	if orientation < 0 {
		n := len(replacement.Outer)
		l := make([]Use, n)
		for i, u := range replacement.Outer {
			u.UVStart, u.UVEnd, u.Reversed = u.UVEnd, u.UVStart, !u.Reversed
			u.Curve = reversePCurve(u.Curve)
			l[n-1-i] = u
		}
		replacement.Outer, replacement.Reversed = l, !replacement.Reversed
	}
	// Reject a folded replacement chart and sampled self-crossings before
	// assembly. This is local validation; other shell faces remain the
	// caller's embedding responsibility.
	base, ok := replacement.Surface.(*geom.BSplineSurface)
	if !ok {
		return Solid{}, FaceLoopSplit{}, geom.ErrInvalid
	}
	patch, e := geom.NewSurfacePatch(base, geom.SurfacePatchOptions{})
	if e != nil {
		return Solid{}, FaceLoopSplit{}, e
	}
	d := patch.Domain()
	regular := false
	for _, axes := range [][2]int{{0, 1}, {0, 2}, {1, 2}} {
		if e := ctx.Err(); e != nil {
			return Solid{}, FaceLoopSplit{}, e
		}
		q, e := geom.CertifySurfaceInjectivity(patch, geom.SurfaceInjectivityOptions{ProjectionAxes: axes, MaxDepth: 6, MaxCells: 128})
		if e == nil && q.ProvenInjective {
			regular = true
			break
		}
	}
	if !regular {
		return Solid{}, FaceLoopSplit{}, fmt.Errorf("trimmed: replacement chart injectivity is unresolved")
	}
	outer := []Use{
		{UVStart: geom.Vec2{X: d.UMin, Y: d.VMin}},
		{UVStart: geom.Vec2{X: d.UMax, Y: d.VMin}},
		{UVStart: geom.Vec2{X: d.UMax, Y: d.VMax}},
		{UVStart: geom.Vec2{X: d.UMin, Y: d.VMax}},
	}
	for i := range outer {
		outer[i].UVEnd = outer[(i+1)%len(outer)].UVStart
	}
	var samples []geom.Vec2
	for _, u := range replacement.Outer {
		for k := range 4 {
			q, e := u.uv(float64(k) / 4)
			if e != nil {
				return Solid{}, FaceLoopSplit{}, e
			}
			samples = append(samples, q)
		}
	}
	if e = validateLoopSamples(outer, samples); e != nil {
		return Solid{}, FaceLoopSplit{}, e
	}
	faces[h.Inner] = replacement
	out, e := NewSolidContext(ctx, r.vertices, r.edges, faces, r.tolerance)
	if e != nil {
		return Solid{}, FaceLoopSplit{}, e
	}
	return out, h, nil
}

func loopOrientation(points []geom.Vec2) int {
	left := 0
	for i, p := range points {
		q := points[left]
		if p.X < q.X || p.X == q.X && p.Y < q.Y {
			left = i
		}
	}
	n := len(points)
	sign, _ := geom.Orient2D(points[(left+n-1)%n], points[left], points[(left+1)%n])
	return sign
}

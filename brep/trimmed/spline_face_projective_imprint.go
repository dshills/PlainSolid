package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// ProjectiveSplineImprintOptions bounds support preparation, contact tracing,
// exact edge composition, and each reconstructed trim validation.
type ProjectiveSplineImprintOptions struct {
	Contacts          ProjectiveSplineFaceContactOptions
	Trims             trimvalidate.Options
	MaxEdgeOperations int
}

// ProjectiveSplineImprintUse identifies the source of an emitted coedge.
// Outer uses retain their original face-use index, model edge and directed
// use-clock interval. A section use has OriginalUse and OriginalEdge -1.
type ProjectiveSplineImprintUse struct {
	OriginalUse, OriginalEdge int
	OriginalRange             [2]*big.Rat
	Section                   bool
	SectionArc                int // Valid for a section use.
}

// ProjectiveSplineImprint splits each original face into two independently
// indexed native faces. All four fragments use the same ordered exact
// section model curves with their original pcurves and directions.
// Section is set when the contour has one piece. The result does not sew
// a shell by itself.
type ProjectiveSplineImprint struct {
	Fragments       [2][2]SewFace
	Sources         [2][2][]ProjectiveSplineImprintUse
	Section         geom.ProjectiveContactSpline
	Sections        []geom.ProjectiveContactSplineUse
	ExactOperations int
	Complete        bool
	contact         ProjectiveSplineFaceContacts
}

type projectiveImprintBoundary struct {
	uv     [2]geom.Vec2
	model  *geom.ExactSplineCurve
	source ProjectiveSplineImprintUse
}

// ImprintProjectiveSplineFacePair admits two rectangular spline faces with
// one analytic contact arc and one terminal strictly inside a different
// original side of each owner. Endpoints must be exactly representable in
// native topology, and every original boundary subcurve must compose
// analytically in one span. Unsupported arcs return no fragments.
func ImprintProjectiveSplineFacePair(ctx context.Context, a, b Face, axes [2]int,
	o ProjectiveSplineImprintOptions) (r ProjectiveSplineImprint, err error) {
	return imprintProjectiveSplineFaces(ctx, a, b, axes, o, true, false)
}

// ImprintProjectiveSplineFaceContour splits each original rectangular
// spline face along one open analytic contour, retaining every exact
// section piece and original knot boundary. Distinct outer terminals
// and every original UV and model endpoint must be exactly representable
// in native topology. An unproved use publishes no fragments.
func ImprintProjectiveSplineFaceContour(ctx context.Context, a, b Face, axes [2]int,
	o ProjectiveSplineImprintOptions) (r ProjectiveSplineImprint, err error) {
	return imprintProjectiveSplineFaces(ctx, a, b, axes, o, false, false)
}

func imprintProjectiveSplineFaces(ctx context.Context, a, b Face, axes [2]int,
	o ProjectiveSplineImprintOptions, single, matchExterior bool) (r ProjectiveSplineImprint, err error) {
	if ctx == nil || o.MaxEdgeOperations < 0 {
		return r, geom.ErrInvalid
	}
	if len(a.Holes)+len(b.Holes) != 0 {
		return r, geom.ErrConvergence
	}
	if o.MaxEdgeOperations == 0 {
		o.MaxEdgeOperations = 4_000_000
	}
	contacts, err := TraceProjectiveSplineFaceContacts(ctx, a, b, axes, o.Contacts)
	if err != nil {
		return r, err
	}
	if len(contacts.Graph.Contours) != 1 || contacts.Graph.Contours[0].Closed ||
		single && (len(contacts.Graph.Arcs) != 1 || len(contacts.Vertices) != 2) {
		return r, geom.ErrConvergence
	}
	remaining := func() (geom.SurfacePatchOptions, error) {
		p := o.Contacts.Patches
		p.MaxOperations = o.MaxEdgeOperations - r.ExactOperations
		if o.Contacts.Patches.MaxOperations > 0 {
			p.MaxOperations = min(p.MaxOperations, o.Contacts.Patches.MaxOperations)
		}
		if p.MaxOperations < 1 {
			return p, geom.ErrConvergence
		}
		return p, nil
	}
	limits, err := remaining()
	if err != nil {
		return r, err
	}
	var sections []geom.ProjectiveContactSplineUse
	if single {
		section, err := contacts.Graph.BuildArcSpline(ctx, 0, limits)
		r.ExactOperations += section.ExactOperations
		if err != nil {
			return r, err
		}
		sections = []geom.ProjectiveContactSplineUse{{Arc: 0, Spline: section, Vertices: contacts.Graph.Arcs[0].Vertices}}
	} else {
		contour, err := contacts.Graph.BuildContourSplines(ctx, 0, limits)
		r.ExactOperations += contour.ExactOperations
		if err != nil {
			return r, err
		}
		sections = contour.Uses
	}
	terminals := [2]int{sections[0].Vertices[0], sections[len(sections)-1].Vertices[1]}
	var exterior [2][2][]*big.Rat
	if matchExterior {
		limits, err := remaining()
		if err != nil {
			return r, err
		}
		var work int
		exterior, work, err = projectiveImprintExteriorStations(ctx, contacts.Graph, limits)
		r.ExactOperations += work
		if err != nil {
			return r, err
		}
	}
	zero, one := new(big.Rat), big.NewRat(1, 1)
	var fragments [2][2]SewFace
	var origins [2][2][]ProjectiveSplineImprintUse
	for owner, face := range [2]Face{a, b} {
		var points [2]geom.Vec2
		var uses [2]int
		var stations [2]*big.Rat
		for end, v := range terminals {
			attached := contacts.Vertices[v].Uses[owner]
			if len(attached) != 1 || attached[0].Loop != 0 {
				return r, geom.ErrConvergence
			}
			uses[end] = attached[0].Use
			piece := sections[0]
			if end == 1 {
				piece = sections[len(sections)-1]
			}
			t := end
			if piece.Reversed {
				t = 1 - t
			}
			p, err := piece.Spline.UV[owner].ExactPoint(float64(t))
			if err != nil {
				return r, err
			}
			points[end] = geom.Vec2{X: p.X, Y: p.Y}
			original := face.Outer[uses[end]]
			x, y, station := original.UVStart.X, original.UVEnd.X, p.X
			if x == y {
				x, y, station = original.UVStart.Y, original.UVEnd.Y, p.Y
			}
			start := new(big.Rat).SetFloat64(x)
			stations[end] = new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(station), start),
				new(big.Rat).Sub(new(big.Rat).SetFloat64(y), start))
			if stations[end].Sign() <= 0 || stations[end].Cmp(one) >= 0 {
				return r, geom.ErrConvergence
			}
			limits, err := remaining()
			if err != nil {
				return r, err
			}
			sign, err := attached[0].UseParameter.SignPolynomial(ctx, []*big.Rat{new(big.Rat).Neg(stations[end]), one},
				geom.ExactCurvePlaneEventOptions{MaxOperations: limits.MaxOperations, MaxCoefficientBits: limits.MaxCoefficientBits})
			r.ExactOperations += sign.Operations
			if err != nil {
				return r, err
			}
			if sign.Sign != 0 {
				return r, geom.ErrConvergence
			}
		}
		if uses[0] == uses[1] {
			return r, geom.ErrConvergence
		}
		var seams []ExactQuadraticUV
		for _, section := range sections {
			limits, err := remaining()
			if err != nil {
				return r, err
			}
			seam, work, err := projectiveSectionUV(ctx, section.Spline.UV[owner], limits)
			r.ExactOperations += work
			if err != nil {
				return r, err
			}
			seams = append(seams, seam)
		}
		base, pose := parametric.SurfaceMapping(face.Surface)
		limits, err = remaining()
		if err != nil {
			return r, err
		}
		patch, err := exactSplineSupportPatch(base, limits)
		if err != nil {
			return r, err
		}
		if _, ok := base.(*geom.BSplineSurface); ok {
			r.ExactOperations += patch.ExactOperations()
		}
		if pose != geom.Identity() {
			limits, err = remaining()
			if err != nil {
				return r, err
			}
			transformed, work, err := patch.TransformedWithWorkLimit(pose, limits.MaxOperations)
			r.ExactOperations += work
			if err != nil {
				return r, err
			}
			patch = transformed
		}
		boundary := make([]projectiveImprintBoundary, 0, 6)
		next := [2]int{-1, -1}
		for i, u := range face.Outer {
			p := []geom.Vec2{u.UVStart}
			t := []*big.Rat{zero}
			for end, on := range uses {
				if on == i {
					p, t = append(p, points[end]), append(t, stations[end])
				}
			}
			p, t = append(p, u.UVEnd), append(t, one)
			if matchExterior {
				limits, err := remaining()
				if err != nil {
					return r, err
				}
				var work int
				p, t, work, err = projectiveImprintAlignOriginalUse(ctx, p, t, exterior[owner], limits)
				r.ExactOperations += work
				if err != nil {
					return r, err
				}
			}
			for j := 0; j+1 < len(p); j++ {
				limits, err = remaining()
				if err != nil {
					return r, err
				}
				span := ProjectiveSplineImprintUse{OriginalUse: i, OriginalEdge: u.Edge,
					OriginalRange: [2]*big.Rat{new(big.Rat).Set(t[j]), new(big.Rat).Set(t[j+1])}}
				edges, used, err := composeProjectiveImprintUse(ctx, patch, p[j], p[j+1], span, limits, single)
				r.ExactOperations += used
				if err != nil {
					return r, err
				}
				boundary = append(boundary, edges...)
				for end, on := range uses {
					if i == on && p[j+1] == points[end] {
						next[end] = len(boundary)
					}
				}
			}
		}
		for i := range next {
			if next[i] < 0 || next[i] >= len(boundary) {
				return r, geom.ErrConvergence
			}
		}
		for start := range 2 {
			var path []projectiveImprintBoundary
			for j := next[start]; j != next[1-start]; j = (j + 1) % len(boundary) {
				path = append(path, boundary[j])
			}
			part, ancestry, err := projectiveImprintContourFragment(ctx, face, path, sections, seams, start == 0, o.Trims)
			if err != nil {
				return r, err
			}
			fragments[owner][start], origins[owner][start] = part, ancestry
		}
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.Fragments, r.Sources, r.Sections, r.contact, r.Complete = fragments, origins, sections, contacts, true
	if len(sections) == 1 {
		r.Section = sections[0].Spline
	}
	return r, nil
}

func composeProjectiveImprintSide(ctx context.Context, patch *geom.SurfacePatch, a, b geom.Vec2,
	o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, int, error) {
	zero, one := new(big.Rat), big.NewRat(1, 1)
	line, err := geom.NewExactHomogeneousBezier(ctx, [][4]*big.Rat{
		{new(big.Rat).SetFloat64(a.X), new(big.Rat).SetFloat64(a.Y), zero, one},
		{new(big.Rat).SetFloat64(b.X), new(big.Rat).SetFloat64(b.Y), zero, one},
	}, o)
	if err != nil {
		return nil, 0, err
	}
	work := line.ExactOperations()
	o.MaxOperations -= work
	if o.MaxOperations < 1 {
		return nil, work, geom.ErrConvergence
	}
	r, err := patch.ComposeParameterCurvePieces(ctx, line, geom.SurfaceCurveCompositionOptions{SurfacePatchOptions: o})
	work += r.ExactOperations
	if err != nil {
		return nil, work, err
	}
	if !r.Complete || len(r.Pieces) != 1 || r.Pieces[0].Start.Sign() != 0 || r.Pieces[0].End.Cmp(one) != 0 {
		return nil, work, geom.ErrConvergence
	}
	return r.Pieces[0].Curve, work, nil
}

func projectiveSectionUV(ctx context.Context, curve *geom.ExactSplineCurve, limits geom.SurfacePatchOptions) (q ExactQuadraticUV, work int, err error) {
	net, work, err := curve.BezierControlNet(ctx, limits)
	if err != nil {
		return q, work, err
	}
	if len(net.Control) != 2 && len(net.Control) != 3 {
		return q, work, geom.ErrConvergence
	}
	var h [3][3]*big.Rat
	for i, axis := range [3]int{0, 1, 3} {
		h[0][i], h[2][i] = net.Control[0][axis], net.Control[len(net.Control)-1][axis]
		if len(net.Control) == 2 {
			h[1][i] = new(big.Rat).Quo(new(big.Rat).Add(h[0][i], h[2][i]), big.NewRat(2, 1))
		} else {
			h[1][i] = net.Control[1][axis]
		}
	}
	q, err = NewExactQuadraticUV(h)
	return q, work, err
}

func composeProjectiveImprintUse(ctx context.Context, patch *geom.SurfacePatch, a, b geom.Vec2,
	source ProjectiveSplineImprintUse, o geom.SurfacePatchOptions, single bool) ([]projectiveImprintBoundary, int, error) {
	if single {
		edge, work, err := composeProjectiveImprintSide(ctx, patch, a, b, o)
		if err != nil {
			return nil, work, err
		}
		return []projectiveImprintBoundary{{[2]geom.Vec2{a, b}, edge, source}}, work, nil
	}
	zero, one := new(big.Rat), big.NewRat(1, 1)
	x := [2]*big.Rat{new(big.Rat).SetFloat64(a.X), new(big.Rat).SetFloat64(a.Y)}
	y := [2]*big.Rat{new(big.Rat).SetFloat64(b.X), new(big.Rat).SetFloat64(b.Y)}
	line, err := geom.NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{x[0], x[1], zero, one}, {y[0], y[1], zero, one}}, o)
	if err != nil {
		return nil, 0, err
	}
	limits := o
	work := line.ExactOperations()
	limits.MaxOperations -= work
	if limits.MaxOperations < 1 {
		return nil, work, geom.ErrConvergence
	}
	result, err := patch.ComposeParameterCurvePieces(ctx, line, geom.SurfaceCurveCompositionOptions{SurfacePatchOptions: limits})
	work += result.ExactOperations
	if err != nil {
		return nil, work, err
	}
	if !result.Complete || len(result.Pieces) == 0 {
		return nil, work, geom.ErrConvergence
	}
	last := zero
	lerp := func(a, b, t *big.Rat) *big.Rat {
		return new(big.Rat).Add(a, new(big.Rat).Mul(t, new(big.Rat).Sub(b, a)))
	}
	bits := o.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	var pieces []projectiveImprintBoundary
	for _, p := range result.Pieces {
		if err := ctx.Err(); err != nil {
			return nil, work, err
		}
		if p.Start.Cmp(last) != 0 || p.End.Cmp(p.Start) <= 0 {
			return nil, work, geom.ErrConvergence
		}
		if work > o.MaxOperations-18 {
			return nil, work, geom.ErrConvergence
		}
		work += 18
		q := projectiveImprintBoundary{model: p.Curve, source: source}
		for i, t := range [2]*big.Rat{p.Start, p.End} {
			station := lerp(source.OriginalRange[0], source.OriginalRange[1], t)
			u, v := lerp(x[0], y[0], t), lerp(x[1], y[1], t)
			for _, c := range [3]*big.Rat{station, u, v} {
				if c.Num().BitLen() > bits || c.Denom().BitLen() > bits {
					return nil, work, geom.ErrConvergence
				}
			}
			du, err := geom.RepresentedExactScalar(u)
			if err != nil {
				return nil, work, err
			}
			dv, err := geom.RepresentedExactScalar(v)
			if err != nil {
				return nil, work, err
			}
			q.uv[i], q.source.OriginalRange[i] = geom.Vec2{X: du, Y: dv}, station
		}
		pieces = append(pieces, q)
		last = p.End
	}
	if last.Cmp(one) != 0 {
		return nil, work, geom.ErrConvergence
	}
	return pieces, work, nil
}

func projectiveImprintContourFragment(ctx context.Context, original Face, path []projectiveImprintBoundary,
	sections []geom.ProjectiveContactSplineUse, curves []ExactQuadraticUV, reverse bool,
	o trimvalidate.Options) (SewFace, []ProjectiveSplineImprintUse, error) {
	const terminal = 1.0
	n := len(path)
	if n < 2 || len(sections) == 0 || len(sections) != len(curves) {
		return SewFace{}, nil, geom.ErrConvergence
	}
	s := SewFace{Face: Face{Surface: original.Surface, Reversed: original.Reversed}}
	var refs []ProjectiveSplineImprintUse
	for i, p := range path {
		x, err := p.model.ExactPoint(0)
		if err != nil {
			return SewFace{}, nil, err
		}
		s.Vertices = append(s.Vertices, x)
		s.Edges = append(s.Edges, parametric.Edge{Start: i, End: i + 1, Curve: p.model, Range: parametric.Interval{Max: terminal}})
		s.Face.Outer = append(s.Face.Outer, Use{Edge: i, UVStart: p.uv[0], UVEnd: p.uv[1]})
		refs = append(refs, p.source)
	}
	nextUV := path[n-1].uv[1]
	for i := range sections {
		k := i
		if reverse {
			k = len(sections) - 1 - i
		}
		use, curve := sections[k], curves[k]
		rev := use.Reversed != reverse
		start, end := n+i, n+i+1
		if i+1 == len(sections) {
			end = 0
		}
		modelStart := 0.
		if rev {
			start, end = end, start
			curve = curve.reversed()
			modelStart = terminal
		}
		if nextUV != curve.ends[0] {
			return SewFace{}, nil, geom.ErrConvergence
		}
		x, err := use.Spline.Model.ExactPoint(modelStart)
		if err != nil {
			return SewFace{}, nil, err
		}
		s.Vertices = append(s.Vertices, x)
		s.Edges = append(s.Edges, parametric.Edge{Start: start, End: end, Curve: use.Spline.Model, Range: parametric.Interval{Max: terminal}})
		s.Face.Outer = append(s.Face.Outer, Use{Edge: n + i, Reversed: rev, UVStart: curve.ends[0], UVEnd: curve.ends[1], Curve: curve})
		refs = append(refs, ProjectiveSplineImprintUse{OriginalUse: -1, OriginalEdge: -1, Section: true, SectionArc: use.Arc})
		nextUV = curve.ends[1]
	}
	if nextUV != path[0].uv[0] {
		return SewFace{}, nil, geom.ErrConvergence
	}
	for _, e := range s.Edges {
		// A topology join uses exact source endpoints, not a tolerance weld.
		c := e.Curve.(*geom.ExactSplineCurve)
		for i, v := range [2]int{e.Start, e.End} {
			p, err := c.ExactPoint(float64(i))
			if err != nil {
				return SewFace{}, nil, err
			}
			if p != s.Vertices[v] {
				return SewFace{}, nil, geom.ErrConvergence
			}
		}
	}
	validation, err := ValidateFaceTrimLoops(ctx, s.Face, o)
	if err != nil {
		return SewFace{}, nil, err
	}
	if validation.Status != trimvalidate.Certified {
		return SewFace{}, nil, geom.ErrConvergence
	}
	return s, refs, nil
}

// Package trimmed represents surfaces with shared model-space edges and
// oriented straight or curved parameter-space trims.
package trimmed

import (
	"context"
	"fmt"
	"math"
	"slices"

	"plainsolid/brep"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// PCurve evaluates a parameter-space trim on [0,1] in loop direction.
// Implementations must be immutable and safe for concurrent evaluation.
type PCurve interface {
	EvalUV(t float64) (geom.Vec2, error)
}

// Use assigns a shared edge a p-curve between UVStart and UVEnd. With nil
// Curve, it is straight. Reversed swaps the model-edge direction, not the
// parameter direction of Curve. Curve evaluates to UVStart at 0 and to UVEnd
// at 1, to the last bit. An EllipseUV alone may end within the solid's
// tolerance of them instead, which lets arcs of different curves meet at a
// vertex: NewSolid then keeps the arc with its two ends at UVStart and UVEnd.
type Use struct {
	Edge           int
	Reversed       bool
	UVStart, UVEnd geom.Vec2
	Curve          PCurve
}

// Face contains a positively wound outer UV loop and optional negatively
// wound inner UV loops. Reversed flips the physical face orientation.
type Face struct {
	Surface  geom.Surface
	Outer    []Use
	Holes    [][]Use
	Reversed bool
}

// Solid is immutable except that callers must supply immutable, thread-safe
// surface and curve evaluators. Closed periodic edges and collapsed pole
// sides are permitted. A collapsed edge has equal ends, nil model curve,
// and exactly one face use.
type Solid struct {
	vertices           []geom.Vec3
	edges              []parametric.Edge
	faces              []Face
	tolerance          float64
	seeds              [][]seed
	minimumLevel       int
	sampledAnnularCaps bool
	splitSeeds         []bool
	clippedFuseAxis    int // 0 for ordinary shells, 1 or 2 for an original-source clipped Fuse
}
type seed struct {
	uv  [3]geom.Vec2
	ids [3]int
	// side >=0 refers to a shared topology edge. -1 means an interior diagonal.
	side [3]trimSide
	// split asks for a vertex at the centroid before refinement; see
	// smoothTrimCorner.
	split bool
}
type trimSide struct {
	edge int
	use  Use
}

func internalSide() trimSide { return trimSide{edge: -1} }

type samples struct {
	uv   [3]geom.Vec2
	ids  [3]int
	edge [3]trimSide
	// Corresponding edge parameters on each oriented triangle side.
	start, end [3]float64
	// A seed that is split at its centroid before refinement.
	split bool
	// Set only by analytic adaptive refinement after the complete surface
	// and phase checks. New or changed facets must prove themselves again.
	adaptiveValidated bool
}

func (s Solid) Vertices() []geom.Vec3    { return append([]geom.Vec3(nil), s.vertices...) }
func (s Solid) Edges() []parametric.Edge { return append([]parametric.Edge(nil), s.edges...) }
func (s Solid) Faces() []Face {
	r := make([]Face, len(s.faces))
	for i, f := range s.faces {
		r[i] = cloneFace(f)
	}
	return r
}
func cloneFace(f Face) Face {
	f.Outer = append([]Use(nil), f.Outer...)
	holes := make([][]Use, len(f.Holes))
	for i, h := range f.Holes {
		holes[i] = append([]Use(nil), h...)
	}
	f.Holes = holes
	return f
}

// NewSolid validates and copies topology, triangulates the trim domains in
// UV space, and checks sampled edge agreement and manifold incidence.
// It does not certify continuous boundary agreement or detect embedding
// self-intersections. Face limits follow brep.MaxHoledFaceVertices.
// A corner of a loop may lie on the line through its neighbors, as a vertex
// that subdivides a straight edge does, whether or not its rounded UV
// coordinates are collinear with theirs. A face without area is rejected.
func NewSolid(vertices []geom.Vec3, edges []parametric.Edge, faces []Face, tolerance float64) (Solid, error) {
	return NewSolidContext(context.Background(), vertices, edges, faces, tolerance)
}

// NewSolidContext is NewSolid with cooperative cancellation during boundary
// validation and seed mesh construction.
func NewSolidContext(ctx context.Context, vertices []geom.Vec3, edges []parametric.Edge, faces []Face, tolerance float64) (Solid, error) {
	return newSolidContext(ctx, vertices, edges, faces, tolerance, true)
}

// Intermediate graphs in a single analytic construction retain the complete
// topology and sampled coedge agreement checks. The finished graph performs
// display validation before publication.
func newSolidContext(ctx context.Context, vertices []geom.Vec3, edges []parametric.Edge, faces []Face, tolerance float64, validateDisplay bool) (Solid, error) {
	if ctx == nil {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	if len(vertices) == 0 && len(edges) == 0 && len(faces) == 0 {
		return Solid{}, nil
	}
	if tolerance <= 0 || !finite(tolerance) || len(vertices) == 0 || len(edges) == 0 || len(faces) == 0 {
		return Solid{}, fmt.Errorf("trimmed: invalid topology or tolerance")
	}
	s := Solid{vertices: append([]geom.Vec3(nil), vertices...), edges: append([]parametric.Edge(nil), edges...), faces: make([]Face, len(faces)), tolerance: tolerance}
	for i, p := range vertices {
		if !p.IsFinite() {
			return Solid{}, fmt.Errorf("trimmed: nonfinite vertex %d", i)
		}
	}
	used := make([]bool, len(vertices))
	for i, e := range edges {
		if e := ctx.Err(); e != nil {
			return Solid{}, e
		}
		if e.Start < 0 || e.End < 0 || e.Start >= len(vertices) || e.End >= len(vertices) ||
			e.Curve == nil && e.Start != e.End ||
			e.Curve != nil && (!(e.Range.Min < e.Range.Max) || !finite(e.Range.Max-e.Range.Min)) ||
			e.Start != e.End && vertices[e.Start] == vertices[e.End] {
			return Solid{}, fmt.Errorf("trimmed: invalid edge %d", i)
		}
		if e.Start == e.End {
			s.minimumLevel = 2
		}
		for k, id := range [2]int{e.Start, e.End} {
			p, err := s.onEdge(i, float64(k))
			if err != nil || !closePoint(p, vertices[id], tolerance) {
				return Solid{}, fmt.Errorf("trimmed: edge %d endpoint differs from vertex", i)
			}
			used[id] = true
		}
	}
	for i, b := range used {
		if !b {
			return Solid{}, fmt.Errorf("trimmed: unused vertex %d", i)
		}
	}
	count, sign := make([]int, len(edges)), make([]int, len(edges))
	for fi, f := range faces {
		if e := ctx.Err(); e != nil {
			return Solid{}, e
		}
		if f.Surface == nil {
			return Solid{}, fmt.Errorf("trimmed: nil surface at face %d", fi)
		}
		base, _ := parametric.SurfaceMapping(f.Surface)
		if wall, ok := base.(geom.HelicalProfileSurface); ok {
			d := wall.Domain()
			// A helical side can span integral turns between adjacent
			// topology vertices. Keep the initial angular chords at or
			// below a quarter turn to avoid degenerate construction.
			span := d.UMax - d.UMin
			if span > 16 || len(faces) > 34 {
				return Solid{}, parametric.ErrBudget
			}
			for level := 0; span > .25; level++ {
				span /= 2
				s.minimumLevel = max(s.minimumLevel, level+1)
			}
		}
		if wall, ok := base.(geom.TwistedCylinderSurface); ok {
			d, e := faceUVBounds(f)
			if e != nil {
				return Solid{}, e
			}
			_, _, twist, _ := wall.Parameters()
			span := d.UMax - d.UMin + math.Abs(twist)*(d.VMax-d.VMin)
			if !finite(span) || span > 32 || len(faces) > 34 {
				return Solid{}, parametric.ErrBudget
			}
			// Retain the finer initial bound for the older certified charts;
			// the expanded integration range has its own sampling budget.
			step := .25
			if span > 16 {
				step = .5
			}
			for level := 0; span > step; level++ {
				span /= 2
				s.minimumLevel = max(s.minimumLevel, level+1)
			}
		}
		s.faces[fi] = cloneFace(f)
		f = s.faces[fi] // the copy, whose arcs are pinned below
		sides := map[[2]geom.Vec2]trimSide{}
		indices := map[geom.Vec2]int{}
		rings := [][]geom.Vec3{}
		for ri, loop := range append([][]Use{f.Outer}, f.Holes...) {
			if len(loop) < 3 || len(loop) > brep.MaxHoledFaceVertices {
				return Solid{}, fmt.Errorf("trimmed: invalid face %d loop %d size", fi, ri)
			}
			var ring []geom.Vec3
			start, end := make([]int, len(loop)), make([]int, len(loop))
			for j, u := range loop {
				if e := ctx.Err(); e != nil {
					return Solid{}, e
				}
				if u.Edge < 0 || u.Edge >= len(edges) || !uvFinite(u.UVStart) || !uvFinite(u.UVEnd) || u.UVStart == u.UVEnd ||
					u.UVEnd != loop[(j+1)%len(loop)].UVStart {
					return Solid{}, fmt.Errorf("trimmed: invalid p-curve in face %d", fi)
				}
				if arc, ok := u.Curve.(EllipseUV); ok {
					pinned, err := pinArcEnds(f.Surface, arc, u.UVStart, u.UVEnd, tolerance)
					if err != nil {
						return Solid{}, err
					}
					u.Curve, loop[j].Curve = pinned, pinned
				}
				e := edges[u.Edge]
				start[j], end[j] = e.Start, e.End
				if u.Reversed {
					start[j], end[j] = end[j], start[j]
				}
				if _, ok := indices[u.UVStart]; ok {
					return Solid{}, fmt.Errorf("trimmed: repeated UV vertex in face %d", fi)
				}
				indices[u.UVStart] = start[j]
				sides[[2]geom.Vec2{u.UVStart, u.UVEnd}] = trimSide{u.Edge, u}
				ring = append(ring, geom.Vec3{X: u.UVStart.X, Y: u.UVStart.Y})
				count[u.Edge]++
				if u.Reversed != f.Reversed {
					sign[u.Edge]--
				} else {
					sign[u.Edge]++
				}
				for k := 0; k <= 8; k++ {
					t := float64(k) / 8
					uv, err := u.uv(t)
					if err != nil {
						return Solid{}, err
					}
					if k == 0 && uv != u.UVStart || k == 8 && uv != u.UVEnd {
						return Solid{}, fmt.Errorf("trimmed: p-curve endpoints differ from declared UV endpoints")
					}
					p, err := f.Surface.Eval(uv.X, uv.Y)
					if err != nil {
						return Solid{}, err
					}
					if u.Reversed {
						t = 1 - t
					}
					q, err := s.onEdge(u.Edge, t)
					if err != nil || !closePoint(p, q, tolerance) {
						return Solid{}, fmt.Errorf("trimmed: face %d and edge %d disagree", fi, u.Edge)
					}
				}
			}
			for i, id := range end {
				if id != start[(i+1)%len(loop)] {
					return Solid{}, fmt.Errorf("trimmed: open topological loop in face %d", fi)
				}
			}
			// Use the turn at the lexicographically lowest point. For a
			// validated simple ring with no straight corner it is its winding.
			left := 0
			for i, p := range loop {
				q := loop[left].UVStart
				if p.UVStart.X < q.X || p.UVStart.X == q.X && p.UVStart.Y < q.Y {
					left = i
				}
			}
			orientation, _ := geom.Orient2D(loop[(left+len(loop)-1)%len(loop)].UVStart, loop[left].UVStart, loop[(left+1)%len(loop)].UVStart)
			if orientation == 0 {
				// When the extremal point lies on a straight run, walk to
				// its extremal endpoint before testing the turn.
				for k := 1; k < len(loop) && orientation == 0; k++ {
					j := (left + k) % len(loop)
					p := loop[j].UVStart
					if p.X != loop[left].UVStart.X {
						break
					}
					orientation, _ = geom.Orient2D(loop[(j+len(loop)-1)%len(loop)].UVStart, p, loop[(j+1)%len(loop)].UVStart)
				}
			}
			if ri == 0 && orientation != 1 || ri > 0 && orientation != -1 {
				return Solid{}, fmt.Errorf("trimmed: invalid UV winding in face %d", fi)
			}
			rings = append(rings, ring)
		}
		if len(indices) > brep.MaxHoledFaceVertices {
			return Solid{}, fmt.Errorf("trimmed: face vertex limit exceeded")
		}
		triangles, err := triangulateUV(rings)
		if err != nil {
			if len(f.Holes) != 1 {
				return Solid{}, fmt.Errorf("trimmed: face %d domain: %w", fi, err)
			}
			if !s.sampledAnnularCaps {
				ok, shapeErr := supportsCircularAnnularSeedShell(ctx, faces)
				if shapeErr != nil {
					return Solid{}, shapeErr
				}
				if !ok {
					return Solid{}, fmt.Errorf("trimmed: face %d domain: %w", fi, err)
				}
			}
			level, seedErr := circularAnnularSeedLevel(ctx, f)
			if seedErr != nil {
				return Solid{}, fmt.Errorf("trimmed: face %d annular seed: %w", fi, seedErr)
			}
			triangles = nil // Mass integration uses the signed original chord rings.
			s.sampledAnnularCaps = true
			s.minimumLevel = max(s.minimumLevel, level)
		}
		// Triangles without area in space are exchanged first, then curved
		// trims are seated. If some trim still leaves its triangle, the
		// triangles that join two chart copies of a seam vertex may be in
		// its way; the result without them is taken when it seats every
		// trim. One that seats only more of them is not known to be better.
		model := func(p geom.Vec2) geom.Vec3 { return vertices[indices[p]] }
		seated, left, err := seatCurvedTrims(flipDegenerateTriangles(slices.Clone(triangles), model, false), sides, model)
		if err != nil {
			return Solid{}, err
		}
		if left > 0 {
			other, fewer, err := seatCurvedTrims(flipDegenerateTriangles(slices.Clone(triangles), model, true), sides, model)
			if err != nil {
				return Solid{}, err
			}
			if fewer == 0 {
				seated = other
			}
		}
		triangles = seated
		var faceSeeds []seed
		for _, tri := range triangles {
			r := seed{uv: tri, side: [3]trimSide{internalSide(), internalSide(), internalSide()}}
			for j, p := range r.uv {
				r.ids[j] = indices[p]
				if edge, ok := sides[[2]geom.Vec2{p, r.uv[(j+1)%3]}]; ok {
					r.side[j] = edge
				}
			}
			if _, planar := base.(geom.PlaneSurface); !planar {
				if r.split, err = smoothTrimCorner(r); err != nil {
					return Solid{}, err
				}
			}
			faceSeeds = append(faceSeeds, r)
		}
		s.seeds = append(s.seeds, faceSeeds)
	}
	// Two faces may be capped by the very same ring. Split both sets of
	// seeds before dyadic refinement so an ear with two boundary sides
	// cannot create an interior diagonal with the same model IDs on both.
	s.splitSeeds = make([]bool, len(faces))
	type seedSide struct{ a, b int }
	type seedOwner struct{ face, edge int }
	type chartSide struct {
		face int
		ends [2]geom.Vec2
	}
	owners := map[seedSide]seedOwner{}
	chartEdges := map[seedSide]chartSide{}
	for i, ss := range s.seeds {
		for _, tri := range ss {
			for j, side := range tri.side {
				a, b := tri.ids[j], tri.ids[(j+1)%3]
				if a == b {
					// An interior side between the two chart copies of
					// one vertex, on a face that winds around its
					// surface, is a closed curve in space unless it
					// stays at the vertex, as it does along a pole. Like
					// a closed edge it needs two levels: after one, both
					// of its halves join the same two mesh vertices.
					if side.edge < 0 && s.minimumLevel < 2 {
						p := mix(tri.uv[j], tri.uv[(j+1)%3], .5)
						if q, err := numericalWrappedSupport(s.faces[i].Surface).Eval(p.X, p.Y); err == nil && !closePoint(q, vertices[a], tolerance) {
							s.minimumLevel = 2
						}
					}
					continue
				}
				k := seedSide{min(a, b), max(a, b)}
				uv := [2]geom.Vec2{tri.uv[j], tri.uv[(j+1)%3]}
				if a > b {
					uv[0], uv[1] = uv[1], uv[0]
				}
				if prior, ok := owners[k]; ok {
					if prior.face != i && (side.edge < 0 || prior.edge < 0 || prior.edge != side.edge) {
						s.splitSeeds[prior.face], s.splitSeeds[i] = true, true
						s.minimumLevel = max(s.minimumLevel, 1)
					}
					// A periodic quotient can identify an internal chart
					// diagonal with an unrelated edge of the same face.
					// The geometric coedge uses share samples; the internal
					// diagonal must get its own midpoint before mesh checks.
					old := chartEdges[k]
					if old.face == i && old.ends != uv && (prior.edge < 0 || side.edge < 0 || prior.edge != side.edge) {
						s.minimumLevel = max(s.minimumLevel, 1)
					}
				} else {
					owners[k] = seedOwner{i, side.edge}
					chartEdges[k] = chartSide{i, uv}
				}
			}
		}
	}
	for i, n := range count {
		if s.edges[i].Curve == nil {
			if n != 1 {
				return Solid{}, fmt.Errorf("trimmed: collapsed edge %d needs one use", i)
			}
		} else if n != 2 || sign[i] != 0 {
			return Solid{}, fmt.Errorf("trimmed: edge %d needs two opposite uses", i)
		}
	}
	if s.sampledAnnularCaps {
		agreement, err := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if err != nil {
			return Solid{}, err
		}
		if !agreement.Complete {
			return Solid{}, geom.ErrConvergence
		}
		for _, u := range agreement.Uses {
			if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
				return Solid{}, geom.ErrConvergence
			}
		}
	}
	if validateDisplay {
		if _, err := s.refine(ctx, s.minimumLevel, 0, 0); err != nil {
			return Solid{}, err
		}
	}
	if usesRationalEndpoints(s) {
		if err := validateNativeExactQuadratics(ctx, s); err != nil {
			return Solid{}, err
		}
	}
	return s, nil
}

// pinArcEnds gives the p-curve a solid stores for an arc declared between
// start and end. An arc that evaluates to them is stored as it is. Two arcs
// of different curves that meet at a vertex do not, because each rounds the
// shared point in its own way, and every later step (the chart polygon, ray
// crossing counts, strips for integration) needs both uses to name one
// point. Such an arc is stored with its ends replaced by start and end and
// its interior unchanged. The ends may move by no more than the tolerance,
// measured in the chart with the lengths of the surface derivatives, so a
// point one period away or on another part of the curve is refused.
func pinArcEnds(surface geom.Surface, arc EllipseUV, start, end geom.Vec2, tolerance float64) (PCurve, error) {
	var ends [2]geom.Vec2
	for i := range ends {
		var err error
		if ends[i], err = arc.EvalUV(float64(i)); err != nil {
			return nil, err
		}
	}
	if ends[0] == start && ends[1] == end {
		return arc, nil
	}
	for i, declared := range [2]geom.Vec2{start, end} {
		du, dv, err := surface.Partials(declared.X, declared.Y)
		if err != nil {
			return nil, err
		}
		if !(math.Abs(ends[i].X-declared.X)*du.Length()+math.Abs(ends[i].Y-declared.Y)*dv.Length() <= tolerance) {
			return nil, fmt.Errorf("trimmed: p-curve endpoints differ from declared UV endpoints")
		}
	}
	return unionArcUV{EllipseUV: arc, Start: start, End: end}, nil
}

// smoothTrimCorner reports whether two trim sides of a seed, at least one
// of them curved, meet at their common corner without turning, as two arcs
// of one rim do. Refinement cuts off a triangle at that corner whose three
// corners are all on the rim. However small it gets, its plane is the one
// the rim curves in and not the tangent plane of the surface, so on a curved
// surface it meets no bound on the angle between the two. A seed split at
// its centroid has one trim side to a triangle, and the corner gets an
// interior side. Exchanging a side of the seed cannot be relied on for this:
// along three or more such corners in a row, every exchange moves the
// triangle to the next corner.
//
// The directions in which the sides leave the corner are taken from the
// points refinement would reach at its tenth level, and count as opposite
// when they differ by less than a chord that short differs from its tangent.
func smoothTrimCorner(r seed) (bool, error) {
	const near = 1.0 / 1024
	for j, corner := range r.uv {
		in, out := r.side[(j+2)%3], r.side[j]
		if in.edge < 0 || out.edge < 0 || in.use.Curve == nil && out.use.Curve == nil {
			continue
		}
		before, after := r.uv[(j+2)%3], r.uv[(j+1)%3]
		var err error
		if in.use.Curve != nil {
			if before, err = in.use.uv(1 - near); err != nil {
				return false, err
			}
		}
		if out.use.Curve != nil {
			if after, err = out.use.uv(near); err != nil {
				return false, err
			}
		}
		back, on := sub2(before, corner), sub2(after, corner)
		cross, dot := back.X*on.Y-back.Y*on.X, back.X*on.X+back.Y*on.Y
		if dot < 0 && math.Abs(cross) <= 1e-2*math.Abs(dot) {
			return true, nil
		}
	}
	return false, nil
}

func finite(v float64) bool     { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func uvFinite(v geom.Vec2) bool { return finite(v.X) && finite(v.Y) }
func closePoint(a, b geom.Vec3, t float64) bool {
	return a.IsFinite() && b.IsFinite() && a.Sub(b).Length() <= t
}
func mix(a, b geom.Vec2, t float64) geom.Vec2 {
	p := geom.Vec2{X: (1-t)*a.X + t*b.X, Y: (1-t)*a.Y + t*b.Y}
	// A constant trim coordinate must remain the represented constant.
	// Separate multiply/add evaluation (notably WASM) can otherwise round
	// outside a closed support domain even when both endpoints lie on it.
	if a.X == b.X {
		p.X = a.X
	}
	if a.Y == b.Y {
		p.Y = a.Y
	}
	return p
}
func (u Use) uv(t float64) (geom.Vec2, error) {
	if u.Curve == nil {
		return mix(u.UVStart, u.UVEnd, t), nil
	}
	p, e := u.Curve.EvalUV(t)
	if e != nil {
		return geom.Vec2{}, e
	}
	if !uvFinite(p) {
		return geom.Vec2{}, fmt.Errorf("trimmed: nonfinite p-curve point")
	}
	return p, nil
}
func (s trimSide) point(modelT float64) (geom.Vec2, error) {
	if s.use.Reversed {
		modelT = 1 - modelT
	}
	return s.use.uv(modelT)
}

func (s Solid) onEdge(i int, t float64) (geom.Vec3, error) {
	e := s.edges[i]
	if e.Curve == nil {
		return s.vertices[e.Start], nil
	}
	u := e.Range.Min + (e.Range.Max-e.Range.Min)*t
	if t == 1 {
		u = e.Range.Max
	}
	return e.Curve.Eval(u)
}

func (s Solid) seedTriangles() [][]samples {
	out := make([][]samples, len(s.faces))
	for fi, face := range s.seeds {
		for _, r := range face {
			x := samples{uv: r.uv, ids: r.ids, edge: r.side, split: r.split}
			for i, e := range x.edge {
				if e.edge < 0 {
					continue
				}
				x.start[i], x.end[i] = 0, 1
				if e.use.Reversed {
					x.start[i], x.end[i] = 1, 0
				}
			}
			out[fi] = append(out[fi], x)
		}
	}
	return out
}

// Tessellate refines faces using shared model-edge samples. The maximum
// number of UV subdivisions is controlled by MaxSubdivisions (default 128);
// MaxTriangles bounds the resulting triangles. Deflection and normal angles
// are checked at samples. AnalyticBound adds a real-arithmetic interpolation
// bound for straight trims on supported analytic and clamped spline surfaces.
func (s Solid) Tessellate(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
	return s.tessellateWithTrimMemo(ctx, o, &tessellationTrimMemo{limit: tessellationTrimMemoLimit})
}

func (s Solid) tessellateWithTrimMemo(ctx context.Context, o parametric.TessellationOptions, memo *tessellationTrimMemo) (mesh.Mesh, error) {
	o, err := normalizeTrimmedTessellationOptions(ctx, o)
	if err != nil {
		return mesh.Mesh{}, err
	}
	if !s.sampledAnnularCaps && !o.AnalyticBound && s.hasCircularBoreRimBlend() {
		if _, proofErr := s.ValidateWrappedCylinderBoreRimFillet(ctx, geom.SurfacePatchOptions{}); proofErr == nil {
			s.sampledAnnularCaps = true
			s.minimumLevel = max(s.minimumLevel, 1)
		}
	}
	s = s.withTessellationTrimMemo(memo)
	if s.sampledAnnularCaps {
		return s.tessellateCircularAnnularSeeds(ctx, o)
	}
	if s.clippedFuseAxis != 0 && !o.AnalyticBound {
		m, err := s.tessellateClippedPolynomial(ctx, s.clippedFuseAxis-1, o)
		if err != parametric.ErrBudget && err != errTrimFold {
			return m, err
		}
	}
	if m, ok, err := s.compactCylinderMesh(ctx, o); ok {
		return m, err
	}
	n := 0
	maximum := 0.
	for i, triangles := range s.seeds {
		size := len(triangles)
		if s.splitSeeds[i] {
			size *= 3
		} else {
			for _, tri := range triangles {
				if tri.split {
					size += 2
				}
			}
		}
		n += size
		if !o.AnalyticBound {
			continue
		}
		lo, hi := triangles[0].uv[0], triangles[0].uv[0]
		for _, tri := range triangles {
			for _, side := range tri.side {
				if side.edge >= 0 && side.use.Curve != nil {
					return mesh.Mesh{}, parametric.ErrDeflectionUnsupported
				}
			}
			for _, v := range tri.uv {
				lo.X, lo.Y = min(lo.X, v.X), min(lo.Y, v.Y)
				hi.X, hi.Y = max(hi.X, v.X), max(hi.Y, v.Y)
			}
		}
		d, e := parametric.SurfaceDeflectionBoundContext(ctx, s.faces[i].Surface, parametric.Interval{Min: lo.X, Max: hi.X}, parametric.Interval{Min: lo.Y, Max: hi.Y})
		if e != nil {
			return mesh.Mesh{}, e
		}
		maximum = max(maximum, d)
	}
	for range s.minimumLevel {
		if n > o.MaxTriangles/4 {
			return mesh.Mesh{}, parametric.ErrBudget
		}
		n *= 4
	}
	for level, k := s.minimumLevel, 1<<s.minimumLevel; k <= o.MaxSubdivisions && n <= o.MaxTriangles; level, k = level+1, k*2 {
		bound := math.Ldexp(maximum, -2*level)
		if maximum > 0 {
			bound = math.Nextafter(math.Nextafter(bound, math.Inf(1))+s.tolerance, math.Inf(1))
		} else {
			bound = s.tolerance
		}
		if !o.AnalyticBound || bound <= o.Deflection {
			m, err := s.refine(ctx, level, o.Deflection, o.Angle)
			if err != errRefine && err != errTrimFold {
				if err == nil && len(m.Triangles) > o.MaxTriangles {
					return mesh.Mesh{}, parametric.ErrBudget
				}
				return m, err
			}
		}
		if n > o.MaxTriangles/4 {
			break
		}
		n *= 4
	}
	return mesh.Mesh{}, parametric.ErrBudget
}

func normalizeTrimmedTessellationOptions(ctx context.Context, o parametric.TessellationOptions) (parametric.TessellationOptions, error) {
	if ctx == nil || !finite(o.Deflection) || o.Deflection <= 0 || !finite(o.Angle) || o.Angle < 0 || o.Angle > math.Pi ||
		o.MaxSubdivisions < 0 || o.MaxSubdivisions > 1024 || o.MaxTriangles < 0 {
		return parametric.TessellationOptions{}, fmt.Errorf("trimmed: invalid options")
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 128
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 1000000
	}
	return o, nil
}

var errRefine = fmt.Errorf("trimmed: refinement required")
var errTrimFold = fmt.Errorf("trimmed: curved trim refinement folds or degenerates in UV")

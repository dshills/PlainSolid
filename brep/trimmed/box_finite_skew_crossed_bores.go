package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// FiniteSkewCrossedBoresResult preserves the original stock/wall faces.
// Floors are appended in input X, then input Z order; -1 means through.
type FiniteSkewCrossedBoresResult struct {
	BoxCrossedBoresResult
	XFloorFace, ZFloorFace int
}

// BoxFiniteSkewCrossedBores stops either unequal perpendicular bore beyond the
// entire intersection. Zero depth means through. Positive depths run inward
// from the lower cap, or from the upper cap when the corresponding flag is set.
// The axial stop is proved outside the actual transverse intersection.
func BoxFiniteSkewCrossedBores(ctx context.Context, frame geom.Frame, dims, crossing geom.Vec3, xRadius, zRadius, offset, xDepth, zDepth float64, xOpposite, zOpposite bool, tolerance float64) (FiniteSkewCrossedBoresResult, error) {
	return boxFiniteRadicalCrossedBores(ctx, frame, dims, crossing, xRadius, zRadius, offset,
		xDepth, zDepth, xOpposite, zOpposite, tolerance, BoxSkewCrossedBores)
}

type radicalCrossedBoreBuilder func(context.Context, geom.Frame, geom.Vec3, geom.Vec3, float64, float64, float64, float64) (BoxCrossedBoresResult, error)

func boxFiniteRadicalCrossedBores(ctx context.Context, frame geom.Frame, dims, crossing geom.Vec3, xRadius, zRadius, offset, xDepth, zDepth float64, xOpposite, zOpposite bool, tolerance float64, build radicalCrossedBoreBuilder) (FiniteSkewCrossedBoresResult, error) {
	fail := func(e error) (FiniteSkewCrossedBoresResult, error) { return FiniteSkewCrossedBoresResult{}, e }
	if ctx == nil || !finite(xDepth) || !finite(zDepth) || xDepth < 0 || zDepth < 0 {
		return fail(geom.ErrInvalid)
	}
	original, e := build(ctx, frame, dims, crossing, xRadius, zRadius, offset, tolerance)
	if e != nil {
		return fail(e)
	}
	out := FiniteSkewCrossedBoresResult{original, -1, -1}
	for i, depth := range []float64{xDepth, zDepth} {
		if depth == 0 {
			continue
		}
		if e := ctx.Err(); e != nil {
			return fail(e)
		}
		size, c, other, radius, axis, op := dims.X, crossing.X, zRadius, xRadius, frame.XAxis(), xOpposite
		if i == 1 {
			size, c, other, radius, axis, op = dims.Z, crossing.Z, xRadius, zRadius, frame.ZAxis(), zOpposite
		}
		margin := max(8*tolerance, 1024*0x1p-52*max(size, math.Abs(c), other, math.Abs(frame.Origin().X), math.Abs(frame.Origin().Y), math.Abs(frame.Origin().Z)))
		floor := depth
		if op {
			floor = size - depth
		}
		if depth >= size-margin {
			return fail(geom.ErrDomain)
		}
		afterFullRadius := !op && floor > c+other+margin || op && floor < c-other-margin
		if !afterFullRadius && !floorBeyondSkewIntersection(c, floor, margin, radius, other, offset, op) {
			return fail(geom.ErrDomain)
		}
		capNormal := axis
		if op {
			capNormal = axis.Scale(-1)
		}
		cap := -1
		for j, f := range out.Solid.faces[:6] {
			s, ok := f.Surface.(geom.PlaneSurface)
			if ok && s.Frame().ZAxis() == capNormal {
				cap = j
				break
			}
		}
		if cap < 0 {
			return fail(geom.ErrDomain)
		}
		shift := capNormal.Scale(-(size - depth))
		var floorID int
		out.Solid, floorID, e = stopCrossedBoreCap(out.Solid, cap, shift)
		if e != nil {
			return fail(e)
		}
		if i == 0 {
			out.XFloorFace = floorID
		} else {
			out.ZFloorFace = floorID
		}
		out.RemovedVolume -= math.Pi * radius * radius * (size - depth)
		out.SurfaceAreaDelta += -2*math.Pi*radius*(size-depth) + 2*math.Pi*radius*radius
	}
	return out, nil
}

// The other cylinder reaches its largest axial displacement at the
// transverse ordinate nearest its center that still belongs to this bore.
// Compare squared distances exactly, keeping the floor clearance in the
// same represented arithmetic as the input.
func floorBeyondSkewIntersection(c, floor, margin, ownRadius, otherRadius, offset float64, opposite bool) bool {
	advance := new(big.Rat).Sub(ratInset(floor), ratInset(c))
	if opposite {
		advance.Neg(advance)
	}
	advance.Sub(advance, ratInset(margin))
	side := new(big.Rat).Sub(ratInset(math.Abs(offset)), ratInset(ownRadius))
	if advance.Sign() <= 0 || side.Sign() <= 0 {
		return false
	}
	squared := advance.Mul(advance, advance)
	squared.Add(squared, side.Mul(side, side))
	r := ratInset(otherRadius)
	return squared.Cmp(r.Mul(r, r)) > 0
}

// stopCrossedBoreCap acts only on a freshly constructed, fully qualified
// crossed-bore cap beyond its intersection. It retains every curve
// exactly; only circular rim supports and straight axial segments are moved.
func stopCrossedBoreCap(s Solid, cap int, shift geom.Vec3) (Solid, int, error) {
	fail := func(e error) (Solid, int, error) { return Solid{}, 0, e }
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	if cap < 0 || cap >= 6 || len(faces[cap].Holes) != 1 || (len(faces[cap].Holes[0]) != 4 && len(faces[cap].Holes[0]) != 6) {
		return fail(geom.ErrDomain)
	}
	plane, ok := faces[cap].Surface.(geom.PlaneSurface)
	if !ok {
		return fail(geom.ErrDomain)
	}
	pf := plane.Frame()
	ff, e := geom.NewFrame(pf.Origin().Add(shift), pf.XAxis(), pf.ZAxis())
	if e != nil {
		return fail(e)
	}
	fs, e := geom.NewPlaneSurface(ff)
	if e != nil {
		return fail(e)
	}
	ring := faces[cap].Holes[0]
	rim, moved := map[int]bool{}, map[int]bool{}
	for _, u := range ring {
		if _, ok := u.Curve.(unionArcUV); !ok {
			return fail(geom.ErrDomain)
		}
		rim[u.Edge] = true
		moved[edges[u.Edge].Start] = true
		moved[edges[u.Edge].End] = true
	}
	for id := range moved {
		vertices[id] = vertices[id].Add(shift)
	}
	for id, edge := range edges {
		if !moved[edge.Start] && !moved[edge.End] {
			continue
		}
		c, ok := edge.Curve.(surfaceTrimCurve)
		if !ok || edge.Range != (parametric.Interval{Max: 1}) {
			return fail(geom.ErrDomain)
		}
		if rim[id] {
			c.surface = fs
		} else {
			cylinder, ok := c.surface.(geom.CylinderSurface)
			if !ok || c.curve != nil {
				return fail(geom.ErrDomain)
			}
			f, _ := cylinder.Parameters()
			delta := shift.Dot(f.ZAxis())
			if moved[edge.Start] {
				c.a.Y += delta
			}
			if moved[edge.End] {
				c.b.Y += delta
			}
		}
		edges[id].Curve = c
	}
	for i := range faces {
		if i == cap {
			continue
		}
		for _, loop := range append([][]Use{faces[i].Outer}, faces[i].Holes...) {
			for j := range loop {
				u := &loop[j]
				edge := edges[u.Edge]
				start, end := edge.Start, edge.End
				if u.Reversed {
					start, end = end, start
				}
				if !moved[start] && !moved[end] {
					continue
				}
				cylinder, ok := faces[i].Surface.(geom.CylinderSurface)
				if !ok || u.Curve != nil {
					return fail(geom.ErrDomain)
				}
				f, _ := cylinder.Parameters()
				delta := shift.Dot(f.ZAxis())
				if moved[start] {
					u.UVStart.Y += delta
				}
				if moved[end] {
					u.UVEnd.Y += delta
				}
			}
		}
	}
	faces[cap].Holes = nil
	floorID := len(faces)
	faces = append(faces, Face{Surface: fs, Reversed: true, Outer: reverseUnionLoop(ring)})
	out, e := NewSolid(vertices, edges, faces, s.Tolerance())
	return out, floorID, e
}

// TessellateFiniteSkewCrossedBores meshes retained unequal crossing trims and floors.
func (s Solid) TessellateFiniteSkewCrossedBores(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
	if ctx == nil || !finite(o.Deflection) || o.Deflection <= 0 || !finite(o.Angle) || o.Angle < 0 || o.Angle > math.Pi || o.MaxSubdivisions < 0 || o.MaxSubdivisions > 1024 || o.MaxTriangles < 0 {
		return mesh.Mesh{}, geom.ErrInvalid
	}
	if o.AnalyticBound {
		return mesh.Mesh{}, parametric.ErrDeflectionUnsupported
	}
	if e := ctx.Err(); e != nil {
		return mesh.Mesh{}, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 128
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 1000000
	}
	if len(s.faces) < 16 || len(s.faces) > 18 {
		return mesh.Mesh{}, geom.ErrDomain
	}
	radical := false
	for _, f := range s.faces {
		base, _ := parametric.SurfaceMapping(f.Surface)
		switch base.(type) {
		case geom.PlaneSurface, geom.CylinderSurface:
		default:
			return mesh.Mesh{}, geom.ErrDomain
		}
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				switch u.Curve.(type) {
				case unequalBoreTrim:
					radical = true
				case nil, EllipseUV, unionArcUV:
				default:
					return mesh.Mesh{}, geom.ErrDomain
				}
			}
		}
	}
	if !radical {
		return mesh.Mesh{}, geom.ErrDomain
	}
	return s.tessellateAnalyticAdaptive(ctx, o)
}

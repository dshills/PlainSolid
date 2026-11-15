package trimmed

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"slices"

	"plainsolid/brep"
	"plainsolid/geom"
)

// ConvexPrismFilletHistory maps each original cap and wall to its trimmed
// output face. FilletFaces identifies the analytic cylindrical pieces.
type ConvexPrismFilletHistory struct {
	SourceEdge  int
	SourceFaces []int
	FilletFaces []int
}

// ConvexPrismEdgeRadius selects a native vertical stock edge and radius.
type ConvexPrismEdgeRadius struct {
	Edge   int
	Radius float64
}

// ConvexPrismEdgeFillet identifies the cylindrical charts of one transition.
type ConvexPrismEdgeFillet struct {
	SourceEdge int
	Faces      []int
}

// ConvexPrismFilletsHistory maps all original boundary faces and lists the
// generated transitions in counterclockwise profile order.
type ConvexPrismFilletsHistory struct {
	SourceFaces []int
	Fillets     []ConvexPrismEdgeFillet
}

// FilletConvexPrismEdge rounds one vertical edge of an outward, convex
// right prism with horizontal caps. The selected edge is a native stock
// edge index. The two original supporting lines must admit exact rational
// unit directions, and the circle center and tangencies must be exactly
// representable. Other walls must have strict clearance from the circle.
//
// The cylindrical fillet is analytic. Circular trim evaluation uses the
// bounded endpoint tolerance of ExtrudeArcLoop.
func FilletConvexPrismEdge(ctx context.Context, source brep.Solid, edge int, radius, tolerance float64) (Solid, ConvexPrismFilletHistory, error) {
	out, h, e := FilletConvexPrismEdges(ctx, source, []ConvexPrismEdgeRadius{{edge, radius}}, tolerance)
	if e != nil {
		return Solid{}, ConvexPrismFilletHistory{}, e
	}
	return out, ConvexPrismFilletHistory{SourceEdge: edge, SourceFaces: h.SourceFaces, FilletFaces: h.Fillets[0].Faces}, nil
}

// FilletConvexPrismEdges rounds multiple selected vertical edges in one
// operation. Each adjacent pair must leave a positive segment of its
// original wall between the two exact tangent points. All requested
// transitions use the proof and analytic construction of the single-edge
// operation.
func FilletConvexPrismEdges(ctx context.Context, source brep.Solid, requests []ConvexPrismEdgeRadius, tolerance float64) (Solid, ConvexPrismFilletsHistory, error) {
	if !finite(tolerance) || tolerance <= 0 || len(requests) < 1 || len(requests) > 62 {
		return Solid{}, ConvexPrismFilletsHistory{}, geom.ErrInvalid
	}
	for _, q := range requests {
		if q.Edge < 0 || !finite(q.Radius) || q.Radius <= 8*tolerance {
			return Solid{}, ConvexPrismFilletsHistory{}, geom.ErrInvalid
		}
	}
	if _, _, e := source.OffsetConvex(ctx, 0, brep.ConvexOffsetOptions{}); e != nil {
		return Solid{}, ConvexPrismFilletsHistory{}, e
	}
	prism, e := convexFilletPrismProfile(source)
	if e != nil {
		return Solid{}, ConvexPrismFilletsHistory{}, e
	}
	arcs := make([]ExtrusionSegment, len(prism.profile))
	for _, q := range requests {
		if q.Edge >= len(prism.edges) {
			return Solid{}, ConvexPrismFilletsHistory{}, geom.ErrInvalid
		}
		corner := prism.edges[q.Edge]
		if corner < 0 || arcs[corner].Radius != 0 {
			return Solid{}, ConvexPrismFilletsHistory{}, geom.ErrDomain
		}
		arcs[corner], e = exactConvexPrismFilletArc(prism.profile, corner, q.Radius)
		if e != nil {
			return Solid{}, ConvexPrismFilletsHistory{}, e
		}
	}
	segments := make([]ExtrusionSegment, 0, len(prism.profile)+len(requests))
	var arcSources []int
	for i, a := range prism.profile {
		j := (i + 1) % len(prism.profile)
		b := prism.profile[j]
		if arcs[i].Radius != 0 {
			a = arcs[i].End
		}
		if arcs[j].Radius != 0 {
			b = arcs[j].Start
		}
		originalDirection := exactFilletUV(prism.profile[j]).sub(exactFilletUV(prism.profile[i]))
		if exactFilletUV(b).sub(exactFilletUV(a)).dot(originalDirection).Sign() <= 0 {
			return Solid{}, ConvexPrismFilletsHistory{}, fmt.Errorf("trimmed: adjacent fillets meet on original wall %d: %w", prism.walls[i], geom.ErrDomain)
		}
		segments = append(segments, ExtrusionSegment{Start: a, End: b})
		if arcs[j].Radius != 0 {
			segments = append(segments, arcs[j])
			arcSources = append(arcSources, prism.vertical[j])
		}
	}
	frame, e := geom.NewFrame(geom.Vec3{Z: prism.z[0]}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return Solid{}, ConvexPrismFilletsHistory{}, e
	}
	height, e := geom.RepresentedExactScalar(new(big.Rat).Sub(ratInset(prism.z[1]), ratInset(prism.z[0])))
	if e != nil {
		return Solid{}, ConvexPrismFilletsHistory{}, e
	}
	result, e := ExtrudeArcLoop(ctx, frame, segments, height, tolerance)
	if e != nil {
		return Solid{}, ConvexPrismFilletsHistory{}, e
	}
	h := ConvexPrismFilletsHistory{SourceFaces: make([]int, len(prism.walls)+2)}
	for i, cap := range prism.caps {
		h.SourceFaces[cap] = i
	}
	wall := 0
	inFillet := false
	for i, face := range result.Solid.faces[2:] {
		if _, ok := face.Surface.(geom.CylinderSurface); ok {
			if !inFillet {
				if len(h.Fillets) == len(arcSources) {
					return Solid{}, ConvexPrismFilletsHistory{}, geom.ErrConvergence
				}
				h.Fillets = append(h.Fillets, ConvexPrismEdgeFillet{SourceEdge: arcSources[len(h.Fillets)]})
			}
			q := &h.Fillets[len(h.Fillets)-1]
			q.Faces = append(q.Faces, i+2)
			inFillet = true
			continue
		}
		inFillet = false
		if wall == len(prism.walls) {
			return Solid{}, ConvexPrismFilletsHistory{}, geom.ErrConvergence
		}
		h.SourceFaces[prism.walls[wall]] = i + 2
		wall++
	}
	if wall != len(prism.walls) || len(h.Fillets) != len(requests) {
		return Solid{}, ConvexPrismFilletsHistory{}, geom.ErrConvergence
	}
	return result.Solid, h, nil
}

type filletPrismProfile struct {
	profile  []geom.Vec2
	walls    []int
	edges    []int
	vertical []int
	z        [2]float64
	caps     [2]int
}

func convexFilletPrismProfile(s brep.Solid) (filletPrismProfile, error) {
	r := filletPrismProfile{caps: [2]int{-1, -1}}
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	bounds, e := s.Bounds()
	if e != nil || bounds.Min.Z >= bounds.Max.Z {
		return r, geom.ErrDomain
	}
	r.z = [2]float64{bounds.Min.Z, bounds.Max.Z}
	xy := func(v geom.Vec3) geom.Vec2 { return geom.Vec2{X: v.X, Y: v.Y} }
	levels := map[geom.Vec2]int{}
	for _, p := range vertices {
		switch p.Z {
		case r.z[0]:
			levels[xy(p)] |= 1
		case r.z[1]:
			levels[xy(p)] |= 2
		default:
			return r, geom.ErrDomain
		}
	}
	for _, paired := range levels {
		if paired != 3 {
			return r, geom.ErrDomain
		}
	}
	var lower []int
	walls := make(map[[2]int]int, len(levels))
	pair := func(i, j int) [2]int { return [2]int{min(i, j), max(i, j)} }
	for i, f := range faces {
		var bottom []int
		for _, u := range f.Boundary {
			a := edges[u.Edge].Start
			if u.Reversed {
				a = edges[u.Edge].End
			}
			if vertices[a].Z == r.z[0] {
				bottom = append(bottom, a)
			}
		}
		switch {
		case len(bottom) == 0 && r.caps[1] < 0 && len(f.Boundary) == len(levels):
			r.caps[1] = i
		case len(bottom) == len(f.Boundary) && r.caps[0] < 0 && len(bottom) == len(levels):
			r.caps[0], lower = i, bottom
		case len(bottom) == 2 && len(f.Boundary) == 4:
			walls[pair(bottom[0], bottom[1])] = i
		default:
			return r, geom.ErrDomain
		}
	}
	if r.caps[0] < 0 || r.caps[1] < 0 || len(walls) != len(lower) || len(faces) != len(lower)+2 {
		return r, geom.ErrDomain
	}
	// The authenticated bottom cap faces down. Its reversed wire is
	// counterclockwise in the fixed XY chart used by the new prism.
	slices.Reverse(lower)
	profileIndices := make(map[geom.Vec2]int, len(lower))
	for i, v := range lower {
		q := xy(vertices[v])
		r.profile = append(r.profile, q)
		profileIndices[q] = i
		w, ok := walls[pair(v, lower[(i+1)%len(lower)])]
		if !ok {
			return filletPrismProfile{}, geom.ErrDomain
		}
		r.walls = append(r.walls, w)
	}
	r.edges = make([]int, len(edges))
	r.vertical = make([]int, len(r.profile))
	for i := range r.vertical {
		r.vertical[i] = -1
	}
	for i, edge := range edges {
		a, b := vertices[edge.Start], vertices[edge.End]
		r.edges[i] = -1
		if corner, ok := profileIndices[xy(a)]; ok && xy(a) == xy(b) && a.Z != b.Z && r.vertical[corner] < 0 {
			r.edges[i], r.vertical[corner] = corner, i
		}
	}
	return r, nil
}

type filletUV [2]*big.Rat

func exactFilletUV(p geom.Vec2) filletUV { return filletUV{ratInset(p.X), ratInset(p.Y)} }
func (p filletUV) sub(q filletUV) filletUV {
	return filletUV{new(big.Rat).Sub(p[0], q[0]), new(big.Rat).Sub(p[1], q[1])}
}
func (p filletUV) add(q filletUV) filletUV {
	return filletUV{new(big.Rat).Add(p[0], q[0]), new(big.Rat).Add(p[1], q[1])}
}
func (p filletUV) scale(x *big.Rat) filletUV {
	return filletUV{new(big.Rat).Mul(p[0], x), new(big.Rat).Mul(p[1], x)}
}
func (p filletUV) dot(q filletUV) *big.Rat {
	return new(big.Rat).Add(new(big.Rat).Mul(p[0], q[0]), new(big.Rat).Mul(p[1], q[1]))
}
func (p filletUV) cross(q filletUV) *big.Rat {
	return new(big.Rat).Sub(new(big.Rat).Mul(p[0], q[1]), new(big.Rat).Mul(p[1], q[0]))
}
func (p filletUV) represented() (geom.Vec2, error) {
	x, e := geom.RepresentedExactScalar(p[0])
	if e != nil {
		return geom.Vec2{}, e
	}
	y, e := geom.RepresentedExactScalar(p[1])
	return geom.Vec2{X: x, Y: y}, e
}

func exactFilletDirection(p filletUV) (filletUV, *big.Rat, error) {
	q := p.dot(p)
	n, d := new(big.Int).Sqrt(q.Num()), new(big.Int).Sqrt(q.Denom())
	if n.Sign() == 0 || new(big.Int).Mul(n, n).Cmp(q.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(q.Denom()) != 0 {
		return filletUV{}, nil, geom.ErrUnrepresentable
	}
	length := new(big.Rat).SetFrac(n, d)
	return p.scale(new(big.Rat).Inv(length)), length, nil
}

func exactConvexPrismFilletArc(profile []geom.Vec2, corner int, radius float64) (ExtrusionSegment, error) {
	var arc ExtrusionSegment
	count := len(profile)
	before, after := (corner+count-1)%count, (corner+1)%count
	v := exactFilletUV(profile[corner])
	a, la, e := exactFilletDirection(v.sub(exactFilletUV(profile[before])))
	if e != nil {
		return arc, e
	}
	b, lb, e := exactFilletDirection(exactFilletUV(profile[after]).sub(v))
	if e != nil {
		return arc, e
	}
	sin, cos := a.cross(b), a.dot(b)
	if sin.Sign() <= 0 {
		return arc, geom.ErrDomain
	}
	r := ratInset(radius)
	setback := new(big.Rat).Mul(r, new(big.Rat).Quo(sin, new(big.Rat).Add(big.NewRat(1, 1), cos)))
	if setback.Cmp(la) >= 0 || setback.Cmp(lb) >= 0 {
		return arc, geom.ErrDomain
	}
	start, stop := v.sub(a.scale(setback)), v.add(b.scale(setback))
	center := start.add(filletUV{new(big.Rat).Neg(a[1]), a[0]}.scale(r))
	r2 := new(big.Rat).Mul(r, r)
	for i, p := range profile {
		x := exactFilletUV(p)
		side := exactFilletUV(profile[(i+1)%count]).sub(x)
		signed := side.cross(center.sub(x))
		comparison := new(big.Rat).Mul(signed, signed).Cmp(new(big.Rat).Mul(r2, side.dot(side)))
		tangent := i == before || i == corner
		if signed.Sign() <= 0 || tangent && comparison != 0 || !tangent && comparison <= 0 {
			return arc, fmt.Errorf("trimmed: original wall %d lacks circular fillet clearance: %w", i, geom.ErrDomain)
		}
	}
	if arc.Start, e = start.represented(); e != nil {
		return arc, e
	}
	if arc.End, e = stop.represented(); e != nil {
		return arc, e
	}
	if arc.Center, e = center.represented(); e != nil {
		return arc, e
	}
	x, _ := a[0].Float64()
	y, _ := a[1].Float64()
	s, _ := sin.Float64()
	c, _ := cos.Float64()
	arc.Radius, arc.StartAngle, arc.Sweep = radius, math.Atan2(-x, y), math.Atan2(s, c)
	return arc, nil
}

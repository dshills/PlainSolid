package brep

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

// ErrTerminationUncovered means the projected end profile is not contained in
// the closed material region of the selected face.
var ErrTerminationUncovered = errors.New("brep: termination face does not cover the projected profile")

// PrismToFace sweeps a polygon with optional polygon holes along positive
// multiples of direction, stopping at the selected finite polygonal face.
// The two caps may be oblique to each other. Either winding is accepted for
// each input loop. Contact with the target boundary is allowed. The selected
// face is only a termination reference; this operation does not modify target
// or check for earlier collisions with other faces of target.
//
// Coplanarity, ray intersections and finite coverage use exact rational
// predicates on the represented inputs. Projected vertices must be exactly
// representable as float64; the existing planar solid resolution also applies.
// Both profiles have a limit of MaxHoledFaceVertices total boundary vertices.
// A rational coverage sum exceeding 16384 coefficient bits fails with ErrConvergence.
func PrismToFace(ctx context.Context, outer []geom.Vec3, holes [][]geom.Vec3, direction geom.Vec3, target Solid, face int) (Solid, error) {
	if ctx == nil || !direction.IsFinite() || direction == (geom.Vec3{}) || face < 0 || face >= len(target.faces) {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	if len(holes) >= MaxHoledFaceVertices || len(target.faces[face].Holes) >= MaxHoledFaceVertices {
		return Solid{}, geom.ErrConvergence
	}
	input := append([][]geom.Vec3{outer}, holes...)
	targetIDs := make([][]int, 0, 1+len(target.faces[face].Holes))
	targetRings := make([][]geom.Vec3, 0, cap(targetIDs))
	count := 0
	for _, wire := range faceLoops(target.faces[face]) {
		if len(wire) > MaxHoledFaceVertices-count {
			return Solid{}, geom.ErrConvergence
		}
		count += len(wire)
		ids, e := target.wireIndices(wire)
		if e != nil {
			return Solid{}, e
		}
		ring := make([]geom.Vec3, len(ids))
		for i, id := range ids {
			ring[i] = target.vertices[id]
		}
		targetIDs, targetRings = append(targetIDs, ids), append(targetRings, ring)
	}
	sourcePlane, e := terminationPlane(input)
	if e != nil {
		return Solid{}, fmt.Errorf("brep: source profile: %w", e)
	}
	targetPlane, e := terminationPlane(targetRings)
	if e != nil {
		return Solid{}, fmt.Errorf("brep: termination face: %w", e)
	}
	d := terminationVec(direction)
	sourceSide := sourcePlane.direction(d)
	targetSide := targetPlane.direction(d)
	if sourceSide.Sign() == 0 || targetSide.Sign() == 0 {
		return Solid{}, fmt.Errorf("brep: sweep direction is parallel to a cap: %w", geom.ErrDomain)
	}
	// Orient the source outer loop so its normal points toward the end cap.
	// The cyclic projection axes preserve the sign of the normal's dropped component.
	winding := sourceSide.Sign() * sourcePlane.eq[sourcePlane.drop].Sign()
	var bottom, top [][]int
	var sides [][][]int
	var baseVertices, endVertices []geom.Vec3
	for r, ring := range input {
		area := terminationLoopArea(ring, sourcePlane.drop)
		if area == 0 {
			return Solid{}, geom.ErrDegenerate
		}
		want := winding
		if r > 0 {
			want = -want
		}
		ids := make([]int, len(ring))
		for j := range ring {
			i := j
			if area != want {
				i = len(ring) - 1 - j
			}
			p := terminationVec(ring[i])
			t := targetPlane.value(p)
			t.Quo(t.Neg(t), targetSide)
			if t.Sign() <= 0 {
				return Solid{}, fmt.Errorf("brep: termination must be strictly ahead of every source vertex: %w", geom.ErrDomain)
			}
			for k := range p {
				p[k].Add(p[k], new(big.Rat).Mul(t, d[k]))
			}
			q, e := geom.RepresentedExactPoint(p)
			if e != nil {
				return Solid{}, fmt.Errorf("brep: projected vertex: %w", e)
			}
			ids[j] = len(baseVertices)
			baseVertices = append(baseVertices, ring[i])
			endVertices = append(endVertices, q)
		}
		top = append(top, ids)
		reversed := append([]int(nil), ids...)
		reverseIDs(reversed)
		bottom = append(bottom, reversed)
	}
	n := len(baseVertices)
	for _, ring := range top {
		for i, a := range ring {
			b := ring[(i+1)%len(ring)]
			sides = append(sides, [][]int{{a, b, b + n, a + n}})
		}
		for i := range ring {
			ring[i] += n
		}
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	candidate, e := fromPolygonFaces(append(baseVertices, endVertices...), append([][][]int{bottom, top}, sides...))
	if e != nil {
		return Solid{}, e
	}
	// Certify each triangulation in represented planar coordinates before
	// treating its exact triangle areas as a partition of the cap region.
	if _, e := terminationTriangles(ctx, candidate, 0, bottom, sourcePlane.drop); e != nil {
		return Solid{}, e
	}
	ends, e := terminationTriangles(ctx, candidate, 1, top, targetPlane.drop)
	if e != nil {
		return Solid{}, e
	}
	stops, e := terminationTriangles(ctx, target, face, targetIDs, targetPlane.drop)
	if e != nil {
		return Solid{}, e
	}
	for _, a := range ends {
		covered := new(big.Rat)
		for _, b := range stops {
			if e := ctx.Err(); e != nil {
				return Solid{}, e
			}
			if a.overlaps(b) {
				covered.Add(covered, terminationIntersectionArea(a.points, b.points))
				if covered.Num().BitLen() > 16384 || covered.Denom().BitLen() > 16384 {
					return Solid{}, geom.ErrConvergence
				}
			}
		}
		if covered.Cmp(terminationOrient(a.points[0], a.points[1], a.points[2])) != 0 {
			return Solid{}, ErrTerminationUncovered
		}
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	return candidate, nil
}

type sweepStopPlane struct {
	eq   [4]*big.Rat
	drop int
}

func terminationVec(p geom.Vec3) [3]*big.Rat {
	return [3]*big.Rat{new(big.Rat).SetFloat64(p.X), new(big.Rat).SetFloat64(p.Y), new(big.Rat).SetFloat64(p.Z)}
}

func (p sweepStopPlane) direction(v [3]*big.Rat) *big.Rat {
	out := new(big.Rat)
	for i, x := range v {
		out.Add(out, new(big.Rat).Mul(x, p.eq[i]))
	}
	return out
}

func (p sweepStopPlane) value(v [3]*big.Rat) *big.Rat {
	value := p.direction(v)
	return value.Add(value, p.eq[3])
}

func terminationPlane(rings [][]geom.Vec3) (sweepStopPlane, error) {
	count := 0
	for _, ring := range rings {
		if len(ring) < 3 {
			return sweepStopPlane{}, geom.ErrInvalid
		}
		if len(ring) > MaxHoledFaceVertices-count {
			return sweepStopPlane{}, geom.ErrConvergence
		}
		count += len(ring)
		for _, p := range ring {
			if !p.IsFinite() {
				return sweepStopPlane{}, geom.ErrInvalid
			}
		}
	}
	outer := rings[0]
	var eq [4]*big.Rat
	for i := 2; i < len(outer); i++ {
		plane, e := geom.NewRawSupportPlane(outer[0], outer[1], outer[i], geom.SurfacePatchOptions{})
		if errors.Is(e, geom.ErrDegenerate) {
			continue
		}
		if e != nil {
			return sweepStopPlane{}, e
		}
		eq = plane.Coefficients()
		break
	}
	if eq[0] == nil {
		return sweepStopPlane{}, geom.ErrDegenerate
	}
	out := sweepStopPlane{eq: eq}
	for i := 1; i < 3; i++ {
		if new(big.Rat).Abs(eq[i]).Cmp(new(big.Rat).Abs(eq[out.drop])) > 0 {
			out.drop = i
		}
	}
	for _, ring := range rings {
		for _, p := range ring {
			if out.value(terminationVec(p)).Sign() != 0 {
				return sweepStopPlane{}, fmt.Errorf("profile vertices are not exactly coplanar: %w", geom.ErrDomain)
			}
		}
	}
	return out, nil
}

func terminationXY(p geom.Vec3, drop int) geom.Vec2 {
	v := [3]float64{p.X, p.Y, p.Z}
	return geom.Vec2{X: v[(drop+1)%3], Y: v[(drop+2)%3]}
}

type terminationPoint [2]*big.Rat

func terminationRat(p geom.Vec2) terminationPoint {
	return terminationPoint{new(big.Rat).SetFloat64(p.X), new(big.Rat).SetFloat64(p.Y)}
}

func terminationOrient(a, b, c terminationPoint) *big.Rat {
	return new(big.Rat).Sub(
		new(big.Rat).Mul(new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(c[1], a[1])),
		new(big.Rat).Mul(new(big.Rat).Sub(b[1], a[1]), new(big.Rat).Sub(c[0], a[0])),
	)
}

func terminationSign(a, b, c geom.Vec2) int {
	sign, _ := geom.Orient2D(a, b, c) // the represented cap coordinates are finite
	return sign
}

// Points passed to terminationWithin have already been proved collinear.
func terminationWithin(p, a, b geom.Vec2) bool {
	return (a.X <= p.X && p.X <= b.X || b.X <= p.X && p.X <= a.X) &&
		(a.Y <= p.Y && p.Y <= b.Y || b.Y <= p.Y && p.Y <= a.Y)
}

func terminationSegmentsMeet(a, b, c, d geom.Vec2) bool {
	abC, abD := terminationSign(a, b, c), terminationSign(a, b, d)
	cdA, cdB := terminationSign(c, d, a), terminationSign(c, d, b)
	return abC*abD < 0 && cdA*cdB < 0 ||
		abC == 0 && terminationWithin(c, a, b) || abD == 0 && terminationWithin(d, a, b) ||
		cdA == 0 && terminationWithin(a, c, d) || cdB == 0 && terminationWithin(b, c, d)
}

// Returns one inside, minus one on the boundary, and zero outside.
func terminationLocate(p geom.Vec2, ring []int, points []geom.Vec2) int {
	winding := 0
	for i, id := range ring {
		a, b := points[id], points[ring[(i+1)%len(ring)]]
		sign := terminationSign(a, b, p)
		if sign == 0 && terminationWithin(p, a, b) {
			return -1
		}
		if a.Y <= p.Y && b.Y > p.Y && sign > 0 {
			winding++
		} else if a.Y > p.Y && b.Y <= p.Y && sign < 0 {
			winding--
		}
	}
	if winding != 0 {
		return 1
	}
	return 0
}

func terminationLoopArea(ring []geom.Vec3, drop int) int {
	origin := terminationRat(terminationXY(ring[0], drop))
	sum := new(big.Rat)
	for i := 1; i+1 < len(ring); i++ {
		sum.Add(sum, terminationOrient(origin, terminationRat(terminationXY(ring[i], drop)), terminationRat(terminationXY(ring[i+1], drop))))
	}
	return sum.Sign()
}

type terminationTriangle struct {
	points   [3]terminationPoint
	min, max [2]float64
}

func (a terminationTriangle) overlaps(b terminationTriangle) bool {
	return a.min[0] < b.max[0] && b.min[0] < a.max[0] && a.min[1] < b.max[1] && b.min[1] < a.max[1]
}

// A consistently oriented triangle chain whose boundary is the simple outer
// loop minus its holes has multiplicity one in material and zero outside.
// Checking the boundary chain and orientation avoids trusting a floating
// area sum or assuming that the existing tessellator got the exact signs.
func terminationTriangles(ctx context.Context, s Solid, face int, rings [][]int, drop int) ([]terminationTriangle, error) {
	boundary := map[segment2]int{}
	add := func(a, b int) {
		key, sign := edgeKey(a, b), 1
		if a > b {
			sign = -1
		}
		boundary[key] += sign
		if boundary[key] == 0 {
			delete(boundary, key)
		}
	}
	var points []geom.Vec2
	var loopIDs [][]int
	projected := make(map[int]geom.Vec2)
	orientation := 0
	for i, ring := range rings {
		ids := make([]int, len(ring))
		world := make([]geom.Vec3, len(ring))
		for j, a := range ring {
			p := terminationXY(s.vertices[a], drop)
			projected[a] = p
			ids[j] = len(points)
			points = append(points, p)
			world[j] = s.vertices[a]
			add(ring[(j+1)%len(ring)], a)
		}
		sign := terminationLoopArea(world, drop)
		if i == 0 {
			orientation = sign
		}
		if sign == 0 || i > 0 && sign == orientation {
			return nil, geom.ErrDegenerate
		}
		loopIDs = append(loopIDs, ids)
	}
	var segments []segment2
	for i, ring := range loopIDs {
		for j, a := range ring {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			b := ring[(j+1)%len(ring)]
			if points[a] == points[b] || terminationSign(points[ring[(j+len(ring)-1)%len(ring)]], points[a], points[b]) == 0 {
				return nil, geom.ErrDegenerate
			}
			for _, edge := range segments {
				if edge.a != a && edge.b != a && edge.a != b && edge.b != b &&
					terminationSegmentsMeet(points[a], points[b], points[edge.a], points[edge.b]) {
					return nil, geom.ErrInvalid
				}
			}
			segments = append(segments, segment2{a, b})
		}
		if i > 0 {
			if terminationLocate(points[ring[0]], loopIDs[0], points) != 1 {
				return nil, geom.ErrInvalid
			}
			for _, other := range loopIDs[1:i] {
				if terminationLocate(points[ring[0]], other, points) != 0 || terminationLocate(points[other[0]], ring, points) != 0 {
					return nil, geom.ErrInvalid
				}
			}
		}
	}
	triangleCount := func(f Face) int {
		n := len(f.Boundary) - 2
		for _, hole := range f.Holes {
			n += len(hole) + 2
		}
		return n
	}
	start := 0
	for _, f := range s.faces[:face] {
		start += triangleCount(f)
	}
	tris := make([]terminationTriangle, 0, triangleCount(s.faces[face]))
	for _, t := range s.triangles[start : start+triangleCount(s.faces[face])] {
		var result terminationTriangle
		for i, a := range t {
			p, ok := projected[a]
			if !ok {
				return nil, geom.ErrInvalid
			}
			add(a, t[(i+1)%3])
			result.points[i] = terminationRat(p)
			for axis, x := range [2]float64{p.X, p.Y} {
				if i == 0 || x < result.min[axis] {
					result.min[axis] = x
				}
				if i == 0 || x > result.max[axis] {
					result.max[axis] = x
				}
			}
		}
		if terminationOrient(result.points[0], result.points[1], result.points[2]).Sign() != orientation {
			return nil, fmt.Errorf("brep: cap triangulation has an unproved orientation: %w", geom.ErrDegenerate)
		}
		if orientation < 0 {
			result.points[1], result.points[2] = result.points[2], result.points[1]
		}
		tris = append(tris, result)
	}
	if len(boundary) != 0 {
		return nil, fmt.Errorf("brep: cap triangulation does not match its boundary: %w", geom.ErrInvalid)
	}
	return tris, nil
}

// Returns twice the exact intersection area by clipping against the three
// closed half-planes of a counterclockwise triangle.
func terminationIntersectionArea(a, b [3]terminationPoint) *big.Rat {
	polygon := a[:]
	for i, p := range b {
		q := b[(i+1)%3]
		next := make([]terminationPoint, 0, len(polygon)+1)
		for j, u := range polygon {
			v := polygon[(j+1)%len(polygon)]
			du, dv := terminationOrient(p, q, u), terminationOrient(p, q, v)
			if du.Sign() >= 0 {
				next = append(next, u)
			}
			if du.Sign()*dv.Sign() < 0 {
				t := new(big.Rat).Quo(du, new(big.Rat).Sub(du, dv))
				var x terminationPoint
				for k := range x {
					x[k] = new(big.Rat).Add(u[k], new(big.Rat).Mul(t, new(big.Rat).Sub(v[k], u[k])))
				}
				next = append(next, x)
			}
		}
		polygon = next
		if len(polygon) < 3 {
			return new(big.Rat)
		}
	}
	area := new(big.Rat)
	for i := 1; i+1 < len(polygon); i++ {
		area.Add(area, terminationOrient(polygon[0], polygon[i], polygon[i+1]))
	}
	return area
}

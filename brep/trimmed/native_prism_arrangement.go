package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// NativePrismBooleanOptions keeps authentication/materialization and exact
// arrangement quotas separate. The first domain is matching coordinate-axis
// extrusion intervals; no sampled support recognition or near-cap matching occurs.
type NativePrismBooleanOptions struct {
	Native      AffineArrangementPrismOptions
	Arrangement intersect.AffinePolygonContactOptions
	Patches     geom.SurfacePatchOptions
}

// NativePrismWallUse maps each retained source interval back to both original
// native cap edges and the complete original wall. Parameters follow the stored
// native edge directions, independently of their cap coedge direction.
type NativePrismWallUse struct {
	Component, Face, Operand, Wall, LowerCap, UpperCap, LowerEdge, UpperEdge int
	LowerParameter, UpperParameter                                           [2]*big.Rat
}
type NativePrismBoolean struct {
	material  *AffineArrangementPrism
	owners    [2]brep.Solid
	axis      int
	operation intersect.AffineRingOperation
	caps      [2][2]int
	uses      []NativePrismWallUse
}

func (s *NativePrismBoolean) Material() *AffineArrangementPrism {
	if s == nil {
		return nil
	}
	return s.material
}
func (s *NativePrismBoolean) Axis() int {
	if s == nil {
		return -1
	}
	return s.axis
}
func (s *NativePrismBoolean) OriginalOwner(i int) (brep.Solid, error) {
	if s == nil || i < 0 || i > 1 {
		return brep.Solid{}, geom.ErrInvalid
	}
	return s.owners[i], nil
}

// OriginalCaps returns [operand][lower,upper] original face IDs, including deleted caps.
func (s *NativePrismBoolean) OriginalCaps() [2][2]int {
	if s == nil {
		return [2][2]int{{-1, -1}, {-1, -1}}
	}
	return s.caps
}
func (s *NativePrismBoolean) WallUses() []NativePrismWallUse {
	if s == nil {
		return nil
	}
	out := append([]NativePrismWallUse(nil), s.uses...)
	for i, u := range out {
		for j := 0; j < 2; j++ {
			out[i].LowerParameter[j] = new(big.Rat).Set(u.LowerParameter[j])
			out[i].UpperParameter[j] = new(big.Rat).Set(u.UpperParameter[j])
		}
	}
	return out
}

type nativePrismProfileSide struct {
	wall, lower, upper, lowerRing int
	lowerReverse, upperReverse    bool
}
type nativePrismProfile struct {
	trim               intersect.AffinePolygonTrim
	sides              [][]nativePrismProfileSide
	lowerCap, upperCap int
	lo, hi             float64
	bounds             [2][2]float64
}

func nativePrismCoordinate(v geom.Vec3, axis int) float64 {
	switch axis {
	case 0:
		return v.X
	case 1:
		return v.Y
	default:
		return v.Z
	}
}
func nativePrismLoopIndices(loop []brep.EdgeUse, edges []brep.Edge) ([]int, error) {
	if len(loop) < 3 {
		return nil, geom.ErrDomain
	}
	out := make([]int, len(loop))
	ends := make([]int, len(loop))
	for i, u := range loop {
		if u.Edge < 0 || u.Edge >= len(edges) {
			return nil, geom.ErrInvalid
		}
		e := edges[u.Edge]
		a, b := e.Start, e.End
		if u.Reversed {
			a, b = b, a
		}
		out[i], ends[i] = a, b
	}
	for i := range ends {
		if ends[i] != out[(i+1)%len(out)] {
			return nil, geom.ErrInvalid
		}
	}
	return out, nil
}
func authenticateNativePrism(ctx context.Context, s brep.Solid, axis, maxVertices int) (nativePrismProfile, error) {
	out := nativePrismProfile{lowerCap: -1, upperCap: -1}
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	if len(vertices) < 6 || len(vertices) > maxVertices || len(edges) > 3*maxVertices || len(faces) > maxVertices {
		return out, geom.ErrDomain
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	lo, hi := nativePrismCoordinate(vertices[0], axis), nativePrismCoordinate(vertices[0], axis)
	for _, v := range vertices {
		x := nativePrismCoordinate(v, axis)
		if x < lo {
			lo = x
		}
		if x > hi {
			hi = x
		}
	}
	if !(lo < hi) {
		return out, geom.ErrDomain
	}
	out.lo, out.hi = lo, hi
	axes := [2]int{(axis + 1) % 3, (axis + 2) % 3}
	uvkey := func(id int) string {
		return new(big.Rat).SetFloat64(nativePrismCoordinate(vertices[id], axes[0])).RatString() + "," + new(big.Rat).SetFloat64(nativePrismCoordinate(vertices[id], axes[1])).RatString()
	}
	lower, upper := map[string]int{}, map[string]int{}
	for id, v := range vertices {
		x := nativePrismCoordinate(v, axis)
		k := uvkey(id)
		if x == lo {
			if _, ok := lower[k]; ok {
				return out, geom.ErrDomain
			}
			lower[k] = id
		} else if x == hi {
			if _, ok := upper[k]; ok {
				return out, geom.ErrDomain
			}
			upper[k] = id
		} else {
			return out, geom.ErrDomain
		}
	}
	if len(lower) != len(upper) {
		return out, geom.ErrDomain
	}
	for k := range lower {
		if _, ok := upper[k]; !ok {
			return out, geom.ErrDomain
		}
	}
	faceIDs := make([][][]int, len(faces))
	uses := make([][]int, len(edges))
	for fi, f := range faces {
		for _, loop := range append([][]brep.EdgeUse{f.Boundary}, f.Holes...) {
			ids, e := nativePrismLoopIndices(loop, edges)
			if e != nil {
				return out, e
			}
			faceIDs[fi] = append(faceIDs[fi], ids)
			for _, u := range loop {
				uses[u.Edge] = append(uses[u.Edge], fi)
			}
		}
		sameLo, sameHi := true, true
		for _, ring := range faceIDs[fi] {
			for _, id := range ring {
				if id < 0 || id >= len(vertices) {
					return out, geom.ErrInvalid
				}
				x := nativePrismCoordinate(vertices[id], axis)
				sameLo = sameLo && x == lo
				sameHi = sameHi && x == hi
			}
		}
		if sameLo {
			if out.lowerCap >= 0 {
				return out, geom.ErrDomain
			}
			out.lowerCap = fi
		}
		if sameHi {
			if out.upperCap >= 0 {
				return out, geom.ErrDomain
			}
			out.upperCap = fi
		}
	}
	if out.lowerCap < 0 || out.upperCap < 0 {
		return out, geom.ErrDomain
	}
	if len(faceIDs[out.lowerCap]) != len(faceIDs[out.upperCap]) {
		return out, geom.ErrDomain
	}
	type bottomSide struct {
		edge, ring int
		start, end string
	}
	bottom := map[string]bottomSide{}
	pairkey := func(a, b string) string {
		if a > b {
			a, b = b, a
		}
		return a + ";" + b
	}
	lowerLoops := append([][]brep.EdgeUse{faces[out.lowerCap].Boundary}, faces[out.lowerCap].Holes...)
	for ri, loop := range lowerLoops {
		ids := faceIDs[out.lowerCap][ri]
		for i, u := range loop {
			a, b := uvkey(ids[i]), uvkey(ids[(i+1)%len(ids)])
			k := pairkey(a, b)
			if _, ok := bottom[k]; ok {
				return out, geom.ErrDomain
			}
			bottom[k] = bottomSide{u.Edge, ri, a, b}
		}
	}
	wallSeen := map[int]bool{}
	bottomSeen := map[int]bool{}
	upperLoops := append([][]brep.EdgeUse{faces[out.upperCap].Boundary}, faces[out.upperCap].Holes...)
	out.bounds = [2][2]float64{{math.Inf(1), math.Inf(-1)}, {math.Inf(1), math.Inf(-1)}}
	for ri, loop := range upperLoops {
		ids := faceIDs[out.upperCap][ri]
		ring := [][2]*big.Rat{}
		sides := []nativePrismProfileSide{}
		for i, id := range ids {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			next := ids[(i+1)%len(ids)]
			a, b := uvkey(id), uvkey(next)
			bs, ok := bottom[pairkey(a, b)]
			if !ok || bs.start != b || bs.end != a || bottomSeen[bs.edge] {
				return out, geom.ErrDomain
			}
			bottomSeen[bs.edge] = true
			topEdge := loop[i].Edge
			if len(uses[topEdge]) != 2 {
				return out, geom.ErrDomain
			}
			wall := -1
			for _, fi := range uses[topEdge] {
				if fi != out.upperCap {
					wall = fi
				}
			}
			if wall < 0 || wall == out.lowerCap || wallSeen[wall] || len(faceIDs[wall]) != 1 || len(faceIDs[wall][0]) != 4 {
				return out, geom.ErrDomain
			}
			want := map[int]bool{id: true, next: true, lower[a]: true, lower[b]: true}
			for _, vi := range faceIDs[wall][0] {
				if !want[vi] {
					return out, geom.ErrDomain
				}
				delete(want, vi)
			}
			if len(want) != 0 {
				return out, geom.ErrDomain
			}
			if len(uses[bs.edge]) != 2 {
				return out, geom.ErrDomain
			}
			bottomWall := -1
			for _, fi := range uses[bs.edge] {
				if fi != out.lowerCap {
					bottomWall = fi
				}
			}
			if bottomWall != wall {
				return out, geom.ErrDomain
			}
			wallSeen[wall] = true
			sides = append(sides, nativePrismProfileSide{wall: wall, lower: bs.edge, upper: topEdge, lowerRing: bs.ring, lowerReverse: uvkey(edges[bs.edge].Start) != a, upperReverse: uvkey(edges[topEdge].Start) != a})
			uv := [2]*big.Rat{}
			for j, ax := range axes {
				x := nativePrismCoordinate(vertices[id], ax)
				uv[j] = new(big.Rat).SetFloat64(x)
				if x < out.bounds[j][0] {
					out.bounds[j][0] = x
				}
				if x > out.bounds[j][1] {
					out.bounds[j][1] = x
				}
			}
			ring = append(ring, uv)
		}
		if ri == 0 {
			out.trim.Outer = ring
		} else {
			out.trim.Holes = append(out.trim.Holes, ring)
		}
		out.sides = append(out.sides, sides)
	}
	if len(bottomSeen) != len(bottom) || len(wallSeen)+2 != len(faces) {
		return out, geom.ErrDomain
	}
	return out, nil
}

// CombineMatchingNativePrisms authenticates both complete original B-reps as
// coordinate-axis prisms with the same exact cap interval. It recovers original
// profile holes and every side wall before invoking the exact opt-in arrangement
// and native constructor. Unsupported or nonrepresentable cases publish nothing.
// This is additive: existing generic BSP dispatch and tolerance policy are intact.
func CombineMatchingNativePrisms(ctx context.Context, a, c brep.Solid, operation intersect.AffineRingOperation, o NativePrismBooleanOptions) (*NativePrismBoolean, error) {
	if ctx == nil || operation > intersect.AffineRingDifference {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	maxVertices := o.Native.MaxVertices
	if maxVertices == 0 {
		maxVertices = 4096
	}
	if maxVertices < 6 || maxVertices > 16384 {
		return nil, geom.ErrInvalid
	}
	profiles := [2]nativePrismProfile{}
	axis := -1
	for candidate := 0; candidate < 3; candidate++ {
		x, e := authenticateNativePrism(ctx, a, candidate, maxVertices)
		if e != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		y, e := authenticateNativePrism(ctx, c, candidate, maxVertices)
		if e != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		if x.lo == y.lo && x.hi == y.hi {
			axis = candidate
			profiles = [2]nativePrismProfile{x, y}
			break
		}
	}
	if axis < 0 {
		return nil, geom.ErrDomain
	}
	bounds := profiles[0].bounds
	for j := 0; j < 2; j++ {
		if profiles[1].bounds[j][0] < bounds[j][0] {
			bounds[j][0] = profiles[1].bounds[j][0]
		}
		if profiles[1].bounds[j][1] > bounds[j][1] {
			bounds[j][1] = profiles[1].bounds[j][1]
		}
	}
	axes := [2]int{(axis + 1) % 3, (axis + 2) % 3}
	poles := []geom.Vec3{}
	for _, u := range bounds[0] {
		for _, v := range bounds[1] {
			xyz := [3]float64{}
			xyz[axis] = profiles[0].lo
			xyz[axes[0]], xyz[axes[1]] = u, v
			poles = append(poles, geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]})
		}
	}
	surface, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{bounds[0][0], bounds[0][0], bounds[0][1], bounds[0][1]}, []float64{bounds[1][0], bounds[1][0], bounds[1][1], bounds[1][1]}, nil)
	if e != nil {
		return nil, e
	}
	patch := intersect.Patch{Surface: surface, Min: geom.Vec2{X: bounds[0][0], Y: bounds[1][0]}, Max: geom.Vec2{X: bounds[0][1], Y: bounds[1][1]}}
	arrangement, e := intersect.ArrangeGeneralAffinePolygons(ctx, patch, patch, [2]intersect.AffinePolygonTrim{profiles[0].trim, profiles[1].trim}, operation, o.Patches, o.Arrangement)
	if e != nil {
		return nil, e
	}
	delta := new(big.Rat).Sub(new(big.Rat).SetFloat64(profiles[0].hi), new(big.Rat).SetFloat64(profiles[0].lo))
	height, exact := delta.Float64()
	if !exact || math.IsInf(height, 0) {
		return nil, geom.ErrDomain
	}
	rise := [3]float64{}
	rise[axis] = height
	material, e := ExtrudeAffineArrangement(ctx, arrangement, geom.Vec3{X: rise[0], Y: rise[1], Z: rise[2]}, o.Native)
	if e != nil {
		return nil, e
	}
	out := &NativePrismBoolean{material: material, owners: [2]brep.Solid{a, c}, axis: axis, operation: operation, caps: [2][2]int{{profiles[0].lowerCap, profiles[0].upperCap}, {profiles[1].lowerCap, profiles[1].upperCap}}}
	for _, wall := range material.Walls() {
		for _, use := range wall.Sources {
			profile := profiles[use.Owner]
			side := profile.sides[use.Ring][use.Edge]
			history := NativePrismWallUse{Component: wall.Component, Face: wall.Face, Operand: use.Owner, Wall: side.wall, LowerCap: profile.lowerCap, UpperCap: profile.upperCap, LowerEdge: side.lower, UpperEdge: side.upper}
			for j := 0; j < 2; j++ {
				history.LowerParameter[j] = new(big.Rat).Set(use.Parameter[j])
				history.UpperParameter[j] = new(big.Rat).Set(use.Parameter[j])
				if side.lowerReverse {
					history.LowerParameter[j].Sub(big.NewRat(1, 1), history.LowerParameter[j])
				}
				if side.upperReverse {
					history.UpperParameter[j].Sub(big.NewRat(1, 1), history.UpperParameter[j])
				}
			}
			out.uses = append(out.uses, history)
		}
	}
	return out, nil
}

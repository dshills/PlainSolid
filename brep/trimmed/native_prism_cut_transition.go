package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/intersect"
	"sort"
)

// NativePrismCutInterface indexes opposite directed copies of a tool boundary
// segment in the retained (A-B) and floor (A intersect B) arrangements.
type NativePrismCutInterface struct{ RetainedEdge, FloorEdge int }

// NativePrismCutTransition is an immutable complete planar imprint, not a sewn
// solid. Region 0 is retained A-B at the entry cap; region 1 is exposed floor
// A intersect B. Both arrangements' model points lie on the actual tool cap.
// Their cycle orientation is positive in the original cardinal plane; the floor
// outward normal is FloorNormalSign times the positive Axis direction.
type NativePrismCutTransition struct {
	owners                 [2]brep.Solid
	regions                [2]*intersect.AffinePolygonArrangement
	interfaces             []NativePrismCutInterface
	uses                   [2][]NativePrismIntervalWallUse
	axis, normal           int
	floor, entry           *big.Rat
	floorOwner, entryOwner NativePrismCapOwner
	volume                 *big.Rat
	work, arrangementWork  int
}

func (s *NativePrismCutTransition) OriginalOwner(i int) (brep.Solid, error) {
	if s == nil || i < 0 || i > 1 {
		return brep.Solid{}, geom.ErrInvalid
	}
	return s.owners[i], nil
}
func (s *NativePrismCutTransition) Region(i int) *intersect.AffinePolygonArrangement {
	if s == nil || i < 0 || i > 1 {
		return nil
	}
	return s.regions[i]
}
func (s *NativePrismCutTransition) Interfaces() []NativePrismCutInterface {
	if s == nil {
		return nil
	}
	return append([]NativePrismCutInterface(nil), s.interfaces...)
}

// BoundaryUses parallels Region(i).Edges. Face and Component are -1: these are
// original support clips, not published native faces. HeightParameter covers the
// cut span in increasing original height units, independently of cycle direction.
func (s *NativePrismCutTransition) BoundaryUses(i int) []NativePrismIntervalWallUse {
	if s == nil || i < 0 || i > 1 {
		return nil
	}
	q := NativePrismIntervalCommon{uses: s.uses[i]}
	return q.WallUses()
}
func (s *NativePrismCutTransition) Axis() int {
	if s == nil {
		return -1
	}
	return s.axis
}
func (s *NativePrismCutTransition) FloorNormalSign() int {
	if s == nil {
		return 0
	}
	return s.normal
}
func (s *NativePrismCutTransition) Levels() [2]*big.Rat {
	if s == nil {
		return [2]*big.Rat{}
	}
	return [2]*big.Rat{new(big.Rat).Set(s.floor), new(big.Rat).Set(s.entry)}
}
func (s *NativePrismCutTransition) CapOwners() [2]NativePrismCapOwner {
	if s == nil {
		return [2]NativePrismCapOwner{}
	}
	return [2]NativePrismCapOwner{s.floorOwner, s.entryOwner}
}
func (s *NativePrismCutTransition) ExactMaterialVolume() *big.Rat {
	if s == nil {
		return nil
	}
	return new(big.Rat).Set(s.volume)
}
func (s *NativePrismCutTransition) Operations() (int, int) {
	if s == nil {
		return 0, 0
	}
	return s.work, s.arrangementWork
}

// ImprintCrossingNativePrismCut authenticates original cardinal prisms and a
// one-ended blind axial Cut, including original outer and hole rings.
// Strict transverse profile arrangements must retain
// positive A-B and A intersect B. Every original stock boundary interval is
// accounted for exactly once, while every internal tool segment is paired in
// opposite directions. Contact degeneracies remain unsupported. No native shell
// is claimed until a conforming cap/wall subdivision consumes this certificate.
func ImprintCrossingNativePrismCut(ctx context.Context, stock, tool brep.Solid, o NativePrismBooleanOptions) (*NativePrismCutTransition, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	limit := o.Native.MaxVertices
	if limit == 0 {
		limit = 4096
	}
	maximum, bits := o.Native.MaxOperations, o.Native.MaxCoefficientBits
	if maximum == 0 {
		maximum = 8000000
	}
	if bits == 0 {
		bits = 16384
	}
	if limit < 6 || limit > 16384 || maximum < 1 || maximum > 128000000 || bits < 64 || bits > 65536 || o.Native.MaxComponents < 0 || o.Native.MaxComponents > 256 {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	result := &NativePrismCutTransition{owners: [2]brep.Solid{stock, tool}}
	profiles := [2]nativePrismProfile{}
	axis := -1
	topEntry := false
	var floor, entry float64
	for candidate := 0; candidate < 3; candidate++ {
		a, e := authenticateNativePrism(ctx, stock, candidate, limit)
		if e != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		b, e := authenticateNativePrism(ctx, tool, candidate, limit)
		if e != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		up := a.lo < b.lo && b.lo < a.hi && a.hi <= b.hi
		down := b.lo <= a.lo && a.lo < b.hi && b.hi < a.hi
		if !up && !down {
			continue
		}
		bounds := a.bounds
		for j := 0; j < 2; j++ {
			if b.bounds[j][0] < bounds[j][0] {
				bounds[j][0] = b.bounds[j][0]
			}
			if b.bounds[j][1] > bounds[j][1] {
				bounds[j][1] = b.bounds[j][1]
			}
		}
		axes := [2]int{(candidate + 1) % 3, (candidate + 2) % 3}
		poles := []geom.Vec3{}
		for _, u := range bounds[0] {
			for _, v := range bounds[1] {
				xyz := [3]float64{}
				if up {
					xyz[candidate] = b.lo
				} else {
					xyz[candidate] = b.hi
				}
				xyz[axes[0]], xyz[axes[1]] = u, v
				poles = append(poles, geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]})
			}
		}
		surface, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{bounds[0][0], bounds[0][0], bounds[0][1], bounds[0][1]}, []float64{bounds[1][0], bounds[1][0], bounds[1][1], bounds[1][1]}, nil)
		if e != nil {
			return nil, e
		}
		patch := intersect.Patch{Surface: surface, Min: geom.Vec2{X: bounds[0][0], Y: bounds[1][0]}, Max: geom.Vec2{X: bounds[0][1], Y: bounds[1][1]}}
		arrs := [2]*intersect.AffinePolygonArrangement{}
		limits := o.Arrangement
		if limits.MaxOperations == 0 {
			limits.MaxOperations = 8000000
		}
		if limits.MaxOperations < 1 || limits.MaxOperations > 128000000 {
			return nil, geom.ErrInvalid
		}
		spent := 0
		for i, op := range []intersect.AffineRingOperation{intersect.AffineRingDifference, intersect.AffineRingIntersection} {
			remain := limits
			remain.MaxOperations -= spent
			if remain.MaxOperations < 1 {
				return nil, geom.ErrConvergence
			}
			arrs[i], e = intersect.ArrangeTransverseAffinePolygons(ctx, patch, patch, [2]intersect.AffinePolygonTrim{a.trim, b.trim}, op, o.Patches, remain)
			if e != nil {
				return nil, e
			}
			spent += arrs[i].ExactOperations()
		}
		result.regions = arrs
		result.arrangementWork = spent
		profiles = [2]nativePrismProfile{a, b}
		axis = candidate
		topEntry = up
		if up {
			floor, entry = b.lo, a.hi
		} else {
			floor, entry = b.hi, a.lo
		}
		break
	}
	if axis < 0 {
		return nil, geom.ErrDomain
	}

	budget := &nativePrismAffineBudget{ctx: ctx, max: maximum, bits: bits}
	result.axis = axis
	result.normal = -1
	result.floorOwner = NativePrismCapOwner{1, profiles[1].upperCap}
	result.entryOwner = NativePrismCapOwner{0, profiles[0].lowerCap}
	if topEntry {
		result.normal = 1
		result.floorOwner.Face = profiles[1].lowerCap
		result.entryOwner.Face = profiles[0].upperCap
	}
	result.floor = new(big.Rat).SetFloat64(floor)
	result.entry = new(big.Rat).SetFloat64(entry)
	low, high := floor, entry
	if low > high {
		low, high = high, low
	}
	areas := [2]*big.Rat{new(big.Rat), new(big.Rat)}
	stockRings := append([][][2]*big.Rat{profiles[0].trim.Outer}, profiles[0].trim.Holes...)
	stockParts := make([][][][2]*big.Rat, len(stockRings))
	for r, ring := range stockRings {
		stockParts[r] = make([][][2]*big.Rat, len(ring))
	}
	edges := [2][]intersect.AffineArrangementEdge{result.regions[0].Edges(), result.regions[1].Edges()}
	for region, arr := range result.regions {
		for _, cycle := range arr.Cycles() {
			areas[region] = budget.op('+', areas[region], cycle.SignedAreaA)
		}
		if areas[region].Sign() <= 0 {
			return nil, geom.ErrDomain
		}
		for _, edge := range edges[region] {
			p := profiles[edge.Owner]
			side := p.sides[edge.Ring][edge.Edge]
			u := NativePrismIntervalWallUse{Use: NativePrismWallUse{Component: -1, Face: -1, Operand: edge.Owner, Wall: side.wall, LowerCap: p.lowerCap, UpperCap: p.upperCap, LowerEdge: side.lower, UpperEdge: side.upper}}
			height := budget.op('-', new(big.Rat).SetFloat64(p.hi), new(big.Rat).SetFloat64(p.lo))
			for end, z := range []float64{low, high} {
				u.HeightParameter[end] = budget.op('/', budget.op('-', new(big.Rat).SetFloat64(z), new(big.Rat).SetFloat64(p.lo)), height)
				u.Use.LowerParameter[end] = new(big.Rat).Set(edge.Parameter[end])
				u.Use.UpperParameter[end] = new(big.Rat).Set(edge.Parameter[end])
				if side.lowerReverse {
					u.Use.LowerParameter[end] = budget.op('-', big.NewRat(1, 1), u.Use.LowerParameter[end])
				}
				if side.upperReverse {
					u.Use.UpperParameter[end] = budget.op('-', big.NewRat(1, 1), u.Use.UpperParameter[end])
				}
			}
			result.uses[region] = append(result.uses[region], u)
			if edge.Owner == 0 {
				if edge.Parameter[0].Cmp(edge.Parameter[1]) >= 0 {
					return nil, geom.ErrInvalid
				}
				stockParts[edge.Ring][edge.Edge] = append(stockParts[edge.Ring][edge.Edge], edge.Parameter)
			}
		}
	}
	// Complete stock-boundary ownership, independent of area conservation.
	for _, ringParts := range stockParts {
		for _, parts := range ringParts {
			sort.Slice(parts, func(i, j int) bool { return parts[i][0].Cmp(parts[j][0]) < 0 })
			cursor := new(big.Rat)
			for _, p := range parts {
				budget.check(p[0])
				budget.check(p[1])
				if p[0].Cmp(cursor) != 0 {
					return nil, geom.ErrInvalid
				}
				cursor = p[1]
			}
			if cursor.Cmp(big.NewRat(1, 1)) != 0 {
				return nil, geom.ErrInvalid
			}
		}
	}
	used := make([]bool, len(edges[1]))
	for i, a := range edges[0] {
		if a.Owner != 1 {
			continue
		}
		found := -1
		for j, b := range edges[1] {
			budget.check(a.Parameter[0])
			if b.Owner == 1 && a.Ring == b.Ring && a.Edge == b.Edge && a.Parameter[0].Cmp(b.Parameter[1]) == 0 && a.Parameter[1].Cmp(b.Parameter[0]) == 0 {
				if found >= 0 || used[j] {
					return nil, geom.ErrInvalid
				}
				found = j
			}
		}
		if found < 0 {
			return nil, geom.ErrInvalid
		}
		used[found] = true
		result.interfaces = append(result.interfaces, NativePrismCutInterface{i, found})
	}
	for j, e := range edges[1] {
		if e.Owner == 1 && !used[j] {
			return nil, geom.ErrInvalid
		}
	}
	if len(result.interfaces) == 0 {
		return nil, geom.ErrDomain
	}
	// Require genuine crossing rather than a wholly contained circular boundary.
	crossed := false
	for _, ringParts := range stockParts {
		for _, parts := range ringParts {
			if len(parts) > 1 {
				crossed = true
			}
		}
	}
	if !crossed {
		return nil, geom.ErrDomain
	}
	stockArea := new(big.Rat)
	for _, ring := range stockRings {
		for i, v := range ring {
			w := ring[(i+1)%len(ring)]
			stockArea = budget.op('+', stockArea, budget.op('-', budget.op('*', v[0], w[1]), budget.op('*', v[1], w[0])))
		}
	}
	stockArea = budget.op('/', stockArea, big.NewRat(2, 1))
	if budget.op('+', areas[0], areas[1]).Cmp(stockArea) != 0 {
		return nil, geom.ErrInvalid
	}
	stockVolume := budget.op('*', stockArea, budget.op('-', new(big.Rat).SetFloat64(profiles[0].hi), new(big.Rat).SetFloat64(profiles[0].lo)))
	result.volume = budget.op('-', stockVolume, budget.op('*', areas[1], budget.op('-', new(big.Rat).SetFloat64(high), new(big.Rat).SetFloat64(low))))
	if budget.err != nil {
		return nil, budget.err
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	result.work = budget.work
	return result, nil
}

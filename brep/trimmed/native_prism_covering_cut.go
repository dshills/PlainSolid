package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// NativePrismComponentFaceUse binds a component face to its original support.
// Multiple rows may describe collinear source pieces merged into one wall.
type NativePrismComponentFaceUse struct {
	Component, Face, Operand, OriginalFace int
	Reversed                               bool
}
type NativePrismCoveringCut struct {
	material *AffineArrangementPrism
	owners   [2]brep.Solid
	axis     int
	caps     [2][2]int
	uses     []NativePrismIntervalWallUse
	faces    []NativePrismComponentFaceUse
	work     int
}

func (s *NativePrismCoveringCut) Components() []brep.Solid {
	if s == nil {
		return nil
	}
	return s.material.Components()
}
func (s *NativePrismCoveringCut) Material() *AffineArrangementPrism {
	if s == nil {
		return nil
	}
	return s.material
}
func (s *NativePrismCoveringCut) OriginalOwner(i int) (brep.Solid, error) {
	if s == nil || i < 0 || i > 1 {
		return brep.Solid{}, geom.ErrInvalid
	}
	return s.owners[i], nil
}
func (s *NativePrismCoveringCut) OriginalCaps() [2][2]int {
	if s == nil {
		return [2][2]int{{-1, -1}, {-1, -1}}
	}
	return s.caps
}
func (s *NativePrismCoveringCut) Faces() []NativePrismComponentFaceUse {
	if s == nil {
		return nil
	}
	return append([]NativePrismComponentFaceUse(nil), s.faces...)
}
func (s *NativePrismCoveringCut) WallUses() []NativePrismIntervalWallUse {
	if s == nil {
		return nil
	}
	q := NativePrismIntervalCommon{uses: s.uses}
	return q.WallUses()
}
func (s *NativePrismCoveringCut) NativeOperations() int {
	if s == nil {
		return 0
	}
	return s.work
}

// CutCoveringCrossingNativePrisms authenticates complete stock-height coverage
// by an original cardinal tool and a proper transverse profile crossing. The
// complete regularized Difference profile is extruded over the original stock
// interval, preserving multiple components and original holes. Both retained
// caps belong to stock; tool walls reverse with exact original height clipping.
// Nonbinary native vertices, unresolved topology and noncrossing domains reject.
func CutCoveringCrossingNativePrisms(ctx context.Context, a, c brep.Solid, o NativePrismBooleanOptions) (*NativePrismCoveringCut, error) {
	if ctx == nil {
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
		if y.lo <= x.lo && x.hi <= y.hi {
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
	arrangement, e := intersect.ArrangeGeneralAffinePolygons(ctx, patch, patch, [2]intersect.AffinePolygonTrim{profiles[0].trim, profiles[1].trim}, intersect.AffineRingDifference, o.Patches, o.Arrangement)
	if e != nil {
		return nil, e
	}
	maximum, bits := o.Native.MaxOperations, o.Native.MaxCoefficientBits
	if maximum == 0 {
		maximum = 8000000
	}
	if bits == 0 {
		bits = 16384
	}
	if maximum < 1 || maximum > 128000000 || bits < 64 || bits > 65536 {
		return nil, geom.ErrInvalid
	}
	budget := &nativePrismAffineBudget{ctx: ctx, max: maximum, bits: bits}
	// Prove a proper transverse event in retained original owner-edge units.
	// Complete General arrangement still proves all other boundary/contact pieces.
	edges, v := arrangement.Area().Edges(), arrangement.Area().Vertices()
	crossing := false
	for _, x := range edges {
		if x.Owner != 0 {
			continue
		}
		for xe, xi := range []int{x.Start, x.End} {
			t := x.Parameter[xe]
			if t.Sign() <= 0 || t.Cmp(big.NewRat(1, 1)) >= 0 {
				continue
			}
			for _, y := range edges {
				if y.Owner != 1 {
					continue
				}
				for ye, yi := range []int{y.Start, y.End} {
					u := y.Parameter[ye]
					if u.Sign() <= 0 || u.Cmp(big.NewRat(1, 1)) >= 0 {
						continue
					}
					budget.check(t)
					if v[xi].A[0].Cmp(v[yi].A[0]) != 0 || v[xi].A[1].Cmp(v[yi].A[1]) != 0 {
						continue
					}
					dx := [2]*big.Rat{budget.op('-', x.UV[1][0], x.UV[0][0]), budget.op('-', x.UV[1][1], x.UV[0][1])}
					dy := [2]*big.Rat{budget.op('-', y.UV[1][0], y.UV[0][0]), budget.op('-', y.UV[1][1], y.UV[0][1])}
					if budget.op('-', budget.op('*', dx[0], dy[1]), budget.op('*', dx[1], dy[0])).Sign() != 0 {
						crossing = true
					}
				}
			}
		}
	}
	if budget.err != nil {
		return nil, budget.err
	}
	if !crossing {
		return nil, geom.ErrDomain
	}
	delta := budget.op('-', new(big.Rat).SetFloat64(profiles[0].hi), new(big.Rat).SetFloat64(profiles[0].lo))
	height, exact := delta.Float64()
	if !exact || math.IsInf(height, 0) {
		return nil, geom.ErrDomain
	}
	rise := [3]float64{}
	rise[axis] = height
	native := o.Native
	native.MaxOperations = maximum - budget.work
	if budget.err != nil {
		return nil, budget.err
	}
	if native.MaxOperations < 1 {
		return nil, geom.ErrConvergence
	}
	material, e := ExtrudeAffineArrangement(ctx, arrangement, geom.Vec3{X: rise[0], Y: rise[1], Z: rise[2]}, native)
	if e != nil {
		return nil, e
	}
	budget.work += material.ExactOperations()
	out := &NativePrismCoveringCut{material: material, owners: [2]brep.Solid{a, c}, axis: axis, caps: [2][2]int{{profiles[0].lowerCap, profiles[0].upperCap}, {profiles[1].lowerCap, profiles[1].upperCap}}}
	for i := range material.Components() {
		out.faces = append(out.faces, NativePrismComponentFaceUse{i, 0, 0, profiles[0].lowerCap, false}, NativePrismComponentFaceUse{i, 1, 0, profiles[0].upperCap, false})
	}
	for _, wall := range material.Walls() {
		for _, use := range wall.Sources {
			p := profiles[use.Owner]
			side := p.sides[use.Ring][use.Edge]
			h := NativePrismIntervalWallUse{Use: NativePrismWallUse{Component: wall.Component, Face: wall.Face, Operand: use.Owner, Wall: side.wall, LowerCap: p.lowerCap, UpperCap: p.upperCap, LowerEdge: side.lower, UpperEdge: side.upper}}
			span := budget.op('-', new(big.Rat).SetFloat64(p.hi), new(big.Rat).SetFloat64(p.lo))
			for end, z := range []float64{profiles[0].lo, profiles[0].hi} {
				h.HeightParameter[end] = budget.op('/', budget.op('-', new(big.Rat).SetFloat64(z), new(big.Rat).SetFloat64(p.lo)), span)
				h.Use.LowerParameter[end] = new(big.Rat).Set(use.Parameter[end])
				h.Use.UpperParameter[end] = new(big.Rat).Set(use.Parameter[end])
				if side.lowerReverse {
					h.Use.LowerParameter[end] = budget.op('-', big.NewRat(1, 1), h.Use.LowerParameter[end])
				}
				if side.upperReverse {
					h.Use.UpperParameter[end] = budget.op('-', big.NewRat(1, 1), h.Use.UpperParameter[end])
				}
			}
			out.uses = append(out.uses, h)
			out.faces = append(out.faces, NativePrismComponentFaceUse{wall.Component, wall.Face, use.Owner, side.wall, use.Owner == 1})
		}
	}
	if budget.err != nil {
		return nil, budget.err
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	out.work = budget.work
	return out, nil
}

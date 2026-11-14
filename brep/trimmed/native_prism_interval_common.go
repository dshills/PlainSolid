package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/intersect"
)

type NativePrismCommonKind uint8

const (
	NativePrismCommonEmpty NativePrismCommonKind = iota
	NativePrismCommonContact
	NativePrismCommonMaterial
)

type NativePrismCapOwner struct{ Operand, Face int }

// NativePrismIntervalWallUse retains original full-wall/cap edge ancestry and
// exact output lower/upper height fractions in that original wall's [0,1] span.
type NativePrismIntervalWallUse struct {
	Use             NativePrismWallUse
	HeightParameter [2]*big.Rat
}
type NativePrismIntervalCommon struct {
	owners     [2]brep.Solid
	axis       int
	kind       NativePrismCommonKind
	lo, hi     *big.Rat
	controls   [2][]NativePrismCapOwner
	profile    *intersect.AffineGeneralPolygonArrangement
	material   *AffineArrangementPrism
	uses       []NativePrismIntervalWallUse
	nativeWork int
}

// NativeOperations counts materialization and original height-interval arithmetic.
func (s *NativePrismIntervalCommon) NativeOperations() int {
	if s == nil {
		return 0
	}
	return s.nativeWork
}
func (s *NativePrismIntervalCommon) Kind() (NativePrismCommonKind, error) {
	if s == nil || s.profile == nil {
		return 0, geom.ErrInvalid
	}
	return s.kind, nil
}
func (s *NativePrismIntervalCommon) Axis() int {
	if s == nil {
		return -1
	}
	return s.axis
}
func (s *NativePrismIntervalCommon) OriginalOwner(i int) (brep.Solid, error) {
	if s == nil || i < 0 || i > 1 {
		return brep.Solid{}, geom.ErrInvalid
	}
	return s.owners[i], nil
}

// ProjectedProfile is the complete orthogonal profile Common on the lower
// overlap-bound plane; it is not itself a claim of3D contact for disjoint spans.
func (s *NativePrismIntervalCommon) ProjectedProfile() *intersect.AffineGeneralPolygonArrangement {
	if s == nil {
		return nil
	}
	return s.profile
}
func (s *NativePrismIntervalCommon) Material() *AffineArrangementPrism {
	if s == nil {
		return nil
	}
	return s.material
}

// AxialInterval returns exact intersected bounds; lo>hi denotes an empty axial
// intersection. ControllingCaps identifies the actual original faces at these
// bounds; only Material kind publishes corresponding native output cap faces0/1.
func (s *NativePrismIntervalCommon) AxialInterval() [2]*big.Rat {
	if s == nil {
		return [2]*big.Rat{}
	}
	return [2]*big.Rat{new(big.Rat).Set(s.lo), new(big.Rat).Set(s.hi)}
}
func (s *NativePrismIntervalCommon) ControllingCaps() [2][]NativePrismCapOwner {
	var out [2][]NativePrismCapOwner
	if s != nil {
		for i := range out {
			out[i] = append([]NativePrismCapOwner(nil), s.controls[i]...)
		}
	}
	return out
}
func (s *NativePrismIntervalCommon) WallUses() []NativePrismIntervalWallUse {
	if s == nil {
		return nil
	}
	out := append([]NativePrismIntervalWallUse(nil), s.uses...)
	for i, u := range out {
		for j := 0; j < 2; j++ {
			out[i].HeightParameter[j] = new(big.Rat).Set(u.HeightParameter[j])
			out[i].Use.LowerParameter[j] = new(big.Rat).Set(u.Use.LowerParameter[j])
			out[i].Use.UpperParameter[j] = new(big.Rat).Set(u.Use.UpperParameter[j])
		}
	}
	return out
}

// CommonNativePrismIntervals authenticates original coordinate-axis straight
// prisms but allows unequal axial intervals. Their complete profile Common is
// combined with exact max/min cap bounds. Positive material is native; point,
// line, planar and zero-height contacts remain explicit non-solid witnesses.
// Unequal-height Fuse/Cut shoulder sewing is not claimed by this API.
func CommonNativePrismIntervals(ctx context.Context, a, c brep.Solid, o NativePrismBooleanOptions) (*NativePrismIntervalCommon, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	limit := o.Native.MaxVertices
	if limit == 0 {
		limit = 4096
	}
	if limit < 6 || limit > 16384 {
		return nil, geom.ErrInvalid
	}
	if o.Native.MaxComponents < 0 || o.Native.MaxComponents > 256 || o.Native.MaxOperations < 0 || o.Native.MaxOperations > 128000000 || o.Native.MaxCoefficientBits < 0 || o.Native.MaxCoefficientBits > 65536 || (o.Native.MaxCoefficientBits > 0 && o.Native.MaxCoefficientBits < 64) {
		return nil, geom.ErrInvalid
	}
	profiles := [2]nativePrismProfile{}
	axis := -1
	for candidate := 0; candidate < 3; candidate++ {
		x, e := authenticateNativePrism(ctx, a, candidate, limit)
		if e != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		y, e := authenticateNativePrism(ctx, c, candidate, limit)
		if e != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		profiles = [2]nativePrismProfile{x, y}
		axis = candidate
		break
	}
	if axis < 0 {
		return nil, geom.ErrDomain
	}
	lo, hi := profiles[0].lo, profiles[0].hi
	if profiles[1].lo > lo {
		lo = profiles[1].lo
	}
	if profiles[1].hi < hi {
		hi = profiles[1].hi
	}
	out := &NativePrismIntervalCommon{owners: [2]brep.Solid{a, c}, axis: axis, lo: new(big.Rat).SetFloat64(lo), hi: new(big.Rat).SetFloat64(hi)}
	for owner, p := range profiles {
		if p.lo == lo {
			out.controls[0] = append(out.controls[0], NativePrismCapOwner{owner, p.lowerCap})
		}
		if p.hi == hi {
			out.controls[1] = append(out.controls[1], NativePrismCapOwner{owner, p.upperCap})
		}
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
			xyz[axis] = lo
			xyz[axes[0]], xyz[axes[1]] = u, v
			poles = append(poles, geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]})
		}
	}
	surface, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{bounds[0][0], bounds[0][0], bounds[0][1], bounds[0][1]}, []float64{bounds[1][0], bounds[1][0], bounds[1][1], bounds[1][1]}, nil)
	if e != nil {
		return nil, e
	}
	patch := intersect.Patch{Surface: surface, Min: geom.Vec2{X: bounds[0][0], Y: bounds[1][0]}, Max: geom.Vec2{X: bounds[0][1], Y: bounds[1][1]}}
	out.profile, e = intersect.ArrangeGeneralAffinePolygons(ctx, patch, patch, [2]intersect.AffinePolygonTrim{profiles[0].trim, profiles[1].trim}, intersect.AffineRingIntersection, o.Patches, o.Arrangement)
	if e != nil {
		return nil, e
	}
	hasArea := len(out.profile.Area().Cycles()) > 0
	hasContact := hasArea || len(out.profile.SharedBoundarySpans()) > 0 || len(out.profile.IsolatedCommonPoints()) > 0
	if lo > hi || !hasContact {
		return out, nil
	}
	if lo == hi || !hasArea {
		out.kind = NativePrismCommonContact
		return out, nil
	}
	riseRat := new(big.Rat).Sub(out.hi, out.lo)
	rise, exact := riseRat.Float64()
	if !exact || math.IsInf(rise, 0) {
		return nil, geom.ErrDomain
	}
	offset := [3]float64{}
	offset[axis] = rise
	out.material, e = ExtrudeAffineArrangement(ctx, out.profile, geom.Vec3{X: offset[0], Y: offset[1], Z: offset[2]}, o.Native)
	if e != nil {
		return nil, e
	}
	out.kind = NativePrismCommonMaterial
	maximum, bits := o.Native.MaxOperations, o.Native.MaxCoefficientBits
	if maximum == 0 {
		maximum = 8000000
	}
	if bits == 0 {
		bits = 16384
	}
	budget := &nativePrismAffineBudget{ctx: ctx, max: maximum, bits: bits, work: out.material.ExactOperations()}
	for _, wall := range out.material.Walls() {
		for _, use := range wall.Sources {
			p := profiles[use.Owner]
			budget.check(new(big.Rat).SetFloat64(p.lo))
			budget.check(new(big.Rat).SetFloat64(p.hi))
			side := p.sides[use.Ring][use.Edge]
			history := NativePrismIntervalWallUse{Use: NativePrismWallUse{Component: wall.Component, Face: wall.Face, Operand: use.Owner, Wall: side.wall, LowerCap: p.lowerCap, UpperCap: p.upperCap, LowerEdge: side.lower, UpperEdge: side.upper}}
			height := budget.op('-', new(big.Rat).SetFloat64(p.hi), new(big.Rat).SetFloat64(p.lo))
			for end, q := range [2]*big.Rat{out.lo, out.hi} {
				history.HeightParameter[end] = budget.op('/', budget.op('-', q, new(big.Rat).SetFloat64(p.lo)), height)
				history.Use.LowerParameter[end] = new(big.Rat).Set(use.Parameter[end])
				history.Use.UpperParameter[end] = new(big.Rat).Set(use.Parameter[end])
				if side.lowerReverse {
					history.Use.LowerParameter[end] = budget.op('-', big.NewRat(1, 1), history.Use.LowerParameter[end])
				}
				if side.upperReverse {
					history.Use.UpperParameter[end] = budget.op('-', big.NewRat(1, 1), history.Use.UpperParameter[end])
				}
			}
			out.uses = append(out.uses, history)
		}
	}
	if budget.err != nil {
		return nil, budget.err
	}
	out.nativeWork = budget.work
	return out, nil
}

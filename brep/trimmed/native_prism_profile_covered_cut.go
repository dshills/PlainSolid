//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
)

// NativePrismProfileCoveredCut contains one or two retained stock slabs. New
// forming caps reference reversed original tool cap supports; surviving walls
// retain original stock cap-edge parameters and exact axial fractions.
type NativePrismProfileCoveredCut struct {
	source     *NativePrismIntervalCommon
	components []brep.Solid
	faces      []NativePrismComponentFaceUse
	walls      []NativePrismIntervalWallUse
	spans      [][2]*big.Rat
	volume     *big.Rat
	work       int
}

func (s *NativePrismProfileCoveredCut) Components() []brep.Solid {
	if s == nil {
		return nil
	}
	return append([]brep.Solid(nil), s.components...)
}
func (s *NativePrismProfileCoveredCut) CommonProof() *NativePrismIntervalCommon {
	if s == nil {
		return nil
	}
	return s.source
}
func (s *NativePrismProfileCoveredCut) Faces() []NativePrismComponentFaceUse {
	if s == nil {
		return nil
	}
	return append([]NativePrismComponentFaceUse(nil), s.faces...)
}
func (s *NativePrismProfileCoveredCut) WallUses() []NativePrismIntervalWallUse {
	if s == nil {
		return nil
	}
	q := NativePrismIntervalCommon{uses: s.walls}
	return q.WallUses()
}
func (s *NativePrismProfileCoveredCut) AxialIntervals() [][2]*big.Rat {
	if s == nil {
		return nil
	}
	out := make([][2]*big.Rat, len(s.spans))
	for i, p := range s.spans {
		for j, v := range p {
			out[i][j] = new(big.Rat).Set(v)
		}
	}
	return out
}
func (s *NativePrismProfileCoveredCut) ExactMaterialVolume() *big.Rat {
	if s == nil {
		return nil
	}
	return new(big.Rat).Set(s.volume)
}
func (s *NativePrismProfileCoveredCut) NativeOperations() int {
	if s == nil {
		return 0
	}
	return s.work
}

// CutProfileCoveredNativePrisms requires positive axial overlap, complete
// regularized stock footprint coverage by the tool and nonempty retained stock.
// Equal footprint boundaries and holes are supported. No-contact and completely
// consumed stock remain separate normal-dispatch outcomes.
func CutProfileCoveredNativePrisms(ctx context.Context, a, b brep.Solid, o NativePrismBooleanOptions) (*NativePrismProfileCoveredCut, error) {
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
	axis := -1
	for k := 0; k < 3; k++ {
		x, e := authenticateNativePrism(ctx, a, k, limit)
		if e != nil {
			continue
		}
		y, e := authenticateNativePrism(ctx, b, k, limit)
		if e != nil {
			continue
		}
		if x.lo < y.hi && y.lo < x.hi && !(y.lo <= x.lo && x.hi <= y.hi) {
			axis = k
			break
		}
	}
	if axis < 0 {
		return nil, geom.ErrDomain
	}
	common, e := commonNativePrismIntervalsAxis(ctx, a, b, o, axis)
	if e != nil {
		return nil, e
	}
	return buildProfileCoveredNativePrismCut(ctx, common, o.Native, common.nativeWork)
}
func buildProfileCoveredNativePrismCut(ctx context.Context, common *NativePrismIntervalCommon, o AffineArrangementPrismOptions, initial int) (*NativePrismProfileCoveredCut, error) {
	if ctx == nil || common == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if common.kind != NativePrismCommonMaterial {
		return nil, geom.ErrDomain
	}
	limit, max, bits, components := o.MaxVertices, o.MaxOperations, o.MaxCoefficientBits, o.MaxComponents
	if limit == 0 {
		limit = 4096
	}
	if max == 0 {
		max = 8000000
	}
	if bits == 0 {
		bits = 16384
	}
	if components == 0 {
		components = 64
	}
	if limit < 6 || limit > 16384 || max < 1 || max > 128000000 || bits < 64 || bits > 65536 || components < 1 || components > 256 {
		return nil, geom.ErrInvalid
	}
	p, e := authenticateNativePrism(ctx, common.owners[0], common.axis, limit)
	if e != nil {
		return nil, e
	}
	tool, e := authenticateNativePrism(ctx, common.owners[1], common.axis, limit)
	if e != nil {
		return nil, e
	}
	budget := &nativePrismAffineBudget{ctx: ctx, max: max, bits: bits, work: initial}
	area := new(big.Rat)
	for _, ring := range append([][][2]*big.Rat{p.trim.Outer}, p.trim.Holes...) {
		for i, v := range ring {
			w := ring[(i+1)%len(ring)]
			area = budget.op('+', area, budget.op('-', budget.op('*', v[0], w[1]), budget.op('*', v[1], w[0])))
		}
	}
	area = budget.op('/', area, big.NewRat(2, 1))
	ca := new(big.Rat)
	for _, c := range common.profile.Area().Cycles() {
		ca = budget.op('+', ca, c.SignedAreaA)
	}
	if budget.err != nil {
		return nil, budget.err
	}
	if area.Sign() <= 0 || ca.Cmp(area) != 0 {
		return nil, geom.ErrDomain
	}
	out := &NativePrismProfileCoveredCut{source: common, volume: new(big.Rat)}
	lo, hi := new(big.Rat).SetFloat64(p.lo), new(big.Rat).SetFloat64(p.hi)
	if lo.Cmp(common.lo) < 0 {
		out.spans = append(out.spans, [2]*big.Rat{lo, common.lo})
	}
	if common.hi.Cmp(hi) < 0 {
		out.spans = append(out.spans, [2]*big.Rat{common.hi, hi})
	}
	if len(out.spans) == 0 {
		return nil, geom.ErrDomain
	}
	if len(out.spans) > components || len(out.spans)*len(common.owners[0].Vertices()) > limit {
		return nil, geom.ErrConvergence
	}
	height := budget.op('-', hi, lo)
	for component, span := range out.spans {
		verts := common.owners[0].Vertices()
		bounds := [2]float64{}
		for i, z := range span {
			budget.check(z)
			var exact bool
			bounds[i], exact = z.Float64()
			if !exact {
				return nil, geom.ErrDomain
			}
		}
		for i, v := range verts {
			xyz := [3]float64{v.X, v.Y, v.Z}
			if xyz[common.axis] == p.lo {
				xyz[common.axis] = bounds[0]
			} else if xyz[common.axis] == p.hi {
				xyz[common.axis] = bounds[1]
			} else {
				return nil, geom.ErrInvalid
			}
			verts[i] = geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
		}
		solid, e := brep.NewSolid(verts, common.owners[0].Edges(), common.owners[0].Faces())
		if e != nil {
			return nil, e
		}
		for face := range solid.Faces() {
			u := NativePrismComponentFaceUse{Component: component, Face: face, Operand: 0, OriginalFace: face}
			if face == p.lowerCap && span[0].Cmp(lo) != 0 {
				if span[0].Cmp(new(big.Rat).SetFloat64(tool.hi)) != 0 {
					return nil, geom.ErrInvalid
				}
				u.Operand = 1
				u.OriginalFace = tool.upperCap
				u.Reversed = true
			}
			if face == p.upperCap && span[1].Cmp(hi) != 0 {
				if span[1].Cmp(new(big.Rat).SetFloat64(tool.lo)) != 0 {
					return nil, geom.ErrInvalid
				}
				u.Operand = 1
				u.OriginalFace = tool.lowerCap
				u.Reversed = true
			}
			out.faces = append(out.faces, u)
		}
		for _, ring := range p.sides {
			for _, side := range ring {
				u := NativePrismIntervalWallUse{Use: NativePrismWallUse{Component: component, Face: side.wall, Operand: 0, Wall: side.wall, LowerCap: p.lowerCap, UpperCap: p.upperCap, LowerEdge: side.lower, UpperEdge: side.upper}}
				for end, z := range span {
					u.HeightParameter[end] = budget.op('/', budget.op('-', z, lo), height)
					u.Use.LowerParameter[end] = big.NewRat(int64(end), 1)
					u.Use.UpperParameter[end] = big.NewRat(int64(end), 1)
					if side.lowerReverse {
						u.Use.LowerParameter[end] = budget.op('-', big.NewRat(1, 1), u.Use.LowerParameter[end])
					}
					if side.upperReverse {
						u.Use.UpperParameter[end] = budget.op('-', big.NewRat(1, 1), u.Use.UpperParameter[end])
					}
				}
				out.walls = append(out.walls, u)
			}
		}
		out.volume = budget.op('+', out.volume, budget.op('*', area, budget.op('-', span[1], span[0])))
		out.components = append(out.components, solid)
	}
	if budget.err != nil {
		return nil, budget.err
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	out.work = budget.work
	return out, nil
}

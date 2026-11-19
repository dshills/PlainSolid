//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// NativePrismCavityUse preserves the original index of one complete graph item.
// Reversed applies to faces only: every cavity face is reversed, while stored
// edge directions and vertex coordinates retain their original meaning.
type NativePrismCavityUse struct {
	Index, Operand, Original int
	Reversed                 bool
}

// NativePrismCavityCut is an immutable, strictly nested two-shell solid.
type NativePrismCavityCut struct {
	solid                  brep.Solid
	owners                 [2]brep.Solid
	faces, edges, vertices []NativePrismCavityUse
	volume                 *big.Rat
	work                   int
}

func (s *NativePrismCavityCut) Solid() brep.Solid {
	if s == nil {
		return brep.Solid{}
	}
	return s.solid
}
func (s *NativePrismCavityCut) OriginalOwner(i int) (brep.Solid, error) {
	if s == nil || i < 0 || i > 1 {
		return brep.Solid{}, geom.ErrInvalid
	}
	return s.owners[i], nil
}
func (s *NativePrismCavityCut) Faces() []NativePrismCavityUse {
	if s == nil {
		return nil
	}
	return append([]NativePrismCavityUse(nil), s.faces...)
}
func (s *NativePrismCavityCut) Edges() []NativePrismCavityUse {
	if s == nil {
		return nil
	}
	return append([]NativePrismCavityUse(nil), s.edges...)
}
func (s *NativePrismCavityCut) Vertices() []NativePrismCavityUse {
	if s == nil {
		return nil
	}
	return append([]NativePrismCavityUse(nil), s.vertices...)
}
func (s *NativePrismCavityCut) ExactVolume() *big.Rat {
	if s == nil {
		return nil
	}
	return new(big.Rat).Set(s.volume)
}
func (s *NativePrismCavityCut) NativeOperations() int {
	if s == nil {
		return 0
	}
	return s.work
}

// CutContainedNativePrismCavity authenticates simple hole-free cardinal prisms
// and proves the tool lies strictly inside stock in both profile and height.
// It retains every original vertex and edge and reverses all tool face uses.
// The independent containment proof certifies cavity nesting; native validation
// certifies the two closed shell graphs. Contact and intersecting shells reject.
// Profile proof and native arithmetic use separate bounded work stages.
func CutContainedNativePrismCavity(ctx context.Context, stock, tool brep.Solid, o NativePrismBooleanOptions) (*NativePrismCavityCut, error) {
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
	profiles := [2]nativePrismProfile{}
	axis := -1
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
		if len(a.trim.Holes) > 0 || len(b.trim.Holes) > 0 {
			continue
		}
		if !(a.lo < b.lo && b.hi < a.hi) {
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
				xyz[candidate] = a.lo
				xyz[axes[0]], xyz[axes[1]] = u, v
				poles = append(poles, geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]})
			}
		}
		surface, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{bounds[0][0], bounds[0][0], bounds[0][1], bounds[0][1]}, []float64{bounds[1][0], bounds[1][0], bounds[1][1], bounds[1][1]}, nil)
		if e != nil {
			return nil, e
		}
		patch := intersect.Patch{Surface: surface, Min: geom.Vec2{X: bounds[0][0], Y: bounds[1][0]}, Max: geom.Vec2{X: bounds[0][1], Y: bounds[1][1]}}
		proof, e := intersect.SelectNoncontactingAffinePolygonRings(ctx, patch, patch, [2]intersect.AffinePolygonTrim{a.trim, b.trim}, intersect.AffineRingIntersection, o.Patches, o.Arrangement)
		if e != nil {
			return nil, e
		}
		rings := proof.Rings()
		if len(rings) != 1 || rings[0].Owner != 1 || rings[0].Ring != 0 {
			return nil, geom.ErrDomain
		}
		profiles = [2]nativePrismProfile{a, b}
		axis = candidate
		break
	}
	if axis < 0 {
		return nil, geom.ErrDomain
	}

	a, b := profiles[0], profiles[1]
	budget := &nativePrismAffineBudget{ctx: ctx, max: maximum, bits: bits}
	out := &NativePrismCavityCut{owners: [2]brep.Solid{stock, tool}}
	var vertices []geom.Vec3
	var edges []brep.Edge
	var faces []brep.Face
	if len(stock.Vertices())+len(tool.Vertices()) > limit {
		return nil, geom.ErrConvergence
	}
	for owner, s := range out.owners {
		vo, eo := len(vertices), len(edges)
		for i, v := range s.Vertices() {
			for _, x := range []float64{v.X, v.Y, v.Z} {
				budget.check(new(big.Rat).SetFloat64(x))
			}
			out.vertices = append(out.vertices, NativePrismCavityUse{len(vertices), owner, i, false})
			vertices = append(vertices, v)
		}
		for i, e := range s.Edges() {
			out.edges = append(out.edges, NativePrismCavityUse{len(edges), owner, i, false})
			edges = append(edges, brep.Edge{Start: e.Start + vo, End: e.End + vo})
		}
		for i, f := range s.Faces() {
			mapLoop := func(loop []brep.EdgeUse) []brep.EdgeUse {
				mapped := make([]brep.EdgeUse, len(loop))
				for j, u := range loop {
					u.Edge += eo
					if owner == 1 {
						u.Reversed = !u.Reversed
						mapped[len(loop)-1-j] = u
					} else {
						mapped[j] = u
					}
				}
				return mapped
			}
			f.Boundary = mapLoop(f.Boundary)
			for j := range f.Holes {
				f.Holes[j] = mapLoop(f.Holes[j])
			}
			out.faces = append(out.faces, NativePrismCavityUse{len(faces), owner, i, owner == 1})
			faces = append(faces, f)
		}
	}
	area := func(ring [][2]*big.Rat) *big.Rat {
		sum := new(big.Rat)
		for i, v := range ring {
			w := ring[(i+1)%len(ring)]
			sum = budget.op('+', sum, budget.op('-', budget.op('*', v[0], w[1]), budget.op('*', v[1], w[0])))
		}
		return budget.op('/', sum, big.NewRat(2, 1))
	}
	volume := func(p nativePrismProfile) *big.Rat {
		return budget.op('*', area(p.trim.Outer), budget.op('-', new(big.Rat).SetFloat64(p.hi), new(big.Rat).SetFloat64(p.lo)))
	}
	out.volume = budget.op('-', volume(a), volume(b))
	if budget.err != nil {
		return nil, budget.err
	}
	if out.volume.Sign() <= 0 {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	var e error
	out.solid, e = brep.NewSolid(vertices, edges, faces)
	if e != nil {
		return nil, e
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	out.work = budget.work
	return out, nil
}

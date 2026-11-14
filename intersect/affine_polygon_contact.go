package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AffinePolygonContactOptions bounds the complete original contact and trim
// arithmetic cumulatively. Support recovery retains its separate patch limits.
type AffinePolygonContactOptions struct{ MaxEdges, MaxOperations, MaxCoefficientBits int }

// AffinePolygonContact retains a complete convex trimmed contact and original
// zero-based trim side indices. The supplied loops need not have equal winding.
// Holes, repeated vertices and non-strictly-convex loops are outside this API.
type AffinePolygonContact struct {
	contact     *AffinePatchContact
	vertexSides [][2][]int
	edgeSides   [][2][]int
}

func (s *AffinePolygonContact) Contact() *AffinePatchContact {
	if s == nil {
		return nil
	}
	return s.contact
}
func copyAffineTrimSides(in [][2][]int) [][2][]int {
	out := make([][2][]int, len(in))
	for i := range in {
		for j := 0; j < 2; j++ {
			out[i][j] = append([]int(nil), in[i][j]...)
		}
	}
	return out
}
func (s *AffinePolygonContact) VertexTrimSides() [][2][]int {
	if s == nil {
		return nil
	}
	return copyAffineTrimSides(s.vertexSides)
}
func (s *AffinePolygonContact) EdgeTrimSides() [][2][]int {
	if s == nil {
		return nil
	}
	return copyAffineTrimSides(s.edgeSides)
}

type affineTrimBudget struct {
	ctx             context.Context
	work, max, bits int
	err             error
}

func (b *affineTrimBudget) check(q *big.Rat) {
	if b.err != nil {
		return
	}
	if e := b.ctx.Err(); e != nil {
		b.err = e
		return
	}
	b.work++
	if q == nil {
		b.err = geom.ErrInvalid
		return
	}
	if b.work > b.max || q.Num().BitLen() > b.bits || q.Denom().BitLen() > b.bits {
		b.err = geom.ErrConvergence
	}
}
func (b *affineTrimBudget) op(op byte, x, y *big.Rat) *big.Rat {
	if b.err != nil {
		return new(big.Rat)
	}
	q := new(big.Rat)
	switch op {
	case '+':
		q.Add(x, y)
	case '-':
		q.Sub(x, y)
	case '*':
		q.Mul(x, y)
	case '/':
		if y.Sign() == 0 {
			b.err = geom.ErrInvalid
			return q
		}
		q.Quo(x, y)
	}
	b.check(q)
	return q
}
func (b *affineTrimBudget) side(a, c, p [2]*big.Rat) *big.Rat {
	x := b.op('-', c[0], a[0])
	y := b.op('-', c[1], a[1])
	u := b.op('-', p[0], a[0])
	v := b.op('-', p[1], a[1])
	return b.op('-', b.op('*', x, v), b.op('*', y, u))
}
func affineContactUV(v geom.AffineChartIntersectionEndpoint, owner int) [2]*big.Rat {
	if owner == 0 {
		return v.A
	}
	return v.B
}

// IntersectConvexAffinePatches intersects two original affine patches restricted
// by strict convex rational UV loops. Every loop vertex must be in its original
// closed patch rectangle. Exact all-vertices half-plane checks prove convexity
// and simplicity before clipping; no display polygon or rounded UV is used.
// The returned immutable contact is a closed geometric set, not solid material.
func IntersectConvexAffinePatches(ctx context.Context, a, c Patch, loops [2][][2]*big.Rat, p geom.SurfacePatchOptions, o AffinePolygonContactOptions) (*AffinePolygonContact, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if o.MaxEdges == 0 {
		o.MaxEdges = 64
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxEdges < 3 || o.MaxEdges > 256 || o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 65536 {
		return nil, geom.ErrInvalid
	}
	b := &affineTrimBudget{ctx: ctx, max: o.MaxOperations, bits: o.MaxCoefficientBits}
	owners := [2]Patch{a, c}
	signs := [2]int{}
	for owner, loop := range loops {
		if !owners[owner].valid() || len(loop) < 3 || len(loop) > o.MaxEdges {
			return nil, geom.ErrInvalid
		}
		lo := [2]*big.Rat{new(big.Rat).SetFloat64(owners[owner].Min.X), new(big.Rat).SetFloat64(owners[owner].Min.Y)}
		hi := [2]*big.Rat{new(big.Rat).SetFloat64(owners[owner].Max.X), new(big.Rat).SetFloat64(owners[owner].Max.Y)}
		for _, v := range loop {
			for axis, q := range v {
				b.check(q)
				if b.err != nil {
					return nil, b.err
				}
				if q.Cmp(lo[axis]) < 0 || q.Cmp(hi[axis]) > 0 {
					return nil, geom.ErrDomain
				}
			}
		}
		for i, x := range loop {
			j := (i + 1) % len(loop)
			for k, v := range loop {
				if k == i || k == j {
					continue
				}
				s := b.side(x, loop[j], v).Sign()
				if b.err != nil {
					return nil, b.err
				}
				if s == 0 {
					return nil, geom.ErrDomain
				}
				if signs[owner] == 0 {
					signs[owner] = s
				}
				if s != signs[owner] {
					return nil, geom.ErrDomain
				}
			}
		}
	}
	if b.work >= b.max {
		return nil, geom.ErrConvergence
	}
	original, e := IntersectAffinePatches(ctx, a, c, p, geom.AffineChartIntersectionOptions{MaxOperations: b.max - b.work, MaxCoefficientBits: b.bits})
	if e != nil {
		return nil, e
	}
	b.work += original.work
	vertices, _ := original.Vertices()
	// Cyclic clipping also represents a segment as its two opposed edges and a
	// singleton as a self-edge; exact duplicate elimination handles degeneracies.
	for owner, loop := range loops {
		for i, x := range loop {
			if len(vertices) == 0 {
				break
			}
			next := []geom.AffineChartIntersectionEndpoint{}
			for j, v := range vertices {
				w := vertices[(j+1)%len(vertices)]
				sv := b.side(x, loop[(i+1)%len(loop)], affineContactUV(v, owner))
				sw := b.side(x, loop[(i+1)%len(loop)], affineContactUV(w, owner))
				if b.err != nil {
					return nil, b.err
				}
				insideV := sv.Sign()*signs[owner] >= 0
				insideW := sw.Sign()*signs[owner] >= 0
				if insideV {
					next = append(next, v)
				}
				if insideV != insideW {
					t := b.op('/', sv, b.op('-', sv, sw))
					q := geom.AffineChartIntersectionEndpoint{}
					lerp := func(x, y *big.Rat) *big.Rat { return b.op('+', x, b.op('*', t, b.op('-', y, x))) }
					for k := 0; k < 2; k++ {
						q.A[k] = lerp(v.A[k], w.A[k])
						q.B[k] = lerp(v.B[k], w.B[k])
					}
					for k := 0; k < 3; k++ {
						q.Point[k] = lerp(v.Point[k], w.Point[k])
					}
					if b.err != nil {
						return nil, b.err
					}
					next = append(next, q)
				}
			}
			vertices = nil
			for _, v := range next {
				duplicate := false
				for _, w := range vertices {
					if v.A[0].Cmp(w.A[0]) == 0 && v.A[1].Cmp(w.A[1]) == 0 {
						duplicate = true
						break
					}
				}
				if !duplicate {
					vertices = append(vertices, v)
				}
			}
			if len(vertices) > 2*o.MaxEdges+8 {
				return nil, geom.ErrConvergence
			}
		}
	}
	area := new(big.Rat)
	for i, v := range vertices {
		w := vertices[(i+1)%len(vertices)]
		area = b.op('+', area, b.op('-', b.op('*', v.A[0], w.A[1]), b.op('*', v.A[1], w.A[0])))
	}
	area = b.op('/', area, big.NewRat(2, 1))
	if b.err != nil {
		return nil, b.err
	}
	if area.Sign() < 0 {
		return nil, geom.ErrInvalid
	}
	if area.Sign() == 0 && len(vertices) > 2 {
		less := func(x, y geom.AffineChartIntersectionEndpoint) bool {
			if k := x.A[0].Cmp(y.A[0]); k != 0 {
				return k < 0
			}
			return x.A[1].Cmp(y.A[1]) < 0
		}
		low, high := vertices[0], vertices[0]
		for _, v := range vertices[1:] {
			if less(v, low) {
				low = v
			}
			if less(high, v) {
				high = v
			}
		}
		vertices = []geom.AffineChartIntersectionEndpoint{low, high}
	}
	s := &AffinePatchContact{owners: original.owners, exact: original.exact, coplanar: original.coplanar, orientation: original.orientation, vertices: vertices, area: area, squaredArea: new(big.Rat)}
	switch len(vertices) {
	case 0:
		s.kind = AffinePatchContactEmpty
	case 1:
		s.kind = AffinePatchContactPoint
	case 2:
		s.kind = AffinePatchContactSegment
	default:
		s.kind = AffinePatchContactArea
	}
	if area.Sign() > 0 {
		ratio := b.op('/', area, original.area)
		s.squaredArea = b.op('*', original.squaredArea, b.op('*', ratio, ratio))
	}
	out := &AffinePolygonContact{contact: s, vertexSides: make([][2][]int, len(vertices))}
	for i, v := range vertices {
		for owner, loop := range loops {
			uv := affineContactUV(v, owner)
			mask := uint8(0)
			bounds := []float64{owners[owner].Min.X, owners[owner].Max.X, owners[owner].Min.Y, owners[owner].Max.Y}
			for k, x := range bounds {
				if uv[k/2].Cmp(new(big.Rat).SetFloat64(x)) == 0 {
					mask |= 1 << k
				}
			}
			if owner == 0 {
				s.vertices[i].BoundaryA = mask
			} else {
				s.vertices[i].BoundaryB = mask
			}
			for j, x := range loop {
				if b.side(x, loop[(j+1)%len(loop)], uv).Sign() == 0 {
					out.vertexSides[i][owner] = append(out.vertexSides[i][owner], j)
				}
			}
		}
	}
	count := 0
	if s.kind == AffinePatchContactSegment {
		count = 1
	} else if s.kind == AffinePatchContactArea {
		count = len(vertices)
	}
	for i := 0; i < count; i++ {
		j := (i + 1) % len(vertices)
		x, y := s.vertices[i], s.vertices[j]
		s.edges = append(s.edges, AffinePatchContactEdge{Start: i, End: j, Boundary: [2]uint8{x.BoundaryA & y.BoundaryA, x.BoundaryB & y.BoundaryB}})
		sides := [2][]int{}
		for owner := 0; owner < 2; owner++ {
			for _, a := range out.vertexSides[i][owner] {
				for _, c := range out.vertexSides[j][owner] {
					if a == c {
						sides[owner] = append(sides[owner], a)
					}
				}
			}
		}
		out.edgeSides = append(out.edgeSides, sides)
	}
	if b.err != nil {
		return nil, b.err
	}
	s.work = b.work
	return out, nil
}

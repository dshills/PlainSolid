package loft

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// SegmentOccurrence identifies an original cap segment on a merged native edge.
// Start/End increase in input cyclic traversal, relative to its retained corner
// interval [0,1]. Reversed indicates opposition to the native edge direction.
type SegmentOccurrence struct {
	Edge       int
	Start, End *big.Rat
	Reversed   bool
}

// VertexOccurrence identifies an exact original point on a merged cap edge.
// NativeVertex is -1 for a removed collinear breakpoint. Parameter follows the
// original corner traversal, even when Reversed is true.
type VertexOccurrence struct {
	Edge, NativeVertex int
	Parameter          *big.Rat
	Reversed           bool
}
type SubdividedResult struct {
	Result
	Segments [2][]SegmentOccurrence
	Vertices [2][]VertexOccurrence
}

// PlanarSubdivided accepts unequal numbers of forward collinear breakpoints.
// Input index0 must be a genuine corner in both rings; retained corners must
// correspond in cyclic order. No geometric fitting or breakpoint welding occurs.
// Native merged caps omit collinear points; exact interval ancestry retains all
// original occurrences. Options.MaxVertices bounds each ORIGINAL ring, not only
// its simplified corners. All Planar proof and atomic failure rules apply.
func PlanarSubdivided(ctx context.Context, lower, upper []geom.Vec3, o Options) (SubdividedResult, error) {
	empty := SubdividedResult{}
	if ctx == nil {
		return empty, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	limit := o.MaxVertices
	if limit < 0 || limit > 4096 {
		return empty, ErrInvalid
	}
	if limit == 0 {
		limit = 256
	}
	maxBits := o.Support.MaxBits
	if maxBits < 0 {
		return empty, ErrInvalid
	}
	if maxBits == 0 {
		maxBits = 16384
	}
	checked := func(x *big.Rat) bool { return x.Num().BitLen() <= maxBits && x.Denom().BitLen() <= maxBits }
	input := [2][]geom.Vec3{lower, upper}
	var simplified [2][]geom.Vec3
	var corners [2][]int
	var points [2][]qpoint
	for k, ring := range input {
		n := len(ring)
		if n > limit {
			return empty, ErrLimit
		}
		if n < 3 {
			return empty, ErrInvalid
		}
		points[k] = make([]qpoint, n)
		for i, v := range ring {
			if !v.IsFinite() {
				return empty, ErrInvalid
			}
			points[k][i] = rat(v)
			for _, x := range points[k][i] {
				if !checked(x) {
					return empty, ErrLimit
				}
			}
		}
		for i := range ring {
			if e := ctx.Err(); e != nil {
				return empty, e
			}
			before := diff(points[k][i], points[k][(i+n-1)%n])
			after := diff(points[k][(i+1)%n], points[k][i])
			if zero(before) || zero(after) {
				return empty, ErrInvalid
			}
			if zero(cross(before, after)) {
				if dot(before, after).Sign() <= 0 {
					return empty, ErrInvalid
				}
				if i == 0 {
					return empty, ErrUnsupported
				}
				continue
			}
			corners[k] = append(corners[k], i)
			simplified[k] = append(simplified[k], ring[i])
		}
		if len(corners[k]) < 3 {
			return empty, ErrInvalid
		}
	}
	r, e := Planar(ctx, simplified[0], simplified[1], o)
	if e != nil {
		return empty, e
	}
	out := SubdividedResult{Result: r}
	nativeEdges := r.Solid.Edges()
	edgeMaps := [2][]int{r.LowerEdges, r.UpperEdges}
	vertexMaps := [2][]int{r.LowerVertices, r.UpperVertices}
	for k, ring := range input {
		n := len(ring)
		out.Segments[k] = make([]SegmentOccurrence, n)
		out.Vertices[k] = make([]VertexOccurrence, n)
		for j, start := range corners[k] {
			end := corners[k][(j+1)%len(corners[k])]
			if end == 0 {
				end = n
			}
			a, b := points[k][start], points[k][end%n]
			d := diff(b, a)
			axis := 0
			for d[axis].Sign() == 0 {
				axis++
			}
			id := edgeMaps[k][j]
			reversed := nativeEdges[id].Start != vertexMaps[k][j]
			parameter := func(i int) *big.Rat { return new(big.Rat).Quo(sub(points[k][i%n][axis], a[axis]), d[axis]) }
			for i := start; i < end; i++ {
				if e := ctx.Err(); e != nil {
					return empty, e
				}
				lo, hi := parameter(i), parameter(i+1)
				if !checked(lo) || !checked(hi) {
					return empty, ErrLimit
				}
				if lo.Sign() < 0 || hi.Cmp(big.NewRat(1, 1)) > 0 || lo.Cmp(hi) >= 0 {
					return empty, ErrUnsupported
				}
				out.Segments[k][i] = SegmentOccurrence{id, new(big.Rat).Set(lo), hi, reversed}
				native := -1
				if i == start {
					native = vertexMaps[k][j]
				}
				out.Vertices[k][i] = VertexOccurrence{id, native, lo, reversed}
			}
		}
	}
	return out, nil
}

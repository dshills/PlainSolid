// Package loft builds bounded exact planar corresponding-ring lofts.
package loft

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/support"
	"plainsolid/collision"
	"plainsolid/geom"
)

var ErrInvalid = errors.New("loft: invalid rings or options")
var ErrUnsupported = errors.New("loft: nonparallel caps or nonplanar/nonconvex correspondence")
var ErrLimit = errors.New("loft: resource limit")

type Options struct {
	MaxVertices int
	Support     support.Options
	Validation  collision.LocationOptions
}

// Result preserves exact input positions and original cyclic segment ancestry.
// Lower/UpperVertices and Lower/UpperEdges map input indices to native IDs;
// SideFaces[i] is the quad between corresponding original segments i and i+1.
// Caps are ordered lower,upper (names identify input rings, not world height).
type Result struct {
	Solid                                                                            brep.Solid
	ExactVolume                                                                      *big.Rat
	LowerVertices, UpperVertices, LowerEdges, UpperEdges, ConnectingEdges, SideFaces []int
	Caps                                                                             [2]int
	Supports                                                                         support.Report
}
type qpoint [3]*big.Rat

func rat(v geom.Vec3) qpoint {
	return qpoint{new(big.Rat).SetFloat64(v.X), new(big.Rat).SetFloat64(v.Y), new(big.Rat).SetFloat64(v.Z)}
}
func sub(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
func mul(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
func diff(a, b qpoint) qpoint    { return qpoint{sub(a[0], b[0]), sub(a[1], b[1]), sub(a[2], b[2])} }
func cross(a, b qpoint) qpoint {
	return qpoint{sub(mul(a[1], b[2]), mul(a[2], b[1])), sub(mul(a[2], b[0]), mul(a[0], b[2])), sub(mul(a[0], b[1]), mul(a[1], b[0]))}
}
func dot(a, b qpoint) *big.Rat {
	v := new(big.Rat)
	for i := range a {
		v.Add(v, mul(a[i], b[i]))
	}
	return v
}
func zero(a qpoint) bool { return a[0].Sign() == 0 && a[1].Sign() == 0 && a[2].Sign() == 0 }

// Planar connects two equally sized cyclic convex polygons without introducing
// diagonal side edges. Both clockwise and counterclockwise inputs are accepted
// when their correspondence is consistent. Caps must be parallel and distinct;
// every side quad must be exactly planar and the complete shell convex and
// embedded. All binary64 positions are interpreted exactly, never welded or
// repaired. No partial output is returned. Defaults bound each ring to256
// vertices (hard maximum4096); support and embedding have separate budgets.
func Planar(ctx context.Context, lower, upper []geom.Vec3, o Options) (Result, error) {
	return planarPair(ctx, lower, upper, o, true)
}
func planarPair(ctx context.Context, lower, upper []geom.Vec3, o Options, parallel bool) (Result, error) {
	empty := Result{}
	if ctx == nil {
		return empty, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxVertices < 0 || o.MaxVertices > 4096 {
		return empty, ErrInvalid
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 256
	}
	n := len(lower)
	if n > o.MaxVertices || len(upper) > o.MaxVertices {
		return empty, ErrLimit
	}
	if n < 3 || len(upper) != n {
		return empty, ErrInvalid
	}
	if e := preflightSizes(2*n, n+2, o); e != nil {
		return empty, e
	}
	vertices := append(append([]geom.Vec3(nil), lower...), upper...)
	points := make([]qpoint, len(vertices))
	for i, v := range vertices {
		if !v.IsFinite() {
			return empty, ErrInvalid
		}
		points[i] = rat(v)
	}
	normal := cross(diff(points[1], points[0]), diff(points[2], points[0]))
	other := cross(diff(points[n+1], points[n]), diff(points[n+2], points[n]))
	if zero(normal) || zero(other) || (parallel && !zero(cross(normal, other))) || dot(normal, other).Sign() <= 0 {
		return empty, ErrUnsupported
	}
	direction := dot(normal, diff(points[n], points[0])).Sign()
	if direction == 0 {
		return empty, ErrUnsupported
	}
	// Each cap is proved exactly planar before construction; no tolerance gate.
	for i := 0; i < n; i++ {
		if dot(normal, diff(points[i], points[0])).Sign() != 0 || dot(other, diff(points[n+i], points[n])).Sign() != 0 {
			return empty, ErrUnsupported
		}
	}
	loops := make([][]int, n+2)
	for i := 0; i < n; i++ {
		loops[0] = append(loops[0], n-1-i)
		loops[1] = append(loops[1], n+i)
		j := (i + 1) % n
		loops[i+2] = []int{i, j, n + j, n + i}
	}
	if direction < 0 {
		for _, loop := range loops {
			for i, j := 0, len(loop)-1; i < j; i, j = i+1, j-1 {
				loop[i], loop[j] = loop[j], loop[i]
			}
		}
	}
	edges := []brep.Edge{}
	faces := make([]brep.Face, len(loops))
	ids := map[[2]int]int{}
	for fi, loop := range loops {
		for i, a := range loop {
			b := loop[(i+1)%len(loop)]
			key := [2]int{a, b}
			if a > b {
				key = [2]int{b, a}
			}
			id, ok := ids[key]
			if !ok {
				id = len(edges)
				ids[key] = id
				edges = append(edges, brep.Edge{Start: key[0], End: key[1]})
			}
			faces[fi].Boundary = append(faces[fi].Boundary, brep.EdgeUse{Edge: id, Reversed: a > b})
		}
	}
	s, e := brep.NewSolid(vertices, edges, faces)
	if e != nil {
		return empty, e
	}
	so := o.Support
	so.Convex = true
	proof, e := support.Certify(ctx, s, so)
	if e != nil {
		return empty, e
	}
	if _, e = collision.NewPointLocator(ctx, s, o.Validation); e != nil {
		return empty, e
	}
	maxBits := o.Support.MaxBits
	if maxBits == 0 {
		maxBits = 16384
	}
	volume := new(big.Rat)
	for _, loop := range loops {
		a := points[loop[0]]
		for i := 1; i+1 < len(loop); i++ {
			volume.Add(volume, dot(a, cross(points[loop[i]], points[loop[i+1]])))
			if volume.Num().BitLen() > maxBits || volume.Denom().BitLen() > maxBits {
				return empty, ErrLimit
			}
		}
	}
	volume.Quo(volume, big.NewRat(6, 1))
	if volume.Sign() <= 0 {
		return empty, ErrUnsupported
	}
	r := Result{Solid: s, ExactVolume: volume, Supports: proof, Caps: [2]int{0, 1}}
	edgeID := func(a, b int) int {
		if a > b {
			a, b = b, a
		}
		return ids[[2]int{a, b}]
	}
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		r.LowerVertices = append(r.LowerVertices, i)
		r.UpperVertices = append(r.UpperVertices, n+i)
		r.LowerEdges = append(r.LowerEdges, edgeID(i, j))
		r.UpperEdges = append(r.UpperEdges, edgeID(n+i, n+j))
		r.ConnectingEdges = append(r.ConnectingEdges, edgeID(i, n+i))
		r.SideFaces = append(r.SideFaces, i+2)
	}
	return r, nil
}

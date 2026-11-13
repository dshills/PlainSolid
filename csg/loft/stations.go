package loft

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/support"
	"plainsolid/collision"
	"plainsolid/geom"
)

// StationOptions bounds rings and total input vertices in addition to Options.
// Defaults are32 stations and2048 total vertices; hard limits64 and16384.
type StationOptions struct {
	Options
	MaxStations, MaxTotalVertices int
}

// StationResult retains every original ring and band, including coplanar seams.
// RingVertices/Edges are indexed by station then original cyclic vertex/segment;
// BandFaces/ConnectingEdges by station interval then original cyclic segment.
type StationResult struct {
	Solid                                               brep.Solid
	ExactVolume                                         *big.Rat
	Supports                                            support.Report
	Caps                                                [2]int
	RingVertices, RingEdges, BandFaces, ConnectingEdges [][]int
}

// PlanarStations constructs one globally convex closed shell through strictly
// ordered parallel stations. Each side quadrilateral must be exactly planar.
// Coplanar consecutive patches retain distinct faces and their shared ring edge.
// All Planar exact incidence, embedding and atomic failure rules apply.
func PlanarStations(ctx context.Context, rings [][]geom.Vec3, options StationOptions) (StationResult, error) {
	return planarStations(ctx, rings, options, true)
}

func planarStations(ctx context.Context, rings [][]geom.Vec3, options StationOptions, convex bool) (StationResult, error) {
	empty := StationResult{}
	if ctx == nil {
		return empty, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if options.MaxStations < 0 || options.MaxStations > 64 || options.MaxTotalVertices < 0 || options.MaxTotalVertices > 16384 {
		return empty, ErrInvalid
	}
	if options.MaxStations == 0 {
		options.MaxStations = 32
	}
	if options.MaxTotalVertices == 0 {
		options.MaxTotalVertices = 2048
	}
	count := len(rings)
	if count > options.MaxStations {
		return empty, ErrLimit
	}
	if count < 2 {
		return empty, ErrInvalid
	}
	o := options.Options
	if o.MaxVertices < 0 || o.MaxVertices > 4096 {
		return empty, ErrInvalid
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 256
	}
	n := len(rings[0])
	if n < 3 {
		return empty, ErrInvalid
	}
	if n > o.MaxVertices || count > options.MaxTotalVertices/n {
		return empty, ErrLimit
	}
	vertices := make([]geom.Vec3, 0, n*count)
	for _, ring := range rings {
		if len(ring) != n {
			return empty, ErrInvalid
		}
		vertices = append(vertices, ring...)
	}
	points := make([]qpoint, len(vertices))
	for i, v := range vertices {
		if !v.IsFinite() {
			return empty, ErrInvalid
		}
		points[i] = rat(v)
	}
	normal := cross(diff(points[1], points[0]), diff(points[2], points[0]))
	if zero(normal) {
		return empty, ErrUnsupported
	}
	direction := dot(normal, diff(points[n], points[0])).Sign()
	if direction == 0 {
		return empty, ErrUnsupported
	}
	for k := 0; k < count; k++ {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		base := k * n
		other := cross(diff(points[base+1], points[base]), diff(points[base+2], points[base]))
		if zero(other) || !zero(cross(normal, other)) || dot(normal, other).Sign() <= 0 {
			return empty, ErrUnsupported
		}
		if k > 0 && dot(normal, diff(points[base], points[base-n])).Sign() != direction {
			return empty, ErrUnsupported
		}
		for i := 0; i < n; i++ {
			if dot(normal, diff(points[base+i], points[base])).Sign() != 0 {
				return empty, ErrUnsupported
			}
		}
	}
	loops := make([][]int, 2+(count-1)*n)
	for i := 0; i < n; i++ {
		loops[0] = append(loops[0], n-1-i)
		loops[1] = append(loops[1], (count-1)*n+i)
	}
	for k := 0; k < count-1; k++ {
		for i := 0; i < n; i++ {
			j := (i + 1) % n
			loops[2+k*n+i] = []int{k*n + i, k*n + j, (k+1)*n + j, (k+1)*n + i}
		}
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
	so.Convex = convex
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

	r := StationResult{Solid: s, ExactVolume: volume, Supports: proof, Caps: [2]int{0, 1}, RingVertices: make([][]int, count), RingEdges: make([][]int, count), BandFaces: make([][]int, count-1), ConnectingEdges: make([][]int, count-1)}
	edgeID := func(a, b int) int {
		if a > b {
			a, b = b, a
		}
		return ids[[2]int{a, b}]
	}
	for k := 0; k < count; k++ {
		for i := 0; i < n; i++ {
			j := (i + 1) % n
			r.RingVertices[k] = append(r.RingVertices[k], k*n+i)
			r.RingEdges[k] = append(r.RingEdges[k], edgeID(k*n+i, k*n+j))
			if k+1 < count {
				r.BandFaces[k] = append(r.BandFaces[k], 2+k*n+i)
				r.ConnectingEdges[k] = append(r.ConnectingEdges[k], edgeID(k*n+i, (k+1)*n+i))
			}
		}
	}
	return r, nil
}

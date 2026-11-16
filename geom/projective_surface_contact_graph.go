package geom

import (
	"context"
	"math/big"
)

// ProjectiveSurfaceContactGraphOptions bounds local coverage and regular
// global joining. Zero join limits are 16,384 arcs, one million endpoint
// comparisons, and four million exact operations.
type ProjectiveSurfaceContactGraphOptions struct {
	Cover                                          ProjectiveSurfaceContactCoverOptions
	MaxArcs, MaxJoinComparisons, MaxJoinOperations int
}

// ProjectiveContactVertex binds one exact common parameter point to both
// original chart coordinates. Solid topology is not implied.
type ProjectiveContactVertex struct {
	Common            [2]*ExactAlgebraicRoot
	OriginalUV        [2][2]*ExactAlgebraicRoot
	OnOverlapBoundary bool
}

// ProjectiveContactArc references one exact local cover arc and its global
// endpoint indices. If Saddle is true, Local is 2*line+direction in the
// cell saddle's Rays, and Vertices runs from the saddle center to the end.
type ProjectiveContactArc struct {
	Cell, Local int
	Vertices    [2]int
	Saddle      bool
}

type ProjectiveContactArcUse struct {
	Arc      int
	Reversed bool
}

// ProjectiveContactContour is a maximal chain through degree-two vertices.
// It can end on an overlap boundary or at a proved saddle junction.
type ProjectiveContactContour struct {
	Arcs   []ProjectiveContactArcUse
	Closed bool
}

// ProjectiveSurfaceContactGraph contains a complete contour graph from the
// original two surfaces. SaddleVertices contains only proved degree-four
// quadratic junctions, when explicitly requested. A failed join keeps the
// local Cover and publishes no partial global topology.
type ProjectiveSurfaceContactGraph struct {
	Cover                           ProjectiveSurfaceContactCover
	Vertices                        []ProjectiveContactVertex
	Arcs                            []ProjectiveContactArc
	Contours                        []ProjectiveContactContour
	SaddleVertices                  []int
	Complete                        bool
	JoinComparisons, JoinOperations int
	proof                           *projectiveContactGraphProof
}

// TraceProjectiveSurfaceContacts joins the exact regular local coverage
// across all rational common knots. A proved interior quadratic saddle may
// be included through Cover.IsolateQuadraticSaddles. Other higher-valence
// junctions, isolated/coplanar cells and zero arcs supported on a cell
// boundary remain unresolved here.
// Corner-only local cells are admitted only when their exact corner is
// authenticated by a retained arc elsewhere in the complete cover.
func TraceProjectiveSurfaceContacts(ctx context.Context, source, target *SurfacePatch, axes [2]int,
	o ProjectiveSurfaceContactGraphOptions) (r ProjectiveSurfaceContactGraph, err error) {
	if o.MaxArcs == 0 {
		o.MaxArcs = 16_384
	}
	if o.MaxJoinComparisons == 0 {
		o.MaxJoinComparisons = 1_000_000
	}
	if o.MaxJoinOperations == 0 {
		o.MaxJoinOperations = 4_000_000
	}
	if o.MaxArcs < 1 || o.MaxJoinComparisons < 1 || o.MaxJoinOperations < 1 {
		return r, ErrInvalid
	}
	o.Cover.Arcs.ResolveSubdivisionCorners = true
	r.Cover, err = BoundProjectiveSurfaceContacts(ctx, source, target, axes, o.Cover)
	if err != nil {
		return r, err
	}
	bits := o.Cover.Arcs.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	b := &polynomialBudget{maxOperations: o.MaxJoinOperations, maxBits: bits}
	defer func() { r.JoinOperations = b.operations }()
	var vertex []ProjectiveContactVertex
	var arcs []ProjectiveContactArc
	var corner [][2]*ExactAlgebraicRoot
	var saddles []int
	saddleVertex := make(map[int]bool)
	find := func(point [2]*ExactAlgebraicRoot) (int, error) {
		for i, q := range vertex {
			if err := ctx.Err(); err != nil {
				return -1, err
			}
			if r.JoinComparisons >= o.MaxJoinComparisons {
				return -1, ErrConvergence
			}
			r.JoinComparisons++
			equal, err := equalProjectiveContactEndpoint(ctx, point, q.Common, b)
			if err != nil {
				return -1, err
			}
			if equal {
				return i, nil
			}
		}
		return -1, nil
	}
	bind := func(point [2]*ExactAlgebraicRoot) (int, error) {
		id, err := find(point)
		if err != nil || id >= 0 {
			return id, err
		}
		q, err := projectiveContactVertex(point, r.Cover.Atlas.Overlap, b)
		if err != nil {
			return -1, err
		}
		id = len(vertex)
		vertex = append(vertex, q)
		return id, nil
	}
	for cell, isolate := range r.Cover.Isolations {
		if r.Cover.Saddles != nil && r.Cover.Saddles[cell].Proven {
			saddle := r.Cover.Saddles[cell]
			if len(arcs) > o.MaxArcs-4 {
				return r, ErrConvergence
			}
			before := len(vertex)
			center, err := bind(saddle.Center.Common)
			if err != nil {
				return r, err
			}
			if center != before || vertex[center].OnOverlapBoundary {
				return r, ErrConvergence
			}
			saddles = append(saddles, center)
			saddleVertex[center] = true
			for line, rays := range saddle.Rays {
				for direction, ray := range rays {
					end, err := bind(ray.End.Common)
					if err != nil {
						return r, err
					}
					if end == center {
						return r, ErrConvergence
					}
					arcs = append(arcs, ProjectiveContactArc{Cell: cell, Local: 2*line + direction, Vertices: [2]int{center, end}, Saddle: true})
				}
			}
			continue
		}
		if len(isolate.Tangencies)+len(isolate.Coplanar) != 0 || len(isolate.Arcs) > o.MaxArcs-len(arcs) ||
			len(isolate.CornerContacts) > o.MaxArcs-len(corner) {
			return r, ErrConvergence
		}
		common := r.Cover.Atlas.Cells[cell].Common
		for index, a := range isolate.Arcs {
			if a.OnBoundary {
				return r, ErrConvergence
			}
			use := ProjectiveContactArc{Cell: cell, Local: index}
			for end, ep := range a.Endpoints {
				point, err := exactProjectiveContactEndpoint(ep, common, b)
				if err != nil {
					return r, err
				}
				id, err := bind(point)
				if err != nil {
					return r, err
				}
				use.Vertices[end] = id
			}
			if use.Vertices[0] == use.Vertices[1] {
				return r, ErrConvergence
			}
			arcs = append(arcs, use)
		}
		for _, ep := range isolate.CornerContacts {
			x, err := exactProjectiveContactEndpoint(ep, common, b)
			if err != nil {
				return r, err
			}
			corner = append(corner, x)
		}
	}
	for _, x := range corner {
		id, err := find(x)
		if err != nil {
			return r, err
		}
		if id < 0 {
			return r, ErrConvergence
		}
	}
	links := make([][]int, len(vertex))
	for i, a := range arcs {
		for end, v := range a.Vertices {
			links[v] = append(links[v], 2*i+end)
		}
	}
	for v, edge := range links {
		if saddleVertex[v] {
			if len(edge) != 4 {
				return r, ErrConvergence
			}
		} else if len(edge) > 2 || len(edge) == 1 && !vertex[v].OnOverlapBoundary {
			return r, ErrConvergence
		}
	}
	var contours []ProjectiveContactContour
	seen := make([]bool, len(arcs))
	walk := func(entry int) (contour ProjectiveContactContour) {
		start := arcs[entry/2].Vertices[entry%2]
		for {
			id := entry / 2
			contour.Arcs = append(contour.Arcs, ProjectiveContactArcUse{Arc: id, Reversed: entry%2 != 0})
			seen[id] = true
			v := arcs[id].Vertices[1-entry%2]
			if v == start {
				contour.Closed = true
				return contour
			}
			if len(links[v]) != 2 {
				return contour
			}
			entry = links[v][0]
			if entry/2 == id {
				entry = links[v][1]
			}
		}
	}
	for _, group := range links {
		if len(group) != 2 {
			for _, edge := range group {
				if !seen[edge/2] {
					contours = append(contours, walk(edge))
				}
			}
		}
	}
	for i := range arcs {
		if !seen[i] {
			contours = append(contours, walk(2*i))
		}
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.Vertices, r.Arcs, r.Contours, r.Complete = vertex, arcs, contours, true
	r.SaddleVertices = saddles
	r.proof = sealProjectiveContactGraph(r.Cover, arcs)
	return r, nil
}

func exactProjectiveContactEndpoint(e SurfacePlaneEndpoint, area ExactSurfaceRectangle, b *polynomialBudget) ([2]*ExactAlgebraicRoot, error) {
	var out [2]*ExactAlgebraicRoot
	id := e.identity
	if id == nil || id.axis < 0 || id.axis > 1 || id.fixed == nil || id.lo == nil || id.hi == nil {
		return out, ErrConvergence
	}
	one := big.NewRat(1, 1)
	fixed := &ExactGraphLineRoot{polynomial: rationalPolynomial{new(big.Rat).Neg(id.fixed), one}, bounds: [2]*big.Rat{id.fixed, id.fixed}}
	variable := &ExactGraphLineRoot{bounds: [2]*big.Rat{id.lo, id.hi}}
	if id.lo.Cmp(id.hi) == 0 {
		variable.polynomial = rationalPolynomial{new(big.Rat).Neg(id.lo), one}
	} else {
		var err error
		variable.polynomial, err = physicalRootPolynomial(id, b)
		if err != nil {
			return out, err
		}
	}
	local := [2]*ExactGraphLineRoot{fixed, variable}
	if id.axis == 1 {
		local[0], local[1] = local[1], local[0]
	}
	for axis, section := range [2]ExactParameterInterval{area.U, area.V} {
		q, err := affineMapRoot(local[axis], section.Lower, new(big.Rat).Sub(section.Upper, section.Lower), b)
		if err != nil {
			return out, err
		}
		out[axis] = &ExactAlgebraicRoot{root: q}
	}
	return out, nil
}

func equalProjectiveContactEndpoint(ctx context.Context, a, c [2]*ExactAlgebraicRoot, b *polynomialBudget) (bool, error) {
	for i, x := range a {
		y := c[i]
		if x.root.bounds[1].Cmp(y.root.bounds[0]) < 0 || y.root.bounds[1].Cmp(x.root.bounds[0]) < 0 {
			return false, nil
		}
		if x.root.bounds[0].Cmp(x.root.bounds[1]) == 0 && y.root.bounds[0].Cmp(y.root.bounds[1]) == 0 {
			continue // Overlapping singleton intervals prove equality.
		}
		left := b.maxOperations - b.operations
		if left < 1 {
			return false, ErrConvergence
		}
		q, err := x.Compare(ctx, y, ExactCurvePlaneEventOptions{MaxOperations: left, MaxCoefficientBits: b.maxBits})
		b.operations += q.Operations
		if err != nil || !q.Proven {
			if err != nil {
				return false, err
			}
			return false, ErrConvergence
		}
		if q.Order != 0 {
			return false, nil
		}
	}
	return true, nil
}

func projectiveContactVertex(common [2]*ExactAlgebraicRoot, r ProjectiveSurfaceGraphOverlap,
	b *polynomialBudget) (out ProjectiveContactVertex, err error) {
	out.Common = common
	one := big.NewRat(1, 1)
	for a, x := range common {
		for _, bound := range [2]*big.Rat{new(big.Rat), one} {
			if x.root.bounds[0].Cmp(bound) <= 0 && x.root.bounds[1].Cmp(bound) >= 0 {
				v, err := polynomialValue(x.root.polynomial, bound, b)
				if err != nil {
					return out, err
				}
				out.OnOverlapBoundary = out.OnOverlapBoundary || v.Sign() == 0
			}
		}
		bAxis := a
		if r.SwapB {
			bAxis = 1 - a
		}
		y, err := projectiveBoundaryClockRoot(x.root, r.BStationRatios[a], b)
		if err != nil {
			return out, err
		}
		for owner, c := range [2]*ExactGraphLineRoot{x.root, y} {
			i := [2]int{a, bAxis}[owner]
			d := [2]ExactParameterInterval{r.Areas[owner].U, r.Areas[owner].V}[i]
			lo, hi := d.Lower, d.Upper
			if owner == 1 && [2]bool{r.ReverseBU, r.ReverseBV}[a] {
				lo, hi = hi, lo
			}
			station, err := affineMapRoot(c, lo, new(big.Rat).Sub(hi, lo), b)
			if err != nil {
				return out, err
			}
			out.OriginalUV[owner][i] = &ExactAlgebraicRoot{root: station}
		}
	}
	return out, nil
}

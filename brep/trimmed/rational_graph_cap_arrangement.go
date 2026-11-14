package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// RationalGraphCapCut identifies a new boundary endpoint on an original
// section curve. The parameter is normalized to that directed piece.
type RationalGraphCapCut struct {
	Source     RationalGraphCapEdgeSource
	Parameter  *big.Rat
	KeepBefore bool
	Point      [3]*big.Rat
}

// RationalGraphCapSeam links two entries of Cuts, directed with kept cap
// material on the left in Regions.ProjectionAxes.
type RationalGraphCapSeam struct{ From, To int }

// RationalGraphCapArrangement is a complete clipped section arrangement.
// Regions retains original curve ownership; every new line seam has a
// generated owner (Branch, Piece, Face are -1). Its caps can be built on any
// independently proved affine spline support on the original section plane.
type RationalGraphCapArrangement struct {
	Regions *RationalGraphShellRegions
	Cuts    []RationalGraphCapCut
	Seams   []RationalGraphCapSeam
}

// IntersectPolygonalHalfspace clips a certified planar section whose original
// boundaries consist of exactly affine normalized segments. Holes and all
// disconnected retained regions are supported. It intersects every original
// boundary edge, sorts the exact line contacts, and locates an exact interior
// probe in every gap before closing the retained boundary runs. Coincident
// intervals and contacts at original vertices fail atomically.
func (r *RationalGraphShellRegions) IntersectPolygonalHalfspace(ctx context.Context, points [3]geom.Vec3, keepPositive bool, o RationalGraphShellRegionOptions) (RationalGraphCapArrangement, error) {
	return r.intersectCapArrangement(ctx, points, keepPositive, o, true)
}

// IntersectRationalHalfspace clips a certified planar section with rational
// Bezier boundary pieces. Every original contact must be regular, interior
// to a piece, rational, and the only contact of that piece. Result regions,
// notches and seams retain their exact source curve geometry. Irregular or
// unresolved contacts return no arrangement.
func (r *RationalGraphShellRegions) IntersectRationalHalfspace(ctx context.Context, points [3]geom.Vec3, keepPositive bool, o RationalGraphShellRegionOptions) (RationalGraphCapArrangement, error) {
	return r.intersectCapArrangement(ctx, points, keepPositive, o, false)
}

func (r *RationalGraphShellRegions) intersectCapArrangement(ctx context.Context, points [3]geom.Vec3, keepPositive bool, o RationalGraphShellRegionOptions, polygonal bool) (RationalGraphCapArrangement, error) {
	var zero RationalGraphCapArrangement
	if ctx == nil || r == nil || r.certificate == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	for _, p := range points {
		if !p.IsFinite() {
			return zero, geom.ErrInvalid
		}
	}
	other, e := polynomialPlaneFromPoints(points)
	if e != nil {
		return zero, e
	}
	keep := -1
	if keepPositive {
		keep = 1
	}
	return r.intersectGeneralCap(ctx, other, keep, o, polygonal)
}

func (r *RationalGraphShellRegions) intersectGeneralCap(ctx context.Context, other polynomialPlaneEquation, keep int, o RationalGraphShellRegionOptions, polygonal bool) (RationalGraphCapArrangement, error) {
	var zero RationalGraphCapArrangement
	opts, e := o.Topology.WithDefaults()
	if e != nil {
		return zero, e
	}
	if opts.MaxEdges > 4096 {
		return zero, geom.ErrInvalid
	}
	runs, cuts, e := r.capBoundarySections(ctx, other, keep, &opts, polygonal)
	if e != nil {
		return zero, e
	}
	if len(cuts)%2 != 0 || len(cuts)+len(runs) > opts.MaxEdges {
		return zero, geom.ErrConvergence
	}
	// The line tangent (b,-a) puts a*u+b*v+c>0 to its left.
	form := r.capHalfspaceForm(other)
	axis, direction := 0, form[1].Sign()
	if direction == 0 {
		axis, direction = 1, -form[0].Sign()
	}
	direction *= other.orientation * keep
	axes := orientedRationalGraphProjection(r.plane)
	coordinate := axes[axis]
	order := make([]int, len(cuts))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		return cuts[order[i]].Point[coordinate].Cmp(cuts[order[j]].Point[coordinate])*direction < 0
	})
	var seams []RationalGraphCapSeam
	from := make([]int, len(cuts))
	for i := range from {
		from[i] = -1
	}
	for i := 1; i < len(order); i++ {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		a, b := order[i-1], order[i]
		if cuts[a].Point[coordinate].Cmp(cuts[b].Point[coordinate]) == 0 {
			return zero, geom.ErrConvergence
		}
		var mid [3]*big.Rat
		for j := range mid {
			mid[j] = new(big.Rat).Quo(new(big.Rat).Add(cuts[a].Point[j], cuts[b].Point[j]), big.NewRat(2, 1))
			if mid[j].Num().BitLen() > opts.MaxBits || mid[j].Denom().BitLen() > opts.MaxBits {
				return zero, geom.ErrConvergence
			}
		}
		loc, p, e := r.LocateRational(ctx, mid)
		if e != nil {
			return zero, e
		}
		opts.MaxWork -= p.Work
		opts.MaxSubdivisions -= p.Subdivisions
		if opts.MaxWork < 1 || opts.MaxSubdivisions < 1 {
			return zero, geom.ErrConvergence
		}
		if p.Status != trimvalidate.Certified || loc != trimvalidate.Inside && loc != trimvalidate.Outside {
			return zero, geom.ErrConvergence
		}
		if loc == trimvalidate.Inside {
			if !cuts[a].KeepBefore || cuts[b].KeepBefore || from[a] >= 0 {
				return zero, geom.ErrConvergence
			}
			from[a] = b
			seams = append(seams, RationalGraphCapSeam{a, b})
		}
	}
	if len(seams)*2 != len(cuts) {
		return zero, geom.ErrConvergence
	}
	byStart := make(map[int]int, len(runs))
	for i, run := range runs {
		if run.start >= 0 {
			byStart[run.start] = i
		}
	}
	var wires [][]projectedGraphSectionPiece
	seen := make([]bool, len(runs))
	for i, run := range runs {
		if seen[i] {
			continue
		}
		if run.start < 0 {
			wires = append(wires, run.pieces)
			seen[i] = true
			continue
		}
		var wire []projectedGraphSectionPiece
		for j := i; ; {
			if seen[j] {
				if j != i {
					return zero, geom.ErrConvergence
				}
				break
			}
			seen[j] = true
			path := runs[j]
			wire = append(wire, path.pieces...)
			k, ok := byStart[from[path.end]]
			if !ok {
				return zero, geom.ErrConvergence
			}
			a, b := cuts[path.end].Point, cuts[from[path.end]].Point
			h := [][3]*big.Rat{{a[axes[0]], a[axes[1]], big.NewRat(1, 1)}, {}, {b[axes[0]], b[axes[1]], big.NewRat(1, 1)}}
			for c := range h[0] {
				h[1][c] = new(big.Rat).Quo(new(big.Rat).Add(h[0][c], h[2][c]), big.NewRat(2, 1))
			}
			wire = append(wire, projectedGraphSectionPiece{source: RationalGraphCapEdgeSource{Wire: -1, Branch: -1, Piece: -1, Face: -1}, control: h})
			j = k
		}
		wires = append(wires, wire)
	}
	count := 0
	rings := make([]trimvalidate.Loop, len(wires))
	for i, wire := range wires {
		for _, piece := range wire {
			count++
			if count > opts.MaxEdges {
				return zero, geom.ErrConvergence
			}
			p, e := trimvalidate.ExactBezier(piece.control)
			if e != nil {
				return zero, e
			}
			rings[i] = append(rings[i], p)
		}
	}
	regions, e := certifyProjectedRationalGraphRegions(ctx, wires, rings, r.plane, opts)
	if e != nil {
		return zero, e
	}
	out := RationalGraphCapArrangement{Regions: regions, Seams: seams}
	for _, cut := range cuts {
		out.Cuts = append(out.Cuts, RationalGraphCapCut{cut.Source, cut.Parameter, cut.KeepBefore, cut.Point})
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}

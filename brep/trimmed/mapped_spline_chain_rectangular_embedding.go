package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/geom"
)

// MappedChainRectangularEmbedding certifies a continuous outward source shell
// as the boundary of a positive-height region over a model-XY rectangle. The
// piecewise roof retains its two original spline owners and exact shared edge.
// Walls are indexed by xmin, xmax, ymin, ymax. Clearance is a lower bound
// on the vertical roof-to-floor distance over the entire rectangle.
type MappedChainRectangularEmbedding struct {
	Footprint   [2][2]*big.Rat
	FloorHeight *big.Rat
	Clearance   *big.Rat
	FloorFace   int
	WallFaces   [4]int
}

type mappedChainEmbeddingPlane struct {
	axis     int
	constant *big.Rat
	reversed bool
	trims    [][][3]*big.Rat
}

// CertifyRectangularEmbedding proves source embedding from the original chart
// regions, without using their display meshes. It requires two affine-XY graph
// roofs, one horizontal rectangular floor, and four distinct vertical wall
// charts. The common roof chain stays inside the footprint except at its two
// endpoints. Each exact upper wall profile covers its complete rectangle side.
// Proof budgets apply independently to support extraction, affine charts,
// graph projections and exact control evaluations.
func (r MappedSplineChainShell) CertifyRectangularEmbedding(ctx context.Context, o geom.SurfacePatchOptions) (MappedChainRectangularEmbedding, error) {
	var zero MappedChainRectangularEmbedding
	if ctx == nil || r.topology.Faces != 7 || len(r.planar) != 5 {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	cert := MappedChainRectangularEmbedding{FloorFace: -1, WallFaces: [4]int{-1, -1, -1, -1}}
	var planes [5]mappedChainEmbeddingPlane
	for i, p := range r.planar {
		var err error
		planes[i], err = chartMappedChainEmbeddingPlane(ctx, p, o)
		if err != nil {
			return zero, fmt.Errorf("embedding face %d: %w", i+2, err)
		}
		if planes[i].axis == 2 {
			if cert.FloorFace >= 0 {
				return zero, geom.ErrConvergence
			}
			cert.FloorFace = i + 2
		}
	}
	if cert.FloorFace < 0 {
		return zero, geom.ErrConvergence
	}
	floor := planes[cert.FloorFace-2]
	cert.FloorHeight = floor.constant
	footprint, floorSides, err := proveMappedChainFloor(floor)
	if err != nil {
		return zero, fmt.Errorf("embedding floor %d: %w", cert.FloorFace, err)
	}
	cert.Footprint = footprint

	// The analytic shell proof already authenticated every coedge pair.
	// Inspect its incidence here to bind each roof and floor trace to a wall.
	mates := make([][]MappedChainClosedUse, r.topology.Faces)
	for i, p := range r.planar {
		mates[i+2] = make([]MappedChainClosedUse, len(p.Outer))
	}
	for _, s := range r.seams {
		for i, a := range s {
			if a.Face >= 2 {
				mates[a.Face][a.Trim] = s[1-i]
			}
		}
	}
	for i, p := range planes {
		if p.axis == 2 {
			continue
		}
		for j, bound := range footprint[p.axis] {
			if p.constant.Cmp(bound) != 0 {
				continue
			}
			k := 2*p.axis + j
			if cert.WallFaces[k] != -1 {
				return zero, geom.ErrConvergence
			}
			cert.WallFaces[k] = i + 2
		}
	}

	for side, f := range cert.WallFaces {
		if f < 0 {
			return zero, geom.ErrConvergence
		}
		if err := proveMappedChainEmbeddingWall(cert, planes[f-2], side, mates[f], floorSides); err != nil {
			return zero, fmt.Errorf("embedding wall %d: %w", f, err)
		}
	}
	for owner, surface := range r.boundary.supports {
		patch, err := exactSplineSupportPatch(surface, o)
		if err != nil {
			return zero, err
		}
		graph, err := geom.NewAffineXYGraph(ctx, patch, o)
		if err != nil {
			return zero, fmt.Errorf("embedding roof %d projection: %w", owner, err)
		}
		if (graph.Orientation() < 0) != r.reversed[owner] {
			return zero, fmt.Errorf("embedding roof %d orientation: %w", owner, geom.ErrConvergence)
		}
		box, err := patch.Bounds()
		if err != nil {
			return zero, err
		}
		clearance := new(big.Rat).Sub(ratInset(box.Min.Z), cert.FloorHeight)
		if clearance.Sign() <= 0 {
			return zero, fmt.Errorf("embedding roof %d clearance: %w", owner, geom.ErrConvergence)
		}
		if cert.Clearance == nil || cert.Clearance.Cmp(clearance) > 0 {
			cert.Clearance = clearance
		}
		if owner == 0 {
			if err := r.boundary.proveMappedChainInteriorProjection(ctx, graph, footprint, o); err != nil {
				return zero, fmt.Errorf("embedding roof chain: %w", err)
			}
		}
	}
	// Each roof projects injectively and positively. The common trace cancels in
	// the sum of their projected boundaries. The upper wall traces leave exactly
	// the positive rectangle boundary, so winding proves full coverage with
	// disjoint interiors. Original seam agreement glues equal roof heights;
	// the distinct wall planes and strict clearance exclude all other contacts.
	return cert, nil
}

func chartMappedChainEmbeddingPlane(ctx context.Context, f MappedChainPlanarFace, o geom.SurfacePatchOptions) (mappedChainEmbeddingPlane, error) {
	var zero mappedChainEmbeddingPlane
	p, err := exactSplineSupportPatch(f.Surface, o)
	if err != nil {
		return zero, err
	}
	c, err := geom.NewAffineSurfaceChart(ctx, p, geom.AffineSurfaceChartOptions{
		MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
	})
	if err != nil {
		return zero, err
	}
	d := c.Domain()
	var chartCorners [3][3]*big.Rat
	for i, uv := range [3][2]float64{{d.UMin, d.VMin}, {d.UMax, d.VMin}, {d.UMin, d.VMax}} {
		chartCorners[i], err = c.EvalExact(ratInset(uv[0]), ratInset(uv[1]))
		if err != nil {
			return zero, err
		}
	}
	plane := mappedChainEmbeddingPlane{axis: -1, reversed: f.Reversed}
	for k, q := range chartCorners[0] {
		if q.Cmp(chartCorners[1][k]) == 0 && q.Cmp(chartCorners[2][k]) == 0 {
			plane.axis, plane.constant = k, q
		}
	}
	if plane.axis < 0 {
		return zero, geom.ErrConvergence
	}
	for _, trim := range f.Outer {
		h, err := mixedExactControls(trim)
		if err != nil {
			return zero, err
		}
		controls := make([][3]*big.Rat, len(h))
		for j, uv := range h {
			u := new(big.Rat).Quo(uv[0], uv[2])
			v := new(big.Rat).Quo(uv[1], uv[2])
			controls[j], err = c.EvalExact(u, v)
			if err != nil {
				return zero, err
			}
		}
		plane.trims = append(plane.trims, controls)
	}
	return plane, nil
}

func proveMappedChainFloor(f mappedChainEmbeddingPlane) ([2][2]*big.Rat, [4]int, error) {
	var footprint [2][2]*big.Rat
	sides := [4]int{-1, -1, -1, -1}
	if len(f.trims) != 4 {
		return footprint, sides, geom.ErrConvergence
	}
	for i, trim := range f.trims {
		for axis := 0; axis < 2; axis++ {
			x := trim[0][axis]
			if i == 0 || x.Cmp(footprint[axis][0]) < 0 {
				footprint[axis][0] = x
			}
			if i == 0 || x.Cmp(footprint[axis][1]) > 0 {
				footprint[axis][1] = x
			}
		}
	}
	for i, trim := range f.trims {
		for axis := 0; axis < 2; axis++ {
			if trim[0][axis].Cmp(trim[len(trim)-1][axis]) != 0 {
				continue
			}
			for hi, coord := range footprint[axis] {
				if !mappedChainConstantCoordinate(trim, axis, coord) {
					continue
				}
				side := 2*axis + hi
				span, direction := mappedChainMonotoneSpan(trim, 1-axis)
				if direction == 0 || span[0].Cmp(footprint[1-axis][0]) != 0 || span[1].Cmp(footprint[1-axis][1]) != 0 ||
					sides[side] >= 0 || orientedMappedChainDirection(direction, f.reversed) != -mappedChainRectangleDirection(side) {
					return footprint, sides, geom.ErrConvergence
				}
				sides[side] = i
			}
		}
	}
	if slices.Contains(sides[:], -1) {
		return footprint, sides, geom.ErrConvergence
	}
	return footprint, sides, nil
}

func proveMappedChainEmbeddingWall(cert MappedChainRectangularEmbedding, p mappedChainEmbeddingPlane, side int, mates []MappedChainClosedUse, floorSides [4]int) error {
	var floorCount int
	var vertical [2]bool
	var roofSpans [][2]*big.Rat
	rangeAxis := 1 - p.axis
	interval := cert.Footprint[rangeAxis]
	for i, trace := range p.trims {
		m := mates[i]
		switch {
		case m.Face < 2:
			span, direction := mappedChainMonotoneSpan(trace, rangeAxis)
			if direction == 0 || orientedMappedChainDirection(direction, p.reversed) != -mappedChainRectangleDirection(side) {
				return geom.ErrConvergence
			}
			roofSpans = append(roofSpans, span)
		case m.Face == cert.FloorFace:
			if m.Trim != floorSides[side] {
				return geom.ErrConvergence
			}
			floorCount++
		default:
			// The wall-wall pairing must connect the rectangle's actual adjacent
			// sides. Its curve stays on their common vertical line.
			end := -1
			for j, q := range interval {
				if mappedChainConstantCoordinate(trace, rangeAxis, q) {
					end = j
				}
			}
			if end < 0 || vertical[end] || m.Face != cert.WallFaces[2*rangeAxis+end] {
				return geom.ErrConvergence
			}
			span, direction := mappedChainMonotoneSpan(trace, 2)
			if direction == 0 || span[0].Cmp(cert.FloorHeight) != 0 || span[1].Cmp(cert.FloorHeight) <= 0 {
				return geom.ErrConvergence
			}
			vertical[end] = true
		}
	}
	if floorCount != 1 || !vertical[0] || !vertical[1] || len(roofSpans) == 0 {
		return geom.ErrConvergence
	}
	slices.SortFunc(roofSpans, func(a, b [2]*big.Rat) int { return a[0].Cmp(b[0]) })
	cursor := interval[0]
	for _, span := range roofSpans {
		if span[0].Cmp(cursor) != 0 {
			return geom.ErrConvergence
		}
		cursor = span[1]
	}
	if cursor.Cmp(interval[1]) != 0 {
		return geom.ErrConvergence
	}
	return nil
}

func mappedChainRectangleDirection(side int) int {
	if side == 0 || side == 3 {
		return -1
	}
	return 1
}

func orientedMappedChainDirection(direction int, reversed bool) int {
	if reversed {
		return -direction
	}
	return direction
}

func mappedChainConstantCoordinate(controls [][3]*big.Rat, axis int, coordinate *big.Rat) bool {
	for _, p := range controls {
		if p[axis].Cmp(coordinate) != 0 {
			return false
		}
	}
	return true
}

// For positive-weight rational Béziers, monotone scalar controls imply a
// monotone scalar coordinate. The derivative numerator pairs Bernstein terms
// i<j with positive factors times (value[j]-value[i]), so distinct endpoint
// values make the derivative strict on the open interval.
func mappedChainMonotoneSpan(controls [][3]*big.Rat, axis int) ([2]*big.Rat, int) {
	start, end := controls[0][axis], controls[len(controls)-1][axis]
	direction := end.Cmp(start)
	for j := 1; j < len(controls); j++ {
		if direction*controls[j][axis].Cmp(controls[j-1][axis]) < 0 {
			return [2]*big.Rat{}, 0
		}
	}
	if direction < 0 {
		return [2]*big.Rat{end, start}, direction
	}
	return [2]*big.Rat{start, end}, direction
}

func (r MappedSplineChainAgreement) proveMappedChainInteriorProjection(ctx context.Context, graph *geom.AffineXYGraph, bounds [2][2]*big.Rat, o geom.SurfacePatchOptions) error {
	eval := geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	for i := 0; i+1 < len(r.cuts); i++ {
		q, err := r.sources[0].cell(r.cuts[i], r.cuts[i+1])
		if err != nil {
			return err
		}
		h := q.HomogeneousControls()
		var strict [2][2]bool
		for j, uv := range h {
			jet, err := graph.EvaluateRational(ctx, [2]*big.Rat{new(big.Rat).Quo(uv[0], uv[2]), new(big.Rat).Quo(uv[1], uv[2])}, eval)
			if err != nil {
				return err
			}
			on := 0
			for axis, bound := range bounds {
				x := jet.Point[axis]
				lo, hi := x.Cmp(bound[0]), x.Cmp(bound[1])
				if lo < 0 || hi > 0 {
					return geom.ErrConvergence
				}
				strict[axis][0] = strict[axis][0] || lo > 0
				strict[axis][1] = strict[axis][1] || hi < 0
				if lo == 0 || hi == 0 {
					on++
				}
			}
			atFirst := j == 0
			atLast := j == len(h)-1
			if i == 0 && atFirst || i+2 == len(r.cuts) && atLast {
				if on != 1 {
					return geom.ErrConvergence
				}
			} else if (atFirst || atLast) && on != 0 {
				return geom.ErrConvergence
			}
		}
		for _, axis := range strict {
			if !axis[0] || !axis[1] {
				return geom.ErrConvergence
			}
		}
	}
	return nil
}

package geom

import (
	"context"
	"math/big"
	"slices"
)

// ClippedPolynomialHeightOwner names an original support of a wall trace.
type ClippedPolynomialHeightOwner uint8

const (
	ClippedPolynomialNoHeight ClippedPolynomialHeightOwner = iota
	ClippedPolynomialBaseHeight
	ClippedPolynomialRoofAHeight
	ClippedPolynomialRoofBHeight
)

// ClippedPolynomialWall is the open interval between two original support
// traces along a boundary segment. Its zero value means there is no wall.
type ClippedPolynomialWall struct {
	Lower, Upper ClippedPolynomialHeightOwner
}

// ClippedPolynomialStockBoundary is a maximal open interval between tile
// corners and original contact terminals. Points increase along the varying
// physical axis. Tiles gives the incident tile index on the left and right
// of this direction, with -1 for the exterior. OverlapRegion gives the
// original region touching this interval, or -1 away from the overlap.
// EndingRoofs identifies whose original stock wall lies on this interval
// (bit 0 is A and bit 1 is B). It is zero at a tile subdivision within a
// single stock. OwnerUV indexes the base and the incident roofs, then
// endpoints, then UV.
// Common, Fuse and Cut give exposed walls on each side. Cut's first index
// selects the difference order, and its second index selects the side.
type ClippedPolynomialStockBoundary struct {
	Points        [2][2]*big.Rat
	Tiles         [2]int
	OverlapRegion int
	EndingRoofs   uint8
	OwnerUV       [3][2][2]*big.Rat
	Common, Fuse  [2]ClippedPolynomialWall
	Cut           [2][2]ClippedPolynomialWall
}

type clippedMaterialLine struct {
	axis  int
	fixed float64
}

type clippedMaterialSide struct {
	lo, hi *big.Rat
	tile   int
	side   int
}

func clippedMaterialBoundaries(ctx context.Context, overlap ClippedSplineGraphPolynomialArrangement, tiles []ClippedPolynomialStockTile,
	maps [3][2][3]*big.Rat, budget *polynomialBudget) ([]ClippedPolynomialStockBoundary, error) {
	lines := make(map[clippedMaterialLine][]clippedMaterialSide)
	var keys []clippedMaterialLine
	for i, tile := range tiles {
		d := tile.Physical
		for side, key := range [4]clippedMaterialLine{{0, d.VMin}, {1, d.UMax}, {0, d.VMax}, {1, d.UMin}} {
			lo, hi := d.UMin, d.UMax
			if key.axis == 1 {
				lo, hi = d.VMin, d.VMax
			}
			if _, exists := lines[key]; !exists {
				keys = append(keys, key)
			}
			lines[key] = append(lines[key], clippedMaterialSide{new(big.Rat).SetFloat64(lo), new(big.Rat).SetFloat64(hi), i, side})
		}
	}
	slices.SortFunc(keys, func(a, b clippedMaterialLine) int {
		if a.axis != b.axis {
			return a.axis - b.axis
		}
		if a.fixed < b.fixed {
			return -1
		}
		if a.fixed > b.fixed {
			return 1
		}
		return 0
	})

	arrangement := overlap.Projection.Arrangement
	d := arrangement.Domain
	mins := [2]*big.Rat{new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin)}
	maxs := [2]*big.Rat{new(big.Rat).SetFloat64(d.UMax), new(big.Rat).SetFloat64(d.VMax)}
	var out []ClippedPolynomialStockBoundary
	for _, line := range keys {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		sides := lines[line]
		fixed := new(big.Rat).SetFloat64(line.fixed)
		var stations []*big.Rat
		for _, side := range sides {
			stations = append(stations, side.lo, side.hi)
		}
		for _, v := range arrangement.Vertices[:4] {
			if v.Parameters[1-line.axis].Cmp(fixed) == 0 {
				stations = append(stations, v.Parameters[line.axis])
			}
		}
		slices.SortFunc(stations, (*big.Rat).Cmp)
		stations = slices.CompactFunc(stations, func(a, b *big.Rat) bool { return a.Cmp(b) == 0 })
		for i, lo := range stations[:len(stations)-1] {
			hi := stations[i+1]
			edge := ClippedPolynomialStockBoundary{
				Points: [2][2]*big.Rat{graphPolynomialUV(line.axis, lo, fixed), graphPolynomialUV(line.axis, hi, fixed)},
				Tiles:  [2]int{-1, -1}, OverlapRegion: -1,
			}
			var masks [2]uint8
			var overlapSide int
			for _, side := range sides {
				if lo.Cmp(side.lo) < 0 || hi.Cmp(side.hi) > 0 {
					continue
				}
				// Bottom and right have their tiles to the left of the
				// positive varying axis. Top and left have the opposite.
				incident := side.side / 2
				if edge.Tiles[incident] != -1 {
					return nil, ErrConvergence
				}
				edge.Tiles[incident], masks[incident] = side.tile, tiles[side.tile].RoofMask
				if side.tile == 0 {
					overlapSide = side.side
				}
			}
			owners := masks[0] | masks[1]
			if owners == 0 {
				continue
			}
			edge.EndingRoofs = masks[0] ^ masks[1]
			order := [2]ClippedPolynomialHeightOwner{ClippedPolynomialRoofAHeight, ClippedPolynomialRoofBHeight}
			if masks[0] == 3 || masks[1] == 3 {
				if masks[0]&masks[1] == 3 || edge.EndingRoofs != overlap.BoundaryRoofs[overlapSide] {
					return nil, ErrConvergence
				}
				mid := new(big.Rat).Quo(new(big.Rat).Add(lo, hi), big.NewRat(2, 1))
				if e := budget.check(mid); e != nil {
					return nil, e
				}
				region, e := clippedMaterialBoundaryRegion(arrangement, graphPolynomialUV(line.axis, mid, fixed), mins, maxs)
				if e != nil {
					return nil, e
				}
				edge.OverlapRegion = region
				if arrangement.Regions[region].HeightSide > 0 {
					order[0], order[1] = order[1], order[0]
				}
			} else if owners == 3 {
				// A shared A-only/B-only interval would need separate
				// height ordering outside the proved overlap.
				return nil, ErrConvergence
			}
			for k, mapping := range maps {
				if k > 0 && owners&(1<<(k-1)) == 0 {
					continue
				}
				for j, xy := range edge.Points {
					uv, e := clippedMaterialMapUV(mapping, xy, budget)
					if e != nil {
						return nil, e
					}
					edge.OwnerUV[k][j] = uv
				}
			}
			if e := edge.assignWalls(order, masks); e != nil {
				return nil, e
			}
			out = append(out, edge)
		}
	}
	return out, nil
}

// Perimeter positions use CCW side order and an increasing exact coordinate
// along a side. A boundary segment interior cannot contain a corner or a
// terminal after atomization. Its midpoint therefore belongs to exactly one
// proved perimeter arc. Height order comes from that arc's original region.
type clippedMaterialPerimeterPosition struct {
	side  int
	along *big.Rat
}

func (p clippedMaterialPerimeterPosition) cmp(q clippedMaterialPerimeterPosition) int {
	if p.side != q.side {
		return p.side - q.side
	}
	return p.along.Cmp(q.along)
}

func clippedMaterialBoundaryRegion(arrangement SplineGraphPolynomialArrangement, interior, mins, maxs [2]*big.Rat) (int, error) {
	position := func(p [2]*big.Rat) clippedMaterialPerimeterPosition {
		if p[1].Cmp(mins[1]) == 0 {
			return clippedMaterialPerimeterPosition{0, p[0]}
		}
		if p[0].Cmp(maxs[0]) == 0 {
			return clippedMaterialPerimeterPosition{1, p[1]}
		}
		if p[1].Cmp(maxs[1]) == 0 {
			return clippedMaterialPerimeterPosition{2, new(big.Rat).Neg(p[0])}
		}
		return clippedMaterialPerimeterPosition{3, new(big.Rat).Neg(p[1])}
	}
	mid := position(interior)
	selected := -1
	for i, region := range arrangement.Regions {
		arc := region.Perimeter
		if arc == nil {
			continue
		}
		if arc.ParameterDirection != 1 {
			return -1, ErrConvergence
		}
		start, end := position(arrangement.Vertices[arc.Start].Parameters), position(arrangement.Vertices[arc.End].Parameters)
		inArc := start.cmp(mid) < 0 && mid.cmp(end) < 0
		if start.cmp(end) > 0 {
			inArc = start.cmp(mid) < 0 || mid.cmp(end) < 0
		}
		if inArc {
			if selected != -1 || region.HeightSide == 0 {
				return -1, ErrConvergence
			}
			selected = i
		}
	}
	if selected == -1 {
		return -1, ErrConvergence
	}
	return selected, nil
}

func (edge *ClippedPolynomialStockBoundary) assignWalls(order [2]ClippedPolynomialHeightOwner, masks [2]uint8) error {
	selection := func(mask uint8) [4]bool { return [4]bool{mask == 3, mask != 0, mask == 1, mask == 2} }
	lower := ClippedPolynomialBaseHeight
	for _, upper := range order {
		bit := uint8(1 << (upper - ClippedPolynomialRoofAHeight))
		if (masks[0]|masks[1])&bit == 0 {
			continue
		}
		left, right := selection(masks[0]), selection(masks[1])
		for i, walls := range [4]*[2]ClippedPolynomialWall{&edge.Common, &edge.Fuse, &edge.Cut[0], &edge.Cut[1]} {
			if left[i] == right[i] {
				continue
			}
			side := 0
			if right[i] {
				side = 1
			}
			wall := &walls[side]
			switch wall.Upper {
			case ClippedPolynomialNoHeight:
				wall.Lower = lower
			case lower:
			default:
				return ErrConvergence
			}
			wall.Upper = upper
		}
		lower = upper
		masks[0] &^= bit
		masks[1] &^= bit
	}
	return nil
}

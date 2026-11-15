package geom

import (
	"context"
	"math/big"
)

// ClippedPolynomialStockTile is a rectangle of constant stock membership.
// RoofMask uses bit 0 for A and bit 1 for B. SourceDomains and StockVolumes
// give each present stock's original chart restriction and exact volume over
// Physical. SourceDomains is indexed base, A, B. StockVolumes is indexed A,
// B. Absent roof domains and volumes are zero and nil, respectively.
type ClippedPolynomialStockTile struct {
	Physical      ParameterRectangle
	RoofMask      uint8
	SourceDomains [3]ParameterRectangle
	StockVolumes  [2]*big.Rat
}

// ClippedPolynomialOverlapLayer gives the exact volume of a positive
// difference region inside the overlap. Region indexes the original
// arrangement in the report, including when integrating B minus A.
type ClippedPolynomialOverlapLayer struct {
	Region int
	Volume *big.Rat
}

// ClippedSplineGraphPolynomialMaterial gives exact stock coverage, original
// wall traces and physical material volumes. Footprints is in input roof
// order. Tiles starts with the overlap, followed by the A-only and B-only
// strips in west, east, south, north order.
// The open tile interiors are disjoint. Boundaries records every open
// tile edge segment, including internal subdivisions with no Boolean wall.
// Cut[0] and OverlapCut[0] use A minus B. Index 1 uses B minus A.
type ClippedSplineGraphPolynomialMaterial struct {
	Overlap         ClippedSplineGraphPolynomialArrangement
	Footprints      [2]ParameterRectangle
	Tiles           []ClippedPolynomialStockTile
	Boundaries      []ClippedPolynomialStockBoundary
	OverlapCut      [2][]ClippedPolynomialOverlapLayer
	Common, Fuse    *big.Rat
	Cut             [2]*big.Rat
	ExactOperations int
}

// IntegrateClippedSplineGraphPolynomialMaterial proves a complete clipped
// arrangement and integrates the base-to-roof height on its rectangular
// stock tiles and the signed differences inside the overlap. Stock heights
// must have constant homogeneous weight on every restricted Bezier cell.
// Clipped stock rectangles and owner restrictions must be exactly represented.
// Boundary endpoints and original owner UVs are retained as exact rationals.
// Failure returns only the work consumed.
func IntegrateClippedSplineGraphPolynomialMaterial(ctx context.Context, base, a, b *SurfacePatch, axis int,
	o GraphPointContactOptions) (r ClippedSplineGraphPolynomialMaterial, err error) {
	o, e := alignedPolynomialProofOptions(o)
	if e != nil {
		return r, e
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		if err != nil {
			r = ClippedSplineGraphPolynomialMaterial{}
		}
		r.ExactOperations = min(budget.operations, budget.maxOperations)
	}()
	var layers [2]polynomialPositiveVolumes
	r.Overlap, e = discoverClippedSplineGraphPolynomialArrangement(ctx, base, a, b, axis, o,
		polynomialVolumeOrdersDiscovery(nil, [2]*polynomialPositiveVolumes{&layers[0], &layers[1]}))
	budget.operations += r.Overlap.ExactOperations
	if e != nil {
		return r, e
	}
	baseMap, e := r.Overlap.BaseAlignment.MapCoefficients()
	if e != nil {
		return r, e
	}
	maps := [3][2][3]*big.Rat{baseMap, r.Overlap.Projection.Maps[0], r.Overlap.Projection.Maps[1]}
	overlap := r.Overlap.Reference.Domain()
	r.Tiles = []ClippedPolynomialStockTile{{Physical: overlap, RoofMask: 3}}
	for owner, mapping := range maps[1:] {
		full, e := clippedMaterialPhysicalDomain(r.Overlap.OriginalDomains[1+owner], mapping, budget)
		if e != nil {
			return r, e
		}
		if !clippedMaterialContains(full, overlap) {
			return r, ErrConvergence
		}
		r.Footprints[owner] = full
		// West/east retain the full height. South/north then use only
		// the overlap's horizontal span, so their interiors cannot meet.
		for _, d := range [4]ParameterRectangle{
			{full.UMin, overlap.UMin, full.VMin, full.VMax},
			{overlap.UMax, full.UMax, full.VMin, full.VMax},
			{overlap.UMin, overlap.UMax, full.VMin, overlap.VMin},
			{overlap.UMin, overlap.UMax, overlap.VMax, full.VMax},
		} {
			if d.Valid() {
				r.Tiles = append(r.Tiles, ClippedPolynomialStockTile{Physical: d, RoofMask: 1 << owner})
			}
		}
	}

	maxCells := o.MaxCells
	if maxCells == 0 {
		maxCells = 4096
	}
	complement := [2]*big.Rat{new(big.Rat), new(big.Rat)}
	for i := range r.Tiles {
		tile := &r.Tiles[i]
		for j := range i {
			d, other := tile.Physical, r.Tiles[j].Physical
			if max(d.UMin, other.UMin) < min(d.UMax, other.UMax) && max(d.VMin, other.VMin) < min(d.VMax, other.VMax) {
				return r, ErrConvergence
			}
		}
		area := clippedMaterialArea(tile.Physical)
		var baseHeight *big.Rat
		for k, p := range [3]*SurfacePatch{base, a, b} {
			if k > 0 && tile.RoofMask&(1<<(k-1)) == 0 {
				continue
			}
			d, e := clippedMaterialOwnerDomain(tile.Physical, maps[k], budget)
			if e != nil {
				return r, e
			}
			tile.SourceDomains[k] = d
			if !clippedMaterialContains(p.Domain(), d) || i == 0 && d != r.Overlap.RestrictedDomains[k] {
				return r, ErrConvergence
			}
			height, e := integrateClippedMaterialHeight(ctx, p, d, area, maxCells, budget)
			if e != nil {
				return r, e
			}
			if k == 0 {
				baseHeight = height
				continue
			}
			volume := new(big.Rat).Sub(height, baseHeight)
			if e := budget.check(volume); e != nil {
				return r, e
			}
			if volume.Sign() <= 0 {
				return r, ErrConvergence
			}
			tile.StockVolumes[k-1] = volume
			if i > 0 {
				complement[k-1].Add(complement[k-1], volume)
				if e := budget.check(complement[k-1]); e != nil {
					return r, e
				}
			}
		}
	}

	for owner, layer := range layers {
		area := new(big.Rat)
		for _, tile := range r.Tiles {
			if tile.RoofMask&(1<<owner) != 0 {
				area.Add(area, clippedMaterialArea(tile.Physical))
				if e := budget.check(area); e != nil {
					return r, e
				}
			}
		}
		if area.Cmp(clippedMaterialArea(r.Footprints[owner])) != 0 {
			return r, ErrConvergence
		}
		common := new(big.Rat).Sub(r.Tiles[0].StockVolumes[owner], layer.total)
		if e := budget.check(common); e != nil {
			return r, e
		}
		if common.Sign() <= 0 || owner == 1 && common.Cmp(r.Common) != 0 {
			return r, ErrConvergence
		}
		r.Common = common
		r.Cut[owner] = new(big.Rat).Add(complement[owner], layer.total)
		if e := budget.check(r.Cut[owner]); e != nil {
			return r, e
		}
		for i, region := range layer.regions {
			r.OverlapCut[owner] = append(r.OverlapCut[owner], ClippedPolynomialOverlapLayer{region, layer.volumes[i]})
		}
	}
	r.Fuse = new(big.Rat).Add(r.Common, r.Cut[0])
	r.Fuse.Add(r.Fuse, r.Cut[1])
	if e := budget.check(r.Fuse); e != nil {
		return r, e
	}
	r.Boundaries, e = clippedMaterialBoundaries(ctx, r.Overlap, r.Tiles, maps, budget)
	if e != nil {
		return r, e
	}
	return r, ctx.Err()
}

func clippedMaterialContains(full, d ParameterRectangle) bool {
	return d.Valid() && full.UMin <= d.UMin && full.UMax >= d.UMax && full.VMin <= d.VMin && full.VMax >= d.VMax
}

func clippedMaterialArea(d ParameterRectangle) *big.Rat {
	u := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), new(big.Rat).SetFloat64(d.UMin))
	v := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.VMax), new(big.Rat).SetFloat64(d.VMin))
	return u.Mul(u, v)
}

func clippedMaterialPhysicalDomain(d ParameterRectangle, mapping [2][3]*big.Rat, b *polynomialBudget) (ParameterRectangle, error) {
	owner := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	var physical [2][2]float64
	for i, row := range mapping {
		axis := 0
		if row[1].Sign() != 0 {
			axis = 1
		}
		for j, x := range owner[i] {
			q := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(x), row[2]), row[axis])
			if e := b.check(q); e != nil {
				return ParameterRectangle{}, e
			}
			var e error
			physical[axis][j], e = RepresentedExactScalar(q)
			if e != nil {
				return ParameterRectangle{}, e
			}
		}
	}
	return clippedMaterialSortedDomain(physical[0][0], physical[0][1], physical[1][0], physical[1][1]), nil
}

func clippedMaterialOwnerDomain(d ParameterRectangle, mapping [2][3]*big.Rat, b *polynomialBudget) (ParameterRectangle, error) {
	lo, e := clippedMaterialMapUV(mapping, [2]*big.Rat{new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin)}, b)
	if e != nil {
		return ParameterRectangle{}, e
	}
	hi, e := clippedMaterialMapUV(mapping, [2]*big.Rat{new(big.Rat).SetFloat64(d.UMax), new(big.Rat).SetFloat64(d.VMax)}, b)
	if e != nil {
		return ParameterRectangle{}, e
	}
	var uv [4]float64
	for i, q := range [4]*big.Rat{lo[0], hi[0], lo[1], hi[1]} {
		uv[i], e = RepresentedExactScalar(q)
		if e != nil {
			return ParameterRectangle{}, e
		}
	}
	return clippedMaterialSortedDomain(uv[0], uv[1], uv[2], uv[3]), nil
}

func clippedMaterialSortedDomain(u0, u1, v0, v1 float64) ParameterRectangle {
	return ParameterRectangle{min(u0, u1), max(u0, u1), min(v0, v1), max(v0, v1)}
}

func clippedMaterialMapUV(mapping [2][3]*big.Rat, xy [2]*big.Rat, b *polynomialBudget) (uv [2]*big.Rat, err error) {
	for i, row := range mapping {
		uv[i] = new(big.Rat).Add(new(big.Rat).Mul(row[0], xy[0]), new(big.Rat).Mul(row[1], xy[1]))
		uv[i].Add(uv[i], row[2])
		if e := b.check(uv[i]); e != nil {
			return uv, e
		}
	}
	return uv, nil
}

// The proved affine chart gives a constant area scale. In each Bezier cell
// the integral of a tensor Bernstein polynomial is its average coefficient
// times its parameter area. Constant homogeneous weight gives that polynomial
// directly from the original source, with no fitted surface or quadrature.
func integrateClippedMaterialHeight(ctx context.Context, p *SurfacePatch, d ParameterRectangle, physicalArea *big.Rat,
	maxCells int, b *polynomialBudget) (*big.Rat, error) {
	q, e := trimPatchBudget(p, d, b)
	if e != nil {
		return nil, e
	}
	cells, e := graphContactBezierCells(ctx, q, maxCells, b)
	if e != nil {
		return nil, e
	}
	total := new(big.Rat)
	area := clippedMaterialArea(d)
	scale := new(big.Rat).Quo(physicalArea, area)
	if e := b.check(scale); e != nil {
		return nil, e
	}
	for _, cell := range cells {
		weight := cell.net[0][3]
		if weight.Sign() <= 0 {
			return nil, ErrConvergence
		}
		sum := new(big.Rat)
		for _, h := range cell.net {
			if h[3].Cmp(weight) != 0 {
				return nil, ErrConvergence
			}
			sum.Add(sum, h[2])
			if e := b.check(sum); e != nil {
				return nil, e
			}
		}
		sum.Quo(sum, new(big.Rat).Mul(weight, big.NewRat(int64(len(cell.net)), 1)))
		sum.Mul(sum, clippedMaterialArea(cell.Domain()))
		total.Add(total, sum)
		if e := b.check(total); e != nil {
			return nil, e
		}
	}
	total.Mul(total, scale)
	return total, b.check(total)
}

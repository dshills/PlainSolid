package geom

import (
	"context"
	"math/big"
)

// ClippedSplineGraphPolynomialArrangement proves the positive-area physical
// overlap of two independently bounded roofs over one original base. The
// chart order is base, A, B. Reference uses absolute physical XY parameters.
// BoundaryRoofs records which original roof footprints end on each clipped
// side, counterclockwise from YMin. Bit 0 is A and bit 1 is B. The signed
// top regions and exact original roof pcurves are in Projection.
type ClippedSplineGraphPolynomialArrangement struct {
	Reference         *SurfacePatch
	BaseAlignment     *AffineXYChartAlignment
	Projection        ProjectedSplineGraphPolynomialArrangement
	OriginalDomains   [3]ParameterRectangle
	RestrictedDomains [3]ParameterRectangle
	BaseMinusRoof     [2]AlignedGraphHeightRange // over each original roof's full footprint
	BoundaryRoofs     [4]uint8
	ExactOperations   int
}

// DiscoverClippedSplineGraphPolynomialArrangement certifies exactly dyadic
// clipped original UV rectangles sharing an axis-aligned physical footprint.
// The common base must cover both full original roof footprints and be
// strictly below each roof. All branches of the polynomial arrangement must
// stay in the clipped dependent interval. A disjoint footprint or incomplete
// base coverage returns ErrDomain. Touch-only overlap and unresolved signs
// return ErrConvergence. A failed call publishes no partial geometry.
func DiscoverClippedSplineGraphPolynomialArrangement(ctx context.Context, base, a, b *SurfacePatch, axis int,
	o GraphPointContactOptions) (r ClippedSplineGraphPolynomialArrangement, err error) {
	if ctx == nil || base == nil || a == nil || b == nil || len(base.net) == 0 || len(a.net) == 0 || len(b.net) == 0 || axis < 0 || axis > 1 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	o, e := alignedPolynomialProofOptions(o)
	if e != nil {
		return r, e
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		if err != nil {
			r = ClippedSplineGraphPolynomialArrangement{}
		}
		r.ExactOperations = budget.operations
	}()

	remaining := func() int { return budget.maxOperations - budget.operations }
	limits := func() SurfacePatchOptions {
		return SurfacePatchOptions{MaxOperations: remaining(), MaxCoefficientBits: budget.maxBits}
	}
	clip := func(a, b *SurfacePatch) (AffineGraphOverlap, error) {
		if remaining() < 1 {
			return AffineGraphOverlap{}, ErrConvergence
		}
		r, e := ClipAffineXYGraphOverlap(ctx, a, b, limits())
		budget.operations += r.ExactOperations
		return r, e
	}
	trim := func(p *SurfacePatch, d ParameterRectangle) (*SurfacePatch, error) {
		if remaining() < 1 {
			return nil, ErrConvergence
		}
		source := *p
		source.options.MaxCoefficientBits = budget.maxBits
		q, work, e := source.TrimWithWorkLimit(d, remaining())
		budget.operations += work
		return q, e
	}
	align := func(a, b *SurfacePatch) (*AffineXYChartAlignment, error) {
		if remaining() < 1 {
			return nil, ErrConvergence
		}
		m, work, e := AlignAffineXYChartsWithWork(ctx, a, b, limits())
		budget.operations += work
		return m, e
	}

	overlap, e := clip(a, b)
	if e != nil {
		return r, e
	}
	if overlap.Disjoint {
		return r, ErrDomain
	}
	r.OriginalDomains = [3]ParameterRectangle{base.Domain(), a.Domain(), b.Domain()}
	r.RestrictedDomains[1], r.RestrictedDomains[2] = overlap.A, overlap.B
	var baseStock [2]ParameterRectangle
	for i, roof := range [2]*SurfacePatch{a, b} {
		m, e := clip(base, roof)
		if e != nil {
			return r, e
		}
		if m.Disjoint || m.B != roof.Domain() {
			return r, ErrDomain
		}
		baseStock[i] = m.A
		onStock, e := trim(base, m.A)
		if e != nil {
			return r, e
		}
		h, e := align(onStock, roof)
		if e != nil {
			return r, e
		}
		if remaining() < 1 {
			return r, ErrConvergence
		}
		r.BaseMinusRoof[i], e = h.HeightRange(ctx, GeneralSurfaceHeightOptions{
			MaxOperations: remaining(), MaxCoefficientBits: budget.maxBits, MaxCells: o.MaxCells,
		})
		budget.operations += r.BaseMinusRoof[i].ExactOperations
		if e != nil {
			return r, e
		}
		if r.BaseMinusRoof[i].StrictSign != -1 {
			return r, ErrConvergence
		}
	}
	r.RestrictedDomains[0] = ParameterRectangle{
		UMin: max(baseStock[0].UMin, baseStock[1].UMin), UMax: min(baseStock[0].UMax, baseStock[1].UMax),
		VMin: max(baseStock[0].VMin, baseStock[1].VMin), VMax: min(baseStock[0].VMax, baseStock[1].VMax),
	}
	if !r.RestrictedDomains[0].Valid() {
		return r, ErrConvergence
	}
	var clipped [3]*SurfacePatch
	for i, p := range [3]*SurfacePatch{base, a, b} {
		clipped[i], e = trim(p, r.RestrictedDomains[i])
		if e != nil {
			return r, e
		}
	}
	if remaining() < 1 {
		return r, ErrConvergence
	}
	r.Reference, e = NewAffineXYRectangleReference(ctx, clipped[1], limits())
	if e != nil {
		return r, e
	}
	budget.operations += r.Reference.ExactOperations()
	r.BaseAlignment, e = align(r.Reference, clipped[0])
	if e != nil {
		return r, e
	}
	o.MaxOperations = remaining()
	if o.MaxOperations < 1 {
		return r, ErrConvergence
	}
	r.Projection, e = DiscoverProjectedSplineGraphPolynomialArrangement(ctx, r.Reference, clipped[1], clipped[2], axis, o)
	budget.operations += r.Projection.ExactOperations
	if e != nil {
		return r, e
	}
	if r.BoundaryRoofs, e = clippedPolynomialBoundaryRoofs(r.Reference.Domain(), r.OriginalDomains[1:], r.Projection.Maps, budget); e != nil {
		return r, e
	}
	return r, ctx.Err()
}

func clippedPolynomialBoundaryRoofs(d ParameterRectangle, original []ParameterRectangle, maps [2][2][3]*big.Rat,
	budget *polynomialBudget) (out [4]uint8, err error) {
	x0, x1 := new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.UMax)
	y0, y1 := new(big.Rat).SetFloat64(d.VMin), new(big.Rat).SetFloat64(d.VMax)
	xm := new(big.Rat).Quo(new(big.Rat).Add(x0, x1), big.NewRat(2, 1))
	ym := new(big.Rat).Quo(new(big.Rat).Add(y0, y1), big.NewRat(2, 1))
	for side, xy := range [4][2]*big.Rat{{xm, y0}, {x1, ym}, {xm, y1}, {x0, ym}} {
		for owner, mapUV := range maps {
			s := original[owner]
			limits := [2][2]float64{{s.UMin, s.UMax}, {s.VMin, s.VMax}}
			for i, row := range mapUV {
				q := new(big.Rat).Add(new(big.Rat).Mul(row[0], xy[0]), new(big.Rat).Mul(row[1], xy[1]))
				q.Add(q, row[2])
				if e := budget.check(q); e != nil {
					return out, e
				}
				if q.Cmp(new(big.Rat).SetFloat64(limits[i][0])) == 0 || q.Cmp(new(big.Rat).SetFloat64(limits[i][1])) == 0 {
					out[side] |= 1 << owner
				}
			}
		}
		if out[side] == 0 {
			return out, ErrConvergence
		}
	}
	return out, nil
}

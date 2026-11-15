package geom

import (
	"context"
	"math/big"
)

// SplineGraphPolynomialOwnerEdge is a contact edge in an original roof chart.
// FreeRange runs from the common edge's Start to End, even when that means
// decreasing the owner's free coordinate.
type SplineGraphPolynomialOwnerEdge struct {
	FreeRange [2]*big.Rat
	Controls  [][2]*big.Rat
}

// SplineGraphPolynomialOwnerLedger describes one original roof. Vertices,
// Edges and RegionWitnesses are indexed by the arrangement's corresponding
// records. ProjectionOrientation is the sign of the original UV-to-XY
// orientation. Positive model region loops use that sign for the owner's UV
// perimeter direction.
type SplineGraphPolynomialOwnerLedger struct {
	Domain                ParameterRectangle
	FreeAxis              int
	ProjectionOrientation int
	Vertices              [][2]*big.Rat
	Edges                 []SplineGraphPolynomialOwnerEdge
	RegionWitnesses       [][2]*big.Rat
}

// AlignedSplineGraphPolynomialArrangement describes the complete arrangement
// in the first roof's UV chart and gives the exact parameters for each
// original roof in input order. TargetMap expresses the original second UV
// as M*firstUV+c, with one row per target coordinate and columns U, V,
// and constant. The arrangement's powers, tangents and derivative gaps use
// the first chart's parameter units.
type AlignedSplineGraphPolynomialArrangement struct {
	Arrangement     SplineGraphPolynomialArrangement
	TargetMap       [2][3]*big.Rat
	Owners          [2]SplineGraphPolynomialOwnerLedger
	ExactOperations int
}

// DiscoverAlignedSplineGraphPolynomialArrangement proves the same contact
// arrangement as DiscoverSplineGraphPolynomialArrangement for independently
// parameterized affine XY graph roofs. Their full projected domains must
// coincide and their UV charts must relate by a signed axis permutation and
// scaling. Axis selects the free coordinate of the first roof.
// Nonrepresentable remapped knots return ErrUnrepresentable. Failed calls
// return no geometry.
func DiscoverAlignedSplineGraphPolynomialArrangement(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (AlignedSplineGraphPolynomialArrangement, error) {
	return discoverAlignedSplineGraphPolynomialArrangement(ctx, a, b, axis, o, DiscoverSplineGraphPolynomialArrangement)
}

func discoverAlignedSplineGraphPolynomialArrangement(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions,
	discover func(context.Context, *SurfacePatch, *SurfacePatch, int, GraphPointContactOptions) (SplineGraphPolynomialArrangement, error)) (r AlignedSplineGraphPolynomialArrangement, err error) {
	if ctx == nil || a == nil || b == nil || len(a.net) == 0 || len(b.net) == 0 || axis < 0 || axis > 1 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 0 || o.MaxDepth < 0 || o.MaxDepth > 32 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		if err != nil {
			r = AlignedSplineGraphPolynomialArrangement{}
		}
		r.ExactOperations = budget.operations
	}()
	align, work, e := AlignAffineXYChartsWithWork(ctx, a, b, SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	budget.operations += work
	if e != nil {
		return r, e
	}
	o.MaxOperations -= work
	if o.MaxOperations < 1 {
		return r, ErrConvergence
	}
	proof, e := discover(ctx, a, align.TargetOnSource(), axis, o)
	budget.operations += proof.ExactOperations
	if e != nil {
		return r, e
	}
	r.Arrangement = proof
	r.TargetMap, e = align.MapCoefficients()
	if e != nil {
		return r, e
	}
	identity := [2][3]*big.Rat{{big.NewRat(1, 1), new(big.Rat), new(big.Rat)}, {new(big.Rat), big.NewRat(1, 1), new(big.Rat)}}
	mappings := [2][2][3]*big.Rat{identity, r.TargetMap}
	for owner, original := range [2]*SurfacePatch{a, b} {
		r.Owners[owner], e = polynomialArrangementOwner(ctx, original, proof, mappings[owner], owner == 1, budget)
		if e != nil {
			return r, e
		}
	}
	return r, ctx.Err()
}

func polynomialArrangementOwner(ctx context.Context, original *SurfacePatch, proof SplineGraphPolynomialArrangement, mapping [2][3]*big.Rat, recheckOriginal bool, budget *polynomialBudget) (SplineGraphPolynomialOwnerLedger, error) {
	var r SplineGraphPolynomialOwnerLedger
	// Alignment guarantees that each row has exactly one nonzero linear
	// coefficient and that the two rows use distinct columns.
	axes := [2]int{0, 1}
	if mapping[0][0].Sign() == 0 {
		axes = [2]int{1, 0}
	}
	r.Domain = original.Domain()
	r.FreeAxis = axes[proof.FreeAxis]
	r.ProjectionOrientation = proof.Graph.ProjectionOrientation * mapping[0][axes[0]].Sign() * mapping[1][axes[1]].Sign()
	if axes[0] == 1 {
		r.ProjectionOrientation = -r.ProjectionOrientation
	}
	mapUV := func(uv [2]*big.Rat) (out [2]*big.Rat, err error) {
		for row, col := range axes {
			q := new(big.Rat).Mul(mapping[row][col], uv[col])
			if e := budget.check(q); e != nil {
				return out, e
			}
			out[row] = q.Add(q, mapping[row][2])
			if e := budget.check(q); e != nil {
				return out, e
			}
		}
		return out, nil
	}
	r.Vertices = make([][2]*big.Rat, len(proof.Vertices))
	for i, vertex := range proof.Vertices {
		uv, e := mapUV(vertex.Parameters)
		if e != nil {
			return r, e
		}
		r.Vertices[i] = uv
		if recheckOriginal {
			// The full identity follows from alignment and the union-cell
			// proof. Rebind its vertices to the unmodified target net.
			remaining := budget.maxOperations - budget.operations
			if remaining < 1 {
				return r, ErrConvergence
			}
			p, e := original.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: budget.maxBits})
			budget.operations += p.ExactOperations
			if e != nil {
				return r, e
			}
			for k, q := range p.Point {
				if q.Cmp(vertex.Point[k]) != 0 {
					return r, ErrConvergence
				}
			}
		}
	}
	r.Edges = make([]SplineGraphPolynomialOwnerEdge, len(proof.Edges))
	for i, edge := range proof.Edges {
		controls := make([][2]*big.Rat, len(edge.Controls))
		for j, uv := range edge.Controls {
			p, e := mapUV(uv)
			if e != nil {
				return r, e
			}
			controls[j] = p
		}
		r.Edges[i] = SplineGraphPolynomialOwnerEdge{
			FreeRange: [2]*big.Rat{new(big.Rat).Set(r.Vertices[edge.Start][r.FreeAxis]), new(big.Rat).Set(r.Vertices[edge.End][r.FreeAxis])},
			Controls:  controls,
		}
	}
	r.RegionWitnesses = make([][2]*big.Rat, len(proof.Regions))
	for i, region := range proof.Regions {
		p, e := mapUV(region.Witness)
		if e != nil {
			return r, e
		}
		r.RegionWitnesses[i] = p
	}
	return r, nil
}

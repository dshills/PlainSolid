//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// CertifiedAffinePartitionDistance augments represented polygon coverage with
// a two-sided source-fill/display Hausdorff bound, for each corresponding
// fragment and for their union. All squared bounds are exact rationals.
type CertifiedAffinePartitionDistance struct {
	CertifiedAffinePartitionFill
	ProjectionNormSquared            *big.Rat
	PlanarFillSquaredDistance        *big.Rat
	DisplayProjectionSquaredDistance *big.Rat
	FillSquaredDistance              *big.Rat
}

// TessellateAffinePartitionDistanceCertified projects along the omitted
// coordinate onto the exact support plane. This projection preserves the
// certified 2D polygon arrangement. Its exact Frobenius norm bounds transport
// of all source/chord boundary homotopies (including the common straight cut).
// Winding numbers of the simple source and represented loops agree off the
// homotopy's swept boundary. A point in their symmetric difference therefore
// lies within the transported boundary error of both boundaries, proving
// two-sided distance for the closed planar fills. The maximal vertex lift
// error then bounds each projected/display triangle correspondence. We use
// (a+b)^2 <= 2(a^2+b^2) to compose without unproved floating square roots.
// This does not prove that the source shell is embedded, nor exact source area.
func (s AlgebraicSubdivision) TessellateAffinePartitionDistanceCertified(ctx context.Context, face int, line [3]*big.Rat, o AffinePartitionFillOptions) (CertifiedAffinePartitionDistance, error) {
	var empty CertifiedAffinePartitionDistance
	fill, e := s.TessellateAffinePartitionFillCertified(ctx, face, line, o)
	if e != nil {
		return empty, e
	}
	patch, e := exactFaceSupportPatch(s.source.faces[face], o.Boundary.Proof.Composition.SurfacePatchOptions)
	if e != nil {
		return empty, e
	}
	chart, e := geom.NewAffineSurfaceChart(ctx, patch, geom.AffineSurfaceChartOptions{MaxOperations: o.Boundary.Proof.Composition.MaxOperations, MaxCoefficientBits: o.Boundary.Proof.Composition.MaxCoefficientBits})
	if e != nil {
		return empty, e
	}
	origin, e := chart.EvalExact(new(big.Rat), new(big.Rat))
	if e != nil {
		return empty, e
	}
	u, e := chart.EvalExact(big.NewRat(1, 1), new(big.Rat))
	if e != nil {
		return empty, e
	}
	v, e := chart.EvalExact(new(big.Rat), big.NewRat(1, 1))
	if e != nil {
		return empty, e
	}
	for i := 0; i < 3; i++ {
		u[i].Sub(u[i], origin[i])
		v[i].Sub(v[i], origin[i])
	}
	var normal [3]*big.Rat
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		normal[i] = new(big.Rat).Sub(new(big.Rat).Mul(u[j], v[k]), new(big.Rat).Mul(u[k], v[j]))
	}
	axes := fill.ProjectionAxes
	drop := 3 - axes[0] - axes[1]
	if normal[drop].Sign() == 0 {
		return empty, geom.ErrConvergence
	}
	norm := big.NewRat(2, 1)
	for _, i := range axes {
		q := new(big.Rat).Quo(normal[i], normal[drop])
		norm.Add(norm, new(big.Rat).Mul(q, q))
	}
	boundarySquared := new(big.Rat).Set(fill.Boundary.CutSquaredError)
	for _, use := range s.faces[face].Outer {
		errorUpper := fill.Boundary.Boundary.Certificates[use.Edge].MaximumError
		q := ratInset(errorUpper)
		squared := new(big.Rat).Mul(q, q)
		if squared.Cmp(boundarySquared) > 0 {
			boundarySquared.Set(squared)
		}
	}
	planar := new(big.Rat).Mul(norm, boundarySquared)
	display := new(big.Rat)
	for _, point := range fill.Mesh.Vertices {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		p := exactBoundPoint(point)
		distance := new(big.Rat)
		for i := 0; i < 3; i++ {
			distance.Add(distance, new(big.Rat).Mul(normal[i], new(big.Rat).Sub(p[i], origin[i])))
		}
		distance.Quo(distance, normal[drop])
		distance.Mul(distance, distance)
		if distance.Cmp(display) > 0 {
			display.Set(distance)
		}
	}
	total := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Add(planar, display))
	return CertifiedAffinePartitionDistance{fill, norm, planar, display, total}, nil
}

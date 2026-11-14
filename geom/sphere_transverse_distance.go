package geom

import (
	"context"
	"math/big"
)

// NearestTransverseBoundaryRational completely classifies nearest original
// boundary faces of a transverse two-sphere Boolean at an off-axis rational
// query. Source center directions are arbitrary represented vectors. Circle
// radii and all query norms may be irrational. Axis/center minimizer families
// remain outside this isolated-foot API; tangencies and nontransverse pairs
// are rejected. Existing narrower queries preserve their original gates.
//
// Rational projection onto the unnormalized center axis gives circle center c,
// squared radius h² and query radial vector v. The exact circle foot is
// c+h²v/sqrt(h²|v|²). Its squared distance is axialDistance²+|v|²+h²
// -2sqrt(h²|v|²). Hence all three candidate distances remain single radicals,
// with exact guarded pair order and exact retained-cap signs. The shared
// arithmetic budget and original-owner contracts are unchanged.
func (s SpherePairMaterial) NearestTransverseBoundaryRational(ctx context.Context, p [3]*big.Rat, o SphereFullBoundaryDistanceOptions) (SphereOverlapAlgebraicNearest, error) {
	return s.nearestOverlapAlgebraicBoundaryRational(ctx, p, o, true)
}

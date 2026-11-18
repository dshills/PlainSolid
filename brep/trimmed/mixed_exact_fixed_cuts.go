//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// TessellateMixedExactAffineRegionWithCutsCertified preserves exact prescribed
// parameters for each original native trim (outer then holes). Each non-nil
// list includes0/1 and strictly increases; nil leaves that trim adaptive.
// Cuts are limited to2048 total occurrences and4096 coefficient bits. They are
// proposals only: restricted model chords, whole polygon topology and exact
// triangle coverage are all re-certified. Adaptive additional cuts are allowed.
// Per-trim display operations divide between root/restriction proofs and final
// chord proof as in TessellateExactBezierGraphChainWithCutsCertified.
func TessellateMixedExactAffineRegionWithCutsCertified(ctx context.Context, p *geom.SurfacePatch, outer []PCurve, holes [][]PCurve, cuts [][][]*big.Rat, o MixedExactAffineMeshOptions) (CertifiedMixedExactAffineMesh, error) {
	return tessellateMixedExactAffineRegionCuts(ctx, p, outer, holes, cuts, o)
}

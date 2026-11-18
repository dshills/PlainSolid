//go:build research

package trimmed

import "context"

// TessellateCertified applies the same independent source-circle, affine-cap,
// continuous normal, two-sided distance and closed display proofs as the equal
// radius operation. Neither mutable ancestry nor constructor provenance is
// trusted: both entire circular charts are re-proved before any seam is shared.
func (s UnequalSideOpenDualDiskCut) TessellateCertified(ctx context.Context, o SideOpenDualDiskMeshOptions) (CertifiedSideOpenDualDiskMesh, error) {
	return (SideOpenDualDiskCut{Solid: s.Solid, WallFaces: s.WallFaces, FloorFace: s.FloorFace}).TessellateCertified(ctx, o)
}

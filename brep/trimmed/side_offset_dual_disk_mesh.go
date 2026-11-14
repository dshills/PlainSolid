package trimmed

import "context"

// TessellateCertified independently re-proves the stored circular walls and
// affine caps, continuous normal and distance bounds, shared boundary identity,
// and closed embedded display. Constructor metadata is not trusted as proof.
func (s OffsetSideOpenDualDiskCut) TessellateCertified(ctx context.Context, o SideOpenDualDiskMeshOptions) (CertifiedSideOpenDualDiskMesh, error) {
	return (SideOpenDualDiskCut{Solid: s.Solid, WallFaces: s.WallFaces, FloorFace: s.FloorFace}).TessellateCertified(ctx, o)
}

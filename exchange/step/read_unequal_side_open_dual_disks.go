package step

import (
	"context"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// UnequalSideOpenDualDiskProduct contains a separately certified unequal native
// body, never an equal-radius analytical result. The descriptor origin is
// normalized to the stock lower corner and disks are ordered by local X.
// SourceWallFaces refers to that normalized disk order; original call order
// is not uniquely encoded. SourceFloorFace is zero for a through cut.
type UnequalSideOpenDualDiskProduct struct {
	Descriptor                                                                                 UnequalSideOpenDualDiskDescriptor
	Native                                                                                     trimmed.UnequalSideOpenDualDiskCut
	Body, Shell                                                                                int64
	Faces, Edges                                                                               []int64
	SourceWallFaces                                                                            [2]int64
	SourceFloorFace                                                                            int64
	ReversedModelLines, RephasedModelCircles, OppositeNormalModelCircles, OppositeNormalPlanes []int64
}

// ReadUnequalSideOpenDualDiskProduct re-proves the complete canonical rooted
// unequal model-only graph from exact source fields. Equal radii, vendor pcurves
// and unsupported constructions reject. Caller tolerance is not source metadata.
func ReadUnequalSideOpenDualDiskProduct(ctx context.Context, r io.Reader, l Limits, tolerance float64) (UnequalSideOpenDualDiskProduct, error) {
	return ReadUnequalSideOpenDualDiskProductWithOptions(ctx, r, l, tolerance, SideOpenDualDiskReadOptions{})
}

// ReadUnequalSideOpenDualDiskProductWithOptions adds only the selected exact
// line/circle/plane encoding proofs. Each source circle retains its own radius,
// center and native owner; the complete reconstructed graph is still compared.
func ReadUnequalSideOpenDualDiskProductWithOptions(ctx context.Context, r io.Reader, l Limits, tolerance float64, options SideOpenDualDiskReadOptions) (UnequalSideOpenDualDiskProduct, error) {
	result, err := readSideOpenDualDiskProduct(ctx, r, l, tolerance, options, sideDiskUnequal)
	if err != nil {
		return UnequalSideOpenDualDiskProduct{}, err
	}
	out := UnequalSideOpenDualDiskProduct{Descriptor: UnequalSideOpenDualDiskDescriptor(result.Descriptor), Native: result.unequalNative, Body: result.Body, Shell: result.Shell, Faces: result.Faces, Edges: result.Edges, ReversedModelLines: result.ReversedModelLines, RephasedModelCircles: result.RephasedModelCircles, OppositeNormalModelCircles: result.OppositeNormalModelCircles, OppositeNormalPlanes: result.OppositeNormalPlanes}
	for i, ids := range out.Native.WallFaces {
		if len(ids) != 1 || ids[0] < 0 || ids[0] >= len(out.Faces) {
			return UnequalSideOpenDualDiskProduct{}, geom.ErrDomain
		}
		out.SourceWallFaces[i] = out.Faces[ids[0]]
	}
	if out.Native.FloorFace >= 0 {
		if out.Native.FloorFace >= len(out.Faces) {
			return UnequalSideOpenDualDiskProduct{}, geom.ErrDomain
		}
		out.SourceFloorFace = out.Faces[out.Native.FloorFace]
	}
	return out, nil
}

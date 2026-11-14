package step

import (
	"context"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// OffsetSideOpenDualDiskProduct contains a certified external-center homogeneous-wall native
// body, never an equal-radius analytical result. The descriptor origin is
// normalized to the stock lower corner and disks are ordered by local X.
// SourceWallFaces refers to that normalized disk order; original call order
// is not uniquely encoded. SourceFloorFace is zero for a through cut.
type OffsetSideOpenDualDiskProduct struct {
	// RadicalJoin is an owned exact proof value; both coordinates must also be
	// losslessly representable by the current native float-coordinate format.
	RadicalJoin                [2]*big.Rat
	Descriptor                 OffsetSideOpenDualDiskDescriptor
	Native                     trimmed.OffsetSideOpenDualDiskCut
	Body, Shell                int64
	Faces, Edges               []int64
	SourceWallFaces            [2]int64
	SourceFloorFace            int64
	ReversedModelLines         []int64
	RephasedModelCircles       []int64
	OppositeNormalModelCircles []int64
	OppositeNormalPlanes       []int64
}

// ReadOffsetSideOpenDualDiskProduct re-proves the complete canonical rooted
// offset model-only graph from exact source fields and its registered AP203
// product identifier. Side-centered categories, vendor pcurves and
// unsupported constructions reject.
// Caller tolerance is not source metadata.
func ReadOffsetSideOpenDualDiskProduct(ctx context.Context, r io.Reader, l Limits, tolerance float64) (OffsetSideOpenDualDiskProduct, error) {
	return ReadOffsetSideOpenDualDiskProductWithOptions(ctx, r, l, tolerance, SideOpenDualDiskReadOptions{})
}

// ReadOffsetSideOpenDualDiskProductWithOptions adds only the selected exact
// line/circle/plane encoding proofs. Each source circle retains its own radius,
// center and native owner; the complete reconstructed graph is still compared.
func ReadOffsetSideOpenDualDiskProductWithOptions(ctx context.Context, r io.Reader, l Limits, tolerance float64, options SideOpenDualDiskReadOptions) (OffsetSideOpenDualDiskProduct, error) {
	result, err := readSideOpenDualDiskProduct(ctx, r, l, tolerance, options, sideDiskOffset)
	if err != nil {
		return OffsetSideOpenDualDiskProduct{}, err
	}
	out := OffsetSideOpenDualDiskProduct{Descriptor: OffsetSideOpenDualDiskDescriptor(result.Descriptor), Native: result.offsetNative, Body: result.Body, Shell: result.Shell, Faces: result.Faces, Edges: result.Edges, ReversedModelLines: result.ReversedModelLines, RephasedModelCircles: result.RephasedModelCircles, OppositeNormalModelCircles: result.OppositeNormalModelCircles, OppositeNormalPlanes: result.OppositeNormalPlanes}
	for i, ids := range out.Native.WallFaces {
		if len(ids) != 1 || ids[0] < 0 || ids[0] >= len(out.Faces) {
			return OffsetSideOpenDualDiskProduct{}, geom.ErrDomain
		}
		out.SourceWallFaces[i] = out.Faces[ids[0]]
		if _, ok := out.Native.Solid.Faces()[ids[0]].Surface.(*geom.ExactBezierSurface); !ok {
			return OffsetSideOpenDualDiskProduct{}, geom.ErrDomain
		}
	}
	if out.Native.FloorFace >= 0 {
		if out.Native.FloorFace >= len(out.Faces) {
			return OffsetSideOpenDualDiskProduct{}, geom.ErrDomain
		}
		out.SourceFloorFace = out.Faces[out.Native.FloorFace]
	}
	section, err := trimmed.CertifyOffsetSideOpenDualDiskSection(ctx, out.Descriptor.Min, out.Descriptor.Max, out.Descriptor.Discs, trimvalidate.Options{})
	if err != nil {
		return OffsetSideOpenDualDiskProduct{}, err
	}
	for i, q := range section.Join {
		if _, exact := q.Float64(); !exact {
			return OffsetSideOpenDualDiskProduct{}, geom.ErrUnrepresentable
		}
		out.RadicalJoin[i] = new(big.Rat).Set(q)
	}
	return out, nil
}

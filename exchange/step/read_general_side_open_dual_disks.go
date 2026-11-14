package step

import (
	"context"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// ExternalRadicalSide identifies the exact join relative to both local centers.
type ExternalRadicalSide int8

const (
	ExternalRadicalLeft  ExternalRadicalSide = -1
	ExternalRadicalRight ExternalRadicalSide = 1
)

// GeneralSideOpenDualDiskProduct contains a separately certified external-radical native
// body, never an equal-radius analytical result. The descriptor origin is
// normalized to the stock lower corner and disks are ordered by local X.
// SourceWallFaces refers to that normalized disk order; original call order
// is not uniquely encoded. SourceFloorFace is zero for a through cut.
type GeneralSideOpenDualDiskProduct struct {
	RadicalSide ExternalRadicalSide
	// RadicalJoin is an owned exact proof value; both coordinates must also be
	// losslessly representable by the current native float-coordinate format.
	RadicalJoin                [2]*big.Rat
	Descriptor                 GeneralSideOpenDualDiskDescriptor
	Native                     trimmed.GeneralSideOpenDualDiskCut
	Body, Shell                int64
	Faces, Edges               []int64
	SourceWallFaces            [2]int64
	SourceFloorFace            int64
	ReversedModelLines         []int64
	RephasedModelCircles       []int64
	OppositeNormalModelCircles []int64
	OppositeNormalPlanes       []int64
}

// ReadGeneralSideOpenDualDiskProduct re-proves the complete canonical rooted
// external-radical model-only graph from exact source fields. Equal radii,
// between-center joins, vendor pcurves and unsupported constructions reject.
// Caller tolerance is not source metadata.
func ReadGeneralSideOpenDualDiskProduct(ctx context.Context, r io.Reader, l Limits, tolerance float64) (GeneralSideOpenDualDiskProduct, error) {
	return ReadGeneralSideOpenDualDiskProductWithOptions(ctx, r, l, tolerance, SideOpenDualDiskReadOptions{})
}

// ReadGeneralSideOpenDualDiskProductWithOptions adds only the selected exact
// line/circle/plane encoding proofs. Each source circle retains its own radius,
// center and native owner; the complete reconstructed graph is still compared.
func ReadGeneralSideOpenDualDiskProductWithOptions(ctx context.Context, r io.Reader, l Limits, tolerance float64, options SideOpenDualDiskReadOptions) (GeneralSideOpenDualDiskProduct, error) {
	result, err := readSideOpenDualDiskProduct(ctx, r, l, tolerance, options, sideDiskGeneral)
	if err != nil {
		return GeneralSideOpenDualDiskProduct{}, err
	}
	out := GeneralSideOpenDualDiskProduct{Descriptor: GeneralSideOpenDualDiskDescriptor(result.Descriptor), Native: result.generalNative, Body: result.Body, Shell: result.Shell, Faces: result.Faces, Edges: result.Edges, ReversedModelLines: result.ReversedModelLines, RephasedModelCircles: result.RephasedModelCircles, OppositeNormalModelCircles: result.OppositeNormalModelCircles, OppositeNormalPlanes: result.OppositeNormalPlanes}
	for i, ids := range out.Native.WallFaces {
		if len(ids) != 1 || ids[0] < 0 || ids[0] >= len(out.Faces) {
			return GeneralSideOpenDualDiskProduct{}, geom.ErrDomain
		}
		out.SourceWallFaces[i] = out.Faces[ids[0]]
	}
	if out.Native.FloorFace >= 0 {
		if out.Native.FloorFace >= len(out.Faces) {
			return GeneralSideOpenDualDiskProduct{}, geom.ErrDomain
		}
		out.SourceFloorFace = out.Faces[out.Native.FloorFace]
	}
	section, err := trimmed.CertifyGeneralSideOpenDualDiskSection(ctx, out.Descriptor.Min, out.Descriptor.Max, out.Descriptor.Discs, trimvalidate.Options{})
	if err != nil {
		return GeneralSideOpenDualDiskProduct{}, err
	}
	lo, hi := sideRat(out.Descriptor.Discs[0].Center.X), sideRat(out.Descriptor.Discs[1].Center.X)
	if section.Join[0].Cmp(lo) < 0 {
		out.RadicalSide = ExternalRadicalLeft
	} else if section.Join[0].Cmp(hi) > 0 {
		out.RadicalSide = ExternalRadicalRight
	} else {
		return GeneralSideOpenDualDiskProduct{}, geom.ErrDomain
	}
	for i, q := range section.Join {
		if _, exact := q.Float64(); !exact {
			return GeneralSideOpenDualDiskProduct{}, geom.ErrUnrepresentable
		}
		out.RadicalJoin[i] = new(big.Rat).Set(q)
	}
	return out, nil
}

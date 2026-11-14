package step

import (
	"context"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// ExactWallSideOpenDualDiskProduct contains a certified nonbinary homogeneous-wall native
// body, never an equal-radius analytical result. The descriptor origin is
// normalized to the stock lower corner and disks are ordered by local X.
// SourceWallFaces refers to that normalized disk order; original call order
// is not uniquely encoded. SourceFloorFace is zero for a through cut.
type ExactWallSideOpenDualDiskProduct struct {
	RadicalSide ExternalRadicalSide
	// RadicalJoin is an owned exact proof value; both coordinates must also be
	// losslessly representable by the current native float-coordinate format.
	RadicalJoin                [2]*big.Rat
	Descriptor                 ExactWallSideOpenDualDiskDescriptor
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

// ReadExactWallSideOpenDualDiskProduct re-proves the complete canonical rooted
// exact-wall model-only graph from exact source fields and its registered AP203
// product identifier. Ordinary binary-wall categories, vendor pcurves and
// unsupported constructions reject.
// Caller tolerance is not source metadata.
func ReadExactWallSideOpenDualDiskProduct(ctx context.Context, r io.Reader, l Limits, tolerance float64) (ExactWallSideOpenDualDiskProduct, error) {
	return ReadExactWallSideOpenDualDiskProductWithOptions(ctx, r, l, tolerance, SideOpenDualDiskReadOptions{})
}

// ReadExactWallSideOpenDualDiskProductWithOptions adds only the selected exact
// line/circle/plane encoding proofs. Each source circle retains its own radius,
// center and native owner; the complete reconstructed graph is still compared.
func ReadExactWallSideOpenDualDiskProductWithOptions(ctx context.Context, r io.Reader, l Limits, tolerance float64, options SideOpenDualDiskReadOptions) (ExactWallSideOpenDualDiskProduct, error) {
	result, err := readSideOpenDualDiskProduct(ctx, r, l, tolerance, options, sideDiskExactWalls)
	if err != nil {
		return ExactWallSideOpenDualDiskProduct{}, err
	}
	out := ExactWallSideOpenDualDiskProduct{Descriptor: ExactWallSideOpenDualDiskDescriptor(result.Descriptor), Native: result.exactWallNative, Body: result.Body, Shell: result.Shell, Faces: result.Faces, Edges: result.Edges, ReversedModelLines: result.ReversedModelLines, RephasedModelCircles: result.RephasedModelCircles, OppositeNormalModelCircles: result.OppositeNormalModelCircles, OppositeNormalPlanes: result.OppositeNormalPlanes}
	for i, ids := range out.Native.WallFaces {
		if len(ids) != 1 || ids[0] < 0 || ids[0] >= len(out.Faces) {
			return ExactWallSideOpenDualDiskProduct{}, geom.ErrDomain
		}
		out.SourceWallFaces[i] = out.Faces[ids[0]]
		if _, ok := out.Native.Solid.Faces()[ids[0]].Surface.(*geom.ExactBezierSurface); !ok {
			return ExactWallSideOpenDualDiskProduct{}, geom.ErrDomain
		}
	}
	if out.Native.FloorFace >= 0 {
		if out.Native.FloorFace >= len(out.Faces) {
			return ExactWallSideOpenDualDiskProduct{}, geom.ErrDomain
		}
		out.SourceFloorFace = out.Faces[out.Native.FloorFace]
	}
	section, err := trimmed.CertifyGeneralSideOpenDualDiskSection(ctx, out.Descriptor.Min, out.Descriptor.Max, out.Descriptor.Discs, trimvalidate.Options{})
	if err != nil {
		return ExactWallSideOpenDualDiskProduct{}, err
	}
	lo, hi := sideRat(out.Descriptor.Discs[0].Center.X), sideRat(out.Descriptor.Discs[1].Center.X)
	if section.Join[0].Cmp(lo) < 0 {
		out.RadicalSide = ExternalRadicalLeft
	} else if section.Join[0].Cmp(hi) > 0 {
		out.RadicalSide = ExternalRadicalRight
	} else {
		return ExactWallSideOpenDualDiskProduct{}, geom.ErrDomain
	}
	for i, q := range section.Join {
		if _, exact := q.Float64(); !exact {
			return ExactWallSideOpenDualDiskProduct{}, geom.ErrUnrepresentable
		}
		out.RadicalJoin[i] = new(big.Rat).Set(q)
	}
	return out, nil
}

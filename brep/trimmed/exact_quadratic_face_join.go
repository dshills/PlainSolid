package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
)

// ExactQuadraticFaceJoinInput supplies exactly one CCW outer loop representation.
// NativeOuter retains nonbinary controls. SplineOuter is converted only toward
// exact homogeneous controls, never the reverse, and must have binary endpoints.
// At least one of the two inputs must use NativeOuter. Holes remain unsupported.
type ExactQuadraticFaceJoinInput struct {
	Patch       *geom.SurfacePatch
	NativeOuter []ExactQuadraticUV
	SplineOuter []SplineUV
	SharedTrim  int
}
type ExactQuadraticFaceJoinOptions struct {
	Face                         ExactQuadraticRegionMeshOptions
	Agreement                    geom.SurfaceCurveAgreementOptions
	MaxPasses, MaxSharedVertices int
}

// CertifiedExactQuadraticFaceJoin carries a complete symbolic continuous seam
// agreement and exact rational composition partitions on both owners.
// Trims certifies UV-space chord boundaries; the embedded face meshes include
// final model seam replacement and recertification. SourceParameterMaps maps
// each directed local normalized trim parameter to its original input parameter.
// SharedParameters follows face A; face B local traversal uses1-q.
type CertifiedExactQuadraticFaceJoin struct {
	CertifiedQuadraticTrimFaceJoin
	Trims               [2][]CertifiedExactQuadraticTrimPolyline
	SourceParameterMaps [2][]ExactTrimParameterMap
	Agreement           geom.SurfaceCurveAlgebraicAgreement
	Compositions        [2]geom.SurfaceCurveCompositionPieces
}

// StitchExactQuadraticFacesCertified proves whole composed-seam equality using
// geom.CompareSurfaceCurveCompositions before synchronizing rational cuts and
// triangulating either source fill. Native homogeneous controls remain exact.
// Every common model sample is evaluated/rounded once and all triangle errors
// are recertified. Opposite edge incidence and seam vertex links are checked.
// Irrational knot cuts remain unresolved even when algebraic agreement succeeds.
// Per-face work budgets accumulate across retries; no embedding/closed-shell
// guarantee is implied and errors return no partial mesh. Shared-sample
// replacement can create geometric folds despite valid distance bounds; a
// separate exact contact/embedding check is needed to certify a material mesh.
func StitchExactQuadraticFacesCertified(ctx context.Context, a, b ExactQuadraticFaceJoinInput, o ExactQuadraticFaceJoinOptions) (CertifiedExactQuadraticFaceJoin, error) {
	return stitchExactQuadraticFaces(ctx, a, b, o, false)
}
func stitchExactQuadraticFaces(ctx context.Context, a, b ExactQuadraticFaceJoinInput, o ExactQuadraticFaceJoinOptions, fixedBoundary bool) (CertifiedExactQuadraticFaceJoin, error) {
	empty := CertifiedExactQuadraticFaceJoin{}
	if ctx == nil || a.Patch == nil || b.Patch == nil || (len(a.NativeOuter) == 0 && len(b.NativeOuter) == 0) {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	var native [2][]ExactQuadraticUV
	var inputs [2]QuadraticTrimFaceJoinInput
	var maps [2][]ExactTrimParameterMap
	for fi, input := range []ExactQuadraticFaceJoinInput{a, b} {
		if (len(input.NativeOuter) == 0) == (len(input.SplineOuter) == 0) {
			return empty, geom.ErrInvalid
		}
		count := len(input.NativeOuter)
		if count == 0 {
			count = len(input.SplineOuter)
		}
		if count < 2 || count > brep.MaxHoledFaceVertices || input.SharedTrim < 0 || input.SharedTrim >= count {
			return empty, geom.ErrInvalid
		}
		inputs[fi] = QuadraticTrimFaceJoinInput{Patch: input.Patch, Outer: make([]SplineUV, count), SharedTrim: input.SharedTrim}
		for j := 0; j < count; j++ {
			q := ExactQuadraticUV{}
			mapping := ExactTrimParameterMap{Offset: new(big.Rat), Scale: big.NewRat(1, 1), Degree: 2}
			if len(input.NativeOuter) > 0 {
				q = input.NativeOuter[j]
				if q.control[0][0] == nil {
					return empty, geom.ErrInvalid
				}
			} else {
				source := input.SplineOuter[j]
				h, e := quadraticTrimSourceControls(source)
				if e != nil {
					return empty, e
				}
				q, e = NewExactQuadraticUV(h)
				if e != nil {
					return empty, e
				}
				mapping.Offset = ratInset(source.Range.Min)
				mapping.Scale = new(big.Rat).Sub(ratInset(source.Range.Max), mapping.Offset)
				mapping.Degree = source.Curve.Degree()
				if source.Reversed {
					mapping.Offset = ratInset(source.Range.Max)
					mapping.Scale.Neg(mapping.Scale)
				}
			}
			native[fi] = append(native[fi], q)
			maps[fi] = append(maps[fi], mapping)
		}
	}
	result, e := stitchControlledCutFaces(ctx, inputs[0], inputs[1], RationalCutFaceJoinOptions{Face: RationalCutFaceMeshOptions{Face: o.Face.Face, MaxInputBits: o.Face.MaxInputBits}, MaxPasses: o.MaxPasses, MaxSharedVertices: o.MaxSharedVertices}, &native, o.Face.Composition, o.Agreement, fixedBoundary)
	if e != nil {
		return empty, e
	}
	return CertifiedExactQuadraticFaceJoin{CertifiedQuadraticTrimFaceJoin: result.CertifiedQuadraticTrimFaceJoin, Trims: result.nativeTrims, SourceParameterMaps: maps, Agreement: result.nativeAgreement, Compositions: result.nativeCompositions}, nil
}

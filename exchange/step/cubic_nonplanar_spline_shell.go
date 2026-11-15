package step

import (
	"context"
	"io"
)

// CubicNonplanarSplineShellSource retains a strictly authenticated six-face
// nonplanar polynomial spline material with original degree-one owner pcurves.
// Carrier degrees through three and at most four poles per axis are admitted;
// original controls and source geometry are never reduced or rounded.
type CubicNonplanarSplineShellSource NonplanarSplineShellSource

func (m CubicNonplanarSplineShellSource) OriginalSTEP() string { return m.original }
func (m CubicNonplanarSplineShellSource) PCurves() []SplineShellPCurveEvidence {
	return append([]SplineShellPCurveEvidence(nil), m.curves...)
}
func (m CubicNonplanarSplineShellSource) SourceFaces() []int64 {
	return append([]int64(nil), m.faces...)
}
func (m CubicNonplanarSplineShellSource) SourceEdges() []int64 {
	return append([]int64(nil), m.edges...)
}

// ReadCubicNonplanarSplineShellSource reuses the exact original model/pcurve
// and complete native embedding proof with a separately bounded cubic carrier
// gate. The quadratic and multispan UV routes keep their original limits.
func ReadCubicNonplanarSplineShellSource(ctx context.Context, r io.Reader, l Limits) (CubicNonplanarSplineShellSource, error) {
	m, e := readNonplanarSplineShellSource(ctx, r, l, 3)
	return CubicNonplanarSplineShellSource(m), e
}

package step

import (
	"context"
	"io"
)

type affineSupportMode uint8

const (
	supportAxis affineSupportMode = iota
	supportOrthogonal
	supportAffine
)

// AffineSplineShellBody/Model retain exact planar affine source charts and
// topology. These aliases use the same source fields as the narrower readers.
type AffineSplineShellBody = AxisAlignedSplineShellBody
type AffineSplineShellModel = AxisAlignedSplineShellModel

// ReadAffineSplineShells accepts exact nondegenerate clamped bilinear affine
// supports, including shear. Orthogonal UV loops and holes, source model/UV
// identities, closed incidence, and global planar embedding/outward orientation
// must all be proved. Neither model coordinates nor support axes are normalized.
// Nonaffine or curved surfaces remain unsupported.
func ReadAffineSplineShells(r io.Reader, l Limits) (AffineSplineShellModel, error) {
	return ReadAffineSplineShellsContext(context.Background(), r, l)
}
func ReadAffineSplineShellsContext(ctx context.Context, r io.Reader, l Limits) (AffineSplineShellModel, error) {
	return readAffineSplineShellsContext(ctx, r, l, supportAffine)
}

// WriteAffineSplineShells preserves the exact affine source graph and requires
// independent closed-solid readback before emitting bytes. The Solid field is
// derived; Faces, FaceHoles, FaceIDs and Uses define the serialized source.
func WriteAffineSplineShells(w io.Writer, m AffineSplineShellModel) error {
	graph := OrthogonalSplineOpenShellModel{LengthUnit: m.LengthUnit}
	for _, b := range m.Bodies {
		graph.Shells = append(graph.Shells, OrthogonalSplineOpenShell{Name: b.Name, Faces: b.Faces, FaceHoles: b.FaceHoles, FaceIDs: b.FaceIDs, Uses: b.Uses})
	}
	return writeOrthogonalSplineShells(w, graph, true, supportAffine)
}

package step

import (
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WrappedFrustumBody is a positive-radius homothetic rational draft support
// with exact planar caps and a separately certified material embedding.
type WrappedFrustumBody struct {
	Name      string
	Face      WrappedSplineFace
	CapNames  [2]string
	Solid     trimmed.Solid
	Embedding geom.WrappedFrustumEmbedding
}
type WrappedFrustumModel struct {
	Bodies     []WrappedFrustumBody
	LengthUnit float64
}

// ReadWrappedFrusta shares exact capped graph validation with the cylinder
// reader but requires the distinct affine-radius frustum material proof.
func ReadWrappedFrusta(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedFrustumModel, error) {
	m, e := readWrappedCappedBodies(r, l, o, true)
	if e != nil {
		return WrappedFrustumModel{}, e
	}
	out := WrappedFrustumModel{LengthUnit: m.LengthUnit}
	for _, b := range m.Bodies {
		out.Bodies = append(out.Bodies, WrappedFrustumBody{Name: b.Name, Face: b.Face, CapNames: b.CapNames, Solid: b.Solid, Embedding: b.FrustumProof})
	}
	return out, nil
}

// WriteWrappedFrusta regenerates exact shared rim curves and planar pcurves,
// preserving all face names and units. Derived Solid and Embedding are re-proved.
func WriteWrappedFrusta(w io.Writer, m WrappedFrustumModel) error {
	input := WrappedCylindricalModel{LengthUnit: m.LengthUnit}
	for _, b := range m.Bodies {
		input.Bodies = append(input.Bodies, WrappedCylindricalBody{Name: b.Name, Face: b.Face, CapNames: b.CapNames})
	}
	return writeWrappedCappedBodies(w, input, true)
}

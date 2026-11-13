package revolution

import (
	"io"
	"plainsolid/exchange/step"
)

// WriteRationalRevolutionSurfacesJSON explicitly writes version2 of the native
// standalone analytic revolution envelope. Version2 accepts the bounded exact
// rational-meridian proof in addition to polynomial profiles. It preserves the
// represented normalized weights; no approximation or topology is introduced.
// Existing WriteRevolutionSurfacesJSON continues to emit polynomial-only version1.
func WriteRationalRevolutionSurfacesJSON(w io.Writer, m step.RevolutionSurfaceModel) error {
	return writeRevolutionSurfacesJSON(w, m, 2)
}

// ReadRationalRevolutionSurfacesJSON accepts canonical versions1 and2, applying
// the original polynomial restriction to version1 and the bounded rational
// derivative proof to version2. Other versions fail; any error returns an empty
// model. Existing ReadRevolutionSurfacesJSON continues to reject version2.
func ReadRationalRevolutionSurfacesJSON(r io.Reader, l RevolutionSurfaceJSONLimits) (step.RevolutionSurfaceModel, error) {
	return readRevolutionSurfacesJSON(r, l, true)
}

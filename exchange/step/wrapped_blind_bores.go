package step

import "io"

// WrappedBlindBoreBody is a single-opening coaxial rational bore. CapNames
// identify outer lower/upper faces, one annular; FloorName identifies the
// separate inward-material floor. It never represents a closed cavity.
type WrappedBlindBoreBody WrappedTubeBody

func ReadWrappedBlindBore(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedBlindBoreBody, error) {
	m, e := readWrappedBore(r, l, o, true)
	return WrappedBlindBoreBody(m), e
}
func WriteWrappedBlindBore(w io.Writer, m WrappedBlindBoreBody) error {
	return writeWrappedBore(w, WrappedTubeBody(m), true)
}

package step

import (
	"context"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WrappedAnnularFrustumBody has exactly one band on each coaxial circular wall,
// with common heights and strictly nested radii. At least one wall must taper;
// two cylindrical walls remain exclusively in the WrappedTube API.
// Outer and Inner each have exactly two stations and one optional wall chart.
type WrappedAnnularFrustumBody WrappedStationBoreBody

func wrappedAxialBoreTapered(m WrappedStationBoreBody) bool {
	for _, b := range []WrappedStationLoftBody{m.Outer, m.Inner} {
		if len(b.Stations) == 2 && b.Stations[0].Radius != b.Stations[1].Radius {
			return true
		}
	}
	return false
}

// ReadWrappedAnnularFrustum requires an explicit four-face manifold shell and
// exact model/pcurve correspondence, annular bound roles and material nesting.
func ReadWrappedAnnularFrustum(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedAnnularFrustumBody, error) {
	m, e := readWrappedAxialBore(r, l, o, true)
	return WrappedAnnularFrustumBody(m), e
}

// WriteWrappedAnnularFrustum certifies source supports and the final closed graph
// before output. Neither cached Solid nor source topology IDs supply the proof.
func WriteWrappedAnnularFrustum(w io.Writer, m WrappedAnnularFrustumBody) error {
	return writeWrappedAxialBore(w, WrappedStationBoreBody(m), true)
}

// WrappedAnnularFrustumFromSolid validates original native topology and recovers
// exact represented station data and the original support charts.
func WrappedAnnularFrustumFromSolid(ctx context.Context, s trimmed.Solid, name string, unit float64, o geom.SurfacePatchOptions) (WrappedAnnularFrustumBody, error) {
	m, e := wrappedAxialBoreFromSolid(ctx, s, name, unit, o, true)
	return WrappedAnnularFrustumBody(m), e
}

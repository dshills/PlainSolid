package step

import (
	"context"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/cuspidal"
	"plainsolid/geom"
)

// WriteCuspidalCubicCut transfers the original closed Cut shell with
// the two exact cusp germs and their unchanged roof supports.
// With zero pole tolerance, a cubic wall pcurve at a shared original
// cusp knot may be elevated exactly to quartic for decimal transfer.
func WriteCuspidalCubicCut(ctx context.Context, w io.Writer, cut *trimmed.CuspidalCubicCut, name string, unit float64, l Limits, o CuspidalCubicSTEPOptions) error {
	if cut == nil {
		return geom.ErrInvalid
	}
	h := cut.History()
	origin := cuspidal.KnotOrigin{Vertex: h.Vertices[0], RoofFaces: h.RoofFaces, Germs: h.Germs}
	return writeCuspidalCubicSTEPWithWallElevation(ctx, w, cut.Boundary(), name, unit, l, o, origin)
}

// VerifyCuspidalCubicCutSTEP checks the canonical topology, physical
// units, original supports and both directed edge uses against the source.
func VerifyCuspidalCubicCutSTEP(ctx context.Context, input io.Reader, cut *trimmed.CuspidalCubicCut, name string, unit float64, l Limits, o CuspidalCubicSTEPOptions) error {
	if cut == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleSTEP(ctx, input, l, "cuspidal cubic Cut", func(w io.Writer) error {
		return WriteCuspidalCubicCut(ctx, w, cut, name, unit, l, o)
	})
}

package iges

import (
	"context"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/cuspidal"
	"plainsolid/geom"
)

// WriteCuspidalCubicCut transfers the original Cut layer as a closed type
// 186 solid, retaining the exact cusp germs and their original coedges.
// With zero pole tolerance, a cubic wall pcurve at a shared original
// cusp knot may be elevated exactly to quartic for decimal transfer.
func WriteCuspidalCubicCut(ctx context.Context, w io.Writer, cut *trimmed.CuspidalCubicCut, unit float64, l Limits, o CuspidalCubicIGESOptions) error {
	if cut == nil {
		return geom.ErrInvalid
	}
	h := cut.History()
	origin := cuspidal.KnotOrigin{Vertex: h.Vertices[0], RoofFaces: h.RoofFaces, Germs: h.Germs}
	return writeCuspidalCubicIGESWithWallElevation(ctx, w, cut.Boundary(), unit, l, o, origin)
}

// VerifyCuspidalCubicCutIGES authenticates the topology, original
// owner curves and intended physical units against the native Cut.
func VerifyCuspidalCubicCutIGES(ctx context.Context, input io.Reader, cut *trimmed.CuspidalCubicCut, unit float64, l Limits, o CuspidalCubicIGESOptions) error {
	if cut == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleIGES(ctx, input, l, "cuspidal cubic Cut", func(w io.Writer) error {
		return WriteCuspidalCubicCut(ctx, w, cut, unit, l, o)
	})
}

package step

import (
	"context"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WrappedRationalStationLoftBody is a canonical weighted-quadrant station loft.
// Walls are the original represented spline supports, ordered from bottom to top.
// Writer input is Walls, names and LengthUnit; Solid is a rederived cache, never
// an authority. STEP uses full weighted spline rims, not circular substitutes.
// Numeric exchange follows the existing represented binary64 STEP spline dialect.
// Original authored profile scale factors are not recoverable from physical walls.
type WrappedRationalStationLoftBody struct {
	Name       string
	Walls      []WrappedSplineFace
	CapNames   [2]string
	Solid      trimmed.Solid
	LengthUnit float64
}

// ReadWrappedRationalStationLoft authenticates one closed shell with 1..16
// original weighted bands and two planar caps, including full model/pcurve
// correspondence and shared station edge identity. It is a bounded paired
// dialect, not a generic spline-solid importer.
func ReadWrappedRationalStationLoft(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedRationalStationLoftBody, error) {
	m, e := readWrappedStationLoft(r, l, o, false)
	if e != nil {
		return WrappedRationalStationLoftBody{}, e
	}
	return WrappedRationalStationLoftBody{Name: m.Name, Walls: m.Walls, CapNames: m.CapNames, Solid: m.Solid, LengthUnit: m.LengthUnit}, nil
}

// WriteWrappedRationalStationLoft proves the original source and stages the
// entire shared topology before writing. Unsupported profiles or unequal
// adjacent rims fail without writing a partial model.
func WriteWrappedRationalStationLoft(w io.Writer, m WrappedRationalStationLoftBody) error {
	if w == nil || !validLengthUnit(m.LengthUnit) || len(m.Walls) < 1 || len(m.Walls) > 16 {
		return geom.ErrInvalid
	}
	supports := make([]*geom.WrappedSurfacePatch, len(m.Walls))
	for i := range m.Walls {
		supports[i] = m.Walls[i].Geometry
	}
	if _, _, e := trimmed.NewWrappedRationalStationLoft(context.Background(), supports, 1e-10, geom.SurfacePatchOptions{}); e != nil {
		return e
	}
	parts := WrappedFrustumModel{LengthUnit: m.LengthUnit}
	for _, wall := range m.Walls {
		parts.Bodies = append(parts.Bodies, WrappedFrustumBody{Face: wall, CapNames: m.CapNames})
	}
	return writeWrappedStationGraph(w, m.Name, parts, false)
}

// WrappedRationalStationLoftFromSolid retains each authenticated original wall.
func WrappedRationalStationLoftFromSolid(ctx context.Context, s trimmed.Solid, name string, unit float64, o geom.SurfacePatchOptions) (WrappedRationalStationLoftBody, error) {
	if !validLengthUnit(unit) {
		return WrappedRationalStationLoftBody{}, geom.ErrInvalid
	}
	p, e := s.ValidateWrappedRationalStationLoft(ctx, o)
	if e != nil {
		return WrappedRationalStationLoftBody{}, e
	}
	if !p.Complete {
		return WrappedRationalStationLoftBody{}, geom.ErrConvergence
	}
	out := WrappedRationalStationLoftBody{Name: name, Solid: s, LengthUnit: unit}
	faces := s.Faces()
	for _, id := range p.History.WallFaces {
		wall, ok := faces[id].Surface.(*geom.WrappedSurfacePatch)
		if !ok {
			return WrappedRationalStationLoftBody{}, geom.ErrConvergence
		}
		out.Walls = append(out.Walls, WrappedSplineFace{Geometry: wall})
	}
	return out, nil
}

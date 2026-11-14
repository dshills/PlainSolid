package plaincadbridge

import (
	"io"
	"math"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func (p radialSector) writeSectorSTEP(w io.Writer) error {
	body := step.CylinderSectorBody{
		Name: "PlainSolid", Frame: p.frame, Radius: p.radius, Height: p.height, Angle: p.angle,
	}
	return step.WriteCylinderSectors(w, step.CylinderSectorModel{Bodies: []step.CylinderSectorBody{body}, LengthUnit: .001})
}

// cylinderSectorFromSTEP recognizes the exact native five-face cylinder
// sector rebuilt by ReadAnalyticBodies after its complete graph validation.
func cylinderSectorFromSTEP(m step.AnalyticModel) (radialSector, bool) {
	if len(m.Bodies) != 1 {
		return radialSector{}, false
	}
	s := m.Bodies[0].Solid
	faces := s.Faces()
	if len(faces) != 5 || len(s.Vertices()) != 6 || len(s.Edges()) != 11 {
		return radialSector{}, false
	}
	wall := faces[0]
	c, ok := wall.Surface.(geom.CylinderSurface)
	if !ok || wall.Reversed || wall.U.Min != 0 || wall.U.Max <= 0 || wall.U.Max >= 2*math.Pi || wall.V.Min != 0 || wall.V.Max <= 0 {
		return radialSector{}, false
	}
	f, radius := c.Parameters()
	scale := m.LengthUnit / .001
	if !finite(scale) || scale <= 0 {
		return radialSector{}, false
	}
	if scale != 1 {
		var err error
		f, err = geom.NewFrame(f.Origin().Scale(scale), f.XAxis(), f.ZAxis())
		if err != nil {
			return radialSector{}, false
		}
	}
	return radialSector{frame: f, radius: radius * scale, height: wall.V.Max * scale, angle: wall.U.Max}, true
}

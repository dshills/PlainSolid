package plaincadbridge

import (
	"io"
	"math"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func (p radialSector) writeTubeSTEP(w io.Writer) error {
	body := step.TubeBody{Name: "PlainSolid", Frame: p.frame, InnerRadius: p.inner, OuterRadius: p.radius, Height: p.height}
	return step.WriteTubes(w, step.TubeModel{Bodies: []step.TubeBody{body}, LengthUnit: .001})
}

func tubeFromSTEP(m step.AnalyticModel) (radialSector, bool) {
	if len(m.Bodies) != 1 {
		return radialSector{}, false
	}
	frame, outer, inner, height, ok := m.Bodies[0].Solid.ConcentricTubeParameters()
	scale := m.LengthUnit / .001
	if !ok || !finite(scale) || scale <= 0 {
		return radialSector{}, false
	}
	if scale != 1 {
		var err error
		frame, err = geom.NewFrame(frame.Origin().Scale(scale), frame.XAxis(), frame.ZAxis())
		if err != nil {
			return radialSector{}, false
		}
	}
	return radialSector{frame: frame, inner: inner * scale, radius: outer * scale, height: height * scale, angle: 2 * math.Pi}, true
}

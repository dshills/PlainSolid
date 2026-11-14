package plaincadbridge

import (
	"io"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

type torusSweep struct {
	frame               geom.Frame
	major, minor, angle float64
}

func (p torusSweep) body() (*analyticBody, error) {
	var solid parametric.Solid
	var err error
	if p.angle == 2*math.Pi {
		solid, err = parametric.Torus(p.frame, p.major, p.minor)
	} else {
		solid, err = parametric.TorusElbow(p.frame, p.major, p.minor, p.angle)
	}
	if err != nil {
		return nil, err
	}
	body, err := revolutionPatches(solid)
	if err != nil {
		return nil, err
	}
	body.torus = &p
	return body, nil
}

func (p torusSweep) transformed(m geom.Affine) (*torusSweep, error) {
	z := m.ApplyVector(p.frame.ZAxis())
	if m.Determinant() < 0 {
		// Reversing the chart normal retains the reflected direction of the
		// sweep. The circular meridian is symmetric about the sweep plane.
		z = z.Scale(-1)
	}
	f, err := geom.NewFrame(m.Apply(p.frame.Origin()), m.ApplyVector(p.frame.XAxis()), z)
	if err != nil {
		return nil, err
	}
	p.frame = f
	return &p, nil
}

func (p torusSweep) writeSTEP(w io.Writer) error {
	if p.angle == 2*math.Pi {
		body := step.TorusBody{Name: "PlainSolid", Frame: p.frame, MajorRadius: p.major, MinorRadius: p.minor}
		return step.WriteTori(w, step.TorusModel{LengthUnit: .001, Bodies: []step.TorusBody{body}})
	}
	body := step.TorusElbowBody{Name: "PlainSolid", Frame: p.frame, MajorRadius: p.major, MinorRadius: p.minor, Angle: p.angle}
	return step.WriteTorusElbows(w, step.TorusElbowModel{LengthUnit: .001, Bodies: []step.TorusElbowBody{body}})
}

// torusFromSTEP only accepts the canonical solid topology reconstructed by
// ReadAnalyticBodies after checking the STEP face, rim, and seam records.
func torusFromSTEP(m step.AnalyticModel) (torusSweep, bool) {
	if len(m.Bodies) != 1 {
		return torusSweep{}, false
	}
	faces := m.Bodies[0].Solid.Faces()
	if len(faces) != 1 && len(faces) != 3 {
		return torusSweep{}, false
	}
	face := faces[0]
	torus, ok := face.Surface.(geom.TorusSurface)
	if !ok || face.Reversed || face.U.Min != 0 || face.V.Min != 0 || face.V.Max != 2*math.Pi {
		return torusSweep{}, false
	}
	angle := face.U.Max
	if (angle == 2*math.Pi) != (len(faces) == 1) {
		return torusSweep{}, false
	}
	f, major, minor := torus.Parameters()
	scale := m.LengthUnit / .001
	if !finite(scale) || scale <= 0 {
		return torusSweep{}, false
	}
	if scale != 1 {
		var err error
		f, err = geom.NewFrame(f.Origin().Scale(scale), f.XAxis(), f.ZAxis())
		if err != nil {
			return torusSweep{}, false
		}
	}
	return torusSweep{frame: f, major: major * scale, minor: minor * scale, angle: angle}, true
}

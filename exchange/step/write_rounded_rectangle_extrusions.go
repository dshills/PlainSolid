package step

import (
	"context"
	"io"
	"math"
	"unicode/utf8"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type RoundedRectangleExtrusionBody struct {
	Name    string
	Frame   geom.Frame
	Profile trimmed.RoundedRectangle
	Holes   []trimmed.ExtrusionCircle
	Blind   []trimmed.ExtrusionBlindHole
	Height  float64
}

type RoundedRectangleExtrusionModel struct {
	Bodies     []RoundedRectangleExtrusionBody
	LengthUnit float64
}

type roundedSTEPArc struct {
	center  geom.Vec2
	radius  float64
	quarter int
}

// WriteRoundedRectangleExtrusions exports the native mixed line and
// quarter-circle profile. Curved walls and rims are cylinders and circular
// model curves with explicit pcurves. The capped B-rep is fully analytic.
func WriteRoundedRectangleExtrusions(w io.Writer, m RoundedRectangleExtrusionModel) error {
	if w == nil || len(m.Bodies) == 0 || len(m.Bodies) > 32 {
		return geom.ErrInvalid
	}
	bodies := make([]circularExtrusionExport, len(m.Bodies))
	for i, b := range m.Bodies {
		if !utf8.ValidString(b.Name) || len(b.Name) > 4096 {
			return geom.ErrInvalid
		}
		s, e := trimmed.ExtrudeRoundedRectangle(context.Background(), b.Frame, b.Profile, b.Holes, b.Blind, b.Height)
		if e != nil {
			return e
		}
		faces := s.Faces()
		var p []geom.Vec2
		var arcs []roundedSTEPArc
		for _, u := range faces[0].Outer {
			p = append(p, u.UVStart)
			a := roundedSTEPArc{}
			if u.Curve != nil {
				// The native constructor proved exact cardinal joins. The
				// embedded EllipseUV is exposed as exact represented data by
				// its promoted method; no sampled curve is reconstructed.
				c, ok := u.Curve.(interface{ EllipseParameters() trimmed.EllipseUV })
				if !ok {
					return geom.ErrInvalid
				}
				q := c.EllipseParameters()
				a = roundedSTEPArc{center: q.Center, radius: q.Cos.X, quarter: int(math.Round(q.From / (math.Pi / 2)))}
			}
			arcs = append(arcs, a)
		}
		bodies[i] = circularExtrusionExport{
			CircularHoleExtrusionBody: CircularHoleExtrusionBody{
				Name: b.Name, Frame: b.Frame, Height: b.Height, Outer: p,
				Holes: append([]trimmed.ExtrusionCircle(nil), b.Holes...),
				Blind: append([]trimmed.ExtrusionBlindHole(nil), b.Blind...),
			},
			arcs: arcs,
		}
	}
	return writeCircularExtrusionSTEP(w, m.LengthUnit, bodies)
}

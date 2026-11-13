package step

import (
	"bytes"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ReadRoundedRectangleExtrusions accepts canonical analytic rounded-rectangle
// exports. It recovers the rectangular bounds and corner radii, revalidates
// the native extrusion, then checks exact represented geometry, trim
// parameters, orientation, and sharing against the regenerated graph.
// An extrusion with no blind pockets is normalized to positive height.
func ReadRoundedRectangleExtrusions(r io.Reader, l Limits) (RoundedRectangleExtrusionModel, error) {
	f, rep, unit, ids, e := parseCanonicalCircularExtrusions(r, l)
	if e != nil {
		return RoundedRectangleExtrusionModel{}, e
	}
	m := RoundedRectangleExtrusionModel{LengthUnit: unit}
	for _, id := range ids {
		var profile trimmed.RoundedRectangle
		b, e := recoverCircularExtrusion(f, id, nil, &profile)
		if e != nil {
			return RoundedRectangleExtrusionModel{}, e
		}
		m.Bodies = append(m.Bodies, RoundedRectangleExtrusionBody{
			Name: b.Name, Frame: b.Frame, Profile: profile, Holes: b.Holes, Blind: b.Blind, Height: b.Height,
		})
	}
	var out bytes.Buffer
	if e := WriteRoundedRectangleExtrusions(&out, m); e != nil {
		return RoundedRectangleExtrusionModel{}, e
	}
	if e := verifyCanonicalCircularExtrusionGraph(f, rep, &out); e != nil {
		return RoundedRectangleExtrusionModel{}, e
	}
	return m, nil
}

func recoverRoundedRectangle(f *File, cap geom.Frame, outer []geom.Vec2, faces []int64) (trimmed.RoundedRectangle, error) {
	p := trimmed.RoundedRectangle{Min: outer[0], Max: outer[0]}
	for _, v := range outer {
		p.Min.X, p.Min.Y = min(p.Min.X, v.X), min(p.Min.Y, v.Y)
		p.Max.X, p.Max.Y = max(p.Max.X, v.X), max(p.Max.Y, v.Y)
	}
	if p.Min.X >= p.Max.X || p.Min.Y >= p.Max.Y {
		return p, geom.ErrDegenerate
	}
	// In the canonical CCW cap loop a rounded corner starts at the
	// incoming tangent, which lies on a unique rectangle side.
	for i, id := range faces {
		face, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil {
			return p, e
		}
		sid, e := ref(face.Args[2])
		if e != nil {
			return p, e
		}
		surface, e := DecodeAnalyticSurface(f, sid)
		if e != nil {
			return p, e
		}
		if _, ok := surface.(geom.PlaneSurface); ok {
			continue
		}
		c, ok := surface.(geom.CylinderSurface)
		if !ok {
			return p, geom.ErrDomain
		}
		fr, r := c.Parameters()
		center := cap.ToLocal(fr.Origin())
		a := outer[i]
		k := -1
		switch {
		case a.X == p.Min.X && a.Y > p.Min.Y && a.Y < p.Max.Y:
			k = 0
		case a.Y == p.Min.Y && a.X > p.Min.X && a.X < p.Max.X:
			k = 1
		case a.X == p.Max.X && a.Y > p.Min.Y && a.Y < p.Max.Y:
			k = 2
		case a.Y == p.Max.Y && a.X > p.Min.X && a.X < p.Max.X:
			k = 3
		}
		if k < 0 || p.Radii[k] != 0 || r <= 0 || center.Z != 0 {
			return p, geom.ErrDomain
		}
		p.Radii[k] = r
	}
	return p, nil
}

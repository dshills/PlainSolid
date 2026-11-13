package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// CapsuleExtrusionBody is one exact horizontal stadium or concentric stadium
// ring. The frame carries its world placement; radii never become chord loops.
type CapsuleExtrusionBody struct {
	Name                        string
	Frame                       geom.Frame
	Left, Right                 geom.Vec2
	Radius, InnerRadius, Height float64
}
type CapsuleExtrusionModel struct {
	Bodies     []CapsuleExtrusionBody
	LengthUnit float64
}

func capsuleSolid(b CapsuleExtrusionBody) (trimmed.Solid, error) {
	if b.InnerRadius == 0 {
		return trimmed.ExtrudeCapsule(context.Background(), b.Frame, b.Left, b.Right, b.Radius, b.Height)
	}
	return trimmed.ExtrudeCapsuleRing(context.Background(), b.Frame, b.Left, b.Right, b.Radius, b.InnerRadius, b.Height)
}
func WriteCapsuleExtrusions(w io.Writer, m CapsuleExtrusionModel) error {
	if len(m.Bodies) != 1 {
		return geom.ErrDomain
	}
	b := m.Bodies[0]
	s, e := capsuleSolid(b)
	if e != nil {
		return e
	}
	return writeNativeLongEdgeTopology(w, m.LengthUnit, b.Name, s)
}

// ReadCapsuleExtrusions discovers the frame, radii, centers and height from
// native plane/cylinder supports, then checks every source graph coefficient,
// pcurve, loop and shared edge against the regenerated analytic solid.
func ReadCapsuleExtrusions(r io.Reader, l Limits) (CapsuleExtrusionModel, error) {
	f, rep, unit, ids, e := parseCanonicalCircularExtrusions(r, l)
	if e != nil {
		return CapsuleExtrusionModel{}, e
	}
	if len(ids) != 1 {
		return CapsuleExtrusionModel{}, geom.ErrDomain
	}
	g, e := readCircularSourceGraph(f, ids[0])
	if e != nil {
		return CapsuleExtrusionModel{}, e
	}
	if len(g.faces) != 8 && len(g.faces) != 14 {
		return CapsuleExtrusionModel{}, geom.ErrDomain
	}
	if g.faces[0].kind != "PLANE" || g.faces[1].kind != "PLANE" {
		return CapsuleExtrusionModel{}, geom.ErrDomain
	}
	frame := g.faces[0].frame
	delta := frame.ToLocal(g.faces[1].frame.Origin())
	if delta.X != 0 || delta.Y != 0 || delta.Z <= 0 {
		return CapsuleExtrusionModel{}, geom.ErrDomain
	}
	record, e := simple(f, ids[0], "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return CapsuleExtrusionModel{}, e
	}
	name, e := textValue(record.Args[0])
	if e != nil {
		return CapsuleExtrusionModel{}, e
	}
	b := CapsuleExtrusionBody{Name: name, Frame: frame, Height: delta.Z}
	centers := []geom.Vec2{}
	for _, face := range g.faces[2:] {
		if face.kind != "CYLINDER" {
			continue
		}
		p := frame.ToLocal(face.frame.Origin())
		if p.Z != 0 {
			return CapsuleExtrusionModel{}, geom.ErrDomain
		}
		c := geom.Vec2{X: p.X, Y: p.Y}
		found := false
		for _, a := range centers {
			if a == c {
				found = true
			}
		}
		if !found {
			centers = append(centers, c)
		}
		if b.Radius == 0 {
			b.Radius = face.radius
		} else if face.radius != b.Radius {
			if b.InnerRadius != 0 && b.InnerRadius != face.radius {
				return CapsuleExtrusionModel{}, geom.ErrDomain
			}
			b.InnerRadius = face.radius
		}
	}
	if len(centers) != 2 {
		return CapsuleExtrusionModel{}, geom.ErrDomain
	}
	b.Left, b.Right = centers[0], centers[1]
	if b.Left.X > b.Right.X {
		b.Left, b.Right = b.Right, b.Left
	}
	if b.InnerRadius > b.Radius {
		b.Radius, b.InnerRadius = b.InnerRadius, b.Radius
	}
	m := CapsuleExtrusionModel{Bodies: []CapsuleExtrusionBody{b}, LengthUnit: unit}
	var out bytes.Buffer
	if e = WriteCapsuleExtrusions(&out, m); e != nil {
		return CapsuleExtrusionModel{}, e
	}
	if e = verifyCanonicalCircularExtrusionGraph(f, rep, &out); e != nil {
		return CapsuleExtrusionModel{}, e
	}
	return m, nil
}

// WriteCapsuleBoundary accepts only a retained boundary whose entire emitted
// graph can be recovered and exactly regenerated as a native capsule recipe.
// Placement is read from the supports; no external feature metadata is used.
func WriteCapsuleBoundary(w io.Writer, s trimmed.Solid, unit float64, name string) error {
	var out bytes.Buffer
	if e := writeNativeLongEdgeTopology(&out, unit, name, s); e != nil {
		return e
	}
	if _, e := ReadCapsuleExtrusions(bytes.NewReader(out.Bytes()), Limits{}); e != nil {
		return e
	}
	_, e := w.Write(out.Bytes())
	return e
}

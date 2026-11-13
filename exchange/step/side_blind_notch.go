package step

import (
	"bytes"
	"context"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// SideBlindNotchBody retains a finite side-opening cut on rectangular stock.
// The cylinder is centered at the frame origin. The box spans [-Left,Right],
// [-Inset,Back], and [0,Length] in the local X, Y and Z directions.
type SideBlindNotchBody struct {
	Name                                            string
	Frame                                           geom.Frame
	Left, Right, Back, Inset, Length, Radius, Depth float64
	Through                                         []trimmed.ExtrusionCircle
	Blind                                           []trimmed.ExtrusionBlindHole
}

type SideBlindNotchModel struct {
	Body       SideBlindNotchBody
	LengthUnit float64
}

func WriteSideBlindNotch(w io.Writer, m SideBlindNotchModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var out bytes.Buffer
	if err := writeSideBlindNotchBody(&out, m); err != nil {
		return err
	}
	if _, err := ReadSideBlindNotch(bytes.NewReader(out.Bytes()), Limits{}); err != nil {
		return err
	}
	_, err := io.Copy(w, &out)
	return err
}

func writeSideBlindNotchBody(w io.Writer, m SideBlindNotchModel) error {
	p := m.Body
	s, err := trimmed.BoxSideCylinderBlindWithBores(context.Background(), p.Frame, p.Left, p.Right, p.Back, p.Inset, p.Length, p.Radius, p.Depth, p.Through, p.Blind, 1e-9)
	if err != nil {
		return err
	}
	return writeNativeLongEdgeTopology(w, m.LengthUnit, p.Name, s)
}

// ReadSideBlindNotch recovers a side-opening blind cut from its actual plane
// and cylinder supports and verifies the regenerated graph, including all uses.
func ReadSideBlindNotch(r io.Reader, limits Limits) (SideBlindNotchModel, error) {
	f, rep, unit, ids, err := parseCanonicalCircularExtrusions(r, limits)
	if err != nil || len(ids) != 1 {
		return SideBlindNotchModel{}, circularSourceFailure()
	}
	p, err := recoverSideBlindNotch(f, ids[0])
	if err != nil {
		return SideBlindNotchModel{}, err
	}
	m := SideBlindNotchModel{Body: p, LengthUnit: unit}
	var out bytes.Buffer
	if err := writeSideBlindNotchBody(&out, m); err != nil {
		return SideBlindNotchModel{}, err
	}
	if err := verifyCanonicalCircularExtrusionGraph(f, rep, &out); err != nil {
		return SideBlindNotchModel{}, err
	}
	return m, nil
}

func recoverSideBlindNotch(f *File, id int64) (SideBlindNotchBody, error) {
	fail := func() (SideBlindNotchBody, error) { return SideBlindNotchBody{}, circularSourceFailure() }
	// This canonical reader compares every edge sense against regenerated STEP.
	g, err := readCircularGraph(f, id, true)
	if err != nil || len(g.faces) < 14 || len(g.faces) > 64 || g.faces[0].kind != "PLANE" || g.faces[1].kind != "PLANE" {
		return fail()
	}
	frame := g.faces[0].frame
	floor := frame.ToLocal(g.faces[1].frame.Origin())
	if floor.X != 0 || floor.Y != 0 || floor.Z <= 0 {
		return fail()
	}
	var vertices []geom.Vec3
	for _, edge := range g.edges {
		for _, vertex := range []int64{edge.a, edge.b} {
			p, err := analyticVertex(f, vertex)
			if err != nil {
				return fail()
			}
			vertices = append(vertices, frame.ToLocal(p))
		}
	}
	bounds, err := geom.Bounds(vertices)
	if err != nil || bounds.Min.Z != 0 || bounds.Max.Z <= floor.Z {
		return fail()
	}
	body, err := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	if err != nil {
		return fail()
	}
	name, err := textValue(body.Args[0])
	if err != nil {
		return fail()
	}
	p := SideBlindNotchBody{Name: name, Frame: frame, Depth: floor.Z, Length: bounds.Max.Z,
		Left: -bounds.Min.X, Right: bounds.Max.X, Back: bounds.Max.Y, Inset: -bounds.Min.Y}
	seen := map[trimmed.ExtrusionCircle]bool{}
	for _, face := range g.faces {
		if face.kind != "CYLINDER" {
			continue
		}
		center := frame.ToLocal(face.frame.Origin())
		if center.Z != 0 || face.frame.XAxis() != frame.XAxis() || face.frame.ZAxis() != frame.ZAxis() || face.sense {
			return fail()
		}
		if center == (geom.Vec3{}) {
			if p.Radius != 0 && p.Radius != face.radius {
				return fail()
			}
			p.Radius = face.radius
			continue
		}
		hole := trimmed.ExtrusionCircle{Center: geom.Vec2{X: center.X, Y: center.Y}, Radius: face.radius}
		if seen[hole] {
			continue
		}
		seen[hole] = true
		var low, high float64
		for _, use := range face.loops[0] {
			edge := g.edges[use.id]
			if edge.kind == "CIRCLE" {
				z := frame.ToLocal(edge.frame.Origin()).Z
				low, high = min(low, z), max(high, z)
			}
		}
		if low != 0 || high <= 0 || high > p.Length {
			return fail()
		}
		if high == p.Length {
			p.Through = append(p.Through, hole)
		} else {
			p.Blind = append(p.Blind, trimmed.ExtrusionBlindHole{ExtrusionCircle: hole, Depth: high})
		}
	}
	if p.Radius <= 0 {
		return fail()
	}
	return p, nil
}

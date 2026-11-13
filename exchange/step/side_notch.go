package step

import (
	"bytes"
	"context"
	"io"
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// SideNotchBody retains a blind or through side or corner cut on rectangular stock.
// The cylinder is centered at the frame origin. The box spans [-Left,Right],
// [-Inset,Back], and [0,Length] in the local X, Y and Z directions. The notch
// opens on both caps when Depth equals Length. A corner cut has Left and
// Inset both zero.
type SideNotchBody struct {
	Name                                            string
	Frame                                           geom.Frame
	Left, Right, Back, Inset, Length, Radius, Depth float64
	Through                                         []trimmed.ExtrusionCircle
	Blind                                           []trimmed.ExtrusionBlindHole
}

type SideNotchModel struct {
	Body       SideNotchBody
	LengthUnit float64
}

// Solid reconstructs the qualified analytic boundary.
func (p SideNotchBody) Solid(ctx context.Context) (trimmed.Solid, error) {
	const tolerance = 1e-9
	if p.Left == 0 && p.Inset == 0 {
		return trimmed.BoxCornerCylinderCutWithBores(ctx, p.Frame, p.Right, p.Back, p.Length, p.Radius, p.Depth, p.Through, p.Blind, tolerance)
	}
	return trimmed.BoxSideCylinderWithBores(ctx, p.Frame, p.Left, p.Right, p.Back, p.Inset, p.Length, p.Radius, p.Depth, p.Through, p.Blind, tolerance)
}

func WriteSideNotch(w io.Writer, m SideNotchModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var out bytes.Buffer
	if err := writeSideNotchBody(&out, m); err != nil {
		return err
	}
	if _, err := ReadSideNotch(bytes.NewReader(out.Bytes()), Limits{}); err != nil {
		return err
	}
	_, err := io.Copy(w, &out)
	return err
}

func writeSideNotchBody(w io.Writer, m SideNotchModel) error {
	p := m.Body
	s, err := p.Solid(context.Background())
	if err != nil {
		return err
	}
	return writeNativeLongEdgeTopology(w, m.LengthUnit, p.Name, s)
}

// ReadSideNotch recovers a side-opening cut from its actual plane
// and cylinder supports and verifies the regenerated graph, including all uses.
func ReadSideNotch(r io.Reader, limits Limits) (SideNotchModel, error) {
	f, rep, unit, ids, err := parseCanonicalCircularExtrusions(r, limits)
	if err != nil || len(ids) != 1 {
		return SideNotchModel{}, circularSourceFailure()
	}
	p, err := recoverSideNotch(f, ids[0])
	if err != nil {
		return SideNotchModel{}, err
	}
	m := SideNotchModel{Body: p, LengthUnit: unit}
	var out bytes.Buffer
	if err := writeSideNotchBody(&out, m); err != nil {
		return SideNotchModel{}, err
	}
	if err := verifyCanonicalCircularExtrusionGraph(f, rep, &out); err != nil {
		return SideNotchModel{}, err
	}
	return m, nil
}

func recoverSideNotch(f *File, id int64) (SideNotchBody, error) {
	fail := func() (SideNotchBody, error) { return SideNotchBody{}, circularSourceFailure() }
	// This canonical reader compares every edge sense against regenerated STEP.
	g, err := readCircularGraph(f, id, true)
	if err != nil || len(g.faces) < 7 || len(g.faces) > 64 || g.faces[0].kind != "PLANE" || g.faces[1].kind != "PLANE" {
		return fail()
	}
	frame := g.faces[0].frame
	end := frame.ToLocal(g.faces[1].frame.Origin())
	if end.X != 0 || end.Y != 0 || end.Z <= 0 {
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
	if err != nil || bounds.Min.Z != 0 || bounds.Max.Z < end.Z {
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
	p := SideNotchBody{Name: name, Frame: frame, Depth: end.Z, Length: bounds.Max.Z,
		Left: -bounds.Min.X, Right: bounds.Max.X, Back: bounds.Max.Y, Inset: -bounds.Min.Y}
	if p.Left == 0 && p.Inset == 0 {
		if end.Z != p.Length {
			return fail()
		}
		if g.faces[2].kind == "PLANE" {
			floor := frame.ToLocal(g.faces[2].frame.Origin())
			if floor.X == 0 && floor.Y == 0 && floor.Z > 0 && floor.Z < p.Length {
				p.Depth = floor.Z
			}
		}
	}
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
		low, high := math.Inf(1), math.Inf(-1)
		for _, use := range face.loops[0] {
			edge := g.edges[use.id]
			if edge.kind == "CIRCLE" {
				z := frame.ToLocal(edge.frame.Origin()).Z
				low, high = min(low, z), max(high, z)
			}
		}
		if low < 0 || high <= low || high > p.Length {
			return fail()
		}
		switch {
		case low == 0 && high == p.Length:
			p.Through = append(p.Through, hole)
		case low == 0:
			p.Blind = append(p.Blind, trimmed.ExtrusionBlindHole{ExtrusionCircle: hole, Depth: high})
		case high == p.Length:
			p.Blind = append(p.Blind, trimmed.ExtrusionBlindHole{ExtrusionCircle: hole, Depth: high - low, OppositeCap: true})
		default:
			return fail()
		}
	}
	if p.Radius <= 0 {
		return fail()
	}
	return p, nil
}

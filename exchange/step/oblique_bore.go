package step

import (
	"bytes"
	"io"
	"math"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ObliqueBoreBody is an axis-aligned rectangular stock with one through bore.
// Both elliptical openings must be complete and strictly interior to their
// entry and exit faces. The cylinder axis is directed from entry to exit.
type ObliqueBoreBody struct {
	Name                 string
	Stock                geom.Box
	Entry, Exit          int
	AxisPoint, Direction geom.Vec3
	Radius               float64
}

type ObliqueBoreModel struct {
	Body       ObliqueBoreBody
	LengthUnit float64
}

// WriteObliqueBore writes exact conic and planar/cylindrical geometry with a
// shared closed topology, then proves canonical readback before publishing it.
func WriteObliqueBore(w io.Writer, m ObliqueBoreModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var out bytes.Buffer
	if err := writeObliqueBore(&out, m); err != nil {
		return err
	}
	if _, err := ReadObliqueBore(bytes.NewReader(out.Bytes()), Limits{}); err != nil {
		return err
	}
	_, err := io.Copy(w, &out)
	return err
}

func writeObliqueBore(w io.Writer, m ObliqueBoreModel) error {
	p := m.Body
	base, err := brep.Box(p.Stock.Min, p.Stock.Max)
	if err != nil {
		return err
	}
	p, err = canonicalObliqueBore(p, base)
	if err != nil {
		return err
	}
	solid, _, err := trimmed.BoreCylinderAlong(base, p.Entry, p.Exit, p.AxisPoint, p.Direction, p.Radius, 1e-9)
	if err != nil {
		return err
	}
	limits := nativeLongEdgeLimits{10, 24, 16, 2, 4}
	return writeNativeLongEdgeTopologyWithDirection(w, m.LengthUnit, p.Name, solid, limits, &p.Direction)
}

// Represent the direction by ratios, as STEP DIRECTION permits, and choose
// the center on the entry cap as the one canonical point of the axis. This
// representation survives exact graph readback without re-normalization or
// a floating-point residual from a second plane intersection.
func canonicalObliqueBore(p ObliqueBoreBody, base brep.Solid) (ObliqueBoreBody, error) {
	n := p.Direction
	scale := max(math.Abs(n.X), math.Abs(n.Y), math.Abs(n.Z))
	if !n.IsFinite() || scale == 0 || !p.AxisPoint.IsFinite() {
		return ObliqueBoreBody{}, geom.ErrInvalid
	}
	p.Direction = geom.Vec3{X: n.X / scale, Y: n.Y / scale, Z: n.Z / scale}
	for _, fi := range []int{p.Entry, p.Exit} {
		plane, err := base.FacePlane(fi)
		if err != nil || plane.Normal().Cross(p.Direction) == (geom.Vec3{}) || plane.Normal().Dot(p.Direction) == 0 {
			return ObliqueBoreBody{}, geom.ErrDomain
		}
		if fi != p.Entry {
			continue
		}
		normal, origin := plane.Normal(), plane.Origin()
		if normal.X != 0 {
			u := (origin.X - p.AxisPoint.X) / p.Direction.X
			p.AxisPoint = p.AxisPoint.Add(p.Direction.Scale(u))
			p.AxisPoint.X = origin.X
		} else if normal.Y != 0 {
			u := (origin.Y - p.AxisPoint.Y) / p.Direction.Y
			p.AxisPoint = p.AxisPoint.Add(p.Direction.Scale(u))
			p.AxisPoint.Y = origin.Y
		} else {
			u := (origin.Z - p.AxisPoint.Z) / p.Direction.Z
			p.AxisPoint = p.AxisPoint.Add(p.Direction.Scale(u))
			p.AxisPoint.Z = origin.Z
		}
	}
	return p, nil
}

// ReadObliqueBore accepts the canonical closed graph for a bore with two
// elliptical openings and recovers the corresponding stock and cylinder.
func ReadObliqueBore(r io.Reader, limits Limits) (ObliqueBoreModel, error) {
	f, rep, unit, ids, err := parseCanonicalCircularExtrusions(r, limits)
	if err != nil || len(ids) != 1 {
		return ObliqueBoreModel{}, circularSourceFailure()
	}
	p, err := recoverObliqueBore(f, ids[0])
	if err != nil {
		return ObliqueBoreModel{}, err
	}
	m := ObliqueBoreModel{Body: p, LengthUnit: unit}
	var out bytes.Buffer
	if err := writeObliqueBore(&out, m); err != nil {
		return ObliqueBoreModel{}, err
	}
	if err := verifyCanonicalCircularExtrusionGraph(f, rep, &out); err != nil {
		return ObliqueBoreModel{}, err
	}
	return m, nil
}

func recoverObliqueBore(f *File, id int64) (ObliqueBoreBody, error) {
	fail := func() (ObliqueBoreBody, error) { return ObliqueBoreBody{}, circularSourceFailure() }
	body, err := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	if err != nil {
		return fail()
	}
	name, err := textValue(body.Args[0])
	if err != nil {
		return fail()
	}
	sid, err := ref(body.Args[1])
	if err != nil {
		return fail()
	}
	shell, err := simple(f, sid, "CLOSED_SHELL", 2)
	if err != nil {
		return fail()
	}
	faces, err := listRefs(shell.Args[1], 10)
	if err != nil || len(faces) != 10 {
		return fail()
	}
	support, err := readCircularSourceFaceBounded(f, faces[6], 1, 4)
	if err != nil || support.kind != "CYLINDER" || support.sense {
		return fail()
	}
	direction, err := obliqueBoreDirection(f, support.surface)
	if err != nil {
		return fail()
	}
	result := ObliqueBoreBody{
		Name: name, Entry: -1, Exit: -1, AxisPoint: support.frame.Origin(),
		Direction: direction, Radius: support.radius,
	}
	var vertices []geom.Vec3
	for i, id := range faces[:6] {
		face, err := readCircularSourceFaceBounded(f, id, 2, 4)
		if err != nil || face.kind != "PLANE" || !face.sense || len(face.loops[0]) != 4 {
			return fail()
		}
		if len(face.loops) == 2 {
			if len(face.loops[1]) != 4 {
				return fail()
			}
			dot := face.frame.ZAxis().Dot(result.Direction)
			if dot < 0 && result.Entry == -1 {
				result.Entry = i
			} else if dot > 0 && result.Exit == -1 {
				result.Exit = i
			} else {
				return fail()
			}
		}
		for _, use := range face.loops[0] {
			start, _, _, err := analyticEdgeGeometry(f, use.id, true)
			if err != nil {
				return fail()
			}
			point, err := analyticVertex(f, start)
			if err != nil {
				return fail()
			}
			vertices = append(vertices, point)
		}
	}
	if result.Entry < 0 || result.Exit < 0 {
		return fail()
	}
	result.Stock, err = geom.Bounds(vertices)
	return result, err
}

func obliqueBoreDirection(f *File, surface int64) (geom.Vec3, error) {
	cylinder, err := simple(f, surface, "CYLINDRICAL_SURFACE", 3)
	if err != nil {
		return geom.Vec3{}, err
	}
	id, err := ref(cylinder.Args[1])
	if err != nil {
		return geom.Vec3{}, err
	}
	placement, err := simple(f, id, "AXIS2_PLACEMENT_3D", 4)
	if err != nil {
		return geom.Vec3{}, err
	}
	dir, err := ref(placement.Args[2])
	if err != nil {
		return geom.Vec3{}, err
	}
	return triple(f, dir, "DIRECTION")
}

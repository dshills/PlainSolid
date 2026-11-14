package step

import (
	"bytes"
	"io"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// DisjointObliqueBoreModel retains a bounded set of independently separated
// complete through holes, including at least one oblique cylinder.
type DisjointObliqueBoreModel struct {
	Name       string
	Stock      geom.Box
	Cuts       []trimmed.CylinderCut
	LengthUnit float64
}

func WriteDisjointObliqueBores(w io.Writer, m DisjointObliqueBoreModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var b bytes.Buffer
	if e := writeDisjointObliqueBores(&b, m); e != nil {
		return e
	}
	if _, e := ReadDisjointObliqueBores(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
		return e
	}
	_, e := io.Copy(w, &b)
	return e
}
func canonicalDisjointObliqueCut(base brep.Solid, c trimmed.CylinderCut) (trimmed.CylinderCut, error) {
	scale := max(math.Abs(c.Direction.X), math.Abs(c.Direction.Y), math.Abs(c.Direction.Z))
	if !c.Direction.IsFinite() || !c.AxisPoint.IsFinite() || scale == 0 {
		return c, geom.ErrInvalid
	}
	c.Direction = geom.Vec3{X: c.Direction.X / scale, Y: c.Direction.Y / scale, Z: c.Direction.Z / scale}
	p, e := base.FacePlane(c.Entry)
	if e != nil {
		return c, e
	}
	n, o := p.Normal(), p.Origin()
	if n.Dot(c.Direction) >= 0 {
		return c, geom.ErrDomain
	}
	if n.X != 0 {
		u := (o.X - c.AxisPoint.X) / c.Direction.X
		c.AxisPoint = c.AxisPoint.Add(c.Direction.Scale(u))
		c.AxisPoint.X = o.X
	} else if n.Y != 0 {
		u := (o.Y - c.AxisPoint.Y) / c.Direction.Y
		c.AxisPoint = c.AxisPoint.Add(c.Direction.Scale(u))
		c.AxisPoint.Y = o.Y
	} else {
		u := (o.Z - c.AxisPoint.Z) / c.Direction.Z
		c.AxisPoint = c.AxisPoint.Add(c.Direction.Scale(u))
		c.AxisPoint.Z = o.Z
	}
	return c, nil
}
func writeDisjointObliqueBores(w io.Writer, m DisjointObliqueBoreModel) error {
	if len(m.Cuts) < 2 || len(m.Cuts) > 3 {
		return geom.ErrDomain
	}
	base, e := brep.Box(m.Stock.Min, m.Stock.Max)
	if e != nil {
		return e
	}
	cuts := append([]trimmed.CylinderCut(nil), m.Cuts...)
	oblique := false
	for i, c := range cuts {
		cuts[i], e = canonicalDisjointObliqueCut(base, c)
		if e != nil {
			return e
		}
		n := cuts[i].Direction
		if (n.X != 0 && n.Y != 0) || (n.X != 0 && n.Z != 0) || (n.Y != 0 && n.Z != 0) {
			oblique = true
		}
	}
	if !oblique {
		return geom.ErrDomain
	}
	s, h, e := trimmed.BoreCylinders(base, cuts, 1e-9)
	if e != nil {
		return e
	}
	dirs := map[int]geom.Vec3{}
	for i, g := range h {
		for _, f := range g.WallFaces {
			dirs[f] = cuts[i].Direction
		}
	}
	return writeNativeMixedDisjointTopology(w, m.LengthUnit, m.Name, s, nativeLongEdgeLimits{18, 48, 32, 4, 4}, false, 0, nil, false, false, dirs)
}
func ReadDisjointObliqueBores(r io.Reader, limits Limits) (DisjointObliqueBoreModel, error) {
	f, rep, unit, ids, e := parseCanonicalCircularExtrusions(r, limits)
	if e != nil || len(ids) != 1 {
		return DisjointObliqueBoreModel{}, circularSourceFailure()
	}
	m, e := recoverDisjointObliqueBores(f, ids[0])
	if e != nil {
		return m, e
	}
	m.LengthUnit = unit
	var b bytes.Buffer
	if e = writeDisjointObliqueBores(&b, m); e != nil {
		return m, e
	}
	if e = verifyCanonicalCircularExtrusionGraph(f, rep, &b); e != nil {
		return m, e
	}
	return m, nil
}
func recoverDisjointObliqueBores(f *File, id int64) (DisjointObliqueBoreModel, error) {
	var m DisjointObliqueBoreModel
	fail := func() (DisjointObliqueBoreModel, error) {
		return m, circularSourceFailure()
	}
	b, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return fail()
	}
	m.Name, e = textValue(b.Args[0])
	if e != nil {
		return fail()
	}
	sid, e := ref(b.Args[1])
	if e != nil {
		return fail()
	}
	sh, e := simple(f, sid, "CLOSED_SHELL", 2)
	if e != nil {
		return fail()
	}
	ids, e := listRefs(sh.Args[1], 14)
	if e != nil || len(ids) < 14 || len(ids) > 18 || (len(ids)-6)%4 != 0 {
		return fail()
	}
	faces := make([]circularSourceFace, len(ids))
	var vertices []geom.Vec3
	for i, id := range ids {
		faces[i], e = readCircularSourceFaceBounded(f, id, 4, 4)
		if e != nil {
			return fail()
		}
		q := faces[i]
		if i < 6 {
			if q.kind != "PLANE" || !q.sense || len(q.loops[0]) != 4 {
				return fail()
			}
			for _, u := range q.loops[0] {
				a, _, _, e := analyticEdgeGeometry(f, u.id, true)
				if e != nil {
					return fail()
				}
				p, e := analyticVertex(f, a)
				if e != nil {
					return fail()
				}
				vertices = append(vertices, p)
			}
		} else if q.kind != "CYLINDER" || q.sense || len(q.loops) != 1 || len(q.loops[0]) != 4 {
			return fail()
		}
	}
	m.Stock, e = geom.Bounds(vertices)
	if e != nil {
		return fail()
	}
	for g := 6; g < len(faces); g += 4 {
		q := faces[g]
		d, e := obliqueBoreDirection(f, q.surface)
		if e != nil {
			return fail()
		}
		c := trimmed.CylinderCut{Entry: -1, Exit: -1, AxisPoint: q.frame.Origin(), Direction: d, Radius: q.radius}
		owned := map[int64]bool{}
		for _, wall := range faces[g : g+4] {
			for _, u := range wall.loops[0] {
				owned[u.id] = true
			}
		}
		count := 0
		for i, cap := range faces[:6] {
			for _, loop := range cap.loops[1:] {
				hits := 0
				for _, u := range loop {
					if owned[u.id] {
						hits++
					}
				}
				if hits == 0 {
					continue
				}
				if hits != 4 {
					return fail()
				}
				count++
				dot := cap.frame.ZAxis().Dot(d)
				if dot < 0 && c.Entry < 0 {
					c.Entry = i
				} else if dot > 0 && c.Exit < 0 {
					c.Exit = i
				} else {
					return fail()
				}
			}
		}
		if count != 2 || c.Entry < 0 || c.Exit < 0 {
			return fail()
		}
		m.Cuts = append(m.Cuts, c)
	}
	return m, nil
}

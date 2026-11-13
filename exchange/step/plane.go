package step

import (
	"errors"
	"math"

	"plainsolid/brep"
	"plainsolid/geom"
)

func triple(f *File, id int64, kind string) (geom.Vec3, error) {
	r, e := simple(f, id, kind, 2)
	if e != nil {
		return geom.Vec3{}, e
	}
	if _, e = textValue(r.Args[0]); e != nil {
		return geom.Vec3{}, e
	}
	v := r.Args[1]
	if v.Kind != Aggregate || len(v.Items) != 3 {
		return geom.Vec3{}, errors.New("step: expected three coordinates")
	}
	p := geom.Vec3{}
	for i, x := range []*float64{&p.X, &p.Y, &p.Z} {
		*x, e = numeric(v.Items[i])
		if e != nil {
			return geom.Vec3{}, e
		}
	}
	return p, nil
}
func direction(f *File, v Value, fallback geom.Vec3) (geom.Vec3, error) {
	if v.Kind == Omitted {
		return fallback, nil
	}
	id, e := ref(v)
	if e != nil {
		return geom.Vec3{}, e
	}
	p, e := triple(f, id, "DIRECTION")
	if e != nil {
		return geom.Vec3{}, e
	}
	return p.Unit()
}
func facePlane(f *File, face Record) (*geom.Plane, error) {
	if face.Type == "FACE" {
		return nil, nil
	}
	id, e := ref(face.Args[2])
	if e != nil {
		return nil, e
	}
	p, e := simple(f, id, "PLANE", 2)
	if e != nil {
		return nil, e
	}
	if _, e = textValue(p.Args[0]); e != nil {
		return nil, e
	}
	id, e = ref(p.Args[1])
	if e != nil {
		return nil, e
	}
	axis, e := simple(f, id, "AXIS2_PLACEMENT_3D", 4)
	if e != nil {
		return nil, e
	}
	if _, e = textValue(axis.Args[0]); e != nil {
		return nil, e
	}
	id, e = ref(axis.Args[1])
	if e != nil {
		return nil, e
	}
	origin, e := triple(f, id, "CARTESIAN_POINT")
	if e != nil {
		return nil, e
	}
	normal, e := direction(f, axis.Args[2], geom.Vec3{Z: 1})
	if e != nil {
		return nil, e
	}
	fallback := geom.Vec3{X: 1}
	if math.Abs(normal.X) > 1-1e-12 {
		fallback = geom.Vec3{Y: 1}
	}
	x, e := direction(f, axis.Args[3], fallback)
	if e != nil {
		return nil, e
	}
	if _, e = x.Sub(normal.Scale(x.Dot(normal))).Unit(); e != nil {
		return nil, errors.New("step: parallel axis and reference direction")
	}
	sense := face.Args[3]
	if sense.Kind != Enumeration || (sense.Text != "T" && sense.Text != "F") {
		return nil, errors.New("step: invalid face surface sense")
	}
	if sense.Text == "F" {
		normal = normal.Scale(-1)
	}
	plane, e := geom.NewPlane(origin, normal)
	if e != nil {
		return nil, e
	}
	return &plane, nil
}
func validateFacePlane(s brep.Solid, index int, p *geom.Plane, vertices []geom.Vec3, edges []brep.Edge, face brep.Face) error {
	if p == nil {
		return nil
	}
	actual, e := s.FacePlane(index)
	if e != nil {
		return e
	}
	if actual.Normal().Dot(p.Normal()) < 1-1e-8 {
		return errors.New("step: face bounds disagree with supporting-plane orientation")
	}
	var ids []int
	for _, loop := range append([][]brep.EdgeUse{face.Boundary}, face.Holes...) {
		for _, u := range loop {
			edge := edges[u.Edge]
			id := edge.Start
			if u.Reversed {
				id = edge.End
			}
			ids = append(ids, id)
		}
	}
	origin := vertices[ids[0]]
	scale := 0.
	for _, id := range ids {
		scale = math.Max(scale, vertices[id].Sub(origin).Length())
	}
	if scale == 0 || math.IsInf(scale, 0) {
		return errors.New("step: invalid face scale")
	}
	for _, id := range ids {
		d := p.SignedDistance(vertices[id])
		if math.IsNaN(d) || math.IsInf(d, 0) || math.Abs(d/scale) > 1e-9 {
			return errors.New("step: polygon does not lie on declared plane")
		}
	}
	return nil
}

package plaincadbridge

import (
	"errors"
	"math"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// newUnionBore retains exact layered circular footprints from a common
// opening side. Through tools can be reoriented without changing geometry;
// opposed blind pockets remain outside this bounded layered representation.
func newUnionBore(base brep.Solid, cylinders []*cylinderBody, cuts []trimmed.CylinderCut, blind []bool) (*boreBody, error) {
	fail := func(message string) (*boreBody, error) { return nil, &failureError{"UNSUPPORTED", message} }
	if len(cylinders) < 2 {
		return fail("cylindrical cut is outside the strictly contained convex-stock domain")
	}
	anchor := 0
	for i, isBlind := range blind {
		if isBlind {
			anchor = i
			break
		}
	}
	axis := cuts[anchor].Direction
	entry, exit := cuts[anchor].Entry, cuts[anchor].Exit
	depths := make([]float64, len(cylinders))
	for i, c := range cylinders {
		if cuts[i].Direction != axis {
			if blind[i] || cuts[i].Direction != axis.Scale(-1) {
				return fail("overlapping blind cylinders require a common opening side")
			}
			finish := c.frame.Origin().Add(c.frame.ZAxis().Scale(c.height))
			frame, err := geom.NewFrame(finish, c.frame.XAxis(), c.frame.ZAxis().Scale(-1))
			if err != nil {
				return nil, err
			}
			if frame.XAxis() != c.frame.XAxis() || frame.YAxis() != c.frame.YAxis().Scale(-1) || frame.ZAxis() != c.frame.ZAxis().Scale(-1) || finish.Add(frame.ZAxis().Scale(c.height)) != c.frame.Origin() {
				return fail("cylinder union reversal cannot preserve represented geometry")
			}
			c, err = newCylinder(frame, c.radius, c.height)
			if err != nil {
				return nil, err
			}
			cylinders[i] = c
			cut, _, _, err := boreSpan(base, c, false)
			if err != nil {
				return nil, err
			}
			cuts[i] = cut
		}
		if cuts[i].Entry != entry || cuts[i].Exit != exit {
			return fail("overlapping cylinders require common stock cap planes")
		}
		if blind[i] {
			_, start, _, err := boreSpan(base, c, false)
			if err != nil {
				return nil, err
			}
			depths[i] = c.height - start
		}
	}
	uniform := true
	floor := cylinders[0].frame.Origin().Add(axis.Scale(cylinders[0].height))
	for i, c := range cylinders {
		if blind[i] != blind[0] || depths[i] != depths[0] || (blind[i] && exactDotDifference(c.frame.Origin().Add(axis.Scale(c.height)), floor, axis) != 0) {
			uniform = false
		}
	}
	body := &boreBody{base: base, cylinders: append([]*cylinderBody(nil), cylinders...), cuts: cuts, blind: blind}
	if uniform {
		solid, h, err := trimmed.BoreCylinderUnion(base, cuts, depths[0], 1e-9)
		if err != nil {
			return fail("cylindrical cutter union is outside the qualified domain: " + err.Error())
		}
		body.solid, body.union = solid, &h
	} else {
		solid, h, err := trimmed.BoreCylinderDepthUnion(base, cuts, depths, 1e-9)
		if err != nil {
			return fail("layered cylindrical union is outside the qualified domain: " + err.Error())
		}
		body.solid, body.layered = solid, &h
	}
	return body, nil
}

func (body *boreBody) unionPlanarFace(origin, normal geom.Vec3) (PlanarFace, error) {
	u := body.union
	areas := make([]float64, len(u.FloorFaces))
	for i := range areas {
		areas[i] = u.Area
	}
	exitArea := 0.
	if !u.Blind {
		exitArea = u.Area
	}
	return body.cutPlanarFace(origin, normal, u.Entry, u.Exit, u.Area, exitArea, u.FloorFaces, areas)
}
func (body *boreBody) cutPlanarFace(origin, normal geom.Vec3, entry, exit int, entryArea, exitArea float64, floors []int, areas []float64) (PlanarFace, error) {
	if len(floors) != len(areas) {
		return PlanarFace{}, errors.New("invalid floor metadata")
	}
	n, err := normal.Unit()
	if err != nil || !origin.IsFinite() {
		return PlanarFace{}, errors.New("invalid planar reference")
	}
	var floor *PlanarFace
	for i, id := range floors {
		p, ok := body.solid.Faces()[id].Surface.(geom.PlaneSurface)
		if !ok {
			return PlanarFace{}, errors.New("union floor is not planar")
		}
		f := p.Frame()
		if math.Abs(math.Abs(f.ZAxis().Dot(n))-1) > 1e-9 || math.Abs(f.Origin().Sub(origin).Dot(f.ZAxis())) > 1e-7 {
			continue
		}
		if floor != nil {
			return PlanarFace{}, errors.New("planar face reference is ambiguous")
		}
		outward := f.ZAxis()
		if body.solid.Faces()[id].Reversed {
			outward = outward.Scale(-1)
		}
		q := PlanarFace{id, coordinates(f.Origin()), coordinates(outward), areas[i]}
		floor = &q
	}
	f, err := matchingPlanarFace(body.base, origin, normal)
	if floor != nil {
		if err == nil {
			return PlanarFace{}, errors.New("planar face reference is ambiguous")
		}
		return *floor, nil
	}
	if err != nil {
		return PlanarFace{}, err
	}
	if f.Face == entry {
		f.Area -= entryArea
	}
	if f.Face == exit {
		f.Area -= exitArea
	}
	if f.Area <= 1e-14 {
		return PlanarFace{}, errors.New("planar face has no measurable retained area")
	}
	return f, nil
}

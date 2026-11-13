package plaincadbridge

import (
	"errors"
	"math"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// newUnionBore is a bounded fallback for intersecting tools with one common
// entry plane and stop plane. Unequal-depth and opposed blind pockets remain
// unsupported rather than being replaced by an infinite cylinder union.
func newUnionBore(base brep.Solid, cylinders []*cylinderBody, cuts []trimmed.CylinderCut, blind []bool) (*boreBody, error) {
	fail := func(message string) (*boreBody, error) { return nil, &failureError{"UNSUPPORTED", message} }
	if len(cylinders) < 2 {
		return fail("cylindrical cut is outside the strictly contained convex-stock domain")
	}
	axis := cuts[0].Direction
	first := cylinders[0]
	floor := first.frame.Origin().Add(axis.Scale(first.height))
	for i, c := range cylinders {
		if blind[i] != blind[0] {
			return fail("overlapping cylinders require equal finite stop planes")
		}
		if cuts[i].Direction != axis {
			if blind[i] || cuts[i].Direction != axis.Scale(-1) {
				return fail("overlapping cylinders require the same represented axis direction")
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
		if cuts[i].Entry != cuts[0].Entry || cuts[i].Exit != cuts[0].Exit {
			return fail("overlapping cylinders require common stock cap planes")
		}
		if blind[0] && exactDotDifference(c.frame.Origin().Add(axis.Scale(c.height)), floor, axis) != 0 {
			return fail("overlapping blind cylinders require equal finite floor planes")
		}
	}
	depth := 0.
	if blind[0] {
		_, start, _, err := boreSpan(base, first, false)
		if err != nil {
			return nil, err
		}
		depth = first.height - start
	}
	solid, h, err := trimmed.BoreCylinderUnion(base, cuts, depth, 1e-9)
	if err != nil {
		return fail("cylindrical cutter union is outside the qualified domain: " + err.Error())
	}
	return &boreBody{solid: solid, base: base, cylinders: append([]*cylinderBody(nil), cylinders...), cuts: cuts, blind: blind, union: &h}, nil
}

func (body *boreBody) unionPlanarFace(origin, normal geom.Vec3) (PlanarFace, error) {
	u := body.union
	n, err := normal.Unit()
	if err != nil || !origin.IsFinite() {
		return PlanarFace{}, errors.New("invalid planar reference")
	}
	var floor *PlanarFace
	for _, id := range u.FloorFaces {
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
		q := PlanarFace{id, coordinates(f.Origin()), coordinates(outward), u.Area}
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
	if f.Face == u.Entry || !u.Blind && f.Face == u.Exit {
		f.Area -= u.Area
	}
	if f.Area <= 1e-14 {
		return PlanarFace{}, errors.New("planar face has no measurable retained area")
	}
	return f, nil
}

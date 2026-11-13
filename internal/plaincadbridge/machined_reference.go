package plaincadbridge

import (
	"errors"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func (s stored) retainedTrimmed() (trimmed.Solid, geom.Affine, bool) {
	m := geom.Identity()
	switch {
	case s.bore != nil:
		return s.bore.solid, m, true
	case s.treated != nil:
		return s.treated.chain.Solid, frameMap(s.treated.frame), true
	case s.analytic != nil:
		return s.analytic.solid, m, true
	default:
		return trimmed.Solid{}, m, false
	}
}

func (s stored) pocketFloor(q request) error {
	mismatch := &failureError{"MISMATCH", "Pocket floor is missing, trimmed, split or ambiguous. Reselect a supported upstream face"}
	if q.Origin == nil || q.Normal == nil || len(q.Outer) < 3 || len(q.Outer) > MaxProfileVertices || len(q.Holes) != 0 {
		return errors.New("pocket floor proof requires one bounded straight profile")
	}
	n, err := q.Normal.vec().Unit()
	if err != nil {
		return err
	}
	origin := q.Origin.vec()
	for _, p := range q.Outer {
		if math.Abs(p.vec().Sub(origin).Dot(n)) > 1e-7 {
			return errors.New("pocket floor profile is outside the authored plane")
		}
	}
	want := loop(q.Outer)
	faceCount := 0
	matches := false
	onPlane := func(p, normal geom.Vec3, ring []geom.Vec3) {
		if math.Abs(math.Abs(normal.Dot(n))-1) > 1e-9 || math.Abs(p.Sub(origin).Dot(n)) > 1e-7 {
			return
		}
		faceCount++
		if normal.Dot(n) > 0 && matchingPolygonRing(want, ring) {
			matches = true
		}
	}
	if s.planar() {
		v, e := s.solid.Vertices(), s.solid.Edges()
		for i, f := range s.solid.Faces() {
			p, err := s.solid.FacePlane(i)
			if err != nil {
				return err
			}
			var ring []geom.Vec3
			if len(f.Holes) == 0 {
				for _, use := range f.Boundary {
					edge := e[use.Edge]
					id := edge.Start
					if use.Reversed {
						id = edge.End
					}
					ring = append(ring, v[id])
				}
			}
			onPlane(p.Origin(), p.Normal(), ring)
		}
	} else if native, placement, ok := s.retainedTrimmed(); ok {
		if err := compareTrimmedFloor(native, placement, onPlane); err != nil {
			return err
		}
	} else if s.cylinder != nil {
		return mismatch
	}
	if faceCount > 1 {
		return errors.New("Pocket floor has ambiguous native supports")
	}
	if !matches {
		return mismatch
	}
	return nil
}

func compareTrimmedFloor(s trimmed.Solid, placement geom.Affine, visit func(geom.Vec3, geom.Vec3, []geom.Vec3)) error {
	v, e := s.Vertices(), s.Edges()
	for _, f := range s.Faces() {
		base, m := parametric.SurfaceMapping(f.Surface)
		p, ok := base.(geom.PlaneSurface)
		if !ok {
			continue
		}
		m = placement.Mul(m)
		frame := p.Frame()
		normal, err := m.ApplyVector(frame.XAxis()).Cross(m.ApplyVector(frame.YAxis())).Unit()
		if err != nil {
			return err
		}
		if f.Reversed {
			normal = normal.Scale(-1)
		}
		var ring []geom.Vec3
		if len(f.Holes) == 0 {
			for _, u := range f.Outer {
				if u.Curve != nil {
					ring = nil
					break
				}
				edge := e[u.Edge]
				id := edge.Start
				if u.Reversed {
					id = edge.End
				}
				ring = append(ring, placement.Apply(v[id]))
			}
		}
		visit(m.Apply(frame.Origin()), normal, ring)
	}
	return nil
}

func matchingPolygonRing(a, b []geom.Vec3) bool {
	if len(a) != len(b) || len(a) < 3 {
		return false
	}
	for i, p := range b {
		if !nearPoint(a[0], p) {
			continue
		}
		for _, d := range []int{1, -1} {
			ok := true
			for j, v := range a {
				ok = ok && nearPoint(v, b[(i+len(b)+j*d)%len(b)])
			}
			if ok {
				return true
			}
		}
	}
	return false
}

type measuredRim struct {
	Center vector  `json:"center"`
	Radius float64 `json:"radius"`
}

func (s stored) holeRim(q request) (*measuredRim, error) {
	if q.Center == nil || q.Normal == nil || q.Radius <= 0 {
		return nil, errors.New("hole rim requires center, outward normal and radius")
	}
	n, err := q.Normal.vec().Unit()
	if err != nil {
		return nil, err
	}
	drawing, err := s.drawingGeometry()
	if err != nil {
		return nil, err
	}
	center, radius := q.Center.vec(), float64(q.Radius)
	var found *measuredRim
	for _, bore := range drawing.Bores {
		r := bore.Diameter / 2
		if math.Abs(radius-r) > max(1e-7, r*1e-9) {
			continue
		}
		start, axis := bore.Start.vec(), bore.Direction.vec()
		for i, endpoint := range []geom.Vec3{start, start.Add(axis.Scale(bore.Length))} {
			outward := axis.Scale(float64(2*i - 1))
			if !nearPoint(endpoint, center) || outward.Dot(n) <= 1-1e-8 {
				continue
			}
			if !s.rimOnCap(endpoint, outward, r) {
				continue
			}
			if found != nil {
				return nil, nil
			}
			found = &measuredRim{coordinates(endpoint), r}
		}
	}
	return found, nil
}

func (s stored) rimOnCap(center, outward geom.Vec3, radius float64) bool {
	native, placement, ok := s.retainedTrimmed()
	if !ok {
		return false
	}
	faces := native.Faces()
	for _, f := range faces {
		base, m := parametric.SurfaceMapping(f.Surface)
		p, ok := base.(geom.PlaneSurface)
		if !ok {
			continue
		}
		m = placement.Mul(m)
		frame := p.Frame()
		n, err := m.ApplyVector(frame.XAxis()).Cross(m.ApplyVector(frame.YAxis())).Unit()
		if err != nil {
			continue
		}
		if f.Reversed {
			n = n.Scale(-1)
		}
		if n.Dot(outward) < 1-1e-8 || math.Abs(center.Sub(m.Apply(frame.Origin())).Dot(n)) > 1e-7 {
			continue
		}
		for _, hole := range f.Holes {
			if trimmed.CircularPlanarWire(hole, m.Mul(frameMap(frame)), center, radius, 1e-7) {
				return true
			}
		}
	}
	return false
}

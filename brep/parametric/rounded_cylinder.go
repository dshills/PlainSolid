package parametric

import (
	"math"

	"plainsolid/geom"
)

// RoundedCylinder retains a cylindrical side, planar caps and optional
// quarter-torus rim blends. Lower and upper are independent rolling radii.
// A zero radius leaves that rim sharp.
func RoundedCylinder(frame geom.Frame, radius, height, lower, upper float64) (Solid, error) {
	if !finite(radius) || !finite(height) || !finite(lower) || !finite(upper) ||
		radius <= 0 || height <= 0 || min(lower, upper) < 0 || lower+upper >= height ||
		max(lower, upper) >= radius/2 {
		return Solid{}, geom.ErrDomain
	}
	tol := max(radius, height) * 1e-12
	if height-lower-upper <= 8*tol || lower > 0 && lower <= 8*tol || upper > 0 && upper <= 8*tol {
		return Solid{}, geom.ErrUnrepresentable
	}
	var vertices []geom.Vec3
	var edges []Edge
	var faces []Face
	wall, err := geom.NewCylinderSurface(frame, radius)
	if err != nil {
		return Solid{}, err
	}
	face := Face{Surface: wall, U: circleDomain, V: Interval{lower, height - upper},
		Boundary: [4]Use{{0, false}, {1, false}, {2, true}, {1, true}}}
	vertices = append(vertices, frame.ToWorld(geom.Vec3{X: radius, Z: lower}), frame.ToWorld(geom.Vec3{X: radius, Z: height - upper}))
	edges = append(edges,
		Edge{Start: 0, End: 0, Curve: iso{face, 0}, Range: normalized},
		Edge{Start: 0, End: 1, Curve: iso{face, 1}, Range: normalized},
		Edge{Start: 1, End: 1, Curve: iso{face, 2}, Range: normalized})
	// Side 2 on the patch traverses decreasing angle. Retain the rim curve
	// in increasing angle so all adjacent charts use a common direction.
	edges[2].Curve = iso{Face{Surface: wall, U: circleDomain, V: Interval{height - upper, height - upper + 1}}, 0}
	faces = append(faces, face)
	for end, r := range []float64{lower, upper} {
		z := float64(end) * height
		capRadius := radius - r
		rim := 2 * end
		v := end
		if r > 0 {
			center := lower
			vrange := Interval{-math.Pi / 2, 0}
			if end == 1 {
				center, vrange = height-upper, Interval{0, math.Pi / 2}
			}
			torusFrame, err := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: center}), frame.XAxis(), frame.ZAxis())
			if err != nil {
				return Solid{}, err
			}
			torus, err := geom.NewTorusSurface(torusFrame, capRadius, r)
			if err != nil {
				return Solid{}, err
			}
			face := Face{Surface: torus, U: circleDomain, V: vrange}
			capEdge, seam := len(edges), len(edges)+1
			v = len(vertices)
			vertices = append(vertices, frame.ToWorld(geom.Vec3{X: capRadius, Z: z}))
			start, finish := v, end
			bottom, top := capEdge, rim
			if end == 1 {
				start, finish = end, v
				bottom, top = rim, capEdge
			}
			face.Boundary = [4]Use{{bottom, false}, {seam, false}, {top, true}, {seam, true}}
			u := vrange.Min
			if end == 1 {
				u = vrange.Max
			}
			rimFace := face
			rimFace.V = Interval{u, u + 1}
			edges = append(edges, Edge{Start: v, End: v, Curve: iso{rimFace, 0}, Range: normalized},
				Edge{Start: start, End: finish, Curve: iso{face, 1}, Range: normalized})
			rim = capEdge
			faces = append(faces, face)
		}
		capFrame, err := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: z}), frame.XAxis(), frame.ZAxis())
		if err != nil {
			return Solid{}, err
		}
		pole := len(vertices)
		vertices = append(vertices, capFrame.Origin())
		i := len(edges)
		cap := Face{Surface: disk{capFrame}, U: circleDomain, V: Interval{Max: capRadius}, Reversed: end == 1,
			Boundary: [4]Use{{i, false}, {i + 1, false}, {rim, true}, {i + 1, true}}}
		edges = append(edges, Edge{Start: pole, End: pole}, Edge{Start: pole, End: v, Curve: iso{cap, 1}, Range: normalized})
		faces = append(faces, cap)
	}
	return NewSolid(vertices, edges, faces, tol)
}

package step

import (
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// obliqueBoreEdgeCurve is used only for regenerated circular-bore topology.
// The exact 3D conic is shared by the planar cap and the cylindrical wall.
func obliqueBoreEdgeCurve(a, b geom.Vec3, owners []longEdgeOwner, surfaces []longEdgeSurface) (longEdgeCurve, error) {
	plane, wall := owners[0], owners[1]
	if surfaces[plane.face].kind != "PLANE" {
		plane, wall = wall, plane
	}
	cap, cyl := surfaces[plane.face].frame, surfaces[wall.face]
	arc, valid := wall.use.Curve.(trimmed.EllipseUV)
	if !valid || arc.Linear != (geom.Vec2{X: 1}) || arc.Center.X != 0 ||
		arc.Cos.X != 0 || arc.Sin.X != 0 {
		return longEdgeCurve{}, geom.ErrDomain
	}
	if _, valid := plane.use.Curve.(trimmed.EllipseUV); !valid {
		return longEdgeCurve{}, geom.ErrDomain
	}
	axis, normal := cyl.frame.ZAxis(), cap.ZAxis()
	dot := axis.Dot(normal)
	if dot == 0 || math.Abs(dot) >= 1 {
		return longEdgeCurve{}, geom.ErrDomain
	}
	center := cyl.frame.ToWorld(geom.Vec3{Z: arc.Center.Y})
	// The major direction is the projection of the cylinder axis onto
	// the plane. Orient the ellipse's normal so traversal follows +U.
	x, err := axis.Sub(normal.Scale(dot)).Unit()
	if err != nil {
		return longEdgeCurve{}, err
	}
	if dot < 0 {
		normal = normal.Scale(-1)
	}
	frame, err := geom.NewFrame(center, x, normal)
	if err != nil {
		return longEdgeCurve{}, err
	}
	major := cyl.radius / math.Abs(dot)
	if !finiteOblique(major) || major <= cyl.radius {
		return longEdgeCurve{}, geom.ErrDomain
	}
	tol := 1e-9 + 128*0x1p-52*max(a.Length(), b.Length(), center.Length(), major)
	if !finiteOblique(tol) || tol >= cyl.radius/16 {
		return longEdgeCurve{}, geom.ErrUnrepresentable
	}
	for _, vertex := range []geom.Vec3{a, b} {
		p := frame.ToLocal(vertex)
		if math.Abs(p.Z) > tol || math.Abs(math.Hypot(p.X/major, p.Y/cyl.radius)-1)*cyl.radius > tol {
			return longEdgeCurve{}, geom.ErrDomain
		}
	}
	same := (arc.To > arc.From) != wall.use.Reversed
	return longEdgeCurve{ellipse: true, frame: frame, radius: major, minorRadius: cyl.radius, same: same}, nil
}

package plaincadbridge

import (
	"math"
	"math/big"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func exactDotDifference(a, b, n geom.Vec3) int {
	x, y, z := []float64{a.X, a.Y, a.Z}, []float64{b.X, b.Y, b.Z}, []float64{n.X, n.Y, n.Z}
	sum := new(big.Rat)
	for i := 0; i < 3; i++ {
		d := new(big.Rat).Sub(new(big.Rat).SetFloat64(x[i]), new(big.Rat).SetFloat64(y[i]))
		sum.Add(sum, new(big.Rat).Mul(d, new(big.Rat).SetFloat64(z[i])))
	}
	return sum.Sign()
}

// boreSpan requires a face's exact represented tangent vectors to be orthogonal
// to the cylinder direction. Consequently every point of each circular tool cap
// has the same axial support-plane side, not just the central ray.
func boreSpan(base brep.Solid, c *cylinderBody, through bool) (trimmed.CylinderCut, float64, float64, error) {
	unsupported := func(message string) (trimmed.CylinderCut, float64, float64, error) {
		return trimmed.CylinderCut{}, 0, 0, &failureError{"UNSUPPORTED", message}
	}
	m, owners := base.Tessellate(), base.TriangleFaces()
	faces := base.Faces()
	first := make([][3]int, len(faces))
	seen := make([]bool, len(faces))
	for i, f := range owners {
		if !seen[f] {
			first[f] = m.Triangles[i]
			seen[f] = true
		}
	}
	entry, exit := -1, -1
	start, end := 0., 0.
	axis := c.frame.ZAxis()
	origin := c.frame.Origin()
	if exactDotDifference(c.frame.XAxis(), geom.Vec3{}, axis) != 0 || exactDotDifference(c.frame.YAxis(), geom.Vec3{}, axis) != 0 {
		return unsupported("represented cylinder radial axes do not prove exact cap parallelism")
	}
	for i, t := range first {
		if !seen[i] {
			return unsupported("cylinder cut requires finite planar faces")
		}
		a, b, d := m.Vertices[t[0]], m.Vertices[t[1]], m.Vertices[t[2]]
		if exactDotDifference(b, a, axis) != 0 || exactDotDifference(d, a, axis) != 0 {
			continue
		}
		p, err := base.FacePlane(i)
		if err != nil {
			return trimmed.CylinderCut{}, 0, 0, err
		}
		dot := p.Normal().Dot(axis)
		parameter := p.Origin().Sub(origin).Dot(p.Normal()) / dot
		if !finite(parameter) {
			return unsupported("cylinder support intersection exceeds numeric range")
		}
		if dot < 0 {
			if entry >= 0 {
				return unsupported("multiple possible cylindrical entry planes")
			}
			entry, start = i, parameter
		} else {
			if exit >= 0 {
				return unsupported("multiple possible cylindrical exit planes")
			}
			exit, end = i, parameter
		}
	}
	if entry < 0 || exit < 0 || end <= start {
		return trimmed.CylinderCut{}, 0, 0, unclaimed("cylinder cut requires two parallel perpendicular opening planes")
	}
	if through {
		finish := origin.Add(axis.Scale(c.height))
		if !finish.IsFinite() {
			return unsupported("cylinder endpoint exceeds numeric range")
		}
		for _, v := range []struct {
			face  int
			point geom.Vec3
		}{{entry, origin}, {exit, finish}} {
			t := first[v.face]
			side, err := geom.Orient3D(m.Vertices[t[0]], m.Vertices[t[1]], m.Vertices[t[2]], v.point)
			if err != nil {
				return trimmed.CylinderCut{}, 0, 0, err
			}
			if side > 0 {
				return unsupported("finite cylinder does not cover both opening planes; blind or partial cuts are unsupported")
			}
		}
	}
	return trimmed.CylinderCut{Entry: entry, Exit: exit, AxisPoint: origin, Direction: axis, Radius: c.radius}, start, end, nil
}

func (b *Bridge) curvedBoolean(a, c stored, op string) (any, error) {
	if op == "cut" && boundsSeparated(a.result.Bounds, c.result.Bounds) {
		return nil, &failureError{"UNSUPPORTED", "Boolean cut removed no volume. Move the tool into the target body."}
	}
	if op == "common" && a.cylinder != nil && c.cylinder == nil {
		a, c = c, a
	}
	if c.cylinder == nil || a.cylinder != nil {
		return nil, &failureError{"UNSUPPORTED", "curved Boolean combination is outside the finite entry-open cylinder-cut domain"}
	}
	if op == "cut" {
		// The general Boolean is asked first to cut a cylinder from
		// planar stock, and no legacy path cuts one.
		return nil, unclaimed("a cylinder is cut from planar stock by the general Boolean alone")
	}
	if op == "common" && boundsSeparated(a.result.Bounds, c.result.Bounds) {
		return b.put(brep.Solid{})
	}
	if op == "common" {
		base := a.solid
		cut, start, end, err := boreSpan(base, c.cylinder, false)
		if err != nil {
			return nil, err
		}
		// This construction validates the full radial footprint independently from
		// finite axial clipping, so an intersecting side wall cannot be ignored.
		if _, _, err := trimmed.BoreCylinderAlong(base, cut.Entry, cut.Exit, cut.AxisPoint, cut.Direction, cut.Radius, 1e-9); err != nil {
			return nil, &failureError{"UNSUPPORTED", "cylinder overlap requires a strictly contained radial footprint: " + err.Error()}
		}
		lo, hi := math.Max(0, start), math.Min(c.cylinder.height, end)
		if hi <= lo {
			return b.put(brep.Solid{})
		}
		return b.circle(c.cylinder.frame.Origin().Add(c.cylinder.frame.ZAxis().Scale(lo)), c.cylinder.frame.ZAxis(), c.cylinder.radius, hi-lo)
	}
	return nil, &failureError{"UNSUPPORTED", "curved Boolean combination is outside the finite entry-open cylinder-cut domain"}
}

// Existing shape bounds enclose the original analytic geometry. Strictly
// separated intervals prove a miss without assuming unsupported intersections.
func boundsSeparated(a, b Bounds) bool {
	for i := 0; i < 3; i++ {
		if a.Max[i] < b.Min[i] || b.Max[i] < a.Min[i] {
			return true
		}
	}
	return false
}

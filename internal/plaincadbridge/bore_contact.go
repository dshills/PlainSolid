package plaincadbridge

import (
	"errors"
	"plainsolid/brep"
	"plainsolid/geom"
)

// ErrNonManifoldBoreContact identifies a proved tangent blind-bore boundary,
// not an accepted singular solid or a numerical proximity diagnosis.
var ErrNonManifoldBoreContact = errors.New("blind bore has non-manifold side contact")

// NonManifoldBoreContactError retains original stock face ancestry. SideFaces
// corresponds to Contact.Generators. Rational coordinates remain in world units.
// The bridge deliberately serializes only its UNSUPPORTED diagnostic, not proof
// data or a shape handle. Go callers may use errors.As to inspect this evidence.
type NonManifoldBoreContactError struct {
	ToolIndex, EntryFace int
	SideFaces            []int
	Contact              geom.AxialBlindBoreContact
}

func (e *NonManifoldBoreContactError) Error() string {
	return "blind cylindrical cut has exact tangent side contact; non-manifold boundary topology is unsupported"
}
func (e *NonManifoldBoreContactError) Unwrap() error { return &failureError{"UNSUPPORTED", e.Error()} }
func (e *NonManifoldBoreContactError) Is(target error) bool {
	return target == ErrNonManifoldBoreContact
}

// Only the identity world chart is used: no rounded local transformation may
// turn a near contact into a certified equality.
func exactBlindBoreContact(base brep.Solid, c *cylinderBody, entry, tool int) error {
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	bounds, ok := orthogonalPrism(base, f)
	if !ok {
		return nil
	}
	axis, sign := -1, 0
	for i, n := range []geom.Vec3{{X: 1}, {Y: 1}, {Z: 1}} {
		if c.frame.ZAxis() == n {
			axis, sign = i, 1
		}
		if c.frame.ZAxis() == n.Scale(-1) {
			axis, sign = i, -1
		}
	}
	if axis < 0 {
		return nil
	}
	// Require exact Euclidean radial frame, including unit norms. A general
	// approximately orthonormal represented frame does not prove a true circle.
	x, y, z := c.frame.XAxis(), c.frame.YAxis(), c.frame.ZAxis()
	cardinal := func(v geom.Vec3) bool {
		return v == (geom.Vec3{X: 1}) || v == (geom.Vec3{X: -1}) || v == (geom.Vec3{Y: 1}) || v == (geom.Vec3{Y: -1}) || v == (geom.Vec3{Z: 1}) || v == (geom.Vec3{Z: -1})
	}
	if !cardinal(x) || !cardinal(y) || x.Dot(y) != 0 || x.Dot(z) != 0 || y.Dot(z) != 0 {
		return nil
	}
	contact, e := geom.ClassifyAxialBlindBoreContact(bounds, c.frame.Origin(), c.radius, c.height, axis, sign)
	if e != nil || !contact.ManifoldExcluded {
		return nil
	}
	ids := [6]int{-1, -1, -1, -1, -1, -1}
	vs, es := base.Vertices(), base.Edges()
	lo := [3]float64{bounds.Min.X, bounds.Min.Y, bounds.Min.Z}
	hi := [3]float64{bounds.Max.X, bounds.Max.Y, bounds.Max.Z}
	for id, face := range base.Faces() {
		for a := 0; a < 3; a++ {
			for s, value := range []float64{lo[a], hi[a]} {
				same := true
				for _, u := range face.Boundary {
					v := vs[es[u.Edge].Start]
					q := [3]float64{v.X, v.Y, v.Z}
					same = same && q[a] == value
				}
				if same {
					ids[2*a+s] = id
				}
			}
		}
	}
	side := 0
	if sign < 0 {
		side = 1
	}
	if ids[2*axis+side] != entry {
		return nil
	}
	out := &NonManifoldBoreContactError{ToolIndex: tool, EntryFace: entry, Contact: contact}
	for _, g := range contact.Generators {
		if ids[g.Side] < 0 {
			return nil
		}
		out.SideFaces = append(out.SideFaces, ids[g.Side])
	}
	return out
}

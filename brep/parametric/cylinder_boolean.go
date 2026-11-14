package parametric

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

// DiskProfile is a circle in the XY plane of the supplied cylinder frame.
type DiskProfile struct {
	Center geom.Vec2
	Radius float64
}

// CombineParallelCylinders combines two cylinders with the same
// positive height and cap planes. Their centers are XY coordinates in frame.
// Transverse overlap is represented by exact cylindrical arcs and ruled planar
// cap patches. Tangent union and unresolved near tangency return errors.
// Difference supports disjoint, contained and transversely overlapping disks.
func CombineParallelCylinders(frame geom.Frame, a, b DiskProfile, height float64, op BooleanOp, tolerance float64) (Solid, error) {
	valid := func(d DiskProfile) bool {
		return finite(d.Center.X) && finite(d.Center.Y) && finite(d.Radius) && d.Radius > 0
	}
	if !valid(a) || !valid(b) || !finite(height) || height <= 0 || !finite(tolerance) || tolerance <= 0 ||
		frame.ZAxis().Length() == 0 || op > Difference {
		return Solid{}, geom.ErrInvalid
	}
	world := func(p geom.Vec2) geom.Vec3 { return frame.ToWorld(geom.Vec3{X: p.X, Y: p.Y}) }
	newFrame := func(p geom.Vec2) (geom.Frame, error) { return geom.NewFrame(world(p), frame.XAxis(), frame.ZAxis()) }
	cylinder := func(d DiskProfile) (Solid, error) {
		f, e := newFrame(d.Center)
		if e != nil {
			return Solid{}, e
		}
		return Cylinder(f, d.Radius, height)
	}
	dx, dy := b.Center.X-a.Center.X, b.Center.Y-a.Center.Y
	d := math.Hypot(dx, dy)
	if !finite(d) {
		return Solid{}, geom.ErrInvalid
	}
	if d == 0 {
		if op == Difference {
			if b.Radius >= a.Radius {
				return Solid{}, nil
			}
			return cylinderAnnulus(frame, a, b, height, tolerance)
		}
		if op == Union && b.Radius > a.Radius || op == Intersection && b.Radius < a.Radius {
			return cylinder(b)
		}
		return cylinder(a)
	}
	scale := math.Max(d, math.Max(a.Radius, b.Radius))
	x, y, z := a.Radius/scale, b.Radius/scale, d/scale
	eps := tolerance / scale
	if !finite(eps) || x == 0 || y == 0 || z == 0 {
		return Solid{}, fmt.Errorf("parametric: cylinder size ratio outside range")
	}
	sum, diff := x+y, math.Abs(x-y)
	c0xy, c1xy := geom.Vec3{X: a.Center.X, Y: a.Center.Y}, geom.Vec3{X: b.Center.X, Y: b.Center.Y}
	if z > sum+eps {
		if op == Difference {
			return cylinder(a)
		}
		if op == Intersection {
			return Solid{}, nil
		}
		p, e := cylinder(a)
		if e != nil {
			return Solid{}, e
		}
		q, e := cylinder(b)
		if e != nil {
			return Solid{}, e
		}
		return joinIndependent(p, q)
	}
	if math.Abs(z-sum) <= eps && tangent(c0xy, c1xy, a.Radius, b.Radius, false) {
		if op == Difference {
			return cylinder(a)
		}
		if op == Intersection {
			return Solid{}, nil
		}
		return Solid{}, fmt.Errorf("parametric: tangent cylinder union is nonmanifold")
	}
	if z < diff-eps || math.Abs(z-diff) <= eps && tangent(c0xy, c1xy, a.Radius, b.Radius, true) {
		if op == Difference {
			if b.Radius >= a.Radius {
				return Solid{}, nil
			}
			if z >= diff-eps {
				return Solid{}, fmt.Errorf("parametric: internally tangent cylindrical hole is nonmanifold")
			}
			return cylinderAnnulus(frame, a, b, height, tolerance)
		}
		if op == Union && b.Radius > a.Radius || op == Intersection && b.Radius < a.Radius {
			return cylinder(b)
		}
		return cylinder(a)
	}
	if z >= sum || z <= diff || math.Min(sum-z, z-diff) <= eps {
		return Solid{}, fmt.Errorf("parametric: unresolved near-tangent cylinder Boolean")
	}
	mid := (x - y) * (x + y) / z
	ca, cb := (z+mid)/(2*x), (z-mid)/(2*y)
	if math.Abs(ca) >= 1 || math.Abs(cb) >= 1 {
		return Solid{}, fmt.Errorf("parametric: unresolved cylinder intersection angles")
	}
	aa, ab := math.Acos(ca), math.Acos(cb)
	theta := math.Atan2(dy, dx)
	fa, err := newFrame(a.Center)
	if err != nil {
		return Solid{}, err
	}
	fb, err := newFrame(b.Center)
	if err != nil {
		return Solid{}, err
	}
	center := geom.Vec2{X: a.Center.X + ((z+mid)/2*scale)*(dx/d), Y: a.Center.Y + ((z+mid)/2*scale)*(dy/d)}
	// The chord midpoint is strictly inside both circles for a proper
	// overlap. The planar cap is a fan from this common point.
	sides := [2]struct {
		frame  geom.Frame
		radius float64
		u      Interval
	}{
		{fa, a.Radius, Interval{theta - aa, theta + aa}},
		{fb, b.Radius, Interval{theta + math.Pi - ab, theta + math.Pi + ab}},
	}
	if op == Union {
		sides[0].u = Interval{theta + aa, theta + 2*math.Pi - aa}
		sides[1].u = Interval{theta + math.Pi + ab, theta + 3*math.Pi - ab}
	}
	if op == Difference {
		u := Interval{theta + aa, theta + 2*math.Pi - aa}
		v := Interval{theta + math.Pi - ab, theta + math.Pi + ab}
		return cylinderDifference(fa, fb, a.Radius, b.Radius, height, u, v, tolerance)
	}

	var faces []Face
	c0 := world(center)
	for _, side := range sides {
		s, err := geom.NewCylinderSurface(side.frame, side.radius)
		if err != nil {
			return Solid{}, err
		}
		faces = append(faces, Face{Surface: s, U: side.u, V: Interval{0, height}})
		for i := range 2 {
			off := float64(i) * height
			c := c0.Add(frame.ZAxis().Scale(off))
			fan := arcFan{surface: s, center: c, height: off}
			faces = append(faces, Face{Surface: fan, U: side.u, V: normalized, Reversed: i == 1})
		}
	}
	return joinPrimitivePatches(faces, tolerance)
}

type arcFan struct {
	surface geom.CylinderSurface
	center  geom.Vec3
	height  float64
}

func (f arcFan) Eval(u, v float64) (geom.Vec3, error) {
	if !finite(v) || v < 0 || v > 1 {
		return geom.Vec3{}, geom.ErrDomain
	}
	p, e := f.surface.Eval(u, f.height)
	if e != nil {
		return p, e
	}
	return checked(f.center.Scale(1 - v).Add(p.Scale(v)))
}
func (f arcFan) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	if !finite(v) || v < 0 || v > 1 {
		return geom.Vec3{}, geom.Vec3{}, geom.ErrDomain
	}
	p, e := f.surface.Eval(u, f.height)
	if e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	du, _, e := f.surface.Partials(u, f.height)
	if e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	return du.Scale(v), p.Sub(f.center), nil
}

func joinIndependent(a, b Solid) (Solid, error) {
	v, e, f := a.Vertices(), a.Edges(), a.Faces()
	es, fs := b.Edges(), b.Faces()
	for i := range es {
		es[i].Start += len(v)
		es[i].End += len(v)
	}
	for i := range fs {
		for j := range fs[i].Boundary {
			fs[i].Boundary[j].Edge += len(e)
		}
	}
	return NewSolid(append(v, b.vertices...), append(e, es...), append(f, fs...), math.Max(a.tolerance, b.tolerance))
}

type ruledArcs struct {
	a, b   geom.CylinderSurface
	u, v   Interval
	height float64
}

func (s ruledArcs) point(t, w float64) (p, q geom.Vec3, err error) {
	if !finite(t) || !finite(w) || t < 0 || t > 1 || w < 0 || w > 1 {
		return p, q, geom.ErrDomain
	}
	p, err = s.a.Eval(s.u.at(t), s.height)
	if err != nil {
		return p, q, err
	}
	q, err = s.b.Eval(s.v.at(t), s.height)
	return p, q, err
}
func (s ruledArcs) Eval(t, w float64) (geom.Vec3, error) {
	a, b, e := s.point(t, w)
	if e != nil {
		return geom.Vec3{}, e
	}
	return checked(a.Scale(1 - w).Add(b.Scale(w)))
}
func (s ruledArcs) Partials(t, w float64) (geom.Vec3, geom.Vec3, error) {
	a, b, e := s.point(t, w)
	if e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	da, _, e := s.a.Partials(s.u.at(t), s.height)
	if e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	db, _, e := s.b.Partials(s.v.at(t), s.height)
	if e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	return da.Scale((1 - w) * (s.u.Max - s.u.Min)).Add(db.Scale(w * (s.v.Max - s.v.Min))), b.Sub(a), nil
}

type reparameterizedCylinder struct {
	s geom.CylinderSurface
	u Interval
}

func (s reparameterizedCylinder) Eval(u, v float64) (geom.Vec3, error) { return s.s.Eval(s.u.at(u), v) }
func (s reparameterizedCylinder) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	a, b, e := s.s.Partials(s.u.at(u), v)
	return a.Scale(s.u.Max - s.u.Min), b, e
}
func cylinderDifference(fa, fb geom.Frame, ra, rb, h float64, u, v Interval, tol float64) (Solid, error) {
	a, e := geom.NewCylinderSurface(fa, ra)
	if e != nil {
		return Solid{}, e
	}
	b, e := geom.NewCylinderSurface(fb, rb)
	if e != nil {
		return Solid{}, e
	}
	faces := []Face{
		{Surface: reparameterizedCylinder{a, u}, U: normalized, V: Interval{0, h}},
		{Surface: reparameterizedCylinder{b, v}, U: normalized, V: Interval{0, h}, Reversed: true},
		{Surface: ruledArcs{a, b, u, v, 0}, U: normalized, V: normalized, Reversed: true},
		{Surface: ruledArcs{a, b, u, v, h}, U: normalized, V: normalized},
	}
	return joinPrimitivePatches(faces, tol)
}
func cylinderAnnulus(frame geom.Frame, a, b DiskProfile, h, tol float64) (Solid, error) {
	world := func(p geom.Vec2) geom.Vec3 { return frame.ToWorld(geom.Vec3{X: p.X, Y: p.Y}) }
	fa, e := geom.NewFrame(world(a.Center), frame.XAxis(), frame.ZAxis())
	if e != nil {
		return Solid{}, e
	}
	fb, e := geom.NewFrame(world(b.Center), frame.XAxis(), frame.ZAxis())
	if e != nil {
		return Solid{}, e
	}
	sa, e := geom.NewCylinderSurface(fa, a.Radius)
	if e != nil {
		return Solid{}, e
	}
	sb, e := geom.NewCylinderSurface(fb, b.Radius)
	if e != nil {
		return Solid{}, e
	}
	var faces []Face
	for i := range 4 {
		u := Interval{float64(i) * math.Pi / 2, float64(i+1) * math.Pi / 2}
		faces = append(faces,
			Face{Surface: reparameterizedCylinder{sa, u}, U: normalized, V: Interval{0, h}},
			Face{Surface: reparameterizedCylinder{sb, u}, U: normalized, V: Interval{0, h}, Reversed: true},
			Face{Surface: ruledArcs{sa, sb, u, u, 0}, U: normalized, V: normalized, Reversed: true},
			Face{Surface: ruledArcs{sa, sb, u, u, h}, U: normalized, V: normalized})
	}
	return joinPrimitivePatches(faces, tol)
}

// ConcentricTubeParameters recovers a full tube's native construction recipe.
// The four angular sectors each retain two walls and two ruled planar caps.
func (s Solid) ConcentricTubeParameters() (frame geom.Frame, outer, inner, height float64, ok bool) {
	if len(s.faces) != 16 {
		return
	}
	outerWall, ok := s.faces[0].Surface.(reparameterizedCylinder)
	if !ok {
		return geom.Frame{}, 0, 0, 0, false
	}
	innerWall, ok := s.faces[1].Surface.(reparameterizedCylinder)
	if !ok {
		return geom.Frame{}, 0, 0, 0, false
	}
	frame, outer = outerWall.s.Parameters()
	innerFrame, inner := innerWall.s.Parameters()
	height = s.faces[0].V.Max
	if frame != innerFrame || inner >= outer || inner <= 0 || height <= 0 {
		return geom.Frame{}, 0, 0, 0, false
	}
	for i := range 4 {
		u := Interval{float64(i) * math.Pi / 2, float64(i+1) * math.Pi / 2}
		wall := Interval{Max: height}
		want := [4]Face{
			{Surface: reparameterizedCylinder{outerWall.s, u}, U: normalized, V: wall},
			{Surface: reparameterizedCylinder{innerWall.s, u}, U: normalized, V: wall, Reversed: true},
			{Surface: ruledArcs{outerWall.s, innerWall.s, u, u, 0}, U: normalized, V: normalized, Reversed: true},
			{Surface: ruledArcs{outerWall.s, innerWall.s, u, u, height}, U: normalized, V: normalized},
		}
		for j, expected := range want {
			f := s.faces[4*i+j]
			if f.Surface != expected.Surface || f.U != expected.U || f.V != expected.V || f.Reversed != expected.Reversed {
				return geom.Frame{}, 0, 0, 0, false
			}
		}
	}
	return frame, outer, inner, height, true
}

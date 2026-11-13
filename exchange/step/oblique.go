package step

import (
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// The complete conic loci, seam incidence and cap orientation are checked
// before reconstruction. No sampled point cloud determines the imported shape.
func obliqueCylinderBody(f *File, id int64) (TrimmedBody, error) {
	fail := func() (TrimmedBody, error) {
		return TrimmedBody{}, errors.New("step: unsupported oblique cylinder topology or geometry")
	}
	body, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return fail()
	}
	name, e := textValue(body.Args[0])
	if e != nil {
		return fail()
	}
	sid, e := ref(body.Args[1])
	if e != nil {
		return fail()
	}
	shell, e := simple(f, sid, "CLOSED_SHELL", 2)
	if e != nil {
		return fail()
	}
	if _, e = textValue(shell.Args[0]); e != nil {
		return fail()
	}
	ids, e := listRefs(shell.Args[1], 3)
	if e != nil || len(ids) != 3 {
		return fail()
	}
	var side Record
	var uses []analyticUse
	var caps []int64
	for _, fid := range ids {
		face, e := simple(f, fid, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		sid, e := ref(face.Args[2])
		if e != nil {
			return fail()
		}
		surface := f.Entities[sid]
		if len(surface.Records) != 1 {
			return fail()
		}
		switch surface.Records[0].Type {
		case "CYLINDRICAL_SURFACE":
			if uses != nil {
				return fail()
			}
			side, uses, e = analyticFaceLoop(f, fid, 4)
			if e != nil {
				return fail()
			}
		case "PLANE":
			caps = append(caps, fid)
		default:
			return fail()
		}
	}
	if len(caps) != 2 || len(uses) != 4 {
		return fail()
	}
	sense, e := boolean(side.Args[3])
	if e != nil || !sense {
		return fail()
	}
	sid, e = ref(side.Args[2])
	if e != nil {
		return fail()
	}
	cyl, e := simple(f, sid, "CYLINDRICAL_SURFACE", 3)
	if e != nil {
		return fail()
	}
	pid, e := ref(cyl.Args[1])
	if e != nil {
		return fail()
	}
	frame, e := placement(f, pid)
	if e != nil {
		return fail()
	}
	radius, e := numeric(cyl.Args[2])
	if e != nil || radius <= 0 {
		return fail()
	}
	// Rotate the side cycle to the ellipse's reverse use.
	found := -1
	for i, u := range uses {
		_, _, g, e := analyticEdge(f, u.id)
		if e != nil {
			return fail()
		}
		x := f.Entities[g]
		if len(x.Records) == 1 && x.Records[0].Type == "ELLIPSE" {
			if found >= 0 {
				return fail()
			}
			found = i
		}
	}
	if found < 0 {
		return fail()
	}
	uses = append(append([]analyticUse{}, uses[found:]...), uses[:found]...)
	if uses[0].sense || uses[1].sense || !uses[2].sense || !uses[3].sense || uses[1].id != uses[3].id || uses[0].id == uses[2].id {
		return fail()
	}
	top, top2, eg, e := analyticEdge(f, uses[0].id)
	if e != nil || top != top2 {
		return fail()
	}
	bottom, bottom2, cg, e := analyticEdge(f, uses[2].id)
	if e != nil || bottom != bottom2 || top == bottom {
		return fail()
	}
	a, b, lg, e := analyticEdge(f, uses[1].id)
	if e != nil || a != bottom || b != top {
		return fail()
	}
	ellipse, e := simple(f, eg, "ELLIPSE", 4)
	if e != nil {
		return fail()
	}
	circle, e := simple(f, cg, "CIRCLE", 3)
	if e != nil {
		return fail()
	}
	ep, e := ref(ellipse.Args[1])
	if e != nil {
		return fail()
	}
	ef, e := placement(f, ep)
	if e != nil {
		return fail()
	}
	cp, e := ref(circle.Args[1])
	if e != nil {
		return fail()
	}
	cf, e := placement(f, cp)
	if e != nil {
		return fail()
	}
	major, e := numeric(ellipse.Args[2])
	if e != nil || major <= 0 {
		return fail()
	}
	minor, e := numeric(ellipse.Args[3])
	if e != nil || minor <= 0 {
		return fail()
	}
	cr, e := numeric(circle.Args[2])
	if e != nil {
		return fail()
	}
	// Work in the cylinder frame. Complete projected ellipse must be exactly
	// the circular section; its normal then determines the axial height law.
	center := frame.ToLocal(ef.Origin())
	base := frame.ToLocal(cf.Origin())
	scale := math.Max(radius, math.Max(math.Abs(center.Z), math.Abs(base.Z)))
	tol := 1e-9 * scale
	if !finiteOblique(tol) || tol <= 0 {
		return fail()
	}
	near := func(a, b float64) bool { return math.Abs(a-b) <= tol }
	dir := func(a, b geom.Vec3) bool { return a.Distance(b) <= 1e-9 }
	if !near(center.X, 0) || !near(center.Y, 0) || !near(base.X, 0) || !near(base.Y, 0) || !near(cr, radius) || !dir(cf.ZAxis(), frame.ZAxis()) {
		return fail()
	}
	ex, ey := ef.XAxis().Scale(major), ef.YAxis().Scale(minor)
	ax, ay := ex.Dot(frame.XAxis()), ex.Dot(frame.YAxis())
	bx, by := ey.Dot(frame.XAxis()), ey.Dot(frame.YAxis())
	if math.Abs(math.Hypot(ax, ay)-radius) > tol || math.Abs(math.Hypot(bx, by)-radius) > tol || math.Abs((ax/radius)*(bx/radius)+(ay/radius)*(by/radius)) > 1e-9 || ax*by-ay*bx <= 0 {
		return fail()
	}
	normal := ef.ZAxis()
	nz := normal.Dot(frame.ZAxis())
	if nz <= 1e-9 {
		return fail()
	}
	pv, e := analyticVertex(f, top)
	if e != nil {
		return fail()
	}
	bv, e := analyticVertex(f, bottom)
	if e != nil {
		return fail()
	}
	if pv.Distance(ef.Origin().Add(ex)) > tol || bv.Distance(cf.Origin().Add(cf.XAxis().Scale(radius))) > tol {
		return fail()
	}
	diff := pv.Sub(bv)
	if diff.Dot(frame.ZAxis()) <= tol || diff.Sub(frame.ZAxis().Scale(diff.Dot(frame.ZAxis()))).Length() > tol {
		return fail()
	}
	origin, direction, e := lineGeometry(f, lg, 3)
	if e != nil || !dir(direction, frame.ZAxis()) || bv.Sub(origin).Cross(direction).Length() > tol {
		return fail()
	}
	for _, fid := range caps {
		face, loop, e := analyticFaceLoop(f, fid, 1)
		if e != nil {
			return fail()
		}
		sense, e := boolean(face.Args[3])
		if e != nil {
			return fail()
		}
		pid, e := ref(face.Args[2])
		if e != nil {
			return fail()
		}
		pf, e := planeFrame(f, pid)
		if e != nil {
			return fail()
		}
		n := pf.ZAxis()
		if !sense {
			n = n.Scale(-1)
		}
		switch loop[0].id {
		case uses[0].id:
			if !loop[0].sense || !dir(n, normal) || math.Abs(pf.Origin().Sub(ef.Origin()).Dot(normal)) > tol {
				return fail()
			}
		case uses[2].id:
			if loop[0].sense || !dir(n, frame.ZAxis().Scale(-1)) || math.Abs(pf.Origin().Sub(cf.Origin()).Dot(frame.ZAxis())) > tol {
				return fail()
			}
		default:
			return fail()
		}
	}
	// Both rims must have exactly one cap, even when planes happen to coincide.
	_, l0, _ := analyticFaceLoop(f, caps[0], 1)
	_, l1, _ := analyticFaceLoop(f, caps[1], 1)
	if l0[0].id == l1[0].id {
		return fail()
	}
	newFrame, e := geom.NewFrame(cf.Origin(), frame.XAxis(), frame.ZAxis())
	if e != nil {
		return fail()
	}
	plane, e := geom.NewPlane(ef.Origin(), normal)
	if e != nil {
		return fail()
	}
	spread := radius * math.Hypot(normal.Dot(frame.XAxis()), normal.Dot(frame.YAxis())) / nz
	height := center.Z - base.Z + spread + math.Max(radius, center.Z-base.Z)
	if !finiteOblique(height) || center.Z-base.Z-spread <= tol {
		return fail()
	}
	p, e := parametric.ClipCylinder(newFrame, radius, height, plane, tol)
	if e != nil {
		return TrimmedBody{}, e
	}
	s, e := trimmed.FromParametric(p)
	return TrimmedBody{Name: name, Solid: s}, e
}
func finiteOblique(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

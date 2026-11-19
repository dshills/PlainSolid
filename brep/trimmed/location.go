package trimmed

import (
	"fmt"
	"math"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// RayHit is an isolated contact with an analytic trim face. Faces includes
// all incident faces found at a shared trim. Distance is in model units.
type RayHit struct {
	Point           geom.Vec3
	Distance        float64
	Faces           []int
	Tangent, OnTrim bool
}

// Ray intersects trimmed plane, polar plane, cylinder, cone, sphere and torus
// faces, including affine placements, without tessellation. Supported
// p-curves are straight, anchored circular arcs, or angular EllipseUV arcs
// whose vertical component has no linear term, with or without pinned ends. An overlapping face or
// cylinder generator returns an error because it is not an isolated hit.
// tol is a positive world-space residual and contact-merging tolerance.
func (s Solid) Ray(origin, direction geom.Vec3, maxDistance, tol float64) ([]RayHit, error) {
	if math.IsNaN(maxDistance) || maxDistance <= 0 || !finite(tol) || tol <= 0 {
		return nil, geom.ErrInvalid
	}
	line, e := geom.NewLine(origin, direction)
	if e != nil {
		return nil, e
	}
	contacts, e := s.rayContacts(line, maxDistance, tol)
	if e != nil {
		return nil, e
	}
	var out []RayHit
	for _, c := range contacts {
		if c.distance < 0 || c.distance > maxDistance {
			continue
		}
		if k := len(out) - 1; k >= 0 && c.point.Distance(out[k].Point) <= tol && c.distance-out[k].Distance <= tol &&
			(c.onTrim || out[k].OnTrim) {
			h := &out[k]
			if h.Faces[len(h.Faces)-1] != c.face {
				h.Faces = append(h.Faces, c.face)
			}
			h.Tangent = h.Tangent && c.tangent
			h.OnTrim = true
			continue
		}
		out = append(out, RayHit{c.point, c.distance, []int{c.face}, c.tangent, c.onTrim})
	}
	return out, nil
}

// Classify determines material location by oriented ray crossings on the
// same analytic subset as Ray. The input must be an embedded outward
// boundary. A nearby normal projection inside a trim, or a crossing within
// tolerance, reports Boundary. Degenerate rays are retried in fixed directions.
func (s Solid) Classify(point geom.Vec3, tol float64) (parametric.Location, error) {
	if !point.IsFinite() || tol <= 0 || !finite(tol) {
		return parametric.Outside, geom.ErrInvalid
	}
	for i, f := range s.faces {
		base, m := parametric.SurfaceMapping(f.Surface)
		inverse, e := m.Inverse()
		if e != nil {
			return parametric.Outside, e
		}
		local := inverse.Apply(point)
		p, near := normalProjection(base, local, scaledTolerance(tol, inverse))
		if near {
			world, e := f.Surface.Eval(p.uv.X, p.uv.Y)
			if e != nil {
				return parametric.Outside, e
			}
			if world.Distance(point) > tol {
				continue
			}
			h, e := faceContains(f, p, scaledTolerance(tol, inverse))
			if e != nil {
				return parametric.Outside, fmt.Errorf("trimmed: face %d: %w", i, e)
			}
			if h != parametric.Outside {
				return parametric.Boundary, nil
			}
		}
	}
	directions := [...]geom.Vec3{
		{X: 1, Y: 3, Z: 7}, {X: 2, Y: -5, Z: 1}, {X: -3, Y: 1, Z: 2},
		{X: 4, Y: 1, Z: -9}, {X: -2, Y: 7, Z: 3}, {X: 5, Y: -11, Z: -3},
	}
	for _, dir := range directions {
		line, _ := geom.NewLine(point, dir)
		hits, e := s.rayContacts(line, math.Inf(1), tol)
		if e != nil {
			if e == errRayCoincident {
				continue
			}
			return parametric.Outside, e
		}
		winding := 0
		usable := true
		for _, h := range hits {
			if math.Abs(h.distance) <= tol {
				return parametric.Boundary, nil
			}
			if h.onTrim || h.higherCrossing {
				usable = false
				break
			}
			if h.tangent {
				continue
			}
			if math.Abs(h.dot) < 64*0x1p-52 {
				usable = false
				break
			}
			if h.dot > 0 {
				winding++
			} else {
				winding--
			}
		}
		if usable {
			switch winding {
			case 0:
				return parametric.Outside, nil
			case 1:
				return parametric.Inside, nil
			default:
				return parametric.Outside, fmt.Errorf("trimmed: inconsistent boundary winding %d", winding)
			}
		}
	}
	return parametric.Outside, fmt.Errorf("trimmed: no transverse classification ray")
}

var errRayCoincident = fmt.Errorf("trimmed: ray coincides with a supporting plane or generator")

type rayContact struct {
	point                           geom.Vec3
	distance, dot                   float64
	face                            int
	tangent, onTrim, higherCrossing bool
}

func (s Solid) rayContacts(l geom.Line, maxDistance, tol float64) ([]rayContact, error) {
	var result []rayContact
	for i, f := range s.faces {
		if e := validateRayTrims(f); e != nil {
			return nil, fmt.Errorf("trimmed: face %d: %w", i, e)
		}
		base, m := parametric.SurfaceMapping(f.Surface)
		inv, e := m.Inverse()
		if e != nil {
			return nil, e
		}
		localDir := inv.ApplyVector(l.Direction())
		speed := localDir.Length()
		local, e := geom.NewLine(inv.Apply(l.Origin()), localDir)
		if e != nil || !finite(speed) || speed <= 0 {
			return nil, geom.ErrInvalid
		}
		localTol := scaledTolerance(tol, inv)
		if !finite(localTol) || localTol <= 0 {
			return nil, geom.ErrInvalid
		}
		var candidates []geom.LineSurfaceHit
		var higher map[float64]bool
		switch a := base.(type) {
		case geom.CylinderSurface:
			h, e := geom.IntersectLineCylinder(local, a, geom.LineSurfaceOptions{
				UseParameterRange: true, MinParameter: -tol * speed, MaxParameter: maxDistance * speed, DistanceTolerance: localTol,
			})
			if e != nil {
				return nil, e
			}
			if len(h.Coincident) > 0 {
				return nil, errRayCoincident
			}
			candidates = h.Hits
		case geom.ConeSurface:
			h, e := geom.IntersectLineCone(local, a, geom.LineSurfaceOptions{
				UseParameterRange: true, MinParameter: -tol * speed, MaxParameter: maxDistance * speed, DistanceTolerance: localTol,
			})
			if e != nil {
				return nil, e
			}
			if len(h.Coincident) > 0 {
				return nil, errRayCoincident
			}
			candidates = h.Hits
		case geom.TorusSurface:
			h, e := geom.IntersectLineTorus(local, a, geom.LineTorusOptions{
				UseParameterRange: true, MinParameter: -tol * speed, MaxParameter: maxDistance * speed, DistanceTolerance: localTol,
			})
			if e != nil {
				return nil, e
			}
			for _, x := range h.Hits {
				candidates = append(candidates, x.LineSurfaceHit)
				if x.Crossing && x.Multiplicity > 1 {
					if higher == nil {
						higher = make(map[float64]bool)
					}
					higher[x.Parameter] = true
				}
			}
		case geom.SphereSurface:
			fr, r := a.Parameters()
			s, e := geom.NewSphere(fr.Origin(), r)
			if e != nil {
				return nil, e
			}
			x, e := geom.IntersectLineSphere(local, s, localTol)
			if e != nil {
				return nil, e
			}
			for _, p := range x {
				t := p.Sub(local.Origin()).Dot(local.Direction())
				if t >= -tol*speed && t <= maxDistance*speed {
					candidates = append(candidates, geom.LineSurfaceHit{Point: p, Parameter: t, Tangent: len(x) == 1})
				}
			}
		default:
			fr, ok := parametric.PolarPlaneFrame(base)
			if pl, plane := base.(geom.PlaneSurface); plane {
				fr, ok = pl.Frame(), true
			}
			if !ok {
				return nil, fmt.Errorf("trimmed: ray face %d has unsupported surface %T", i, f.Surface)
			}
			var e error
			candidates, e = planeRayCandidates(fr, local, -tol*speed, maxDistance*speed, localTol)
			if e != nil {
				return nil, e
			}
		}
		for _, h := range candidates {
			p, near := normalProjection(base, h.Point, localTol)
			if !near {
				return nil, fmt.Errorf("trimmed: ray surface residual exceeds tolerance")
			}
			loc, e := faceContains(f, p, localTol)
			if e != nil {
				return nil, fmt.Errorf("trimmed: face %d: %w", i, e)
			}
			if loc == parametric.Outside {
				continue
			}
			n, e := m.ApplyNormal(p.normal)
			if e != nil {
				return nil, e
			}
			if m.Determinant() < 0 {
				n = n.Scale(-1)
			}
			if f.Reversed {
				n = n.Scale(-1)
			}
			distance := h.Parameter / speed
			world, e := l.Eval(distance)
			if e != nil || !closePoint(world, m.Apply(h.Point), tol) {
				return nil, fmt.Errorf("trimmed: mapped ray residual exceeds tolerance")
			}
			result = append(result, rayContact{world, distance, n.Dot(l.Direction()), i, h.Tangent, loc == parametric.Boundary, higher[h.Parameter]})
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].distance < result[j].distance })
	return result, nil
}

func planeRayCandidates(fr geom.Frame, line geom.Line, lo, hi, tolerance float64) ([]geom.LineSurfaceHit, error) {
	n := fr.ZAxis()
	d, v := fr.Origin().Sub(line.Origin()).Dot(n), line.Direction().Dot(n)
	if !finite(d) || !finite(v) {
		return nil, geom.ErrInvalid
	}
	if v == 0 {
		if math.Abs(d) <= tolerance {
			return nil, errRayCoincident
		}
		return nil, nil
	}
	t := d / v
	if t < lo || t > hi {
		return nil, nil
	}
	p, e := line.Eval(t)
	if e != nil {
		return nil, e
	}
	return []geom.LineSurfaceHit{{Point: p, Parameter: t}}, nil
}

func scaledTolerance(tol float64, inverse geom.Affine) float64 {
	if inverse == geom.Identity() {
		return tol
	}
	n := 0.
	for _, r := range inverse {
		for j := range 3 {
			n = math.Hypot(n, r[j])
		}
	}
	return tol * n
}

func validateRayTrims(f Face) error {
	for _, l := range append([][]Use{f.Outer}, f.Holes...) {
		for _, u := range l {
			if u.Curve == nil {
				continue
			}
			c, ok := rayEllipseTrim(u.Curve)
			if !ok || c.Linear.Y != 0 || math.Abs(c.From) > 8*math.Pi || math.Abs(c.To) > 8*math.Pi {
				return fmt.Errorf("unsupported ray p-curve %T", u.Curve)
			}
		}
	}
	return nil
}

func rayEllipseTrim(p PCurve) (EllipseUV, bool) {
	switch q := p.(type) {
	case EllipseUV:
		return q, true
	case anchoredCircularUV:
		return q.EllipseUV, true
	case unionArcUV:
		// The crossing rule reads the pinned ends through EvalUV and the
		// unchanged arc between them.
		return q.EllipseUV, true
	}
	return EllipseUV{}, false
}

type chartProjection struct {
	uv        geom.Vec2
	normal    geom.Vec3
	xScale    float64
	yScale    float64
	periodic  bool
	periodicV bool
	pole      bool
}

func normalProjection(s geom.Surface, p geom.Vec3, tol float64) (chartProjection, bool) {
	switch a := s.(type) {
	case geom.PlaneSurface:
		f := a.Frame()
		q := f.ToLocal(p)
		return chartProjection{uv: geom.Vec2{X: q.X, Y: q.Y}, normal: f.ZAxis(), xScale: 1, yScale: 1}, q.IsFinite() && math.Abs(q.Z) <= tol
	case geom.CylinderSurface:
		f, r := a.Parameters()
		q := f.ToLocal(p)
		rho := math.Hypot(q.X, q.Y)
		if !q.IsFinite() || rho <= 0 || math.Abs(rho-r) > tol {
			return chartProjection{}, false
		}
		n := f.XAxis().Scale(q.X / rho).Add(f.YAxis().Scale(q.Y / rho))
		return chartProjection{uv: geom.Vec2{X: math.Atan2(q.Y, q.X), Y: q.Z}, normal: n, xScale: r, yScale: 1, periodic: true}, true
	case geom.ConeSurface:
		f, slope := a.Parameters()
		q := f.ToLocal(p)
		rho := math.Hypot(q.X, q.Y)
		l := math.Hypot(1, slope)
		h := (q.Z/l + rho*(slope/l)) / l
		if !q.IsFinite() || !finite(h) || h < -tol || math.Abs(rho/l-q.Z*(slope/l)) > tol {
			return chartProjection{}, false
		}
		h = max(0, h)
		u := math.Atan2(q.Y, q.X)
		n := f.XAxis().Scale(math.Cos(u) / l).Add(f.YAxis().Scale(math.Sin(u) / l)).Sub(f.ZAxis().Scale(slope / l))
		return chartProjection{uv: geom.Vec2{X: u, Y: h}, normal: n, xScale: max(tol, h*slope), yScale: l, periodic: true, pole: rho <= tol && h <= tol}, true
	case geom.SphereSurface:
		f, r := a.Parameters()
		q := f.ToLocal(p)
		length := q.Length()
		if !q.IsFinite() || length <= 0 || math.Abs(length-r) > tol {
			return chartProjection{}, false
		}
		rho := math.Hypot(q.X, q.Y)
		v := math.Atan2(q.Z, rho)
		n, e := p.Sub(f.Origin()).Unit()
		if e != nil {
			return chartProjection{}, false
		}
		return chartProjection{uv: geom.Vec2{X: math.Atan2(q.Y, q.X), Y: v}, normal: n, xScale: r, yScale: r, periodic: true, pole: rho <= tol}, true
	case geom.TorusSurface:
		f, major, minor := a.Parameters()
		q := f.ToLocal(p)
		rho := math.Hypot(q.X, q.Y)
		tube := math.Hypot(rho-major, q.Z)
		if !q.IsFinite() || rho <= 0 || tube <= 0 || math.Abs(tube-minor) > tol {
			return chartProjection{}, false
		}
		u, v := math.Atan2(q.Y, q.X), math.Atan2(q.Z, rho-major)
		n := f.XAxis().Scale(math.Cos(u) * math.Cos(v)).Add(f.YAxis().Scale(math.Sin(u) * math.Cos(v))).Add(f.ZAxis().Scale(math.Sin(v)))
		return chartProjection{uv: geom.Vec2{X: u, Y: v}, normal: n, xScale: major + minor*math.Cos(v), yScale: minor, periodic: true, periodicV: true}, true
	default:
		if f, ok := parametric.PolarPlaneFrame(s); ok {
			q := f.ToLocal(p)
			r := math.Hypot(q.X, q.Y)
			return chartProjection{uv: geom.Vec2{X: math.Atan2(q.Y, q.X), Y: r}, normal: f.ZAxis().Scale(-1),
				xScale: max(r, tol), yScale: 1, periodic: true, pole: r <= tol}, q.IsFinite() && math.Abs(q.Z) <= tol
		}
	}
	return chartProjection{}, false
}

func faceContains(f Face, p chartProjection, tol float64) (parametric.Location, error) {
	if p.pole {
		for _, u := range f.Outer {
			if u.Curve == nil && u.UVStart.Y == u.UVEnd.Y && math.Abs(u.UVStart.Y-p.uv.Y)*p.yScale <= tol {
				return parametric.Boundary, nil
			}
		}
	}
	periods := func(periodic bool, origin, scale float64, axis int) (float64, float64, error) {
		if !periodic {
			return 0, 0, nil
		}
		coord := func(p geom.Vec2) float64 {
			if axis == 1 {
				return p.Y
			}
			return p.X
		}
		lo, hi := coord(f.Outer[0].UVStart), coord(f.Outer[0].UVStart)
		for _, u := range f.Outer {
			lo, hi = min(lo, coord(u.UVStart)), max(hi, coord(u.UVStart))
			if c, ok := rayEllipseTrim(u.Curve); ok {
				extent := math.Hypot(coord(c.Cos), coord(c.Sin))
				a, b := coord(c.Center)+coord(c.Linear)*c.From, coord(c.Center)+coord(c.Linear)*c.To
				lo, hi = min(lo, min(a, b)-extent), max(hi, max(a, b)+extent)
			}
		}
		first, last := math.Ceil((lo-origin-tol/scale)/(2*math.Pi)), math.Floor((hi-origin+tol/scale)/(2*math.Pi))
		if !finite(first) || !finite(last) || last-first > 8 || math.Abs(first) > 1<<48 || math.Abs(last) > 1<<48 {
			return 0, 0, geom.ErrDomain
		}
		return first, last, nil
	}
	first, last, e := periods(p.periodic, p.uv.X, p.xScale, 0)
	if e != nil {
		return parametric.Outside, e
	}
	startV, stopV, e := periods(p.periodicV, p.uv.Y, p.yScale, 1)
	if e != nil {
		return parametric.Outside, e
	}
	best := parametric.Outside
	for k := first; k <= last; k++ {
		for j := startV; j <= stopV; j++ {
			uv := geom.Vec2{X: p.uv.X + k*2*math.Pi, Y: p.uv.Y + j*2*math.Pi}
			n := 0
			for _, l := range append([][]Use{f.Outer}, f.Holes...) {
				for _, u := range l {
					w, near, e := trimCrossing(u, uv, p.xScale, p.yScale, tol)
					if e != nil {
						return parametric.Outside, e
					}
					if near {
						return parametric.Boundary, nil
					}
					n += w
				}
			}
			if n == 1 {
				best = parametric.Inside
			} else if n != 0 {
				return parametric.Outside, fmt.Errorf("invalid trim winding %d", n)
			}
		}
	}
	return best, nil
}

func trimCrossing(use Use, p geom.Vec2, xScale, yScale, tol float64) (int, bool, error) {
	if use.Curve == nil {
		a, b := use.UVStart, use.UVEnd
		turn, e := geom.Orient2D(a, b, p)
		d, q := sub2(b, a), sub2(p, a)
		d.X *= xScale
		q.X *= xScale
		d.Y *= yScale
		q.Y *= yScale
		length := math.Hypot(d.X, d.Y)
		if e != nil || !finite(length) || length == 0 || !uvFinite(q) {
			return 0, false, geom.ErrInvalid
		}
		x := max(0, min(length, q.X*(d.X/length)+q.Y*(d.Y/length)))
		if math.Hypot(q.X-d.X/length*x, q.Y-d.Y/length*x) <= tol {
			return 0, true, nil
		}
		switch {
		case a.Y <= p.Y && p.Y < b.Y && turn > 0:
			return 1, false, nil
		case b.Y <= p.Y && p.Y < a.Y && turn < 0:
			return -1, false, nil
		}
		return 0, false, nil
	}
	c, ok := rayEllipseTrim(use.Curve)
	if !ok || c.Linear.Y != 0 || math.Abs(c.From) > 8*math.Pi || math.Abs(c.To) > 8*math.Pi {
		return 0, false, fmt.Errorf("unsupported ray p-curve %T", use.Curve)
	}
	// Split the angular range at vertical extrema. Each remaining piece is
	// monotone in Y and obeys the same half-open crossing rule as a line.
	ts := []float64{0, 1}
	if c.Cos.Y != 0 || c.Sin.Y != 0 {
		start := math.Atan2(c.Sin.Y, c.Cos.Y)
		lo, hi := min(c.From, c.To), max(c.From, c.To)
		for k := math.Ceil((lo - start) / math.Pi); k <= math.Floor((hi-start)/math.Pi); k++ {
			t := (start + k*math.Pi - c.From) / (c.To - c.From)
			if t > 0 && t < 1 {
				ts = append(ts, t)
			}
		}
		sort.Float64s(ts)
	}
	near := func(a geom.Vec2) bool { return math.Hypot((a.X-p.X)*xScale, (a.Y-p.Y)*yScale) <= tol }
	w := 0
	for j := 0; j+1 < len(ts); j++ {
		l, r := ts[j], ts[j+1]
		a, e := use.Curve.EvalUV(l)
		if e != nil {
			return 0, false, e
		}
		b, e := use.Curve.EvalUV(r)
		if e != nil {
			return 0, false, e
		}
		if near(a) || near(b) {
			return 0, true, nil
		}
		sign := 0
		switch {
		case a.Y <= p.Y && p.Y < b.Y:
			sign = 1
		case b.Y <= p.Y && p.Y < a.Y:
			sign = -1
		case a.Y == b.Y && math.Abs(a.Y-p.Y)*yScale <= tol && c.Cos.X == 0 && c.Sin.X == 0:
			if p.X >= min(a.X, b.X) && p.X <= max(a.X, b.X) {
				return 0, true, nil
			}
		}
		if sign == 0 {
			continue
		}
		for range 53 {
			t := l + (r-l)/2
			q, e := c.EvalUV(t)
			if e != nil {
				return 0, false, e
			}
			if q.Y == p.Y || t == l || t == r {
				l, r = t, t
				break
			}
			if (q.Y < p.Y) == (sign > 0) {
				l = t
			} else {
				r = t
			}
		}
		q, e := c.EvalUV(l + (r-l)/2)
		if e != nil {
			return 0, false, e
		}
		if near(q) {
			return 0, true, nil
		}
		if q.X > p.X {
			w += sign
		}
	}
	return w, false, nil
}

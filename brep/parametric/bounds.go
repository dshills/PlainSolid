package parametric

import (
	"context"
	"fmt"
	"math"

	"plainsolid/geom"
)

// Bounds returns a conservative world-space AABB for supported patches.
// Analytic supports are bounded on their parameter rectangles; splines use
// their positive-weight control hull. Affine maps transform the local bounds.
// General custom surfaces return an explicit error. The result includes the
// boundary agreement tolerance and a roundoff allowance.
func (s Solid) Bounds() (geom.Box, error) {
	var box geom.Box
	if len(s.faces) == 0 {
		return box, fmt.Errorf("parametric: empty solid has no bounds")
	}
	for i, f := range s.faces {
		b, e := SurfaceBounds(f.Surface, f.U, f.V)
		if e != nil {
			return geom.Box{}, fmt.Errorf("parametric: face %d bounds: %w", i, e)
		}
		if i == 0 {
			box = b
		} else {
			box, e = box.Union(b)
			if e != nil {
				return geom.Box{}, e
			}
		}
	}
	for _, p := range s.vertices {
		var e error
		box, e = box.Union(geom.Box{Min: p, Max: p})
		if e != nil {
			return geom.Box{}, e
		}
	}
	if box.Min == box.Max {
		return geom.Box{}, fmt.Errorf("parametric: degenerate bounds")
	}
	// Generated caps and transformed patches may round a convex combination
	// just outside its endpoint range. Include the stored geometric tolerance
	// and a small allowance proportional to the evaluated coordinate scale.
	pad := func(lo, hi float64) (float64, float64) {
		mag := math.Max(math.Abs(lo), math.Abs(hi))
		roundoff := 32 * (math.Nextafter(1, 2) - 1) * mag
		allowance := s.tolerance + roundoff
		a, b := math.Nextafter(lo-allowance, math.Inf(-1)), math.Nextafter(hi+allowance, math.Inf(1))
		if math.IsInf(a, -1) {
			a = -math.MaxFloat64
		}
		if math.IsInf(b, 1) {
			b = math.MaxFloat64
		}
		return a, b
	}
	box.Min.X, box.Max.X = pad(box.Min.X, box.Max.X)
	box.Min.Y, box.Max.Y = pad(box.Min.Y, box.Max.Y)
	box.Min.Z, box.Max.Z = pad(box.Min.Z, box.Max.Z)

	return box, nil
}

type scalarBounds struct{ lo, hi float64 }

func multiply(a, b scalarBounds) scalarBounds {
	p, q, r, s := a.lo*b.lo, a.lo*b.hi, a.hi*b.lo, a.hi*b.hi
	return scalarBounds{math.Min(math.Min(p, q), math.Min(r, s)), math.Max(math.Max(p, q), math.Max(r, s))}
}
func trig(u Interval, sine bool) scalarBounds {
	if u.Max-u.Min >= 2*math.Pi || math.Abs(u.Min) > 1e6 || math.Abs(u.Max) > 1e6 {
		return scalarBounds{-1, 1}
	}
	fn := math.Cos
	shift := 0.
	if sine {
		fn = math.Sin
		shift = math.Pi / 2
	}
	a, b := fn(u.Min), fn(u.Max)
	r := scalarBounds{math.Min(a, b), math.Max(a, b)}
	for k := int(math.Floor((u.Min - shift) / math.Pi)); k <= int(math.Ceil((u.Max-shift)/math.Pi)); k++ {
		t := shift + float64(k)*math.Pi
		if t >= u.Min && t <= u.Max {
			if k%2 == 0 {
				r.hi = 1
			} else {
				r.lo = -1
			}
		}
	}
	return r
}

// SurfaceBounds encloses the supported surface on a rectangular parameter
// domain. It includes the real-arithmetic geometry; a caller combining
// floating evaluations and shared edges should add its agreement tolerance
// and evaluation roundoff allowance.
func SurfaceBounds(s geom.Surface, u, v Interval) (geom.Box, error) {
	if !u.valid() || !v.valid() {
		return geom.Box{}, geom.ErrDomain
	}
	return surfaceBounds(s, u, v)
}

func surfaceBounds(s geom.Surface, u, v Interval) (geom.Box, error) {
	var f geom.Frame
	var x, y, z scalarBounds
	c, si := trig(u, false), trig(u, true)
	radial := func(r scalarBounds) { x, y = multiply(r, c), multiply(r, si) }
	switch a := s.(type) {
	case geom.PlaneSurface:
		f = a.Frame()
		x, y = scalarBounds{u.Min, u.Max}, scalarBounds{v.Min, v.Max}
	case geom.CylinderSurface:
		frame, r := a.Parameters()
		f = frame
		radial(scalarBounds{r, r})
		z = scalarBounds{v.Min, v.Max}
	case geom.TwistedCylinderSurface:
		frame, r, _, _ := a.Parameters()
		f = frame
		x, y, z = scalarBounds{-r, r}, scalarBounds{-r, r}, scalarBounds{v.Min, v.Max}
	case geom.TwistedAnnularSurface:
		frame, _, r, height, _ := a.Parameters()
		f = frame
		x, y, z = scalarBounds{-r, r}, scalarBounds{-r, r}, scalarBounds{height, height}
	case geom.ConeSurface:
		frame, r := a.Parameters()
		f = frame
		radial(scalarBounds{v.Min * r, v.Max * r})
		z = scalarBounds{v.Min, v.Max}
	case geom.SphereSurface:
		frame, r := a.Parameters()
		f = frame
		cv, sv := trig(v, false), trig(v, true)
		radial(multiply(cv, scalarBounds{r, r}))
		z = multiply(sv, scalarBounds{r, r})
	case offsetBoreChart:
		if u.Min < 0 || u.Max > 2*math.Pi || v.Min < 0 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		f = a.frame
		switch a.kind {
		case offsetBoreSphere:
			radial(scalarBounds{0, a.radius})
			z = scalarBounds{-a.radius, a.radius}
		case offsetBoreWall, offsetBoreFloor:
			radial(scalarBounds{0, upAdd(a.offset, a.bore)})
			z = scalarBounds{a.floor, a.floor}
			if a.kind == offsetBoreWall {
				z.hi = a.radius
			}
		default:
			return geom.Box{}, geom.ErrInvalid
		}
	case steinmetzExteriorCylinder:
		if !a.valid() || u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < 0 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		var err error
		f, err = geom.NewFrame(a.center, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if err != nil {
			return geom.Box{}, err
		}
		x, y, z = scalarBounds{math.Min(0, a.end), math.Max(0, a.end)}, scalarBounds{-a.radius, a.radius}, scalarBounds{-a.radius, a.radius}
	case steinmetzDualCapChart:
		if !a.valid() || u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < 0 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		var err error
		f, err = geom.NewFrame(a.center, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if err != nil {
			return geom.Box{}, err
		}
		// Rounded split angles can leave a chart endpoint a few ulps past a cap.
		x, y, z = scalarBounds{-a.radius, a.radius}, scalarBounds{-a.radius, a.radius}, scalarBounds{-a.radius, a.radius}
		if a.large > 0 {
			if a.wideAxis == 0 {
				x = scalarBounds{-a.large, a.large}
			} else {
				z = scalarBounds{-a.large, a.large}
			}
		}
	case steinmetzSlabChart:
		if !a.valid() || u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < 0 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		var err error
		f, err = geom.NewFrame(a.center, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if err != nil {
			return geom.Box{}, err
		}
		x, y, z = scalarBounds{-a.radius, a.radius}, scalarBounds{-a.radius, a.radius}, scalarBounds{-a.radius, a.radius}
	case unequalUnionChart:
		if !a.valid() || u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < 0 || v.Max > 1 || ((a.kind == 1 || a.kind == 3) && u.Min < 0) {
			return geom.Box{}, geom.ErrDomain
		}
		var err error
		f, err = geom.NewFrame(a.center, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if err != nil {
			return geom.Box{}, err
		}
		x, y, z = scalarBounds{-max(a.xHalf, a.narrow), max(a.xHalf, a.narrow)}, scalarBounds{-a.wide, a.wide}, scalarBounds{-max(a.zHalf, a.wide), max(a.zHalf, a.wide)}
	case steinmetzUnionChart:
		if u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < 0 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		var err error
		f, err = geom.NewFrame(a.center, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if err != nil {
			return geom.Box{}, err
		}
		x, y, z = scalarBounds{-a.halfLength, a.halfLength}, scalarBounds{-a.radius, a.radius}, scalarBounds{-a.radius, a.radius}
		if a.axis == 1 {
			x, z = z, x
		}
	case splitCylinderCutChart:
		if u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < 0 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		var e error
		f, e = geom.NewFrame(a.center, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if e != nil {
			return geom.Box{}, e
		}
		x, y, z = scalarBounds{-a.end, a.end}, scalarBounds{-a.stock, a.stock}, scalarBounds{-a.stock, a.stock}
	case obliqueSplitChart:
		if u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < 0 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		var e error
		f, e = geom.NewFrame(a.center, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if e != nil {
			return geom.Box{}, e
		}
		d := a.stockAxis
		n := upAdd(upAdd(math.Abs(d.X), math.Abs(d.Y)), math.Abs(d.Z))
		m := upAdd(upMul(n, max(math.Abs(a.stockCaps[0]), math.Abs(a.stockCaps[1]))), a.stock)
		x, y, z = scalarBounds{-m, m}, scalarBounds{-m, m}, scalarBounds{-m, m}
	case steinmetzClippedCylinder:
		if u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < 0 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		var err error
		f, err = geom.NewFrame(a.center, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if err != nil {
			return geom.Box{}, err
		}
		x, y, z = scalarBounds{-a.radius, a.cap}, scalarBounds{-a.radius, a.radius}, scalarBounds{-a.radius, a.radius}
	case steinmetzDisk:
		if u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < -1 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		var err error
		f, err = geom.NewFrame(a.center, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if err != nil {
			return geom.Box{}, err
		}
		x, y, z = scalarBounds{0, 0}, scalarBounds{-a.radius, a.radius}, scalarBounds{-a.radius, a.radius}
	case steinmetzChart:
		if u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < -1 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		var err error
		f, err = geom.NewFrame(a.center, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if err != nil {
			return geom.Box{}, err
		}
		x, y, z = scalarBounds{-a.radius, a.radius}, scalarBounds{-a.radius, a.radius}, scalarBounds{-a.radius, a.radius}
	case unequalCylinderCommonChart:
		if u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < -1 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		var err error
		f, err = geom.NewFrame(a.center, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if err != nil {
			return geom.Box{}, err
		}
		x, y, z = scalarBounds{-a.small, a.small}, scalarBounds{-a.small, a.small}, scalarBounds{-a.large, a.large}
		if a.swap {
			x, z = z, x
		}
	case skewCylinderChart:
		if u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < -1 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		var err error
		f, err = geom.NewFrame(a.center, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if err != nil {
			return geom.Box{}, err
		}
		x, y, z = scalarBounds{-a.small, a.small}, scalarBounds{-a.small, a.small}, scalarBounds{-a.large, a.large}
		if a.swap {
			x, z = z, x
		} else {
			y = scalarBounds{math.Nextafter(a.shift-a.small, math.Inf(-1)), math.Nextafter(a.shift+a.small, math.Inf(1))}
		}
	case skewOverlapCylinderChart:
		if u.Min < 0 || u.Max > math.Pi/2 || v.Min < -1 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		var err error
		f, err = geom.NewFrame(a.center, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if err != nil {
			return geom.Box{}, err
		}
		x, y, z = scalarBounds{-a.radiusZ, a.radiusZ}, scalarBounds{math.Nextafter(a.radiusX-a.length, math.Inf(-1)), a.radiusX}, scalarBounds{-a.radiusX, a.radiusX}
		if a.reflected {
			y = scalarBounds{-y.hi, -y.lo}
		}
	case obliqueCylinderChart:
		if u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < -1 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		var err error
		f, err = geom.NewFrame(a.center, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if err != nil {
			return geom.Box{}, err
		}
		// Since ∂²P/∂u²=−(P−center), the exact Hessian bound
		// also bounds the radius of a conservative model-space box.
		h, err := a.HessianBounds()
		if err != nil {
			return geom.Box{}, err
		}
		m := upperBoreRat(h.UU)
		x, y, z = scalarBounds{-m, m}, scalarBounds{-m, m}, scalarBounds{-m, m}
	case obliqueCylinderUnionChart:
		if u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < 0 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		var e error
		f, e = geom.NewFrame(a.center, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if e != nil {
			return geom.Box{}, e
		}
		d := a.u
		if a.owner == 1 {
			d = a.v
		}
		w := a.caps[a.owner]
		n := upAdd(upAdd(math.Abs(d.X), math.Abs(d.Y)), math.Abs(d.Z))
		m := upAdd(upMul(n, max(math.Abs(w[0]), math.Abs(w[1]))), a.radius)
		x, y, z = scalarBounds{-m, m}, scalarBounds{-m, m}, scalarBounds{-m, m}
	case generalCylinderCommonChart:
		if u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < -1 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		coverage, e := geom.ClassifyCylinderCommonCaps(context.Background(), a.u, a.v, a.ru, a.rv, nil, nil, geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			return geom.Box{}, e
		}
		n := upAdd(upAdd(math.Abs(a.u.X), math.Abs(a.u.Y)), math.Abs(a.u.Z))
		m := upAdd(upMul(upperBoreRat(coverage.Extents[0].Bounds[1]), n), max(a.ru, a.rv))
		f, e = geom.NewFrame(a.center, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if e != nil {
			return geom.Box{}, e
		}
		x, y, z = scalarBounds{-m, m}, scalarBounds{-m, m}, scalarBounds{-m, m}
	case toolBoreChart:
		if u.Min < 0 || u.Max > 2*math.Pi || v.Min < 0 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		f = a.frame
		x, y = scalarBounds{-a.radius, a.radius}, scalarBounds{-a.radius, a.radius}
		switch a.kind {
		case toolBoreLower, toolBoreLowerAnnulus:
			z = scalarBounds{-a.radius, 0}
		case toolBoreUpper:
			z = scalarBounds{0, a.radius}
		case toolBoreWall, toolBoreFloor, toolBoreThroughWall, toolBoreUnionWall, toolBoreUnionCap:
			x, y = scalarBounds{-a.bore, upAdd(a.offset, a.bore)}, scalarBounds{-a.bore, a.bore}
			z = scalarBounds{a.floor, a.floor}
			if a.kind == toolBoreWall {
				z.hi = a.radius
			} else if a.kind == toolBoreThroughWall {
				z = scalarBounds{-a.radius, a.radius}
			} else if a.kind == toolBoreUnionWall {
				z.lo = 0
			}
		default:
			return geom.Box{}, geom.ErrInvalid
		}
	case offsetCommonChart:
		if u.Min < 0 || u.Max > 2*math.Pi || v.Min < 0 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		f = a.frame
		x, y = scalarBounds{-a.r, upAdd(a.offset, a.r)}, scalarBounds{-a.r, a.r}
		z = scalarBounds{max(-a.R, a.lower), min(a.R, a.upper)}
	case offsetPartialCommon:
		if u.Min < 0 || u.Max > 2*math.Pi || v.Min < 0 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		f = a.frame
		x, y = scalarBounds{-a.r, upAdd(a.offset, a.r)}, scalarBounds{-a.r, a.r}
		z = scalarBounds{a.lower, a.upper}
	case offsetDoubleCommon:
		if u.Min < 0 || u.Max > 2*math.Pi || v.Min < 0 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		f = a.frame
		x, y = scalarBounds{-a.r, upAdd(a.d, a.r)}, scalarBounds{-a.r, a.r}
		z = scalarBounds{a.lower, a.upper}
	case offsetSameSideCommon:
		if u.Min < 0 || u.Max > 2*math.Pi || v.Min < 0 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		f = a.frame
		x, y = scalarBounds{-a.r, upAdd(a.d, a.r)}, scalarBounds{-a.r, a.r}
		z = scalarBounds{a.lower, a.upper}
	case torusCylinderOuterWall:
		if v.Min < 0 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		f, _ = geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		x, y = scalarBounds{-a.source.c, a.source.c}, scalarBounds{-a.source.c, a.source.c}
		z = scalarBounds{-a.source.r, a.cap}
		if a.lower {
			z = scalarBounds{a.cap, a.source.r}
		}
	case torusCylinderChart:
		if v.Min < -1 || v.Max > 1 {
			return geom.Box{}, geom.ErrDomain
		}
		f, _ = geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		extent := upAdd(a.R, a.r)
		x, y, z = scalarBounds{-extent, extent}, scalarBounds{-extent, extent}, scalarBounds{-a.r, a.r}
	case geom.TorusSurface:
		frame, r, t := a.Parameters()
		f = frame
		cv, sv := trig(v, false), trig(v, true)
		radial(scalarBounds{r + t*cv.lo, r + t*cv.hi})
		z = multiply(sv, scalarBounds{t, t})
	case geom.OuterTorusSurface:
		if v.Min < -math.Pi/2 || v.Max > math.Pi/2 {
			return geom.Box{}, geom.ErrDomain
		}
		frame, r, t := a.Parameters()
		f = frame
		cv, sv := trig(v, false), trig(v, true)
		radial(scalarBounds{r + t*cv.lo, r + t*cv.hi})
		z = multiply(sv, scalarBounds{t, t})
	case reparameterizedCylinder:
		return surfaceBounds(a.s, Interval{a.u.at(u.Min), a.u.at(u.Max)}, v)
	case ruledArcs:
		w := Interval{a.height, a.height}
		p, e := surfaceBounds(a.a, Interval{a.u.at(u.Min), a.u.at(u.Max)}, w)
		if e != nil {
			return geom.Box{}, e
		}
		q, e := surfaceBounds(a.b, Interval{a.v.at(u.Min), a.v.at(u.Max)}, w)
		if e != nil {
			return geom.Box{}, e
		}
		return p.Union(q)
	case cutCylinder:
		k := math.Hypot(a.rim.cosine, a.rim.sine)
		return surfaceBounds(a.rim.cyl, u, Interval{math.Min(a.constant, a.rim.center-k), math.Max(a.constant, a.rim.center+k)})
	case slantedFan:
		k := math.Hypot(a.rim.cosine, a.rim.sine)
		b, e := surfaceBounds(a.rim.cyl, u, Interval{a.rim.center - k, a.rim.center + k})
		if e != nil {
			return geom.Box{}, e
		}
		return b.Union(geom.Box{Min: a.center, Max: a.center})
	case arcFan:
		b, e := surfaceBounds(a.surface, u, Interval{a.height, a.height})
		if e != nil {
			return geom.Box{}, e
		}
		return b.Union(geom.Box{Min: a.center, Max: a.center})
	case disk:
		f = a.frame
		radial(scalarBounds{v.Min, v.Max})
	case revolvedSide:
		f = a.frame
		radial(scalarBounds{math.Min(a.a.X, a.b.X), math.Max(a.a.X, a.b.X)})
		z = scalarBounds{math.Min(a.a.Y, a.b.Y), math.Max(a.a.Y, a.b.Y)}
	case geom.HelicalProfileSurface:
		return a.Bounds(geom.ParameterRectangle{UMin: u.Min, UMax: u.Max, VMin: v.Min, VMax: v.Max})
	case ellipticPatch:
		f = a.frame
		x = multiply(c, scalarBounds{math.Min(a.lower.X, a.upper.X), math.Max(a.lower.X, a.upper.X)})
		y = multiply(si, scalarBounds{math.Min(a.lower.Y, a.upper.Y), math.Max(a.lower.Y, a.upper.Y)})
		z = scalarBounds{0, a.height}
	case ellipticBand:
		f = a.frame
		x = multiply(c, scalarBounds{min(a.x[0], a.x[1], a.x[2], a.x[3]), max(a.x[0], a.x[1], a.x[2], a.x[3])})
		y = multiply(si, scalarBounds{min(a.y[0], a.y[1], a.y[2], a.y[3]), max(a.y[0], a.y[1], a.y[2], a.y[3])})
		z = scalarBounds{a.z + a.h*v.Min, a.z + a.h*v.Max}
	case *geom.BSplineSurface:
		p, e := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
		if e == nil {
			p, e = p.Trim(geom.ParameterRectangle{UMin: u.Min, UMax: u.Max, VMin: v.Min, VMax: v.Max})
			if e == nil {
				return p.Bounds()
			}
		}
		// Unclamped or discontinuous splines still lie in the full positive
		// control hull. The fallback is conservative, just less tight.
		return geom.Bounds(a.Data().Poles)
	case *geom.ExactBezierSurface:
		return exactPatchSurfaceBounds(a.ExactPatch(), geom.Identity(), u, v)
	case *geom.ExactTensorSurface:
		return exactPatchSurfaceBounds(a.ExactPatch(), geom.Identity(), u, v)
	case *geom.WrappedSurfacePatch:
		if a == nil || a.FundamentalPatch() == nil {
			return geom.Box{}, geom.ErrInvalid
		}
		p := a.FundamentalPatch()
		d, period := p.Domain(), a.Periods()
		if period[0] == nil && (u.Min < d.UMin || u.Max > d.UMax) || period[1] == nil && (v.Min < d.VMin || v.Max > d.VMax) {
			return geom.Box{}, geom.ErrDomain
		}
		r := geom.ParameterRectangle{UMin: u.Min, UMax: u.Max, VMin: v.Min, VMax: v.Max}
		if r.UMin >= d.UMin && r.UMax <= d.UMax && r.VMin >= d.VMin && r.VMax <= d.VMax {
			q, e := p.Trim(r)
			if e == nil {
				return q.Bounds()
			}
		}
		return p.Bounds()
	case mappedSurface:
		switch exact := a.surface.(type) {
		case *geom.ExactBezierSurface:
			return exactPatchSurfaceBounds(exact.ExactPatch(), a.transform, u, v)
		case *geom.ExactTensorSurface:
			return exactPatchSurfaceBounds(exact.ExactPatch(), a.transform, u, v)
		}
		b, e := surfaceBounds(a.surface, u, v)
		if e != nil {
			return geom.Box{}, e
		}
		return a.transform.ApplyBox(b)
	default:
		return geom.Box{}, fmt.Errorf("unsupported surface type %T", s)
	}
	var corners []geom.Vec3
	for _, a := range []float64{x.lo, x.hi} {
		for _, b := range []float64{y.lo, y.hi} {
			for _, c := range []float64{z.lo, z.hi} {
				corners = append(corners, f.ToWorld(geom.Vec3{X: a, Y: b, Z: c}))
			}
		}
	}
	return geom.Bounds(corners)
}

func exactPatchSurfaceBounds(p *geom.SurfacePatch, placement geom.Affine, u, v Interval) (geom.Box, error) {
	if p == nil {
		return geom.Box{}, geom.ErrInvalid
	}
	p, e := p.Trim(geom.ParameterRectangle{UMin: u.Min, UMax: u.Max, VMin: v.Min, VMax: v.Max})
	if e != nil {
		return geom.Box{}, e
	}
	p, e = p.Transformed(placement)
	if e != nil {
		return geom.Box{}, e
	}
	return p.Bounds()
}

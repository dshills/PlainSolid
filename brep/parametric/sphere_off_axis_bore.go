package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// SphereOffAxisBlindBoreResult identifies the three native boundary support
// groups. The spherical exterior, inward cylinder wall and planar floor
// share the same azimuth parameter at both circular or curved junctions.
type SphereOffAxisBlindBoreResult struct {
	Solid                                        Solid
	SphericalFaces, CylindricalFaces, FloorFaces []int
}

// SphereOffAxisBlindBore cuts a vertical round pocket into the north side
// of a sphere. The sphere is centered on frame.Origin; the tool axis is
// offset along the local X axis. This construction requires 0 < offset <
// boreRadius and offset+boreRadius < sphereRadius. The tool extends upward
// from floor, which lies strictly between the lower and upper wall/sphere
// intersections at every azimuth. Heights are in local model units.
//
// The supports and their intersection are analytic. The parameterization
// covers the sphere from the south pole to the positive wall intersection
// along each ray, without fitting a spline to the seam. Incidence is sewn
// at the supplied native tolerance. Singular and unresolved inputs fail.
func SphereOffAxisBlindBore(ctx context.Context, frame geom.Frame, sphereRadius, boreRadius, offset, floor, tolerance float64) (SphereOffAxisBlindBoreResult, error) {
	var empty SphereOffAxisBlindBoreResult
	if ctx == nil || !finite(tolerance) || tolerance <= 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	cfg := offsetBoreChart{frame: frame, radius: sphereRadius, bore: boreRadius, offset: offset, floor: floor}
	if e := cfg.validate(tolerance); e != nil {
		return empty, e
	}
	var faces []Face
	var out SphereOffAxisBlindBoreResult
	for i := range 4 {
		u := Interval{float64(i) * math.Pi / 2, float64(i+1) * math.Pi / 2}
		out.SphericalFaces = append(out.SphericalFaces, len(faces))
		faces = append(faces, Face{Surface: cfg, U: u, V: Interval{0, 1}})
		cfg.kind = offsetBoreWall
		out.CylindricalFaces = append(out.CylindricalFaces, len(faces))
		faces = append(faces, Face{Surface: cfg, U: u, V: Interval{0, 1}, Reversed: true})
		cfg.kind = offsetBoreFloor
		out.FloorFaces = append(out.FloorFaces, len(faces))
		faces = append(faces, Face{Surface: cfg, U: u, V: Interval{0, 1}, Reversed: true})
		cfg.kind = offsetBoreSphere
	}
	var e error
	out.Solid, e = joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return empty, e
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return out, nil
}

const (
	offsetBoreSphere = iota
	offsetBoreWall
	offsetBoreFloor
)

// All three charts share p=ρ(θ)eθ at their common edges. Since the bore
// circle contains the sphere axis, each ray exits its disk exactly once.
// At latitude λ the sphere radius is R cos(λ); its only retained interval
// is [-π/2, atan2(h,ρ)] because the floor is inside both rim heights.
type offsetBoreChart struct {
	frame                       geom.Frame
	radius, bore, offset, floor float64
	kind                        int
}

func (s offsetBoreChart) validate(tolerance float64) error {
	if !finite(s.radius) || !finite(s.bore) || !finite(s.offset) || !finite(s.floor) ||
		s.offset <= 0 || s.bore <= 0 || s.radius <= 0 || s.frame.ZAxis().Length() == 0 ||
		s.kind < offsetBoreSphere || s.kind > offsetBoreFloor {
		return geom.ErrInvalid
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sq := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	pad := new(big.Rat).Mul(q(tolerance), big.NewRat(8, 1))
	outer := new(big.Rat).Add(q(s.offset), q(s.bore))
	if new(big.Rat).Sub(q(s.bore), q(s.offset)).Cmp(pad) <= 0 ||
		new(big.Rat).Sub(q(s.radius), outer).Cmp(pad) <= 0 {
		return geom.ErrConvergence
	}
	near := new(big.Rat).Add(q(math.Abs(s.floor)), pad)
	if new(big.Rat).Add(sq(outer), sq(near)).Cmp(sq(q(s.radius))) >= 0 {
		return geom.ErrConvergence
	}
	return nil
}

func (s offsetBoreChart) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.evaluate(u, v)
	return p, e
}

func (s offsetBoreChart) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, a, b, e := s.evaluate(u, v)
	return a, b, e
}

func (s offsetBoreChart) secondDerivativeBounds(ctx context.Context, u, v Interval) (float64, float64, float64, error) {
	if u.Min < 0 || u.Max > 2*math.Pi || v.Min < 0 || v.Max > 1 {
		return 0, 0, 0, geom.ErrDomain
	}
	b, e := geom.BoundSphereCenteredBore(ctx, s.radius, s.bore, s.offset, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return 0, 0, 0, e
	}
	upper := func(r *big.Rat) float64 {
		v, exact := r.Float64()
		if !exact {
			return math.Nextafter(v, math.Inf(1))
		}
		return v
	}
	var uu, uv, vv float64
	rho, p, q := upper(b.Rho[1]), upper(b.RhoFirst), upper(b.RhoSecond)
	xyUU := upAdd(upAdd(rho, upMul(2, p)), q)
	switch s.kind {
	case offsetBoreSphere:
		// Latitude is v*(L(u)+π/2)-π/2. Each first and second
		// partial of the underlying sphere has norm at most R.
		a, b := upper(b.LatitudeFirst), upper(b.LatitudeSecond)
		pi := up(math.Pi)
		uu = upMul(s.radius, upAdd(upAdd(1, upMul(2, a)), upAdd(upMul(a, a), b)))
		uv = upMul(s.radius, upAdd(upMul(upAdd(1, a), pi), a))
		vv = upMul(s.radius, upMul(pi, pi))
	case offsetBoreWall:
		uu = upAdd(xyUU, upper(b.HeightSecond))
		uv = upper(b.HeightFirst)
	case offsetBoreFloor:
		uu, uv = xyUU, upAdd(rho, p)
	default:
		return 0, 0, 0, geom.ErrInvalid
	}
	// Bound the operator norm of the represented affine frame by its
	// Frobenius norm. This also covers non-coordinate frame roundoff.
	norm := 0.
	for _, a := range [...]geom.Vec3{s.frame.XAxis(), s.frame.YAxis(), s.frame.ZAxis()} {
		for _, x := range [...]float64{a.X, a.Y, a.Z} {
			norm = upAdd(norm, upMul(math.Abs(x), math.Abs(x)))
		}
	}
	norm = up(math.Sqrt(norm))
	return upMul(uu, norm), upMul(uv, norm), upMul(vv, norm), nil
}

func (s offsetBoreChart) evaluate(u, v float64) (p, du, dv geom.Vec3, err error) {
	if !finite(u) || !finite(v) || u < 0 || u > 2*math.Pi || v < 0 || v > 1 {
		return p, du, dv, geom.ErrDomain
	}
	if u == 2*math.Pi {
		u = 0
	}
	sin, cos := math.Sincos(u)
	// Work in scaled coordinates to avoid forming squared model radii.
	d, r := s.offset/s.radius, s.bore/s.radius
	x := d * sin
	root := math.Sqrt((r - x) * (r + x))
	if !finite(root) || root <= 0 {
		return p, du, dv, geom.ErrConvergence
	}
	rho := d*cos + root
	if cos < 0 {
		// Rationalized positive root avoids loss of small radial exits.
		rho = (r - d) * (r + d) / (root - d*cos)
	}
	if rho <= 0 || rho >= 1 || !finite(rho) {
		return p, du, dv, geom.ErrConvergence
	}
	dRho := -d * sin * rho / root
	h := math.Sqrt((1 - rho) * (1 + rho))
	dH := -rho * dRho / h
	radial := geom.Vec3{X: cos, Y: sin}
	azimuth := geom.Vec3{X: -sin, Y: cos}
	z := geom.Vec3{Z: 1}
	rimXY := radial.Scale(rho)
	dRimXY := azimuth.Scale(rho).Add(radial.Scale(dRho))
	floor := s.floor / s.radius
	switch s.kind {
	case offsetBoreSphere:
		span := math.Atan2(h, rho) + math.Pi/2
		lat := v*span - math.Pi/2
		sl, cl := math.Sincos(lat)
		if v == 0 {
			sl, cl = -1, 0
		} else if v == 1 {
			// Use precisely the shared rim equation at its native edge.
			sl, cl = h, rho
		}
		p = radial.Scale(cl).Add(z.Scale(sl))
		tangent := radial.Scale(-sl).Add(z.Scale(cl))
		du = azimuth.Scale(cl).Add(tangent.Scale(-v * dRho / h))
		dv = tangent.Scale(span)
	case offsetBoreWall:
		p = rimXY.Add(z.Scale(floor + v*(h-floor)))
		if v == 1 {
			p.Z = h
		}
		du = dRimXY.Add(z.Scale(v * dH))
		dv = z.Scale(h - floor)
	case offsetBoreFloor:
		p = rimXY.Scale(v).Add(z.Scale(floor))
		du = dRimXY.Scale(v)
		dv = rimXY
	default:
		return p, du, dv, geom.ErrInvalid
	}
	apply := func(v geom.Vec3) geom.Vec3 {
		return s.frame.XAxis().Scale(v.X).Add(s.frame.YAxis().Scale(v.Y)).Add(s.frame.ZAxis().Scale(v.Z))
	}
	p = s.frame.ToWorld(p.Scale(s.radius))
	du, dv = apply(du.Scale(s.radius)), apply(dv.Scale(s.radius))
	if !p.IsFinite() || !du.IsFinite() || !dv.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrConvergence
	}
	return p, du, dv, nil
}

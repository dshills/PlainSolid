package step

import (
	"bytes"
	"io"
	"math/big"
	"plainsolid/geom"
)

// OffAxisRimCorrespondence bounds the full closed model spline against the
// exact positive-Z intersection of its authored sphere and cylinder. The
// azimuth-preserving correspondence is onto in both directions. It does not
// certify authored edge trim intervals, other edges, face coverage or material.
// All distances and tolerance use source model length units.
type OffAxisRimCorrespondence struct {
	BodyID, EdgeID, ModelCurveID, SphereID, CylinderID    int64
	Winding                                               PolynomialWindingEvidence
	SphereResidual, CylinderResidual                      *big.Rat
	SourceHeightLower, TargetHeightLower, HorizontalUpper *big.Rat
	RadialErrorUpper, VerticalErrorUpper, HausdorffUpper  *big.Rat
	FullDomainCorrespondence, WithinTolerance             bool
}

// BoundOffAxisRimCorrespondence requires exact canonical +XYZ primitive frames,
// a positive X offset, R>d+r, a positive source Z control hull, and exact
// source closure with one strictly positive azimuth turn. Source support data
// must convert losslessly to the existing exact residual API's float inputs.
// Only the polynomial curve may approximate the exact support intersection;
// no support is rounded, repaired or inferred from nearby geometry.
func BoundOffAxisRimCorrespondence(r io.Reader, l Limits, bodyID int64, tolerance float64, o PolynomialWindingOptions) (out OffAxisRimCorrespondence, err error) {
	fail := func() (OffAxisRimCorrespondence, error) { return OffAxisRimCorrespondence{}, geom.ErrDomain }
	if r == nil || !finiteNumber(tolerance) || tolerance <= 0 {
		return fail()
	}
	l, err = l.defaults()
	if err != nil {
		return out, err
	}
	raw, err := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if err != nil {
		return out, err
	}
	if int64(len(raw)) > l.MaxBytes {
		return fail()
	}
	top, err := ValidateOffAxisBoreTopology(bytes.NewReader(raw), l, bodyID)
	if err != nil {
		return out, err
	}
	f, err := Parse(bytes.NewReader(raw), l)
	if err != nil {
		return out, err
	}
	var sphereID int64
	for _, face := range top.Faces {
		if face.SurfaceKind == "SPHERICAL_SURFACE" {
			if sphereID != 0 {
				return fail()
			}
			sphereID = face.SurfaceID
		}
	}
	support := func(id int64, kind string) ([3]*big.Rat, *big.Rat, error) {
		var zero [3]*big.Rat
		rec, e := simple(f, id, kind, 3)
		if e != nil {
			return zero, nil, e
		}
		pid, e := ref(rec.Args[1])
		if e != nil {
			return zero, nil, e
		}
		origin, axes, e := sphereLiftFrame(f, pid)
		if e != nil {
			return zero, nil, e
		}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				want := int64(0)
				if i == j {
					want = 1
				}
				if axes[i][j].Cmp(big.NewRat(want, 1)) != 0 {
					return zero, nil, geom.ErrDomain
				}
			}
		}
		radius, e := liftSourceRat(rec.Args[2])
		if e != nil || radius.Sign() <= 0 {
			return zero, nil, geom.ErrDomain
		}
		return origin, radius, nil
	}
	so, R, err := support(sphereID, "SPHERICAL_SURFACE")
	if err != nil {
		return out, err
	}
	co, rad, err := support(top.Intersection.CylinderID, "CYLINDRICAL_SURFACE")
	if err != nil {
		return out, err
	}
	d := new(big.Rat).Sub(co[0], so[0])
	if d.Sign() <= 0 || co[1].Cmp(so[1]) != 0 {
		return fail()
	}
	h0 := new(big.Rat).Sub(R, new(big.Rat).Add(d, rad))
	if h0.Sign() <= 0 {
		return fail()
	}
	vector := func(v [3]*big.Rat) (geom.Vec3, error) {
		var p geom.Vec3
		for i, x := range []*float64{&p.X, &p.Y, &p.Z} {
			var exact bool
			*x, exact = v[i].Float64()
			if !exact || !finiteNumber(*x) {
				return p, geom.ErrDomain
			}
		}
		return p, nil
	}
	sc, e := vector(so)
	if e != nil {
		return fail()
	}
	cc, e := vector(co)
	if e != nil {
		return fail()
	}
	rf, exact := R.Float64()
	if !exact || !finiteNumber(rf) {
		return fail()
	}
	cf, exact := rad.Float64()
	if !exact || !finiteNumber(cf) {
		return fail()
	}
	_, poles, _, e := readResidualPolynomial(bytes.NewReader(raw), l, top.Intersection.ModelCurveID)
	if e != nil {
		return out, e
	}
	var z0 *big.Rat
	H := new(big.Rat)
	for _, p := range poles {
		z := new(big.Rat).Sub(p[2], so[2])
		if z0 == nil || z.Cmp(z0) < 0 {
			z0 = z
		}
		h := new(big.Rat).Add(new(big.Rat).Abs(new(big.Rat).Sub(p[0], so[0])), new(big.Rat).Abs(new(big.Rat).Sub(p[1], so[1])))
		if h.Cmp(H) > 0 {
			H = h
		}
	}
	if z0 == nil || z0.Sign() <= 0 {
		return fail()
	}
	winding, e := BoundPolynomialSplineWinding(bytes.NewReader(raw), l, top.Intersection.ModelCurveID, cc, o)
	if e != nil {
		return out, e
	}
	if winding.Turns != 1 {
		return fail()
	}
	sr, e := BoundPolynomialSplineSphereResidual(bytes.NewReader(raw), l, top.Intersection.ModelCurveID, sc, rf)
	if e != nil {
		return out, e
	}
	cr, e := BoundPolynomialSplineCylinderResidual(bytes.NewReader(raw), l, top.Intersection.ModelCurveID, cc, geom.Vec3{Z: 1}, cf)
	if e != nil {
		return out, e
	}
	delta := new(big.Rat).Quo(cr.MaxAbsoluteResidual, rad)
	// F(projectXY(C),Cz) differs from F(C) by at most 2H delta+delta².
	numerator := new(big.Rat).Add(sr.MaxAbsoluteResidual, new(big.Rat).Mul(new(big.Rat).Mul(big.NewRat(2, 1), H), delta))
	numerator.Add(numerator, new(big.Rat).Mul(delta, delta))
	vertical := new(big.Rat).Quo(numerator, new(big.Rat).Add(z0, h0))
	bound := new(big.Rat).Add(delta, vertical)
	for _, v := range []*big.Rat{h0, z0, H, delta, numerator, vertical, bound} {
		if !sphereLiftBits(v) {
			return fail()
		}
	}
	return OffAxisRimCorrespondence{BodyID: bodyID, EdgeID: top.Intersection.EdgeID, ModelCurveID: top.Intersection.ModelCurveID, SphereID: sphereID, CylinderID: top.Intersection.CylinderID, Winding: winding, SphereResidual: sr.MaxAbsoluteResidual, CylinderResidual: cr.MaxAbsoluteResidual, SourceHeightLower: z0, TargetHeightLower: h0, HorizontalUpper: H, RadialErrorUpper: delta, VerticalErrorUpper: vertical, HausdorffUpper: bound, FullDomainCorrespondence: true, WithinTolerance: bound.Cmp(new(big.Rat).SetFloat64(tolerance)) <= 0}, nil
}

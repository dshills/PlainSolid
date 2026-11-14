package step

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"reflect"
)

// MeasureIndependentCircularFrustumHierarchy computes exact symbolic volume,
// centroid and inertia for all five local families and every original use.
// The result is an occurrence sum, not a regularized union. A noncircular
// rational frustum fails atomically even though its geometry remains supported.
func MeasureIndependentCircularFrustumHierarchy(ctx context.Context, m IndependentFrustumHierarchy, o RationalHierarchyMassOptions) (RationalHierarchyMass, error) {
	bad := func(e error) (RationalHierarchyMass, error) { return RationalHierarchyMass{}, e }
	if ctx == nil {
		return bad(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return bad(e)
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 65536
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxTriangles < 1 || o.MaxTriangles > 1048576 || o.MaxCoefficientBits < 256 || o.MaxCoefficientBits > 8192 {
		return bad(geom.ErrInvalid)
	}
	actual, c, e := frustumHierarchyEnvelope(ctx, m)
	if e != nil {
		return bad(e)
	}
	proof, e := ReadIndependentFrustumHierarchy(ctx, bytes.NewBufferString(actual.Proof.Source), o.Source, actual.Proof.Tolerance)
	if e != nil {
		return bad(e)
	}
	expected, d, e := frustumHierarchyEnvelope(ctx, proof)
	if e != nil {
		return bad(e)
	}
	if !reflect.DeepEqual(actual, expected) || !reflect.DeepEqual(c, d) {
		return bad(geom.ErrInvalid)
	}
	instances := make([]RationalHierarchyInstance, 0, len(proof.Instances))
	for _, u := range proof.Instances {
		b := mappedRationalBasis{}
		for i := 0; i < 3; i++ {
			b.Origin[i] = new(big.Rat).SetFloat64(u.Transform[i][3])
			b.X[i] = new(big.Rat).SetFloat64(u.Transform[i][0])
			b.Y[i] = new(big.Rat).SetFloat64(u.Transform[i][1])
			b.Z[i] = new(big.Rat).SetFloat64(u.Transform[i][2])
		}
		instances = append(instances, RationalHierarchyInstance{ProductID: u.ProductID, Path: append([]int64(nil), u.Path...), pose: b})
	}
	// Exact signed-loop planar integration avoids depending on triangulation of
	// concave or holed original planar faces in this additive query.
	return measureAuthenticatedHierarchyProducts(ctx, proof.Assembly.Source, proof.Products, instances, proof.Assembly.LengthUnit, o, 65536)
}

func integrateHierarchyCircularFrustum(p geom.WrappedFrustumEmbedding, a *massArithmetic) (SymbolicMaterialMoments, error) {
	var m SymbolicMaterialMoments
	if !p.Complete || !p.Circular {
		return m, geom.ErrDomain
	}
	for _, x := range []*big.Rat{p.CenterX, p.CenterY, p.LowerZ, p.UpperZ, p.LowerRadius, p.UpperRadius} {
		if x == nil {
			return m, geom.ErrInvalid
		}
		a.check(x)
	}
	h := a.check(new(big.Rat).Sub(p.UpperZ, p.LowerZ))
	if h.Sign() <= 0 || p.LowerRadius.Sign() <= 0 || p.UpperRadius.Sign() <= 0 {
		return m, geom.ErrInvalid
	}
	// t in [0,1], rho(t)=r0+(r1-r0)t. Section area is pi*rho²;
	// its x² and y² integrals are pi*rho⁴/4. Integrate these exact
	// polynomials before translating from the lower cap's axis origin.
	mul := func(x, y []*big.Rat) []*big.Rat {
		z := make([]*big.Rat, len(x)+len(y)-1)
		for i := range z {
			z[i] = new(big.Rat)
		}
		for i, u := range x {
			for j, v := range y {
				z[i+j] = a.check(new(big.Rat).Add(z[i+j], a.check(new(big.Rat).Mul(u, v))))
			}
		}
		return z
	}
	r := []*big.Rat{p.LowerRadius, a.check(new(big.Rat).Sub(p.UpperRadius, p.LowerRadius))}
	r2 := mul(r, r)
	r4 := mul(r2, r2)
	integral := func(poly []*big.Rat, power int) *big.Rat {
		v := new(big.Rat)
		for i, x := range poly {
			v = a.check(new(big.Rat).Add(v, a.check(new(big.Rat).Quo(x, big.NewRat(int64(i+power+1), 1)))))
		}
		return v
	}
	h2 := a.check(new(big.Rat).Mul(h, h))
	h3 := a.check(new(big.Rat).Mul(h2, h))
	m.v = a.scalar(a.check(new(big.Rat).Mul(h, integral(r2, 0))), 1)
	m.f[2] = a.scalar(a.check(new(big.Rat).Mul(h2, integral(r2, 1))), 1)
	m.q[2][2] = a.scalar(a.check(new(big.Rat).Mul(h3, integral(r2, 2))), 1)
	radial := a.check(new(big.Rat).Quo(a.check(new(big.Rat).Mul(h, integral(r4, 0))), big.NewRat(4, 1)))
	m.q[0][0] = a.scalar(radial, 1)
	m.q[1][1] = m.q[0][0]
	var pose [3][4]*big.Rat
	for i := 0; i < 3; i++ {
		for j := 0; j < 4; j++ {
			pose[i][j] = new(big.Rat)
		}
		pose[i][i].SetInt64(1)
	}
	pose[0][3] = p.CenterX
	pose[1][3] = p.CenterY
	pose[2][3] = p.LowerZ
	m = a.transform(m, pose)
	if a.err != nil {
		return SymbolicMaterialMoments{}, a.err
	}
	return m, nil
}

package geom

import (
	"context"
	"math/big"
)

type exactPlaneEquation struct{ coefficients [4]*big.Rat }

// RawSupportPlane retains exact rational coefficients of N·P+D=0. Coefficients
// are never unit-normalized or rounded. It is intentionally distinct from Plane:
// coefficient values and ranges are not signed Euclidean distances.
type RawSupportPlane struct {
	plane      Plane
	operations int
}

// Coefficients returns caller-owned N.x,N.y,N.z,D, or nil entries for zero value.
func (p RawSupportPlane) Coefficients() (r [4]*big.Rat) {
	if p.plane.raw == nil {
		return
	}
	for i, x := range p.plane.raw.coefficients {
		r[i] = new(big.Rat).Set(x)
	}
	return
}
func (p RawSupportPlane) ExactOperations() int { return p.operations }

func exactPlaneCoefficients(p Plane) (r [4]*big.Rat) {
	if p.raw != nil {
		for i, x := range p.raw.coefficients {
			r[i] = new(big.Rat).Set(x)
		}
		return
	}
	r[3] = new(big.Rat)
	for axis := 0; axis < 3; axis++ {
		r[axis] = new(big.Rat).SetFloat64(coordinate(p.normal, axis))
		r[3].Sub(r[3], new(big.Rat).Mul(r[axis], new(big.Rat).SetFloat64(coordinate(p.origin, axis))))
	}
	return
}
func exactPlaneHomogeneousValue(eq [4]*big.Rat, h exactHomogeneous) *big.Rat {
	v := new(big.Rat)
	for i := 0; i < 4; i++ {
		v.Add(v, new(big.Rat).Mul(eq[i], h[i]))
	}
	return v
}

func rawPlaneFromFrame(origin, u, v [3]*big.Rat, o SurfacePatchOptions) (r RawSupportPlane, err error) {
	options, e := patchOptions(o)
	if e != nil {
		return r, e
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { r.operations = budget.operations }()
	eq := [4]*big.Rat{}
	pivot := -1
	largest := new(big.Rat)
	eq[3] = new(big.Rat)
	for i := 0; i < 3; i++ {
		for _, x := range []*big.Rat{origin[i], u[i], v[i]} {
			if e := budget.check(x); e != nil {
				return r, e
			}
		}
		j, k := (i+1)%3, (i+2)%3
		eq[i] = new(big.Rat).Sub(new(big.Rat).Mul(u[j], v[k]), new(big.Rat).Mul(u[k], v[j]))
		if e := budget.check(eq[i]); e != nil {
			return r, e
		}
		m := new(big.Rat).Abs(eq[i])
		if m.Cmp(largest) > 0 {
			largest.Set(m)
			pivot = i
		}
		eq[3].Sub(eq[3], new(big.Rat).Mul(eq[i], origin[i]))
	}
	if pivot < 0 {
		return r, ErrDegenerate
	}
	if e := budget.check(eq[3]); e != nil {
		return r, e
	}
	// The private approximate normal is only a finite validity/display value.
	// Every raw trace certificate uses the retained equation instead.
	var xyz [3]float64
	for i := 0; i < 3; i++ {
		xyz[i], _ = new(big.Rat).Quo(eq[i], largest).Float64()
	}
	normal, e := (Vec3{xyz[0], xyz[1], xyz[2]}).Unit()
	if e != nil {
		return r, e
	}
	r.plane = Plane{normal: normal, raw: &exactPlaneEquation{eq}}
	return r, nil
}

// NewRawSupportPlane forms ((b-a)×(c-a))·(P-a)=0 exactly from represented inputs.
func NewRawSupportPlane(a, b, c Vec3, o SurfacePatchOptions) (RawSupportPlane, error) {
	if !a.IsFinite() || !b.IsFinite() || !c.IsFinite() {
		return RawSupportPlane{}, ErrInvalid
	}
	var origin, u, v [3]*big.Rat
	for i := 0; i < 3; i++ {
		origin[i] = new(big.Rat).SetFloat64(coordinate(a, i))
		u[i] = new(big.Rat).Sub(new(big.Rat).SetFloat64(coordinate(b, i)), origin[i])
		v[i] = new(big.Rat).Sub(new(big.Rat).SetFloat64(coordinate(c, i)), origin[i])
	}
	return rawPlaneFromFrame(origin, u, v, o)
}

// RawSupportPlane constructs the exact support of a proved affine chart, even
// when its corners or normal have no lossless float64 representation.
func (c *AffineSurfaceChart) RawSupportPlane(o SurfacePatchOptions) (RawSupportPlane, error) {
	if c == nil || c.determinant == nil {
		return RawSupportPlane{}, ErrInvalid
	}
	return rawPlaneFromFrame(c.origin, c.u, c.v, o)
}

func TraceSurfaceRawPlane(p *SurfacePatch, plane RawSupportPlane, o SurfaceTraceOptions) (SurfacePlaneTrace, error) {
	return TraceSurfaceRawPlaneContext(context.Background(), p, plane, o)
}

// TraceSurfaceRawPlaneContext retains exact coefficients throughout isolation,
// stitching, arc evaluation, endpoint recovery and later tube discovery.
func TraceSurfaceRawPlaneContext(ctx context.Context, p *SurfacePatch, plane RawSupportPlane, o SurfaceTraceOptions) (SurfacePlaneTrace, error) {
	if plane.plane.raw == nil {
		return SurfacePlaneTrace{}, ErrInvalid
	}
	return TraceSurfacePlaneContext(ctx, p, plane.plane, o)
}
func CertifySurfaceRawPlaneArcsContext(ctx context.Context, p *SurfacePatch, plane RawSupportPlane, o SurfacePlaneArcOptions) (SurfacePlaneArcs, error) {
	if plane.plane.raw == nil {
		return SurfacePlaneArcs{}, ErrInvalid
	}
	return CertifySurfacePlaneArcsContext(ctx, p, plane.plane, o)
}

// RawPlaneBounds encloses the raw functional N·P+D, not Euclidean distance.
func (p *SurfacePatch) RawPlaneBounds(plane RawSupportPlane) (SurfacePlaneRange, error) {
	if plane.plane.raw == nil {
		return SurfacePlaneRange{}, ErrInvalid
	}
	return p.PlaneBounds(plane.plane)
}

// CertifySurfaceRawPlaneArcs isolates arcs using exact raw support coefficients.
func CertifySurfaceRawPlaneArcs(p *SurfacePatch, plane RawSupportPlane, o SurfacePlaneArcOptions) (SurfacePlaneArcs, error) {
	return CertifySurfaceRawPlaneArcsContext(context.Background(), p, plane, o)
}

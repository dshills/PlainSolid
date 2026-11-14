package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// exactPlanarShellConstraint recognizes a plane independently of the speed of
// its stored chart. Corners propose only: the whole positive homogeneous net
// must prove coincidence. A rounded corner is never used as a plane witness.
func exactPlanarShellConstraint(ctx context.Context, p *geom.SurfacePatch, o geom.SurfaceCurveAgreementOptions) (plane [4]*big.Rat, err error) {
	if ctx == nil || p == nil {
		return plane, geom.ErrInvalid
	}
	work, bits := o.MaxOperations, o.MaxCoefficientBits
	if work == 0 {
		work = 1000000
	}
	if bits == 0 {
		bits = 16384
	}
	if work < 1 || bits < 64 {
		return plane, geom.ErrInvalid
	}
	d := p.Domain()
	var points [3]geom.Vec3
	var exact [3][3]*big.Rat
	for i, uv := range [3][2]float64{{d.UMin, d.VMin}, {d.UMax, d.VMin}, {d.UMin, d.VMax}} {
		if work < 1 {
			return plane, geom.ErrConvergence
		}
		jet, e := p.EvaluateRational(ctx, new(big.Rat).SetFloat64(uv[0]), new(big.Rat).SetFloat64(uv[1]), geom.SurfacePatchEvaluationOptions{MaxOperations: work, MaxCoefficientBits: bits})
		if e != nil {
			return plane, e
		}
		work -= jet.ExactOperations
		exact[i] = jet.Point
		var xyz [3]float64
		for j := range xyz {
			xyz[j], e = geom.RepresentedExactScalar(jet.Point[j])
			if e != nil {
				return plane, geom.ErrConvergence
			}
		}
		points[i] = geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
	}
	if work < 1 {
		return plane, geom.ErrConvergence
	}
	proof, e := geom.CertifySurfaceSupportingPlane(ctx, p, points[0], points[1], points[2], geom.SurfaceSupportingPlaneOptions{MaxOperations: work, MaxCoefficientBits: bits})
	if e != nil {
		return plane, e
	}
	work -= proof.ExactOperations
	if !proof.Complete || !proof.Coincident {
		return plane, geom.ErrConvergence
	}
	var u, v [3]*big.Rat
	for i := range u {
		u[i] = new(big.Rat).Sub(exact[1][i], exact[0][i])
		v[i] = new(big.Rat).Sub(exact[2][i], exact[0][i])
	}
	plane[3] = new(big.Rat)
	for i := range u {
		j, k := (i+1)%3, (i+2)%3
		plane[i] = new(big.Rat).Sub(new(big.Rat).Mul(u[j], v[k]), new(big.Rat).Mul(u[k], v[j]))
		plane[3].Sub(plane[3], new(big.Rat).Mul(plane[i], exact[0][i]))
		work -= 6
		if work < 0 || plane[i].Num().BitLen() > bits || plane[i].Denom().BitLen() > bits || plane[3].Num().BitLen() > bits || plane[3].Denom().BitLen() > bits {
			return [4]*big.Rat{}, geom.ErrConvergence
		}
	}
	return plane, nil
}

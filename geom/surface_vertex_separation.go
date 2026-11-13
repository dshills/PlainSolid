package geom

import (
	"context"
	"errors"
	"math/big"
)

// SurfaceVertexSeparationOptions limits total work across candidate planes.
// Zero limits select 26 candidates, depth24, 16384 cells, 8M operations and
// 16384 coefficient bits. The finite search uses small integer normal directions.
type SurfaceVertexSeparationOptions struct{ MaxCandidates, MaxDepth, MaxCells, MaxOperations, MaxCoefficientBits int }

// SurfaceVertexSeparationCertificate proves that the full patch images intersect
// exactly at Point, with all other points strictly in opposite halfspaces of the
// raw determinant plane PlanePoints. No injectivity assumption is needed for this
// image intersection statement. Any error leaves Complete and Separated false.
type SurfaceVertexSeparationCertificate struct {
	Complete, Separated                       bool
	Point                                     Vec3
	PlanePoints                               [3]Vec3
	A, B                                      SurfaceSupportingPlaneCertificate
	Candidates, VisitedCells, ExactOperations int
}

// CertifySurfaceVertexSeparation requires equal exactly representable corners.
// Corner indices 0..3 are (UMin,VMin),(UMax,VMin),(UMax,VMax),(UMin,VMax).
// Candidate generation is heuristic; acceptance proves exactly one-point contact.
// Extra edge/corner contacts, no found plane or nonrepresentable points remain
// ErrConvergence. This is not a complete vertex-contact decision procedure.
func CertifySurfaceVertexSeparation(ctx context.Context, a, b *SurfacePatch, cornerA, cornerB int, o SurfaceVertexSeparationOptions) (r SurfaceVertexSeparationCertificate, err error) {
	if ctx == nil || a == nil || b == nil || len(a.net) == 0 || len(b.net) == 0 || cornerA < 0 || cornerA > 3 || cornerB < 0 || cornerB > 3 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxCandidates == 0 {
		o.MaxCandidates = 26
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 24
	}
	if o.MaxCells == 0 {
		o.MaxCells = 16384
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCandidates < 1 || o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxCells < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	exact := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations += exact.operations }()
	corner := func(p *SurfacePatch, index int) ([3]*big.Rat, error) {
		indices := [4]int{0, (p.countU - 1) * p.countV, len(p.net) - 1, p.countV - 1}
		h := p.net[indices[index]]
		var point [3]*big.Rat
		for axis := 0; axis < 3; axis++ {
			point[axis] = new(big.Rat).Quo(h[axis], h[3])
			if e := exact.check(point[axis]); e != nil {
				return point, e
			}
		}
		return point, nil
	}
	ap, e := corner(a, cornerA)
	if e != nil {
		return r, e
	}
	bp, e := corner(b, cornerB)
	if e != nil {
		return r, e
	}
	var coordinates [3]float64
	for axis := 0; axis < 3; axis++ {
		if ap[axis].Cmp(bp[axis]) != 0 {
			return r, ErrConvergence
		}
		f, e := exactRepresentableFloat(ap[axis])
		if e != nil {
			return r, ErrConvergence
		}
		coordinates[axis] = f
	}
	shared := Vec3{coordinates[0], coordinates[1], coordinates[2]}
	cross := func(a, b [3]int) [3]int {
		return [3]int{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
	}
	shifted := func(direction [3]int, scale *big.Rat) (Vec3, error) {
		var values [3]float64
		for axis := 0; axis < 3; axis++ {
			x := new(big.Rat).Add(ap[axis], new(big.Rat).Mul(big.NewRat(int64(direction[axis]), 1), scale))
			if e := exact.check(x); e != nil {
				return Vec3{}, e
			}
			f, e := exactRepresentableFloat(x)
			if e != nil {
				return Vec3{}, e
			}
			values[axis] = f
		}
		return Vec3{values[0], values[1], values[2]}, nil
	}
	for nx := -1; nx <= 1; nx++ {
		for ny := -1; ny <= 1; ny++ {
			for nz := -1; nz <= 1; nz++ {
				normal := [3]int{nx, ny, nz}
				first := 0
				for _, v := range normal {
					if v != 0 {
						first = v
						break
					}
				}
				if first <= 0 {
					continue
				}
				basis := [3]int{1, 0, 0}
				if ny == 0 && nz == 0 {
					basis = [3]int{0, 1, 0}
				}
				tangent := cross(normal, basis)
				other := cross(normal, tangent)
				for _, scale := range []*big.Rat{big.NewRat(1, 1), big.NewRat(1, 2)} {
					if e := ctx.Err(); e != nil {
						return r, e
					}
					// Share the operation allowance between candidate arithmetic and proofs.
					exact.maxOperations = o.MaxOperations - r.ExactOperations
					p, e := shifted(tangent, scale)
					if e != nil {
						if errors.Is(e, ErrUnrepresentable) {
							continue
						}
						return r, e
					}
					q, e := shifted(other, scale)
					if e != nil {
						if errors.Is(e, ErrUnrepresentable) {
							continue
						}
						return r, e
					}
					if r.Candidates >= o.MaxCandidates {
						return r, ErrConvergence
					}
					r.Candidates++
					plane := [3]Vec3{shared, p, q}
					certify := func(patch *SurfacePatch) (SurfaceSupportingPlaneCertificate, error) {
						operations, cells := o.MaxOperations-r.ExactOperations-exact.operations, o.MaxCells-r.VisitedCells
						if operations < 1 || cells < 1 {
							return SurfaceSupportingPlaneCertificate{}, ErrConvergence
						}
						cert, e := CertifySurfaceSupportingPlane(ctx, patch, plane[0], plane[1], plane[2], SurfaceSupportingPlaneOptions{MaxDepth: o.MaxDepth, MaxCells: cells, MaxOperations: operations, MaxCoefficientBits: o.MaxCoefficientBits})
						r.ExactOperations += cert.ExactOperations
						r.VisitedCells += cert.VisitedCells
						return cert, e
					}
					ac, e := certify(a)
					if e != nil {
						if errors.Is(e, ErrConvergence) || errors.Is(e, ErrDegenerate) {
							continue
						}
						return r, e
					}
					if ac.Coincident || ac.SideMask != 0 || ac.CornerMask != 1<<cornerA {
						continue
					}
					bc, e := certify(b)
					if e != nil {
						if errors.Is(e, ErrConvergence) || errors.Is(e, ErrDegenerate) {
							continue
						}
						return r, e
					}
					if bc.Coincident || bc.SideMask != 0 || bc.CornerMask != 1<<cornerB || ac.HalfspaceSign != -bc.HalfspaceSign {
						continue
					}
					r.Complete, r.Separated = true, true
					r.Point = shared
					r.PlanePoints = plane
					r.A, r.B = ac, bc
					return r, nil
				}
			}
		}
	}
	return r, ErrConvergence
}

package geom

import (
	"context"
	"math/big"
	"runtime"
)

// AffineRadialContours retains exact original charts and a complete radial
// identity on their common positive physical rectangle. Original projected rectangles
// may differ. Only signed-axis affine maps and single Bezier spans are accepted.
// Rational clip values are never rounded into surface controls.
type AffineRadialContours struct {
	original   [2]*SurfacePatch
	canonical  [2]*SurfacePatch
	inverse    [2][2][3]*big.Rat
	radial     *SplineGraphRadialContours
	operations int
	physical   [2][2]*big.Rat
}

// CanonicalSupports uses unit UV but retains the original physical XYZ values.
// Unequal rectangle widths therefore remain unequal model projection scales.
func (r *AffineRadialContours) CanonicalSupports() [2]*SurfacePatch {
	if r == nil {
		return [2]*SurfacePatch{}
	}
	return r.canonical
}
func (r *AffineRadialContours) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.operations
}

// OriginalBox maps a rectangle expressed in physical XY to original owner UV.
// It returns an outward proposal only. Its subsequent uniform proof
// and exact original endpoint membership are mandatory for coverage.
func (r *AffineRadialContours) OriginalBox(ctx context.Context, owner int, box ParameterRectangle, o SurfacePatchOptions) (ParameterRectangle, bool, int, error) {
	if r == nil || r.radial == nil || owner < 0 || owner > 1 {
		return ParameterRectangle{}, false, 0, ErrInvalid
	}
	a := &AffineXYChartAlignment{target: r.canonical[owner], mapping: r.inverse[owner]}
	return a.MapSourceBox(ctx, box, o)
}
func (r *AffineRadialContours) OriginalAxis(owner, physicalAxis int) (int, error) {
	if r == nil || r.radial == nil || owner < 0 || owner > 1 || physicalAxis < 0 || physicalAxis > 1 {
		return 0, ErrInvalid
	}
	for i := 0; i < 2; i++ {
		if r.inverse[owner][i][physicalAxis].Sign() != 0 {
			return i, nil
		}
	}
	return 0, ErrInvalid
}

// CertifyTubeCoverage verifies original chart endpoints and original supports.
// Signed-axis affine free coordinates are monotone physical X or Y; the
// open/closed circle-cover argument therefore survives independent chart scales.
func (r *AffineRadialContours) CertifyTubeCoverage(ctx context.Context, tubes []VerifiedSurfaceTube, o RadialTubeCoverageOptions) (*RadialTubeCoverage, error) {
	if r == nil || r.radial == nil {
		return nil, ErrInvalid
	}
	return r.radial.certifyTubeCoverageOriginal(ctx, tubes, o, r.original, false)
}

// CertifyAffineRadialGraphContours computes the exact positive rectangular
// intersection of the original projected domains, then proves the complete
// physical XY circle family there. Each circle must lie strictly inside.
// Proof UV is normalized independently on each axis: the polynomial factors
// retain both physical scales, never treating a normalized ellipse as a circle.
// Outside the projected intersection no common model point can exist.
func CertifyAffineRadialGraphContours(ctx context.Context, a, c *SurfacePatch, center [2]*big.Rat, radii []*big.Rat, o GraphPointContactOptions) (*AffineRadialContours, error) {
	if ctx == nil || a == nil || c == nil {
		return nil, ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 64000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 16384 {
		return nil, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	r := &AffineRadialContours{original: [2]*SurfacePatch{a, c}}
	var ranges [2][2][2]*big.Rat
	for owner, p := range r.original {
		if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 || p.degreeU > 8 || p.degreeV > 8 {
			return nil, ErrConvergence
		}
		_, inv, e := normalizeAffineXYGraph(ctx, p, b)
		if e != nil {
			return nil, e
		}
		r.inverse[owner] = inv
		d := p.Domain()
		uv := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
		for i, row := range inv {
			axis := -1
			for j := 0; j < 2; j++ {
				if row[j].Sign() != 0 {
					if axis >= 0 {
						return nil, ErrConvergence
					}
					axis = j
				}
			}
			if axis < 0 {
				return nil, ErrDegenerate
			}
			lo := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(uv[i][0]), row[2]), row[axis])
			hi := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(uv[i][1]), row[2]), row[axis])
			runtime.KeepAlive(&row) // golang/go#82026: keeps row in the stack map. Remove once the Go fix is in use.
			if lo.Cmp(hi) > 0 {
				lo, hi = hi, lo
			}
			if e = b.check(lo); e != nil {
				return nil, e
			}
			if e = b.check(hi); e != nil {
				return nil, e
			}
			ranges[owner][axis] = [2]*big.Rat{lo, hi}
		}
	}
	for axis := 0; axis < 2; axis++ {
		lo, hi := ranges[0][axis][0], ranges[0][axis][1]
		if ranges[1][axis][0].Cmp(lo) > 0 {
			lo = ranges[1][axis][0]
		}
		if ranges[1][axis][1].Cmp(hi) < 0 {
			hi = ranges[1][axis][1]
		}
		if lo.Cmp(hi) >= 0 {
			return nil, ErrDomain
		}
		r.physical[axis] = [2]*big.Rat{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}
	}
	for owner, p := range r.original {
		d := p.Domain()
		uv := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
		var local [2][2]*big.Rat
		var axes [2]int
		var reverse [2]bool
		for i, row := range r.inverse[owner] {
			axis := 0
			if row[1].Sign() != 0 {
				axis = 1
			}
			axes[i] = axis
			reverse[i] = row[axis].Sign() < 0
			lo := new(big.Rat).Add(row[2], new(big.Rat).Mul(row[axis], r.physical[axis][0]))
			hi := new(big.Rat).Add(row[2], new(big.Rat).Mul(row[axis], r.physical[axis][1]))
			if lo.Cmp(hi) > 0 {
				lo, hi = hi, lo
			}
			width := new(big.Rat).Sub(new(big.Rat).SetFloat64(uv[i][1]), new(big.Rat).SetFloat64(uv[i][0]))
			local[i] = [2]*big.Rat{new(big.Rat).Quo(new(big.Rat).Sub(lo, new(big.Rat).SetFloat64(uv[i][0])), width), new(big.Rat).Quo(new(big.Rat).Sub(hi, new(big.Rat).SetFloat64(uv[i][0])), width)}
			for _, q := range local[i] {
				if e := b.check(q); e != nil {
					return nil, e
				}
				if q.Sign() < 0 || q.Cmp(big.NewRat(1, 1)) > 0 {
					return nil, ErrDomain
				}
			}
		}
		net, e := restrictComparisonTensor(comparisonTensor{net: p.net, nu: p.countU, nv: p.countV}, local[0][0], local[0][1], local[1][0], local[1][1], b)
		if e != nil {
			return nil, e
		}
		counts := [2]int{p.countU, p.countV}
		var nc [2]int
		for i, axis := range axes {
			nc[axis] = counts[i]
		}
		controls := make([][4]*big.Rat, len(net))
		for u := 0; u < nc[0]; u++ {
			for v := 0; v < nc[1]; v++ {
				ij := [2]int{u, v}
				var old [2]int
				for i, axis := range axes {
					old[i] = ij[axis]
					if reverse[i] {
						old[i] = counts[i] - 1 - old[i]
					}
				}
				controls[u*nc[1]+v] = [4]*big.Rat(net[old[0]*p.countV+old[1]])
			}
		}
		remaining := b.maxOperations - b.operations
		if remaining < 1 {
			return nil, ErrConvergence
		}
		canonical, e := NewExactHomogeneousBezierSurface(ctx, nc[0]-1, nc[1]-1, controls, SurfacePatchOptions{MaxOperations: remaining, MaxCoefficientBits: b.maxBits})
		if e != nil {
			return nil, e
		}
		b.operations += canonical.operations
		r.canonical[owner] = canonical
	}
	remaining := b.maxOperations - b.operations
	if remaining < 1 {
		return nil, ErrConvergence
	}
	proof, e := certifyRadialContoursOnRectangle(ctx, r.canonical[0], r.canonical[1], center, radii, GraphPointContactOptions{MaxOperations: remaining, MaxCoefficientBits: b.maxBits}, r.physical)
	if e != nil {
		return nil, e
	}
	b.operations += proof.ExactOperations()
	r.radial = proof
	r.operations = b.operations
	return r, nil
}

// PhysicalRectangle returns owned exact bounds in model XY, not normalized UV.
func (r *AffineRadialContours) PhysicalRectangle() [2][2]*big.Rat {
	var out [2][2]*big.Rat
	if r == nil || r.radial == nil {
		return out
	}
	for i := range out {
		for j := range out[i] {
			out[i][j] = new(big.Rat).Set(r.physical[i][j])
		}
	}
	return out
}
func (r *AffineRadialContours) Center() [2]*big.Rat {
	if r == nil {
		return [2]*big.Rat{}
	}
	return r.radial.Center()
}
func (r *AffineRadialContours) Radii() []*big.Rat {
	if r == nil {
		return nil
	}
	return r.radial.Radii()
}

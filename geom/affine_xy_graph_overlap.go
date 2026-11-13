package geom

import (
	"context"
	"math/big"
)

// AffineXYGraphOverlap proves identity throughout the complete positive-area
// projected overlap, with original rational chart ranges [axis][lower/upper].
// Map maps A UV to B UV. Orientation is its determinant sign. NoArea says only
// that no finite-area projected overlap exists; boundary contact is not excluded.
// Nonidentity is unresolved: this API never infers separation from inequality.
type AffineXYGraphOverlap struct {
	Coincident, NoArea             bool
	A, B                           [2][2]*big.Rat
	Map                            [2][3]*big.Rat
	Orientation                    int
	ProjectionOrientation          [2]int // signs of original projected UV Jacobians
	ComparedCells, ExactOperations int
}

// CertifyAffineXYGraphOverlap supports partial rectangular overlap under exact
// signed-axis affine correspondence. Overlap endpoints and union knot cuts
// remain exact rationals throughout restriction and coefficient comparison. Different degrees
// and positive weight fields are compared by homogeneous polynomial identities.
func CertifyAffineXYGraphOverlap(ctx context.Context, source, target *SurfacePatch, o SurfacePatchOptions) (out AffineXYGraphOverlap, err error) {
	if ctx == nil || source == nil || target == nil {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	options, e := patchOptions(o)
	if e != nil {
		return out, e
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { out.ExactOperations = b.operations }()
	var inverse [2][2][3]*big.Rat
	for i, p := range []*SurfacePatch{source, target} {
		_, inverse[i], e = normalizeAffineXYGraph(ctx, p, b)
		if e != nil {
			return out, e
		}
	}

	for i, inv := range inverse {
		determinant := new(big.Rat).Sub(new(big.Rat).Mul(inv[0][0], inv[1][1]), new(big.Rat).Mul(inv[0][1], inv[1][0]))
		if e := b.check(determinant); e != nil {
			return out, e
		}
		out.ProjectionOrientation[i] = determinant.Sign()
	}
	// Invert source inverse XY map, then compose with target inverse.
	a := inverse[0]
	det := new(big.Rat).Sub(new(big.Rat).Mul(a[0][0], a[1][1]), new(big.Rat).Mul(a[0][1], a[1][0]))
	forward := [2][3]*big.Rat{{new(big.Rat).Quo(a[1][1], det), new(big.Rat).Quo(new(big.Rat).Neg(a[0][1]), det), nil}, {new(big.Rat).Quo(new(big.Rat).Neg(a[1][0]), det), new(big.Rat).Quo(a[0][0], det), nil}}
	for i := 0; i < 2; i++ {
		forward[i][2] = new(big.Rat).Neg(new(big.Rat).Add(new(big.Rat).Mul(forward[i][0], a[0][2]), new(big.Rat).Mul(forward[i][1], a[1][2])))
	}
	result := &AffineXYGraphOverlap{}
	var axes [2]int
	for i := 0; i < 2; i++ {
		for j := 0; j < 3; j++ {
			q := new(big.Rat)
			for k := 0; k < 2; k++ {
				q.Add(q, new(big.Rat).Mul(inverse[1][i][k], forward[k][j]))
			}
			if j == 2 {
				q.Add(q, inverse[1][i][2])
			}
			if e = b.check(q); e != nil {
				return out, e
			}
			result.Map[i][j] = q
		}
		axes[i] = -1
		for j := 0; j < 2; j++ {
			if result.Map[i][j].Sign() != 0 {
				if axes[i] >= 0 {
					return out, ErrConvergence
				}
				axes[i] = j
			}
		}
		if axes[i] < 0 {
			return out, ErrDegenerate
		}
	}
	if axes[0] == axes[1] {
		return out, ErrDegenerate
	}

	sd, td := source.Domain(), target.Domain()
	sr := [2][2]float64{{sd.UMin, sd.UMax}, {sd.VMin, sd.VMax}}
	tr := [2][2]float64{{td.UMin, td.UMax}, {td.VMin, td.VMax}}
	out.Map = result.Map
	determinant := new(big.Rat).Sub(new(big.Rat).Mul(out.Map[0][0], out.Map[1][1]), new(big.Rat).Mul(out.Map[0][1], out.Map[1][0]))
	out.Orientation = determinant.Sign()
	for i, j := range axes {
		low := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(tr[i][0]), out.Map[i][2]), out.Map[i][j])
		high := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(tr[i][1]), out.Map[i][2]), out.Map[i][j])
		if low.Cmp(high) > 0 {
			low, high = high, low
		}
		sl, sh := new(big.Rat).SetFloat64(sr[j][0]), new(big.Rat).SetFloat64(sr[j][1])
		if low.Cmp(sl) < 0 {
			low = sl
		}
		if high.Cmp(sh) > 0 {
			high = sh
		}
		for _, q := range []*big.Rat{low, high} {
			if e := b.check(q); e != nil {
				return out, e
			}
		}
		if low.Cmp(high) >= 0 {
			out.NoArea = true
			return out, nil
		}
		out.A[j] = [2]*big.Rat{low, high}
		bl := new(big.Rat).Add(new(big.Rat).Mul(out.Map[i][j], low), out.Map[i][2])
		bh := new(big.Rat).Add(new(big.Rat).Mul(out.Map[i][j], high), out.Map[i][2])
		if bl.Cmp(bh) > 0 {
			bl, bh = bh, bl
		}
		out.B[i] = [2]*big.Rat{bl, bh}
	}
	remaining := options.MaxOperations - b.operations
	if remaining < 1 {
		return out, ErrConvergence
	}
	swap := axes[0] == 1
	reverse := [2]bool{}
	for i, j := range axes {
		reverse[j] = out.Map[i][j].Sign() < 0
	}
	domains := [2][2][2]*big.Rat{out.A, out.B}
	proof, e := compareExactSurfaceRationalDomains(ctx, source, target, ExactSurfaceComparisonOptions{NormalizeDomains: true, SwapB: swap, ReverseBU: reverse[0], ReverseBV: reverse[1], MaxOperations: remaining, MaxCoefficientBits: options.MaxCoefficientBits}, &domains)
	b.operations += proof.ExactOperations
	out.ComparedCells = proof.ComparedCells
	if e != nil {
		return out, e
	}
	if !proof.Complete || !proof.Equal {
		return out, ErrConvergence
	}
	out.Coincident = true
	return out, nil
}

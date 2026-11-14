package geom

import (
	"context"
	"math/big"
)

// OriginalOrientation returns the sign of the original owner chart's physical
// XY Jacobian. It is zero for an invalid certificate or owner.
func (r *AffineRadialContours) OriginalOrientation(owner int) int {
	if r == nil || r.radial == nil || owner < 0 || owner > 1 {
		return 0
	}
	m := r.inverse[owner]
	if m[0][0].Sign() != 0 {
		return m[0][0].Sign() * m[1][1].Sign()
	}
	return -m[0][1].Sign() * m[1][0].Sign()
}

// PhysicalBox maps a closed original UV rectangle to its exact physical XY
// rectangle, using the certificate's authenticated signed-axis affine map.
// It returns owned rational values; no represented/outward rounding is used.
func (r *AffineRadialContours) PhysicalBox(ctx context.Context, owner int, d ParameterRectangle, o SurfacePatchOptions) (out [2][2]*big.Rat, work int, err error) {
	if ctx == nil || r == nil || r.radial == nil || owner < 0 || owner > 1 {
		return out, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, 0, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 16384 {
		return out, 0, ErrInvalid
	}
	full := r.original[owner].Domain()
	if !d.Valid() || d.UMin < full.UMin || d.UMax > full.UMax || d.VMin < full.VMin || d.VMax > full.VMax {
		return out, 0, ErrDomain
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		work = b.operations
		if err != nil {
			out = [2][2]*big.Rat{}
		}
	}()
	uv := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	for i, row := range r.inverse[owner] {
		axis := 0
		if row[1].Sign() != 0 {
			axis = 1
		}
		for _, q := range row {
			if e := b.check(q); e != nil {
				return out, 0, e
			}
		}
		for end := range 2 {
			q := new(big.Rat).SetFloat64(uv[i][end])
			if e := b.check(q); e != nil {
				return out, 0, e
			}
			q.Sub(q, row[2])
			if e := b.check(q); e != nil {
				return out, 0, e
			}
			q.Quo(q, row[axis])
			if e := b.check(q); e != nil {
				return out, 0, e
			}
			out[axis][end] = q
		}
		if out[axis][0].Cmp(out[axis][1]) > 0 {
			out[axis][0], out[axis][1] = out[axis][1], out[axis][0]
		}
	}
	return out, b.operations, nil
}

// BandSigns retains the ordered original support height-difference signs.
func (r *AffineRadialContours) BandSigns() []int {
	if r == nil || r.radial == nil {
		return nil
	}
	return r.radial.BandSigns()
}

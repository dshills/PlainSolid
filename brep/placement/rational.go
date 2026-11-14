package placement

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// RationalRigid is an immutable proper rigid motion in exact model units.
// Its zero value is invalid. Coordinates and matrix rows cannot be mutated
// through constructor inputs or returned results.
type RationalRigid struct{ matrix [3][4]*big.Rat }

const rigidDefaultBits = 16384

func checkRigidRat(q *big.Rat, bits int) error {
	if q == nil {
		return geom.ErrInvalid
	}
	if q.Num().BitLen() > bits || q.Denom().BitLen() > bits {
		return geom.ErrConvergence
	}
	return nil
}

func rigidBits(bits int) (int, error) {
	if bits == 0 {
		return rigidDefaultBits, nil
	}
	if bits < 64 || bits > 65536 {
		return 0, geom.ErrInvalid
	}
	return bits, nil
}

// NewRationalRigid checks exact orthonormality and proper orientation of the
// linear part. Each row contains three direction coefficients and a translation.
func NewRationalRigid(matrix [3][4]*big.Rat, maxBits int) (out RationalRigid, err error) {
	bits, e := rigidBits(maxBits)
	if e != nil {
		return out, e
	}
	for _, row := range matrix {
		for _, q := range row {
			if e := checkRigidRat(q, bits); e != nil {
				return out, e
			}
		}
	}
	for i := range matrix {
		for j := 0; j <= i; j++ {
			dot := new(big.Rat)
			for k := 0; k < 3; k++ {
				x := new(big.Rat).Mul(matrix[i][k], matrix[j][k])
				if e := checkRigidRat(x, bits); e != nil {
					return out, e
				}
				dot.Add(dot, x)
				if e := checkRigidRat(dot, bits); e != nil {
					return out, e
				}
			}
			want := int64(0)
			if i == j {
				want = 1
			}
			if dot.Cmp(big.NewRat(want, 1)) != 0 {
				return out, ErrInvalid
			}
		}
		j, k := (i+1)%3, (i+2)%3
		x := new(big.Rat).Sub(new(big.Rat).Mul(matrix[0][j], matrix[1][k]), new(big.Rat).Mul(matrix[0][k], matrix[1][j]))
		if e := checkRigidRat(x, bits); e != nil {
			return out, e
		}
		if x.Cmp(matrix[2][i]) != 0 {
			return out, ErrInvalid
		}
	}
	for i, row := range matrix {
		for j, q := range row {
			out.matrix[i][j] = new(big.Rat).Set(q)
		}
	}
	return out, nil
}

// Matrix returns an independent rational copy of the proper pose.
func (r RationalRigid) Matrix() (out [3][4]*big.Rat, err error) {
	if r.matrix[0][0] == nil {
		return out, ErrInvalid
	}
	for i, row := range r.matrix {
		for j, q := range row {
			out[i][j] = new(big.Rat).Set(q)
		}
	}
	return out, nil
}

// TransformPoint applies the original pose without float conversion. Set
// inverse to pull a world point back into the original model frame.
func (r RationalRigid) TransformPoint(ctx context.Context, p [3]*big.Rat, inverse bool, maxBits int) (out [3]*big.Rat, err error) {
	return r.transform(ctx, p, inverse, false, maxBits)
}

// TransformVector changes only the direction frame; no translation is used.
func (r RationalRigid) TransformVector(ctx context.Context, p [3]*big.Rat, inverse bool, maxBits int) (out [3]*big.Rat, err error) {
	return r.transform(ctx, p, inverse, true, maxBits)
}

func (r RationalRigid) transform(ctx context.Context, p [3]*big.Rat, inverse, vector bool, maxBits int) (out [3]*big.Rat, err error) {
	defer func() {
		if err != nil {
			out = [3]*big.Rat{}
		}
	}()
	if ctx == nil || r.matrix[0][0] == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	bits, e := rigidBits(maxBits)
	if e != nil {
		return out, e
	}
	for _, row := range r.matrix {
		for _, q := range row {
			if e := checkRigidRat(q, bits); e != nil {
				return out, e
			}
		}
	}
	for i, q := range p {
		if e := checkRigidRat(q, bits); e != nil {
			return out, e
		}
		if inverse && !vector {
			p[i] = new(big.Rat).Sub(q, r.matrix[i][3])
			if e := checkRigidRat(p[i], bits); e != nil {
				return out, e
			}
		}
	}
	for i := range out {
		out[i] = new(big.Rat)
		if !inverse && !vector {
			out[i].Set(r.matrix[i][3])
		}
		for j, q := range p {
			v := r.matrix[i][j]
			if inverse {
				v = r.matrix[j][i]
			}
			x := new(big.Rat).Mul(q, v)
			if e := checkRigidRat(x, bits); e != nil {
				return out, e
			}
			out[i].Add(out[i], x)
			if e := checkRigidRat(out[i], bits); e != nil {
				return out, e
			}
		}
	}
	return out, ctx.Err()
}

// Package placement provides lossless planar component placement in signed
// coordinate frames. Arbitrary-angle and curved evaluator transforms remain
// available through the underlying geometry APIs, outside this certificate.
package placement

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
)

var ErrInvalid = errors.New("placement: invalid proper signed-coordinate frame")
var ErrLimit = errors.New("placement: vertex budget exceeded")

type Rigid struct {
	matrix geom.Affine
	valid  bool
}

// New defines output coordinates by signed input axes ±1,±2,±3 followed by
// translation. Each axis occurs once and determinant must be +1 (no reflection).
func New(axes [3]int, translation geom.Vec3) (Rigid, error) {
	if !translation.IsFinite() {
		return Rigid{}, ErrInvalid
	}
	var m geom.Affine
	seen := [3]bool{}
	for i, a := range axes {
		sign := 1.
		if a < 0 {
			sign = -1
			a = -a
		}
		if a < 1 || a > 3 || seen[a-1] {
			return Rigid{}, ErrInvalid
		}
		seen[a-1] = true
		m[i][a-1] = sign
	}
	m[0][3], m[1][3], m[2][3] = translation.X, translation.Y, translation.Z
	if m.Determinant() != 1 {
		return Rigid{}, ErrInvalid
	}
	return Rigid{m, true}, nil
}
func (r Rigid) Matrix() (geom.Affine, error) {
	if !r.valid {
		return geom.Affine{}, ErrInvalid
	}
	return r.matrix, nil
}

// Inverse retains exact signed axes. Its translation is only a signed
// permutation, so no arithmetic rounding occurs.
func (r Rigid) Inverse() (Rigid, error) {
	if !r.valid {
		return Rigid{}, ErrInvalid
	}
	var m geom.Affine
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			m[i][j] = r.matrix[j][i]
			if m[i][j] != 0 {
				m[i][3] = -m[i][j] * r.matrix[j][3]
			}
		}
	}
	return Rigid{m, true}, nil
}

// ApplyPlanar returns a solid with identical edge and face IDs only when every
// translated coordinate is exactly representable in binary64. Otherwise no
// rounded or collapsed component is returned. maxVertices defaults to100000.
func (r Rigid) ApplyPlanar(ctx context.Context, s brep.Solid, maxVertices int) (brep.Solid, error) {
	if ctx == nil || !r.valid || maxVertices < 0 {
		return brep.Solid{}, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return brep.Solid{}, e
	}
	if maxVertices == 0 {
		maxVertices = 100000
	}
	v := s.Vertices()
	if len(v) > maxVertices {
		return brep.Solid{}, ErrLimit
	}
	if len(v) == 0 {
		return brep.Solid{}, ErrInvalid
	}
	for i, p := range v {
		if e := ctx.Err(); e != nil {
			return brep.Solid{}, e
		}
		in := [3]float64{p.X, p.Y, p.Z}
		var out [3]float64
		for j := 0; j < 3; j++ {
			q := new(big.Rat).SetFloat64(r.matrix[j][3])
			for k := 0; k < 3; k++ {
				if r.matrix[j][k] != 0 {
					q.Add(q, new(big.Rat).Mul(new(big.Rat).SetFloat64(in[k]), big.NewRat(int64(r.matrix[j][k]), 1)))
				}
			}
			x, e := geom.RepresentedExactScalar(q)
			if e != nil || math.IsInf(x, 0) {
				if e != nil {
					return brep.Solid{}, e
				}
				return brep.Solid{}, ErrInvalid
			}
			out[j] = x
		}
		v[i] = geom.Vec3{X: out[0], Y: out[1], Z: out[2]}
	}
	return brep.NewSolid(v, s.Edges(), s.Faces())
}

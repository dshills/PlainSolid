package geom

import "math"

// Affine stores the first three rows of a homogeneous affine transform.
// The implicit fourth row is [0,0,0,1]. The zero value collapses all points.
type Affine [3][4]float64

func Identity() Affine { return Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}} }
func Translation(v Vec3) Affine {
	m := Identity()
	m[0][3] = v.X
	m[1][3] = v.Y
	m[2][3] = v.Z
	return m
}
func Scaling(v Vec3) Affine { return Affine{{v.X, 0, 0, 0}, {0, v.Y, 0, 0}, {0, 0, v.Z, 0}} }
func Rotation(axis Vec3, angle float64) (Affine, error) {
	if !finite(angle) {
		return Affine{}, ErrInvalid
	}
	n, e := axis.Unit()
	if e != nil {
		return Affine{}, e
	}
	c, s := math.Cos(angle), math.Sin(angle)
	t := 1 - c
	x, y, z := n.X, n.Y, n.Z
	return Affine{{t*x*x + c, t*x*y - s*z, t*x*z + s*y, 0}, {t*x*y + s*z, t*y*y + c, t*y*z - s*x, 0}, {t*x*z - s*y, t*y*z + s*x, t*z*z + c, 0}}, nil
}
func (m Affine) IsFinite() bool {
	for _, row := range m {
		for _, x := range row {
			if !finite(x) {
				return false
			}
		}
	}
	return true
}
func (m Affine) Apply(v Vec3) Vec3 { return m.ApplyVector(v).Add(Vec3{m[0][3], m[1][3], m[2][3]}) }
func (m Affine) ApplyVector(v Vec3) Vec3 {
	return Vec3{m[0][0]*v.X + m[0][1]*v.Y + m[0][2]*v.Z, m[1][0]*v.X + m[1][1]*v.Y + m[1][2]*v.Z, m[2][0]*v.X + m[2][1]*v.Y + m[2][2]*v.Z}
}

// Mul composes transforms: a.Mul(b).Apply(p) equals a.Apply(b.Apply(p)).
func (a Affine) Mul(b Affine) Affine {
	var c Affine
	for i := 0; i < 3; i++ {
		for j := 0; j < 4; j++ {
			for k := 0; k < 3; k++ {
				c[i][j] += a[i][k] * b[k][j]
			}
		}
		c[i][3] += a[i][3]
	}
	return c
}
func (m Affine) Determinant() float64 {
	return m[0][0]*(m[1][1]*m[2][2]-m[1][2]*m[2][1]) - m[0][1]*(m[1][0]*m[2][2]-m[1][2]*m[2][0]) + m[0][2]*(m[1][0]*m[2][1]-m[1][1]*m[2][0])
}

// Inverse uses scaled partial pivoting; singular/non-finite results return errors.
func (m Affine) Inverse() (Affine, error) {
	if !m.IsFinite() {
		return Affine{}, ErrInvalid
	}
	var a [3][6]float64
	var scales [3]float64
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			a[i][j] = m[i][j]
			scales[i] = math.Max(scales[i], math.Abs(a[i][j]))
		}
		a[i][i+3] = 1
		if scales[i] == 0 {
			return Affine{}, ErrDegenerate
		}
	}
	for k := 0; k < 3; k++ {
		pivot := k
		for i := k + 1; i < 3; i++ {
			if math.Abs(a[i][k])/scales[i] > math.Abs(a[pivot][k])/scales[pivot] {
				pivot = i
			}
		}
		if a[pivot][k] == 0 {
			return Affine{}, ErrDegenerate
		}
		a[k], a[pivot] = a[pivot], a[k]
		scales[k], scales[pivot] = scales[pivot], scales[k]
		d := a[k][k]
		for j := 0; j < 6; j++ {
			a[k][j] /= d
		}
		for i := 0; i < 3; i++ {
			if i != k {
				f := a[i][k]
				for j := 0; j < 6; j++ {
					a[i][j] -= f * a[k][j]
				}
			}
		}
	}
	var r Affine
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			r[i][j] = a[i][j+3]
		}
	}
	v := r.ApplyVector(Vec3{m[0][3], m[1][3], m[2][3]}).Scale(-1)
	r[0][3], r[1][3], r[2][3] = v.X, v.Y, v.Z
	if !r.IsFinite() {
		return Affine{}, ErrInvalid
	}
	return r, nil
}

// ApplyNormal uses the inverse transpose and renormalizes.
func (m Affine) ApplyNormal(n Vec3) (Vec3, error) {
	inv, e := m.Inverse()
	if e != nil {
		return Vec3{}, e
	}
	return (Vec3{inv[0][0]*n.X + inv[1][0]*n.Y + inv[2][0]*n.Z, inv[0][1]*n.X + inv[1][1]*n.Y + inv[2][1]*n.Z, inv[0][2]*n.X + inv[1][2]*n.Y + inv[2][2]*n.Z}).Unit()
}
func (m Affine) ApplyBox(b Box) (Box, error) {
	if !b.Valid() || !m.IsFinite() {
		return Box{}, ErrInvalid
	}
	p := make([]Vec3, 0, 8)
	for _, x := range []float64{b.Min.X, b.Max.X} {
		for _, y := range []float64{b.Min.Y, b.Max.Y} {
			for _, z := range []float64{b.Min.Z, b.Max.Z} {
				p = append(p, m.Apply(Vec3{x, y, z}))
			}
		}
	}
	return Bounds(p)
}

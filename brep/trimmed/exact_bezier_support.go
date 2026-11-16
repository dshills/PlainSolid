package trimmed

import (
	"math"
	"math/big"

	"plainsolid/geom"
)

// BSplineSupport gives the original spline when present, or an exact
// bilinear spline with the affine plane's unchanged UV coordinate system.
// The bounded chart encloses the homogeneous trim control hull.
func (f ExactBezierBoundaryFace) BSplineSupport() (*geom.BSplineSurface, error) {
	if f.Spline != nil && f.Affine == nil {
		return f.Spline, nil
	}
	if f.Affine == nil || f.Spline != nil {
		return nil, geom.ErrInvalid
	}
	lo, hi := [2]float64{math.Inf(1), math.Inf(1)}, [2]float64{math.Inf(-1), math.Inf(-1)}
	if f.affineUnitU {
		lo[0], hi[0] = 0, 1
	}
	for _, loop := range f.Loops {
		for _, u := range loop {
			for _, h := range u.UV.Control {
				if h[0] == nil || h[1] == nil || h[3] == nil || h[3].Sign() <= 0 {
					return nil, geom.ErrDomain
				}
				for axis := 0; axis < 2; axis++ {
					if f.affineUnitU && axis == 0 {
						if h[0].Sign() < 0 || h[0].Cmp(h[3]) > 0 {
							return nil, geom.ErrDomain
						}
						continue
					}
					v := new(big.Rat).Quo(h[axis], h[3])
					x, _ := v.Float64()
					if !finite(x) {
						return nil, geom.ErrUnrepresentable
					}
					lo[axis] = math.Min(lo[axis], math.Floor(math.Nextafter(x, math.Inf(-1))))
					hi[axis] = math.Max(hi[axis], math.Ceil(math.Nextafter(x, math.Inf(1))))
				}
			}
		}
	}
	if !finite(lo[0]) || !finite(hi[0]) || lo[0] >= hi[0] || !finite(lo[1]) || !finite(hi[1]) || lo[1] >= hi[1] {
		return nil, geom.ErrConvergence
	}
	w := f.Affine
	if !w.Origin.IsFinite() || !w.U.IsFinite() || !w.V.IsFinite() {
		return nil, geom.ErrInvalid
	}
	poles := make([]geom.Vec3, 0, 4)
	for _, u := range []float64{lo[0], hi[0]} {
		for _, v := range []float64{lo[1], hi[1]} {
			var p [3]float64
			for axis, a := range []float64{w.Origin.X, w.Origin.Y, w.Origin.Z} {
				du, dv := [3]float64{w.U.X, w.U.Y, w.U.Z}[axis], [3]float64{w.V.X, w.V.Y, w.V.Z}[axis]
				q := new(big.Rat).Add(ratInset(a), new(big.Rat).Mul(ratInset(du), ratInset(u)))
				q.Add(q, new(big.Rat).Mul(ratInset(dv), ratInset(v)))
				x, err := geom.RepresentedExactScalar(q)
				if err != nil {
					return nil, err
				}
				p[axis] = x
			}
			poles = append(poles, geom.Vec3{X: p[0], Y: p[1], Z: p[2]})
		}
	}
	return geom.NewBSplineSurface(1, 1, 2, 2, poles,
		[]float64{lo[0], lo[0], hi[0], hi[0]}, []float64{lo[1], lo[1], hi[1], hi[1]}, nil)
}

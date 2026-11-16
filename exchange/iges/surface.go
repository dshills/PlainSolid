package iges

import (
	"errors"
	"math"
	"math/big"

	"plainsolid/geom"
)

// Surface is a type-128 patch with a rectangular parameter interval and an
// expanded finite Geometry. For a proved periodic source, a separate exact
// wrapped parameter domain is available via ExactPeriodicGeometry.
// Closed flags on ordinary directions are metadata.
type Surface struct {
	Geometry                                           *geom.BSplineSurface
	UStart, UEnd, VStart, VEnd                         float64
	ClosedU, ClosedV, Polynomial, PeriodicU, PeriodicV bool
	periodic                                           *periodicSurface128
}

func (s Surface) Validate() error {
	if s.Geometry == nil {
		return errors.New("iges: missing spline surface")
	}
	a, b, c, d := s.Geometry.Domain()
	if math.IsNaN(a) || !finite(s.UStart) || !finite(s.UEnd) || !finite(s.VStart) || !finite(s.VEnd) || s.UStart < a || s.UEnd > b || s.VStart < c || s.VEnd > d || s.UStart >= s.UEnd || s.VStart >= s.VEnd {
		return errors.New("iges: invalid surface interval")
	}
	if s.PeriodicU || s.PeriodicV || s.periodic != nil {
		if s.periodic == nil || s.periodic.expanded != s.Geometry || s.periodic.axes != [2]bool{s.PeriodicU, s.PeriodicV} || s.periodic.polynomial != s.Polynomial ||
			s.PeriodicU && !s.ClosedU || s.PeriodicV && !s.ClosedV ||
			s.UStart != a || s.UEnd != b || s.VStart != c || s.VEnd != d {
			return errors.New("iges: inconsistent periodic surface evidence")
		}
	}
	if s.Polynomial {
		w := s.Geometry.Data().Weights
		for _, x := range w {
			if x != w[0] {
				return errors.New("iges: inconsistent polynomial surface")
			}
		}
	}
	return nil
}

// EvalSurface evaluates normalized parameters in [0,1] and applies placement.
func (e Entity) EvalSurface(u, v float64) (geom.Vec3, error) {
	if err := e.validate(); err != nil {
		return geom.Vec3{}, err
	}
	if e.Surface == nil || !finite(u) || !finite(v) || u < 0 || u > 1 || v < 0 || v > 1 {
		return geom.Vec3{}, errors.New("iges: invalid surface evaluation")
	}
	s := e.Surface
	p, err := s.Geometry.Eval(s.UStart*(1-u)+s.UEnd*u, s.VStart*(1-v)+s.VEnd*v)
	if err == nil && e.Transform != nil {
		p = e.Transform.Apply(p)
	}
	if !p.IsFinite() {
		return geom.Vec3{}, errors.New("iges: transformed surface overflow")
	}
	return p, err
}
func decodeSurface(p []parameter, maxPoints int) (Entity, error) {
	return decodeSurfaceMode(p, maxPoints, false, nil)
}

// For a proved trimmed-face input, declared receives the exact original
// finite bounds. The caller must prove those bounds independently; the
// returned support represents the full underlying cyclic knot lattice.
func decodeSurfaceMode(p []parameter, maxPoints int, allowPeriodic bool, declared *[4]*big.Rat) (Entity, error) {
	if len(p) < 14 {
		return Entity{}, errors.New("iges: truncated surface")
	}
	dims := [4]int{}
	for i := range dims {
		v, e := intValue(p[i+1])
		if e != nil || v < 1 || v >= maxPoints {
			return Entity{}, errors.New("iges: surface dimensions/limit invalid")
		}
		dims[i] = v
	}
	nu, nv, du, dv := dims[0]+1, dims[1]+1, dims[2], dims[3]
	if du >= nu || dv >= nv || nu > maxPoints/nv {
		return Entity{}, errors.New("iges: surface degree/control-point limit")
	}
	n := nu * nv
	ku, kv := nu+du+1, nv+dv+1
	if ku > len(p)-14 || kv > len(p)-14-ku || n > (len(p)-14-ku-kv)/4 {
		return Entity{}, errors.New("iges: truncated surface net")
	}
	flags := [5]bool{}
	for i := range flags {
		v, e := intValue(p[5+i])
		if e != nil || (v != 0 && v != 1) {
			return Entity{}, errors.New("iges: invalid surface flag")
		}
		flags[i] = v == 1
	}
	periodic := flags[3] || flags[4]
	if periodic && (!allowPeriodic || n > maxPeriodicSurface128Poles || flags[3] && !flags[0] || flags[4] && !flags[1]) {
		return Entity{}, errors.New("iges: unsupported periodic surface flags or control net size")
	}
	values := make([]float64, ku+kv+4*n+4)
	for i := range values {
		x, e := realValue(p[10+i])
		if e != nil {
			return Entity{}, e
		}
		values[i] = x
	}
	if e := noPointers(p[10+len(values):]); e != nil {
		return Entity{}, e
	}
	weights := make([]float64, n)
	poles := make([]geom.Vec3, n)
	for v := 0; v < nv; v++ {
		for u := 0; u < nu; u++ {
			src := v*nu + u
			dst := u*nv + v
			weights[dst] = values[ku+kv+src]
			j := ku + kv + n + 3*src
			poles[dst] = geom.Vec3{X: values[j], Y: values[j+1], Z: values[j+2]}
		}
	}
	buildGeometry := geom.NewBSplineSurface
	if periodic {
		buildGeometry = geom.NewBSplineSurfaceWithExactWeights
	}
	g, e := buildGeometry(du, dv, nu, nv, poles, values[:ku], values[ku:ku+kv], weights)
	if e != nil {
		return Entity{}, e
	}
	tail := values[ku+kv+4*n:]
	s := &Surface{Geometry: g, UStart: tail[0], UEnd: tail[1], VStart: tail[2], VEnd: tail[3],
		ClosedU: flags[0], ClosedV: flags[1], Polynomial: flags[2], PeriodicU: flags[3], PeriodicV: flags[4]}
	if periodic {
		raw := p[10 : 10+len(values)]
		bounds := raw[ku+kv+4*n:]
		if declared != nil {
			for i, value := range bounds {
				declared[i], e = igesPlanarExact(value)
				if e != nil {
					return Entity{}, e
				}
			}
			bounds = []parameter{raw[du], raw[nu], raw[ku+dv], raw[ku+nv]}
			s.UStart, s.UEnd, s.VStart, s.VEnd = g.Domain()
		}
		s.periodic, e = provePeriodicSurface128(g, raw[:ku], raw[ku:ku+kv], raw[ku+kv:ku+kv+4*n], bounds,
			[2]int{nu, nv}, [2]int{du, dv}, [2]bool{flags[3], flags[4]}, flags[2])
		if e != nil {
			return Entity{}, e
		}
	}
	if e = s.Validate(); e != nil {
		return Entity{}, e
	}
	return Entity{Surface: s, sourceWeightsRounded: !exactWeightNormalization(weights)}, nil
}

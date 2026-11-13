package step

import (
	"context"
	"math/big"
	"plainsolid/exchange/iges"
	"plainsolid/geom"
)

// CurvedOvalFace is one positively oriented open face on an injective rational
// biquadratic Bezier patch over [0,1]^2. Its four rational quadratic UV arcs are
// a canonical simple oval; up to 16 certified disjoint clockwise oval holes
// are supported. ModelCurves and source IDs are read-only certificates;
// the writer derives authoritative geometry from Geometry, Outer and Holes.
// Source hole order follows the source bounds; curves follow canonical quarter
// order (outer counterclockwise, holes clockwise), beginning at the east pole.
type CurvedOvalFace struct {
	Name            string
	Geometry        *geom.BSplineSurface
	Outer           QuadraticOval
	Holes           []QuadraticOval
	HoleModelCurves [][4]*geom.ExactSplineCurve
	SourceHoleEdges [][4]int64
	LengthUnit      float64
	ModelCurves     [4]*geom.ExactSplineCurve
	SourceFaceID    int64
	ellipseShear    float64
	SourceEdges     [4]int64
}

// CurvedOvalOptions bounds exact composition, identity and injectivity proofs.
// Zero fields select 256 poles, 8M operations, 4096 coefficient bits and 256 cells.
type CurvedOvalOptions struct{ MaxPoles, MaxOperations, MaxCoefficientBits, MaxCells int }

func (o CurvedOvalOptions) defaults() (CurvedOvalOptions, error) {
	if o.MaxPoles == 0 {
		o.MaxPoles = 256
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxCells == 0 {
		o.MaxCells = 256
	}
	if o.MaxPoles < 18 || o.MaxPoles > 4096 || o.MaxOperations < 10 || o.MaxOperations > 100000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 65536 || o.MaxCells < 1 || o.MaxCells > 4096 {
		return o, geom.ErrInvalid
	}
	return o, nil
}
func (o CurvedOvalOptions) patch() geom.SurfacePatchOptions {
	return geom.SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations / 10, MaxCoefficientBits: o.MaxCoefficientBits}
}
func curvedOvalUV(q QuadraticOval) ([4]*geom.BSpline, error) {
	var out [4]*geom.BSpline
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		return out, e
	}
	m, e := iges.NewQuadraticOvalSurface(s, q, 1)
	if e != nil {
		return out, e
	}
	c := m.Entities[m.CurvesOnSurfaces[0].ParameterCurve].Spline.Curve
	p, w := c.Poles(), c.Weights()
	for i := range out {
		out[i], e = geom.NewBSpline(2, p[2*i:2*i+3], []float64{0, 0, 0, 1, 1, 1}, w[2*i:2*i+3])
		if e != nil {
			return out, e
		}
	}
	return out, nil
}
func certifyCurvedOval(m CurvedOvalFace, o CurvedOvalOptions) ([4]*geom.BSpline, [4]*geom.ExactSplineCurve, error) {
	return certifyCurvedOvalMode(m, o, false)
}
func certifyCurvedOvalMode(m CurvedOvalFace, o CurvedOvalOptions, ellipse bool) ([4]*geom.BSpline, [4]*geom.ExactSplineCurve, error) {
	var uv [4]*geom.BSpline
	var models [4]*geom.ExactSplineCurve
	if m.Geometry == nil {
		return uv, models, geom.ErrInvalid
	}
	d := m.Geometry.Data()
	if d.DegreeU != 2 || d.DegreeV != 2 || (!ellipse && (d.CountU != 3 || d.CountV != 3)) {
		return uv, models, geom.ErrConvergence
	}
	u0, u1, v0, v1 := m.Geometry.Domain()
	if u0 != 0 || u1 != 1 || v0 != 0 || v1 != 1 {
		return uv, models, geom.ErrDomain
	}
	var e error
	o, e = o.defaults()
	if e != nil {
		return uv, models, e
	}
	uv, e = curvedOvalUV(m.Outer)
	if e != nil {
		return uv, models, e
	}
	if ellipse {
		uv, e = curvedEllipseUV(m.Outer, m.ellipseShear)
		if e != nil {
			return uv, models, e
		}
	}
	patch, e := geom.NewSurfacePatch(m.Geometry, o.patch())
	if e != nil {
		return uv, models, e
	}
	proof, e := geom.CertifySurfaceInjectivity(patch, geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxDepth: 8, MaxCells: o.MaxCells, MaxOperations: o.MaxOperations / 10, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return uv, models, e
	}
	if !proof.ProvenInjective {
		return uv, models, geom.ErrConvergence
	}
	for i, c := range uv {
		exact, e := geom.NewExactSplineCurve(c, o.patch())
		if e != nil {
			return uv, models, e
		}
		if ellipse && (d.CountU != 3 || d.CountV != 3) {
			pieces, err := patch.ComposeParameterCurvePieces(context.Background(), exact, geom.SurfaceCurveCompositionOptions{SurfacePatchOptions: o.patch(), MaxPieces: 1, MaxRootSubdivisions: 512, MaxHullDepth: 8})
			e = err
			if e == nil {
				if !pieces.Complete || !pieces.Continuous || len(pieces.Pieces) != 1 || pieces.Pieces[0].Start.Sign() != 0 || pieces.Pieces[0].End.Cmp(big.NewRat(1, 1)) != 0 {
					return uv, models, geom.ErrConvergence
				}
				models[i] = pieces.Pieces[0].Curve
			}
		} else {
			models[i], e = patch.ComposeParameterCurve(context.Background(), exact, o.patch())
		}
		if e != nil {
			return uv, models, e
		}
	}
	return uv, models, nil
}

// exactBezierFloats searches only a common integer weight scale; poles must be
// exactly representable. It never rounds coefficients or invokes normalization.
func exactBezierFloats(c *geom.ExactSplineCurve, o geom.SurfacePatchOptions) ([]geom.Vec3, []float64, error) {
	net, _, e := c.BezierControlNet(context.Background(), o)
	if e != nil {
		return nil, nil, e
	}
	if net.Start != 0 || net.End != 1 || len(net.Control) < 2 || len(net.Control) > 17 {
		return nil, nil, geom.ErrConvergence
	}
	poles := make([]geom.Vec3, len(net.Control))
	weights := make([]float64, len(net.Control))
	lcm := big.NewInt(1)
	for i, h := range net.Control {
		var p [3]float64
		for j := range p {
			p[j], e = geom.RepresentedExactScalar(new(big.Rat).Quo(h[j], h[3]))
			if e != nil {
				return nil, nil, e
			}
		}
		poles[i] = geom.Vec3{X: p[0], Y: p[1], Z: p[2]}
		den := h[3].Denom()
		g := new(big.Int).GCD(nil, nil, lcm, den)
		lcm.Mul(new(big.Int).Quo(lcm, g), den)
		if lcm.BitLen() > o.MaxCoefficientBits {
			return nil, nil, geom.ErrConvergence
		}
	}
	ints := make([]*big.Int, len(weights))
	gcd := new(big.Int)
	for i, h := range net.Control {
		q := new(big.Rat).Mul(h[3], new(big.Rat).SetInt(lcm))
		if !q.IsInt() {
			return nil, nil, geom.ErrConvergence
		}
		ints[i] = new(big.Int).Set(q.Num())
		gcd.GCD(nil, nil, gcd, ints[i])
	}
	for i, n := range ints {
		weights[i], e = geom.RepresentedExactScalar(new(big.Rat).SetInt(new(big.Int).Quo(n, gcd)))
		if e != nil {
			return nil, nil, e
		}
	}
	return poles, weights, nil
}

// certifyCurvedOvalHoles uses the canonical UV certificate for containment and
// disjointness, then composes every clockwise hole on the same injective patch.
func certifyCurvedOvalHoles(m CurvedOvalFace, o CurvedOvalOptions) ([][4]*geom.BSpline, [][4]*geom.ExactSplineCurve, error) {
	if len(m.Holes) > 16 {
		return nil, nil, geom.ErrInvalid
	}
	if e := validateCurvedOvalHoles(m); e != nil {
		return nil, nil, e
	}
	// Divide the caller's arithmetic allowance across all independently bounded
	// loop proofs. Individual proof failures remain explicit.
	o.MaxOperations /= 1 + len(m.Holes)
	if o.MaxOperations < 10 {
		return nil, nil, geom.ErrConvergence
	}
	uv, model, e := certifyCurvedOval(m, o)
	if e != nil {
		return nil, nil, e
	}
	us, ms := [][4]*geom.BSpline{uv}, [][4]*geom.ExactSplineCurve{model}
	patch, e := geom.NewSurfacePatch(m.Geometry, o.patch())
	if e != nil {
		return nil, nil, e
	}
	for _, h := range m.Holes {
		ccw, e := curvedOvalUV(h)
		if e != nil {
			return nil, nil, e
		}
		var u [4]*geom.BSpline
		var c [4]*geom.ExactSplineCurve
		for i := range u {
			p, w := ccw[3-i].Poles(), ccw[3-i].Weights()
			for j := 0; j < len(p)/2; j++ {
				k := len(p) - 1 - j
				p[j], p[k] = p[k], p[j]
				w[j], w[k] = w[k], w[j]
			}
			u[i], e = geom.NewBSpline(2, p, []float64{0, 0, 0, 1, 1, 1}, w)
			if e != nil {
				return nil, nil, e
			}
			exact, e := geom.NewExactSplineCurve(u[i], o.patch())
			if e != nil {
				return nil, nil, e
			}
			c[i], e = patch.ComposeParameterCurve(context.Background(), exact, o.patch())
			if e != nil {
				return nil, nil, e
			}
		}
		us = append(us, u)
		ms = append(ms, c)
	}
	return us, ms, nil
}

func validateCurvedOvalHoles(m CurvedOvalFace) error {
	plane, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		return e
	}
	if _, e = iges.NewQuadraticOvalSurfaceWithHoles(plane, m.Outer, m.Holes, 1); e != nil {
		return e
	}
	return nil
}

func curvedEllipseUV(q QuadraticOval, shear float64) ([4]*geom.BSpline, error) {
	var out [4]*geom.BSpline
	if _, e := curvedOvalUV(q); e != nil {
		return out, e
	}
	sh := new(big.Rat).SetFloat64(shear)
	if sh == nil {
		return out, geom.ErrInvalid
	}
	directions := [9][2]int64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	points := make([]geom.Vec3, 9)
	for i, d := range directions {
		xy := [2]*big.Rat{new(big.Rat).SetFloat64(q.Center.X), new(big.Rat).SetFloat64(q.Center.Y)}
		xy[0].Add(xy[0], new(big.Rat).Mul(big.NewRat(d[0], 1), new(big.Rat).SetFloat64(q.Radii.X)))
		xy[0].Add(xy[0], new(big.Rat).Mul(big.NewRat(d[1], 1), sh))
		xy[1].Add(xy[1], new(big.Rat).Mul(big.NewRat(d[1], 1), new(big.Rat).SetFloat64(q.Radii.Y)))
		v := [2]float64{}
		for j := range v {
			var e error
			v[j], e = geom.RepresentedExactScalar(xy[j])
			if e != nil {
				return out, e
			}
			if v[j] <= 0 || v[j] >= 1 {
				return out, geom.ErrDomain
			}
		}
		points[i] = geom.Vec3{X: v[0], Y: v[1]}
	}
	for i := range out {
		var e error
		out[i], e = geom.NewBSpline(2, points[2*i:2*i+3], []float64{0, 0, 0, 1, 1, 1}, []float64{.5, .5, 1})
		if e != nil {
			return out, e
		}
	}
	return out, nil
}
func curvedEllipseDescriptor(uv [4]*geom.BSpline) (QuadraticOval, float64, error) {
	var zero QuadraticOval
	p := [4]geom.Vec3{}
	for i, c := range uv {
		if c == nil || c.Degree() != 2 || len(c.Poles()) != 3 {
			return zero, 0, geom.ErrInvalid
		}
		p[i] = c.Poles()[0]
	}
	north, south := 0, 0
	for i := range p {
		if p[i].Y > p[north].Y {
			north = i
		}
		if p[i].Y < p[south].Y {
			south = i
		}
	}
	if north == south {
		return zero, 0, geom.ErrInvalid
	}
	avg := func(a, b float64, subtract bool) (float64, error) {
		x, y := new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64(b)
		if subtract {
			x.Sub(x, y)
		} else {
			x.Add(x, y)
		}
		return geom.RepresentedExactScalar(x.Quo(x, big.NewRat(2, 1)))
	}
	cx, e := avg(p[north].X, p[south].X, false)
	if e != nil {
		return zero, 0, e
	}
	cy, e := avg(p[north].Y, p[south].Y, false)
	if e != nil {
		return zero, 0, e
	}
	ry, e := avg(p[north].Y, p[south].Y, true)
	if e != nil {
		return zero, 0, e
	}
	var ends []geom.Vec3
	for i, v := range p {
		if i != north && i != south {
			if v.Y != cy {
				return zero, 0, geom.ErrInvalid
			}
			ends = append(ends, v)
		}
	}
	rx, e := avg(max(ends[0].X, ends[1].X), min(ends[0].X, ends[1].X), true)
	if e != nil {
		return zero, 0, e
	}
	mid, e := avg(ends[0].X, ends[1].X, false)
	if e != nil || mid != cx {
		return zero, 0, geom.ErrInvalid
	}
	shear, e := geom.RepresentedExactScalar(new(big.Rat).Sub(new(big.Rat).SetFloat64(p[north].X), new(big.Rat).SetFloat64(cx)))
	if e != nil {
		return zero, 0, e
	}
	return QuadraticOval{Center: geom.Vec2{X: cx, Y: cy}, Radii: geom.Vec2{X: rx, Y: ry}, QuarterWeight: .5}, shear, nil
}

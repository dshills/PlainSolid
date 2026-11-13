package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// ObliqueCylinderSplitCutResult holds the two native bodies of an oblique
// cylinder subtraction with a wider cutter. The ideal gap is at least the
// square root of MinimumGapSquared, measured along the in-plane unit
// normal to the cutter axis.
type ObliqueCylinderSplitCutResult struct {
	Solids                                         [2]Solid
	StockWallFaces, CutterWallFaces, StockCapFaces [2][]int
	MinimumGapSquared                              *big.Rat
	CapCoverage                                    geom.CylinderCommonCaps
}

// ObliqueCylinderSplitCut subtracts a wider finite cylinder from narrower
// finite stock. The raw axes must cross at center. Exact cap clearance
// guarantees that the cutter spans the entire common and stock ends remain
// exposed. The bodies are indexed in increasing stock-axis order.
func ObliqueCylinderSplitCut(ctx context.Context, center, stockAxis, cutterAxis geom.Vec3, stockRadius, cutterRadius float64, stockCaps, cutterCaps [2]float64, tolerance float64) (ObliqueCylinderSplitCutResult, error) {
	var empty ObliqueCylinderSplitCutResult
	if ctx == nil || !finite(tolerance) || tolerance <= 0 {
		return empty, geom.ErrInvalid
	}
	s, proof, e := newObliqueSplitChart(ctx, center, stockAxis, cutterAxis, stockRadius, cutterRadius, stockCaps, cutterCaps)
	if e != nil {
		return empty, e
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	pad := new(big.Rat).Mul(big.NewRat(8, 1), q(tolerance))
	if q(stockRadius).Cmp(pad) <= 0 || new(big.Rat).Sub(q(cutterRadius), q(stockRadius)).Cmp(pad) <= 0 {
		return empty, geom.ErrConvergence
	}
	walls, e := geom.NewCylinderCommonWalls(ctx, center, stockAxis, cutterAxis, stockRadius, cutterRadius, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return empty, e
	}
	out := ObliqueCylinderSplitCutResult{CapCoverage: proof}
	for i, side := range []int{-1, 1} {
		var faces []Face
		u, v := Interval{-math.Pi / 2, math.Pi / 2}, Interval{0, 1}
		for _, half := range []int{-1, 1} {
			for cap := range 2 {
				s.side, s.half, s.cap = side, half, cap == 1
				reverse := side*half < 0
				if s.cap {
					reverse = !reverse
					out.StockCapFaces[i] = append(out.StockCapFaces[i], len(faces))
				} else {
					out.StockWallFaces[i] = append(out.StockWallFaces[i], len(faces))
				}
				faces = append(faces, Face{Surface: s, U: u, V: v, Reversed: reverse})
			}
		}
		index := 2 + i
		w := walls[index]
		_, _, reverse := w.Ownership()
		out.CutterWallFaces[i] = []int{len(faces)}
		faces = append(faces, Face{Surface: generalCylinderCommonChart{w, center, stockAxis, cutterAxis, stockRadius, cutterRadius, index}, U: u, V: Interval{-1, 1}, Reversed: !reverse})
		out.Solids[i], e = joinPrimitivePatches(faces, tolerance)
		if e != nil {
			return empty, e
		}
	}
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	// In oblique sections, the coefficient a separates both bodies;
	// the linear functional extracting a has unit norm (the normal to
	// the cutter axis in the axis plane). This is a model-space gap.
	sq := func(x float64) *big.Rat { return new(big.Rat).Mul(q(x), q(x)) }
	out.MinimumGapSquared = new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Sub(sq(cutterRadius), sq(stockRadius)))
	return out, nil
}

type obliqueSplitChart struct {
	center, stockAxis, cutterAxis geom.Vec3
	stock, cutter                 float64
	stockCaps, cutterCaps         [2]float64
	A, B, W, radial               geom.Vec3
	basisNorm                     *big.Rat
	side, half                    int
	cap                           bool
}

func newObliqueSplitChart(ctx context.Context, center, a, b geom.Vec3, stock, cutter float64, stockCaps, cutterCaps [2]float64) (obliqueSplitChart, geom.CylinderCommonCaps, error) {
	var empty obliqueSplitChart
	var no geom.CylinderCommonCaps
	if !center.IsFinite() || !finite(stock) || !finite(cutter) || stock <= 0 || cutter <= stock {
		return empty, no, geom.ErrInvalid
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	var windows [2][2]*big.Rat
	for i, c := range [2][2]float64{stockCaps, cutterCaps} {
		if !finite(c[0]) || !finite(c[1]) || c[0] >= c[1] {
			return empty, no, geom.ErrInvalid
		}
		windows[i] = [2]*big.Rat{q(c[0]), q(c[1])}
	}
	p, e := geom.ClassifyCylinderCommonCaps(ctx, a, b, stock, cutter, &windows[0], &windows[1], geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return empty, no, e
	}
	if !p.Covers || p.TouchMask != 0 {
		return empty, no, geom.ErrConvergence
	}
	m, e := geom.NewObliqueSkewCylinderMap(ctx, geom.Vec3{}, a, stock, geom.Vec3{}, b, cutter, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return empty, no, e
	}
	A, e := m.Eval(1, 0, 0)
	if e != nil {
		return empty, no, e
	}
	B, e := m.Eval(0, 1, 0)
	if e != nil {
		return empty, no, e
	}
	W, e := m.Eval(0, 0, 1)
	if e != nil {
		return empty, no, e
	}
	U, e := a.Unit()
	if e != nil {
		return empty, no, e
	}
	radial := B.Sub(U.Scale(U.Dot(B)))
	// |A|=|B|=1/sin θ. Bound this by 1/sin² θ in exact arithmetic.
	norm, cross := new(big.Rat), new(big.Rat)
	aa, bb := new(big.Rat), new(big.Rat)
	for i, x := range [3]float64{a.X, a.Y, a.Z} {
		y := [3]float64{b.X, b.Y, b.Z}[i]
		aa.Add(aa, new(big.Rat).Mul(q(x), q(x)))
		bb.Add(bb, new(big.Rat).Mul(q(y), q(y)))
		cross.Add(cross, new(big.Rat).Mul(q(x), q(y)))
	}
	norm.Mul(aa, bb)
	den := new(big.Rat).Sub(norm, new(big.Rat).Mul(cross, cross))
	norm.Quo(norm, den)
	return obliqueSplitChart{center, a, b, stock, cutter, stockCaps, cutterCaps, A, B, W, radial, norm, 0, 0, false}, p, nil
}
func (s obliqueSplitChart) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.evaluate(u, v)
	return p, e
}
func (s obliqueSplitChart) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, a, b, e := s.evaluate(u, v)
	return a, b, e
}
func (s obliqueSplitChart) evaluate(u, v float64) (p, du, dv geom.Vec3, err error) {
	if !finite(u) || !finite(v) || u < -math.Pi/2 || u > math.Pi/2 || v < 0 || v > 1 {
		return p, du, dv, geom.ErrDomain
	}
	sn, cs := math.Sincos(u)
	if math.Abs(u) == math.Pi/2 {
		cs = 0
	}
	y, w := s.stock*sn, s.stock*cs
	half, side := float64(s.half), float64(s.side)
	end := s.stockCaps[0]
	if s.side > 0 {
		end = s.stockCaps[1]
	}
	cap := s.stockAxis.Scale(end)
	radial := s.radial.Scale(half)
	if s.cap {
		p = cap.Add(radial.Scale(v * w)).Add(s.W.Scale(y))
		du = radial.Scale(-v * y).Add(s.W.Scale(w))
		dv = radial.Scale(w)
	} else {
		x := s.cutter * math.Sqrt((1-y/s.cutter)*(1+y/s.cutter))
		if !finite(x) || x <= 0 {
			return p, du, dv, geom.ErrConvergence
		}
		dx := -y / x * w
		rim := s.A.Scale(side * x).Add(s.B.Scale(half * w))
		toCap := cap.Add(radial.Scale(w))
		p = rim.Scale(1 - v).Add(toCap.Scale(v)).Add(s.W.Scale(y))
		du = s.A.Scale(side * dx * (1 - v)).Add(s.B.Scale(-half * y * (1 - v))).Add(radial.Scale(-v * y)).Add(s.W.Scale(w))
		dv = toCap.Sub(rim)
	}
	p = p.Add(s.center)
	if !p.IsFinite() || !du.IsFinite() || !dv.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrConvergence
	}
	return p, du, dv, nil
}

func (s obliqueSplitChart) secondDerivativeBounds(ctx context.Context, u, v Interval) (float64, float64, float64, error) {
	if e := ctx.Err(); e != nil {
		return 0, 0, 0, e
	}
	if u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < 0 || v.Max > 1 {
		return 0, 0, 0, geom.ErrDomain
	}
	if s.cap {
		return upMul(2, s.stock), s.stock, 0, nil
	}
	a, b, _, e := (unequalCylinderCommonChart{small: s.stock, large: s.cutter}).secondDerivativeBounds(ctx, u, v)
	if e != nil {
		return 0, 0, 0, e
	}
	n := upperBoreRat(s.basisNorm)
	return upAdd(upMul(a, n), s.stock), upAdd(upMul(upAdd(b, s.stock), n), s.stock), 0, nil
}

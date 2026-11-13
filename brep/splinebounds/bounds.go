// Package splinebounds certifies conservative real-arithmetic derivative bounds
// for clamped rational tensor-product B-spline surfaces. It does not bound the
// rounding error of a floating-point surface evaluator.
package splinebounds

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"plainsolid/geom"
)

var ErrInvalid = errors.New("splinebounds: invalid data, domain or options")
var ErrUnsupported = errors.New("splinebounds: unsupported knots or continuity")
var ErrLimit = errors.New("splinebounds: resource limit exceeded")
var ErrRange = errors.New("splinebounds: no finite float64 upper bound")

// Domain is a nonempty closed parameter rectangle within the spline domain.
// Derivative values on its edges mean one-sided limits from its interior.
type Domain struct{ U0, U1, V0, V1 float64 }

// Options bounds control-net size (default 16384), degree (default 32, maximum
// 64), exact work (default 2000000), and stored rational numerator/denominator
// bits (default 16384). Zero selects defaults; negative values are invalid.
type Options struct{ MaxControls, MaxDegree, MaxWork, MaxBits int }

const (
	U = iota
	V
	UU
	UV
	VV
)

// Result bounds Euclidean derivative norms. Components contain caller-owned
// exact nonnegative component bounds in U,V,UU,UV,VV order. WeightLower is a
// strictly positive denominator certificate from active homogeneous controls.
// Bounds are for represented poles/weights/knots in real arithmetic. Subdomains
// use intersecting supports, not exact Bezier extraction, and may be loose.
type Result struct {
	DU, DV, DUU, DUV, DVV    float64
	Components               [5][3]*big.Rat
	WeightLower, WeightUpper float64
	ActiveControls           int
	Domain                   Domain
}
type q = *big.Rat
type control [4]q
type net struct {
	pu, pv, nu, nv int
	ku, kv         []q
	c              []control
	zero           bool
}

func number(x float64) q    { return new(big.Rat).SetFloat64(x) }
func add(a, b q) q          { return new(big.Rat).Add(a, b) }
func sub(a, b q) q          { return new(big.Rat).Sub(a, b) }
func mul(a, b q) q          { return new(big.Rat).Mul(a, b) }
func div(a, b q) q          { return new(big.Rat).Quo(a, b) }
func absolute(a q) q        { return new(big.Rat).Abs(a) }
func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

type budget struct {
	ctx        context.Context
	work, bits int
	err        error
}

func (b *budget) tick() bool {
	if b.err != nil {
		return false
	}
	if e := b.ctx.Err(); e != nil {
		b.err = e
		return false
	}
	if b.work == 0 {
		b.err = ErrLimit
		return false
	}
	b.work--
	return true
}
func (b *budget) check(x q) {
	if x.Num().BitLen() > b.bits || x.Denom().BitLen() > b.bits {
		b.err = ErrLimit
	}
}
func (b *budget) derivative(n net, u bool) net {
	if n.zero || u && n.pu == 0 || !u && n.pv == 0 {
		return net{zero: true}
	}
	out := net{pu: n.pu, pv: n.pv, nu: n.nu, nv: n.nv, ku: n.ku, kv: n.kv}
	p, knots := n.pv, n.kv
	if u {
		p, knots = n.pu, n.ku
		out.pu--
		out.nu--
		out.ku = n.ku[1 : len(n.ku)-1]
	} else {
		out.pv--
		out.nv--
		out.kv = n.kv[1 : len(n.kv)-1]
	}
	out.c = make([]control, out.nu*out.nv)
	for i := 0; i < out.nu; i++ {
		for j := 0; j < out.nv; j++ {
			if !b.tick() {
				return net{zero: true}
			}
			a, c, k := i*n.nv+j, i*n.nv+j+1, j
			if u {
				c = (i+1)*n.nv + j
				k = i
			}
			den := sub(knots[k+p+1], knots[k+1])
			for axis := 0; axis < 4; axis++ {
				v := new(big.Rat)
				if den.Sign() != 0 {
					v = div(mul(big.NewRat(int64(p), 1), sub(n.c[c][axis], n.c[a][axis])), den)
				}
				b.check(v)
				out.c[i*out.nv+j][axis] = v
			}
		}
	}
	return out
}
func active(knots []q, degree, index int, lo, hi q) bool {
	return knots[index].Cmp(hi) < 0 && knots[index+degree+1].Cmp(lo) > 0 && knots[index].Cmp(knots[index+degree+1]) < 0
}
func (b *budget) hull(n net, d [4]q) control {
	h := control{new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)}
	if n.zero {
		return h
	}
	for i := 0; i < n.nu; i++ {
		for j := 0; j < n.nv; j++ {
			if !b.tick() {
				return h
			}
			if !active(n.ku, n.pu, i, d[0], d[1]) || !active(n.kv, n.pv, j, d[2], d[3]) {
				continue
			}
			for k, v := range n.c[i*n.nv+j] {
				a := absolute(v)
				if a.Cmp(h[k]) > 0 {
					h[k] = a
				}
			}
		}
	}
	return h
}
func validateKnots(knots []float64, degree, count int, lo, hi float64) error {
	if degree < 1 || count <= degree || len(knots) != count+degree+1 {
		return ErrInvalid
	}
	for i, k := range knots {
		if !finite(k) || i > 0 && k < knots[i-1] {
			return ErrInvalid
		}
	}
	start, end := knots[degree], knots[count]
	if !(start < end) || lo < start || hi > end {
		return ErrInvalid
	}
	for i := 0; i <= degree; i++ {
		if knots[i] != start || knots[len(knots)-1-i] != end {
			return fmt.Errorf("%w: knot vector is not clamped", ErrUnsupported)
		}
	}
	for i := 0; i < len(knots); {
		j := i + 1
		for j < len(knots) && knots[j] == knots[i] {
			j++
		}
		m := j - i
		k := knots[i]
		if k == start || k == end {
			if m != degree+1 {
				return ErrInvalid
			}
		} else {
			if m > degree {
				return fmt.Errorf("%w: discontinuous interior knot", ErrUnsupported)
			}
			if k > lo && k < hi && degree-m < 2 {
				return fmt.Errorf("%w: interior knot lacks C2 continuity", ErrUnsupported)
			}
		}
		i = j
	}
	return nil
}

// Bound reads a copied surface control net and delegates to FromData.
func Bound(ctx context.Context, s *geom.BSplineSurface, d Domain, o Options) (Result, error) {
	if s == nil {
		return Result{}, ErrInvalid
	}
	return FromData(ctx, s.Data(), d, o)
}

// Whole bounds the entire parameter domain. Interior knots must be at least C2.
func Whole(ctx context.Context, s *geom.BSplineSurface, o Options) (Result, error) {
	if s == nil {
		return Result{}, ErrInvalid
	}
	u0, u1, v0, v1 := s.Domain()
	return Bound(ctx, s, Domain{u0, u1, v0, v1}, o)
}

// FromData certifies the exact rational geometry represented by d. All weights
// must be finite and strictly positive (nil means one). Domain boundary knots
// may have lower continuity: the certificate covers their one-sided interior
// limits. Unsupported continuity at strictly interior knots fails explicitly.
func FromData(ctx context.Context, data geom.BSplineSurfaceData, domain Domain, o Options) (Result, error) {
	empty := Result{}
	if ctx == nil || o.MaxControls < 0 || o.MaxDegree < 0 || o.MaxDegree > 64 || o.MaxWork < 0 || o.MaxBits < 0 {
		return empty, ErrInvalid
	}
	if o.MaxControls == 0 {
		o.MaxControls = 16384
	}
	if o.MaxDegree == 0 {
		o.MaxDegree = 32
	}
	if o.MaxWork == 0 {
		o.MaxWork = 2000000
	}
	if o.MaxBits == 0 {
		o.MaxBits = 16384
	}
	b := &budget{ctx: ctx, work: o.MaxWork, bits: o.MaxBits}
	if !b.tick() {
		return empty, b.err
	}
	for _, v := range []float64{domain.U0, domain.U1, domain.V0, domain.V1} {
		if !finite(v) {
			return empty, ErrInvalid
		}
	}
	if !(domain.U0 < domain.U1 && domain.V0 < domain.V1) || data.CountU <= 0 || data.CountV <= 0 {
		return empty, ErrInvalid
	}
	if data.DegreeU > o.MaxDegree || data.DegreeV > o.MaxDegree || data.CountU > o.MaxControls/data.CountV {
		return empty, ErrLimit
	}
	if data.CountU*data.CountV != len(data.Poles) || data.Weights != nil && len(data.Weights) != len(data.Poles) {
		return empty, ErrInvalid
	}
	if e := validateKnots(data.KnotsU, data.DegreeU, data.CountU, domain.U0, domain.U1); e != nil {
		return empty, e
	}
	if e := validateKnots(data.KnotsV, data.DegreeV, data.CountV, domain.V0, domain.V1); e != nil {
		return empty, e
	}
	n := net{pu: data.DegreeU, pv: data.DegreeV, nu: data.CountU, nv: data.CountV}
	for _, k := range data.KnotsU {
		x := number(k)
		b.check(x)
		n.ku = append(n.ku, x)
	}
	for _, k := range data.KnotsV {
		x := number(k)
		b.check(x)
		n.kv = append(n.kv, x)
	}
	n.c = make([]control, len(data.Poles))
	d := [4]q{number(domain.U0), number(domain.U1), number(domain.V0), number(domain.V1)}
	origin := data.Poles[0]
	if !origin.IsFinite() {
		return empty, ErrInvalid
	}
	base := [3]q{number(origin.X), number(origin.Y), number(origin.Z)}
	for _, x := range base {
		b.check(x)
	}
	for _, x := range d {
		b.check(x)
	}
	if b.err != nil {
		return empty, b.err
	}
	position := [3]q{new(big.Rat), new(big.Rat), new(big.Rat)}
	var minWeight, maxWeight q
	result := Result{Domain: domain}
	for i, p := range data.Poles {
		if !b.tick() {
			return empty, b.err
		}
		if !p.IsFinite() {
			return empty, ErrInvalid
		}
		w := 1.
		if data.Weights != nil {
			w = data.Weights[i]
		}
		if !finite(w) || w <= 0 {
			return empty, fmt.Errorf("%w: weights must be positive", ErrInvalid)
		}
		weight := number(w)
		coords := [3]q{number(p.X), number(p.Y), number(p.Z)}
		isActive := active(n.ku, n.pu, i/n.nv, d[0], d[1]) && active(n.kv, n.pv, i%n.nv, d[2], d[3])
		if isActive {
			result.ActiveControls++
			if minWeight == nil || weight.Cmp(minWeight) < 0 {
				minWeight = weight
			}
			if maxWeight == nil || weight.Cmp(maxWeight) > 0 {
				maxWeight = weight
			}
		}
		for k := 0; k < 3; k++ {
			delta := sub(coords[k], base[k])
			n.c[i][k] = mul(weight, delta)
			b.check(n.c[i][k])
			if isActive && absolute(delta).Cmp(position[k]) > 0 {
				position[k] = absolute(delta)
			}
		}
		n.c[i][3] = weight
		b.check(weight)
	}
	if b.err != nil {
		return empty, b.err
	}
	if minWeight == nil {
		return empty, ErrInvalid
	}
	du, dv := b.derivative(n, true), b.derivative(n, false)
	duu, duv, dvv := b.derivative(du, true), b.derivative(du, false), b.derivative(dv, false)
	h := [5]control{b.hull(du, d), b.hull(dv, d), b.hull(duu, d), b.hull(duv, d), b.hull(dvv, d)}
	if b.err != nil {
		return empty, b.err
	}
	for k := 0; k < 3; k++ {
		u := div(add(h[U][k], mul(position[k], h[U][3])), minWeight)
		v := div(add(h[V][k], mul(position[k], h[V][3])), minWeight)
		uu := div(add(add(h[UU][k], mul(big.NewRat(2, 1), mul(u, h[U][3]))), mul(position[k], h[UU][3])), minWeight)
		uv := div(add(add(add(h[UV][k], mul(u, h[V][3])), mul(v, h[U][3])), mul(position[k], h[UV][3])), minWeight)
		vv := div(add(add(h[VV][k], mul(big.NewRat(2, 1), mul(v, h[V][3]))), mul(position[k], h[VV][3])), minWeight)
		for j, x := range []q{u, v, uu, uv, vv} {
			b.check(x)
			result.Components[j][k] = new(big.Rat).Set(x)
		}
	}
	if b.err != nil {
		return empty, b.err
	}
	norms := []*float64{&result.DU, &result.DV, &result.DUU, &result.DUV, &result.DVV}
	for i, p := range norms {
		sum := new(big.Rat)
		for _, x := range result.Components[i] {
			sum.Add(sum, mul(x, x))
		}
		b.check(sum)
		if b.err != nil {
			return empty, b.err
		}
		v, e := sqrtUpper(sum)
		if e != nil {
			return empty, e
		}
		*p = v
	}
	result.WeightLower, _ = minWeight.Float64()
	result.WeightUpper, _ = maxWeight.Float64()
	if !b.tick() {
		return empty, b.err
	}
	return result, nil
}
func sqrtUpper(x q) (float64, error) {
	if x.Sign() == 0 {
		return 0, nil
	}
	f := new(big.Float).SetPrec(256).SetRat(x)
	f.Sqrt(f)
	v, _ := f.Float64()
	if math.IsInf(v, 0) {
		return 0, ErrRange
	}
	for mul(number(v), number(v)).Cmp(x) < 0 {
		v = math.Nextafter(v, math.Inf(1))
		if math.IsInf(v, 0) {
			return 0, ErrRange
		}
	}
	return v, nil
}

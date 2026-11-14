package geom

import (
	"context"
	"math/big"
)

// TorusCylinderAreaOptions bounds transcendental enclosure precision and one
// shared arithmetic ledger. AreaTolerance is a positive rational width.
// PrecisionBits defaults64 and lies in16..256. Insufficient precision is an
// explicit ErrConvergence, never a relaxed requested width.
type TorusCylinderAreaOptions struct {
	AreaTolerance *big.Rat
	PrecisionBits int
	Arithmetic    SurfacePatchOptions
}

// TorusCylinderArea keeps original support ownership: slots are torus, wall,
// lower cap and upper cap. Absent supports have zero area and normal sign0.
// Wall normal sign is -1 in Difference; other retained supports use their
// original outward normals. Bounds are areas of exposed retained boundaries.
type TorusCylinderArea struct {
	Lower, Upper        *big.Rat
	BySupport           [4][2]*big.Rat
	Owners              BoreBoundaryMask
	MaterialNormalSigns [4]int
	ExactOperations     int
}

// torusCylinderAtan encloses atan(x), x>=0, using exact range reduction and
// alternating rational series whose argument is always within[-1/2,1/2].
func torusCylinderAtan(ctx context.Context, x *big.Rat, pi [2]*big.Rat, tol *big.Rat, terms int, a *sphereOverlapArithmetic) ([2]*big.Rat, error) {
	if x.Sign() < 0 {
		return [2]*big.Rat{}, ErrInvalid
	}
	if x.Cmp(big.NewRat(1, 1)) > 0 {
		v, e := torusCylinderAtan(ctx, a.div(big.NewRat(1, 1), x), pi, tol, terms, a)
		if e != nil {
			return v, e
		}
		return [2]*big.Rat{a.sub(a.div(pi[0], big.NewRat(2, 1)), v[1]), a.sub(a.div(pi[1], big.NewRat(2, 1)), v[0])}, a.err
	}
	shifted := x.Cmp(big.NewRat(1, 2)) > 0
	if shifted {
		x = a.div(a.sub(x, big.NewRat(1, 1)), a.add(x, big.NewRat(1, 1)))
	}
	if a.err != nil {
		return [2]*big.Rat{}, a.err
	}
	v, e := helicalAtanSmall(ctx, x, tol, terms, a.b)
	if e != nil {
		return v, e
	}
	if shifted {
		for i := range v {
			v[i] = a.add(v[i], a.div(pi[i], big.NewRat(4, 1)))
		}
	}
	return v, a.err
}

// BoundArea proves retained areas using a=c-R, h=sqrt(r²-a²), and
// alpha=2*atan((r+a)/h). Toroidal Common area is4*pi*r*(R*alpha-h),
// from integrating r*(R+r*cos(theta)) over the retained meridian arc.
// Common/Difference wall area is4*pi*c*h; Union retains its complement in
// the finite cylinder wall and both cap disks. The strict constructor proves
// transverse seams and caps clear of the torus, so seams have zero area.
// All sqrt/pi/angle arithmetic is outward rational, and failures are atomic.
func (s TorusCylinderMaterial) BoundArea(ctx context.Context, o TorusCylinderAreaOptions) (out TorusCylinderArea, err error) {
	defer func() {
		if err != nil {
			out = TorusCylinderArea{}
		}
	}()
	if ctx == nil || s.R == nil || o.AreaTolerance == nil || o.AreaTolerance.Sign() <= 0 {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	bits := o.PrecisionBits
	if bits == 0 {
		bits = 64
	}
	if bits < 16 || bits > 256 {
		return out, ErrInvalid
	}
	limits, e := patchOptions(o.Arithmetic)
	if e != nil {
		return out, e
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	a := &sphereOverlapArithmetic{ctx: ctx, b: b}
	for _, q := range []*big.Rat{s.R, s.r, s.c, s.lower, s.upper, o.AreaTolerance} {
		a.check(q)
	}
	if a.err != nil {
		return out, a.err
	}
	tol := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), uint(bits+16)))
	terms := bits*4 + 64
	u, e := helicalAtanSmall(ctx, big.NewRat(1, 5), tol, terms, b)
	if e != nil {
		return out, e
	}
	v, e := helicalAtanSmall(ctx, big.NewRat(1, 239), tol, terms, b)
	if e != nil {
		return out, e
	}
	pi := [2]*big.Rat{a.sub(a.mul(big.NewRat(16, 1), u[0]), a.mul(big.NewRat(4, 1), v[1])), a.sub(a.mul(big.NewRat(16, 1), u[1]), a.mul(big.NewRat(4, 1), v[0]))}
	delta := a.sub(s.c, s.R)
	h2 := a.sub(a.mul(s.r, s.r), a.mul(delta, delta))
	if a.err != nil {
		return out, a.err
	}
	lo, hi, e := rationalSqrtBounds(h2, bits, b)
	if e != nil {
		return out, e
	}
	if lo.Sign() <= 0 {
		return out, ErrConvergence
	}
	h := [2]*big.Rat{lo, hi}
	num := a.add(s.r, delta)
	al, e := torusCylinderAtan(ctx, a.div(num, hi), pi, tol, terms, a)
	if e != nil {
		return out, e
	}
	au, e := torusCylinderAtan(ctx, a.div(num, lo), pi, tol, terms, a)
	if e != nil {
		return out, e
	}
	alpha := [2]*big.Rat{a.mul(big.NewRat(2, 1), al[0]), a.mul(big.NewRat(2, 1), au[1])}
	positive := func(v *big.Rat) *big.Rat {
		if v.Sign() < 0 {
			return new(big.Rat)
		}
		return v
	}
	commonInner := [2]*big.Rat{positive(a.sub(a.mul(s.R, alpha[0]), h[1])), a.sub(a.mul(s.R, alpha[1]), h[0])}
	var common, full, wall, totalWall [2]*big.Rat
	for i := 0; i < 2; i++ {
		common[i] = a.mul(big.NewRat(4, 1), a.mul(pi[i], a.mul(s.r, commonInner[i])))
		full[i] = a.mul(big.NewRat(4, 1), a.mul(a.mul(pi[i], pi[i]), a.mul(s.R, s.r)))
		wall[i] = a.mul(big.NewRat(4, 1), a.mul(pi[i], a.mul(s.c, h[i])))
		totalWall[i] = a.mul(big.NewRat(2, 1), a.mul(pi[i], a.mul(s.c, a.sub(s.upper, s.lower))))
	}
	for i := range out.BySupport {
		out.BySupport[i] = [2]*big.Rat{new(big.Rat), new(big.Rat)}
	}
	out.BySupport[0] = common
	out.BySupport[1] = wall
	out.Owners = 3
	out.MaterialNormalSigns = [4]int{1, 1, 0, 0}
	if s.op != TorusCylinderCommon {
		out.BySupport[0] = [2]*big.Rat{positive(a.sub(full[0], common[1])), a.sub(full[1], common[0])}
	}
	if s.op == TorusCylinderDifference {
		out.MaterialNormalSigns[1] = -1
	}
	if s.op == TorusCylinderUnion {
		out.BySupport[1] = [2]*big.Rat{positive(a.sub(totalWall[0], wall[1])), a.sub(totalWall[1], wall[0])}
		for j := 2; j < 4; j++ {
			for i := 0; i < 2; i++ {
				out.BySupport[j][i] = a.mul(pi[i], a.mul(s.c, s.c))
			}
		}
		out.Owners = 15
		out.MaterialNormalSigns = [4]int{1, 1, 1, 1}
	}
	out.Lower = new(big.Rat)
	out.Upper = new(big.Rat)
	for _, v := range out.BySupport {
		out.Lower = a.add(out.Lower, v[0])
		out.Upper = a.add(out.Upper, v[1])
	}
	width := a.sub(out.Upper, out.Lower)
	if a.err != nil {
		return out, a.err
	}
	if width.Cmp(o.AreaTolerance) > 0 {
		return out, ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	out.ExactOperations = b.operations
	return out, nil
}

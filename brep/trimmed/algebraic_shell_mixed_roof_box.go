package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

const mixedRoofMaxDegree = geom.ExactPolynomialMaxDegree / 2

// Used only with the independently validated immutable input shells of a mixed
// Boolean. Complete rectangular faces, positive projected injectivity, original
// continuous edge agreement and the six supporting surfaces identify exactly
// the region floor <= Z <= roof(X,Y) over the stated rectangle.
type mixedRoofBox struct {
	roof       *geom.SurfacePatch
	patches    [6]*geom.SurfacePatch
	power      [3][3]*big.Rat
	linePower  [][]*big.Rat
	weight     [][]*big.Rat
	minWeight  *big.Rat
	floor      *big.Rat
	domain     geom.ParameterRectangle
	roofFace   int
	floorFace  int
	sideFaces  [4]int // X min, X max, Y min, Y max.
	operations int
}

func mixedFullRectangle(f Face, d geom.ParameterRectangle) bool {
	if len(f.Outer) != 4 || len(f.Holes) != 0 {
		return false
	}
	corners := [4]geom.Vec2{{X: d.UMin, Y: d.VMin}, {X: d.UMax, Y: d.VMin}, {X: d.UMax, Y: d.VMax}, {X: d.UMin, Y: d.VMax}}
	for start := 0; start < 4; start++ {
		ok := true
		for j, u := range f.Outer {
			ok = ok && u.Curve == nil && u.UVStart == corners[(start+j)%4] && u.UVEnd == corners[(start+j+1)%4]
		}
		if ok {
			return true
		}
	}
	return false
}
func mixedCoordinatePlane(axis int, value float64) [3]geom.Vec3 {
	switch axis {
	case 0:
		return [3]geom.Vec3{{X: value}, {X: value, Y: 1}, {X: value, Z: 1}}
	case 1:
		return [3]geom.Vec3{{Y: value}, {Y: value, Z: 1}, {X: 1, Y: value}}
	default:
		return [3]geom.Vec3{{Z: value}, {X: 1, Z: value}, {Y: 1, Z: value}}
	}
}
func proveMixedRoofBox(ctx context.Context, s Solid, roof int, limit int) (*mixedRoofBox, error) {
	return proveMixedRoofBoxMode(ctx, s, roof, limit, false)
}

func proveMixedRoofBoxMode(ctx context.Context, s Solid, roof int, limit int, rationalHeight bool) (*mixedRoofBox, error) {
	if ctx == nil || len(s.faces) != 6 || roof < 0 || roof >= 6 || s.faces[roof].Reversed {
		return nil, geom.ErrUnrepresentable
	}
	r := &mixedRoofBox{}
	patches := r.patches[:]
	remaining := func() (int, error) {
		n := limit - r.operations
		if n < 1 {
			return 0, geom.ErrConvergence
		}
		return n, nil
	}
	for i, f := range s.faces {
		n, e := remaining()
		if e != nil {
			return nil, e
		}
		p, e := exactFaceSupportPatch(f, geom.SurfacePatchOptions{MaxOperations: n, MaxCoefficientBits: 4096})
		if e != nil {
			return nil, e
		}
		r.operations += p.ExactOperations()
		if !mixedFullRectangle(f, p.Domain()) {
			return nil, geom.ErrUnrepresentable
		}
		patches[i] = p
	}
	identity := func(p *geom.SurfacePatch) error {
		for axis := 0; axis < 2; axis++ {
			n, e := remaining()
			if e != nil {
				return e
			}
			q, e := geom.CertifySurfaceAffineCoordinate(ctx, p, axis, axis, big.NewRat(1, 1), new(big.Rat), geom.SurfaceAffineCoordinateOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: n, MaxCoefficientBits: 4096}})
			r.operations += q.ExactOperations
			if e != nil {
				return e
			}
			if !q.Proven {
				return geom.ErrUnrepresentable
			}
		}
		return nil
	}
	if e := identity(patches[roof]); e != nil {
		return nil, e
	}
	r.roof = patches[roof]
	r.roofFace = roof
	r.domain = r.roof.Domain()
	var err error
	if !rationalHeight {
		r.power, err = mixedRoofPower(r.roof)
		if err != nil {
			return nil, err
		}
	} else {
		n, err := remaining()
		if err != nil {
			return nil, err
		}
		coeff, minWeight, work, err := mixedRoofWholePower(ctx, r.roof, r.domain, n)
		r.operations += work
		if err != nil {
			return nil, err
		}
		r.linePower = make([][]*big.Rat, len(coeff))
		r.weight = make([][]*big.Rat, len(coeff))
		r.minWeight = minWeight
		constant := true
		for a, row := range coeff {
			r.linePower[a] = make([]*big.Rat, len(row))
			r.weight[a] = make([]*big.Rat, len(row))
			for b, c := range row {
				// The identity projection gives Hx=X*W and Hy=Y*W.
				// Each axis of W is one degree lower than its corresponding
				// homogeneous model coordinate, by the proved identity map.
				if (a >= len(coeff)-1 || b >= len(row)-1) && c[3].Sign() != 0 {
					return nil, geom.ErrUnrepresentable
				}
				r.linePower[a][b], r.weight[a][b] = c[2], c[3]
				if a+b != 0 && c[3].Sign() != 0 {
					constant = false
				}
			}
		}
		if constant {
			w := r.weight[0][0]
			for _, row := range r.linePower {
				for _, c := range row {
					c.Quo(c, w)
					r.operations++
					if r.operations > limit || !mixedRoofFits(c, 4096) {
						return nil, geom.ErrConvergence
					}
				}
			}
			r.weight, r.minWeight = nil, nil
		}
	}
	floor := -1
	for i, p := range patches {
		if i == roof {
			continue
		}
		n, err := remaining()
		if err != nil {
			return nil, err
		}
		c, w, work, conversionErr := mixedRoofWholePower(ctx, p, p.Domain(), n)
		r.operations += work
		if conversionErr != nil {
			if conversionErr != geom.ErrUnrepresentable {
				return nil, conversionErr
			}
			continue
		}
		if w.Sign() <= 0 {
			continue
		}
		height := new(big.Rat).Quo(c[0][0][2], c[0][0][3])
		r.operations++
		if r.operations > limit || !mixedRoofFits(height, 4096) {
			return nil, geom.ErrConvergence
		}
		constant := true
		for _, row := range c {
			for _, h := range row {
				value := new(big.Rat).Mul(height, h[3])
				r.operations++
				if r.operations > limit || !mixedRoofFits(value, 4096) {
					return nil, geom.ErrConvergence
				}
				if value.Cmp(h[2]) != 0 {
					constant = false
				}
			}
		}
		if !constant {
			continue
		}
		if floor >= 0 || !s.faces[i].Reversed || p.Domain() != r.domain {
			return nil, geom.ErrUnrepresentable
		}
		if e := identity(p); e != nil {
			return nil, e
		}
		floor = i
		r.floor = height
	}
	if floor < 0 {
		return nil, geom.ErrUnrepresentable
	}
	r.floorFace = floor
	height, exact := r.floor.Float64()
	if !exact {
		return nil, geom.ErrUnrepresentable
	}
	plane := mixedCoordinatePlane(2, height)
	n, e := remaining()
	if e != nil {
		return nil, e
	}
	above, e := geom.CertifySurfaceSupportingPlane(ctx, r.roof, plane[0], plane[1], plane[2], geom.SurfaceSupportingPlaneOptions{MaxOperations: n, MaxCoefficientBits: 4096})
	r.operations += above.ExactOperations
	if e != nil {
		return nil, e
	}
	if !above.Complete || above.Coincident || above.HalfspaceSign != 1 || above.SideMask != 0 || above.CornerMask != 0 {
		return nil, geom.ErrUnrepresentable
	}
	bounds := [4]float64{r.domain.UMin, r.domain.UMax, r.domain.VMin, r.domain.VMax}
	seen := [4]bool{}
	for i, p := range patches {
		if i == roof || i == floor {
			continue
		}
		matched := -1
		for side, value := range bounds {
			plane := mixedCoordinatePlane(side/2, value)
			n, e := remaining()
			if e != nil {
				return nil, e
			}
			q, e := geom.CertifySurfaceSupportingPlane(ctx, p, plane[0], plane[1], plane[2], geom.SurfaceSupportingPlaneOptions{MaxOperations: n, MaxCoefficientBits: 4096})
			r.operations += q.ExactOperations
			if e != nil {
				return nil, e
			}
			if q.Complete && q.Coincident {
				matched = side
				break
			}
		}
		if matched < 0 || seen[matched] {
			return nil, geom.ErrUnrepresentable
		}
		seen[matched] = true
		r.sideFaces[matched] = i
		n, e := remaining()
		if e != nil {
			return nil, e
		}
		axes := [2]int{1, 2}
		if matched/2 == 1 {
			axes = [2]int{0, 2}
		}
		q, e := geom.CertifySurfaceInjectivity(p, geom.SurfaceInjectivityOptions{ProjectionAxes: axes, MaxOperations: n, MaxCoefficientBits: 4096})
		r.operations += q.ExactOperations
		if e != nil {
			return nil, e
		}
		if !q.ProvenInjective {
			return nil, geom.ErrConvergence
		}
		if e := ctx.Err(); e != nil {
			return nil, e
		}
	}
	return r, nil
}

// Polynomial positive constant homogeneous weight only; no rational weight is
// silently discarded. Returned coefficients use normalized original UV.
func mixedRoofPower(p *geom.SurfacePatch) ([3][3]*big.Rat, error) {
	var out [3][3]*big.Rat
	c, _, ok := p.RationalBiquadraticCoefficients()
	if !ok || c[0][0][3].Sign() <= 0 {
		return out, geom.ErrUnrepresentable
	}
	for a := 0; a < 3; a++ {
		for b := 0; b < 3; b++ {
			if a+b > 0 && c[a][b][3].Sign() != 0 {
				return out, geom.ErrUnrepresentable
			}
			out[a][b] = new(big.Rat).Quo(c[a][b][2], c[0][0][3])
			if out[a][b].Num().BitLen() > 4096 || out[a][b].Denom().BitLen() > 4096 {
				return out, geom.ErrConvergence
			}
		}
	}
	return out, nil
}
func mixedRoofTrianglePower(p *geom.SurfacePatch, xy [3][2]*big.Rat) (trianglePower5, error) {
	out := zeroTrianglePower5()
	coeff, e := mixedRoofPower(p)
	if e != nil {
		return out, e
	}
	d := p.Domain()
	var powers [2][3]trianglePower5
	for axis := 0; axis < 2; axis++ {
		lo, hi := d.UMin, d.UMax
		if axis == 1 {
			lo, hi = d.VMin, d.VMax
		}
		v := [3]*big.Rat{}
		for j := 0; j < 3; j++ {
			v[j] = new(big.Rat).Quo(new(big.Rat).Sub(xy[j][axis], ratInset(lo)), new(big.Rat).Sub(ratInset(hi), ratInset(lo)))
			if v[j].Sign() < 0 || v[j].Cmp(big.NewRat(1, 1)) > 0 {
				return out, geom.ErrUnrepresentable
			}
		}
		linear := zeroTrianglePower5()
		linear[0][0].Set(v[0])
		linear[1][0].Sub(v[1], v[0])
		linear[0][1].Sub(v[2], v[0])
		powers[axis][0] = zeroTrianglePower5()
		powers[axis][0][0][0].SetInt64(1)
		powers[axis][1] = linear
		powers[axis][2] = multiplyTrianglePower5(linear, linear)
	}
	for a := 0; a < 3; a++ {
		for b := 0; b < 3; b++ {
			if coeff[a][b].Sign() == 0 {
				continue
			}
			term := multiplyTrianglePower5(powers[0][a], powers[1][b])
			for i := 0; i <= 5; i++ {
				for j := 0; j <= 5-i; j++ {
					out[i][j].Add(out[i][j], new(big.Rat).Mul(coeff[a][b], term[i][j]))
				}
			}
		}
	}
	return out, nil
}
func mixedRoofPolynomialRange(p trianglePower5) (*big.Rat, *big.Rat) {
	var lo, hi *big.Rat
	for i := 0; i <= 5; i++ {
		for j := 0; j <= 5-i; j++ {
			v := new(big.Rat)
			for a := 0; a <= i; a++ {
				for b := 0; b <= j; b++ {
					v.Add(v, new(big.Rat).Mul(p[a][b], &triangleDegreeFiveFactors[i][j][a][b]))
				}
			}
			if lo == nil || v.Cmp(lo) < 0 {
				lo = new(big.Rat).Set(v)
			}
			if hi == nil || v.Cmp(hi) > 0 {
				hi = new(big.Rat).Set(v)
			}
		}
	}
	return lo, hi
}

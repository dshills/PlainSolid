package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// NativeObliqueCavityLineOptions bounds the original line proof separately
// from cumulative native-ownership sign tests. Ownership defaults to 1M exact
// operations, 4096 bits and 1024 subdivisions. Fixed coefficient preparation is
// charged 128 operations; every subsequent algebraic sign proof is charged.
type NativeObliqueCavityLineOptions struct {
	Line      geom.ToroidLineOptions
	Ownership geom.ExactCurvePlaneEventOptions
}

// NativeObliqueCavityLine keeps actual boundary hits and coincident intervals,
// not material interiors. HitFaces and interval endpoint incidence include all
// exact original chart seams. IntervalFaces owns the whole open interval.
type NativeObliqueCavityLine struct {
	Boundary                            geom.BoreLineBoundary
	HitFaces                            [][]int
	IntervalFaces, StartFaces, EndFaces [][]int
	OwnershipOperations                 int
}

func (p *PreparedObliqueTorusCavity) IntersectRationalLineBoundary(ctx context.Context, origin, direction [3]*big.Rat, min, max *big.Rat, o NativeObliqueCavityLineOptions) (out NativeObliqueCavityLine, err error) {
	defer func() {
		if err != nil {
			out = NativeObliqueCavityLine{}
		}
	}()
	if p == nil || p.major == nil {
		return out, geom.ErrInvalid
	}
	proof := o.Ownership
	if proof.MaxOperations == 0 {
		proof.MaxOperations = 1000000
	}
	if proof.MaxCoefficientBits == 0 {
		proof.MaxCoefficientBits = 4096
	}
	if proof.MaxSubdivisions == 0 {
		proof.MaxSubdivisions = 1024
	}
	if proof.MaxOperations < 128 || proof.MaxOperations > 100000000 || proof.MaxCoefficientBits < 64 || proof.MaxCoefficientBits > 4096 || proof.MaxSubdivisions < 1 || proof.MaxSubdivisions > 65536 {
		return out, geom.ErrConvergence
	}
	check := func(q *big.Rat) bool {
		return q != nil && q.Num().BitLen() <= proof.MaxCoefficientBits && q.Denom().BitLen() <= proof.MaxCoefficientBits
	}
	for i := 0; i < 3; i++ {
		if !check(origin[i]) || !check(direction[i]) {
			return out, geom.ErrConvergence
		}
	}
	frame, e := p.source.TorusFrame()
	if e != nil {
		return out, e
	}
	var local, dir [3]*big.Rat
	for i := 0; i < 3; i++ {
		local[i], dir[i] = new(big.Rat), new(big.Rat)
		for j := 0; j < 3; j++ {
			if !check(frame[j][i]) || !check(frame[j][3]) {
				return out, geom.ErrConvergence
			}
			local[i].Add(local[i], new(big.Rat).Mul(frame[j][i], new(big.Rat).Sub(origin[j], frame[j][3])))
			dir[i].Add(dir[i], new(big.Rat).Mul(frame[j][i], direction[j]))
		}
		if !check(local[i]) || !check(dir[i]) {
			return out, geom.ErrConvergence
		}
	}
	poly := [][]*big.Rat{{local[0], dir[0]}, {local[1], dir[1]}, {local[2], dir[2]}, {new(big.Rat).Neg(new(big.Rat).Mul(p.major, p.major)), new(big.Rat), new(big.Rat)}}
	for i := 0; i < 2; i++ {
		poly[3][0].Add(poly[3][0], new(big.Rat).Mul(local[i], local[i]))
		poly[3][1].Add(poly[3][1], new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(local[i], dir[i])))
		poly[3][2].Add(poly[3][2], new(big.Rat).Mul(dir[i], dir[i]))
	}
	for _, row := range poly {
		for _, q := range row {
			if !check(q) {
				return out, geom.ErrConvergence
			}
		}
	}
	out.OwnershipOperations = 128
	out.Boundary, err = p.source.IntersectRationalLineBoundary(ctx, origin, direction, min, max, o.Line)
	if err != nil {
		return out, err
	}
	faces := func(root *geom.ExactAlgebraicRoot, mask geom.BoreBoundaryMask) ([]int, error) {
		var result []int
		for _, owner := range []struct {
			mask geom.BoreBoundaryMask
			face int
		}{{geom.TorusCylinderWall, 0}, {geom.TorusCylinderLowerCap, 1}, {geom.TorusCylinderUpperCap, 2}} {
			if mask&owner.mask != 0 {
				result = append(result, owner.face)
			}
		}
		if mask&geom.TorusCylinderTorus == 0 {
			return result, nil
		}
		var signs [4]int
		for i := range signs {
			left := proof.MaxOperations - out.OwnershipOperations
			if left < 1 {
				return nil, geom.ErrConvergence
			}
			bound := proof
			bound.MaxOperations = left
			s, e := root.SignPolynomial(ctx, poly[i], bound)
			out.OwnershipOperations += s.Operations
			if e != nil {
				return nil, e
			}
			signs[i] = s.Sign
		}
		quarters := func(x, y int) []int {
			var q []int
			for i, s := range [][2]int{{1, 1}, {-1, 1}, {-1, -1}, {1, -1}} {
				if (x == 0 || x == s[0]) && (y == 0 || y == s[1]) {
					q = append(q, i)
				}
			}
			return q
		}
		if signs[0] == 0 && signs[1] == 0 {
			return nil, geom.ErrDomain
		}
		for _, u := range quarters(signs[0], signs[1]) {
			for _, v := range quarters(signs[3], signs[2]) {
				result = append(result, 3+4*u+v)
			}
		}
		return result, nil
	}
	for _, hit := range out.Boundary.Hits {
		ids, e := faces(hit.Parameter, hit.Boundary)
		if e != nil {
			return out, e
		}
		out.HitFaces = append(out.HitFaces, ids)
	}
	for _, interval := range out.Boundary.Intervals {
		if interval.Boundary&geom.TorusCylinderTorus != 0 {
			return out, geom.ErrDomain
		}
		ids, e := faces(nil, interval.Boundary)
		if e != nil {
			return out, e
		}
		start, e := faces(interval.Start, interval.StartBoundary)
		if e != nil {
			return out, e
		}
		end, e := faces(interval.End, interval.EndBoundary)
		if e != nil {
			return out, e
		}
		out.IntervalFaces = append(out.IntervalFaces, ids)
		out.StartFaces = append(out.StartFaces, start)
		out.EndFaces = append(out.EndFaces, end)
	}
	return out, nil
}

// IntersectPlacedRationalLineBoundary preserves the original rational t under
// a proper whole occurrence. Only origin/direction are pulled back; root and
// native owner identities remain unchanged. The two transforms have separate
// 4096-bit limits, in addition to the source/ownership ledgers.
func (p *PreparedObliqueTorusCavity) IntersectPlacedRationalLineBoundary(ctx context.Context, pose placement.RationalRigid, origin, direction [3]*big.Rat, min, max *big.Rat, o NativeObliqueCavityLineOptions) (NativeObliqueCavityLine, error) {
	a, e := pose.TransformPoint(ctx, origin, true, 4096)
	if e != nil {
		return NativeObliqueCavityLine{}, e
	}
	d, e := pose.TransformVector(ctx, direction, true, 4096)
	if e != nil {
		return NativeObliqueCavityLine{}, e
	}
	return p.IntersectRationalLineBoundary(ctx, a, d, min, max, o)
}

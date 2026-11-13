package geom

import "math/big"

// SurfaceInjectivityOptions chooses two distinct world projection coordinates
// (0=x,1=y,2=z); default zero pair selects x,y. The exact center Jacobian inverse
// is a fixed preconditioner for the entire parameter rectangle. Zero limits
// select depth 12, 4096 cells, 2M operations and 16384 coefficient bits.
type SurfaceInjectivityOptions struct {
	ProjectionAxes                                        [2]int
	MaxDepth, MaxCells, MaxOperations, MaxCoefficientBits int
}

// SurfaceInjectivityCell certifies a positive-definite symmetric part of the
// preconditioned projected Jacobian everywhere in Domain. DiagonalMin bounds
// the two diagonal entries below; DeterminantMin bounds its determinant below.
// StrictPositive is exact even when outward float bounds underflow to zero.
type SurfaceInjectivityCell struct {
	Domain         ParameterRectangle
	DiagonalMin    [2]float64
	DeterminantMin float64
	StrictPositive bool
	Depth          int
}

// SurfaceInjectivity reports a sufficient global graph/injectivity proof.
// ProvenInjective means one fixed preconditioner makes the projected map
// strictly monotone over the entire convex parameter rectangle. Consequently
// both its projection and the spatial surface are injective. This is not a
// necessary condition: unresolved cells do not prove a self-intersection.
// C0 interior knots are allowed: continuity and positive one-sided derivatives
// suffice by integrating along parameter segments. C1 continuity is not claimed.
type SurfaceInjectivity struct {
	// ProjectionOrientation is the exact projected Jacobian determinant sign,
	// populated only when ProvenInjective.
	ProjectionOrientation         int
	ProjectionAxes                [2]int
	Certified                     []SurfaceInjectivityCell
	Unresolved, Pending           []ParameterRectangle
	ProvenInjective               bool
	CenterInvertible              bool
	VisitedCells, ExactOperations int
}

func projectedSymmetricCertificate(p *SurfacePatch, axes [2]int, inverse [2][2]*big.Rat, b *polynomialBudget) (SurfaceInjectivityCell, bool, error) {
	cell := SurfaceInjectivityCell{Domain: p.Domain()}
	_, derivative, e := surfaceDifferentialIntervals(p, -1, 0, b)
	if e != nil {
		return cell, false, e
	}
	var matrix [2][2]exactInterval
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			sum := exactInterval{new(big.Rat), new(big.Rat)}
			for k := 0; k < 2; k++ {
				term, e := intervalMultiply(exactInterval{inverse[i][k], inverse[i][k]}, derivative[axes[k]][j], b)
				if e != nil {
					return cell, false, e
				}
				sum, e = boundedExactInterval(new(big.Rat).Add(sum.lo, term.lo), new(big.Rat).Add(sum.hi, term.hi), b)
				if e != nil {
					return cell, false, e
				}
			}
			matrix[i][j] = sum
		}
	}
	cell.DiagonalMin = [2]float64{rationalFloatBound(matrix[0][0].lo, true), rationalFloatBound(matrix[1][1].lo, true)}
	if matrix[0][0].lo.Sign() <= 0 || matrix[1][1].lo.Sign() <= 0 {
		return cell, false, nil
	}
	offLo := new(big.Rat).Quo(new(big.Rat).Add(matrix[0][1].lo, matrix[1][0].lo), big.NewRat(2, 1))
	offHi := new(big.Rat).Quo(new(big.Rat).Add(matrix[0][1].hi, matrix[1][0].hi), big.NewRat(2, 1))
	square := new(big.Rat).Mul(offLo, offLo)
	other := new(big.Rat).Mul(offHi, offHi)
	if other.Cmp(square) > 0 {
		square = other
	}
	det := new(big.Rat).Sub(new(big.Rat).Mul(matrix[0][0].lo, matrix[1][1].lo), square)
	if e := b.check(det); e != nil {
		return cell, false, e
	}
	cell.DeterminantMin = rationalFloatBound(det, true)
	cell.StrictPositive = det.Sign() > 0
	return cell, cell.StrictPositive, nil
}

// CertifySurfaceInjectivity applies a global strong-monotonicity sufficient
// test to a chosen two-coordinate projection. The preconditioner is never
// changed between cells: independently oriented local proofs would not imply
// global injectivity. Exact rational inequalities decide each certificate.
// The input SurfacePatch constructor's continuous clamped-knot contract is
// required; no periodic seam identification or disconnected domain is inferred.
func CertifySurfaceInjectivity(p *SurfacePatch, o SurfaceInjectivityOptions) (result SurfaceInjectivity, err error) {
	if p == nil || len(p.net) == 0 {
		return result, ErrInvalid
	}
	axes := o.ProjectionAxes
	if axes == ([2]int{}) {
		axes = [2]int{0, 1}
	}
	if axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return result, ErrInvalid
	}
	result.ProjectionAxes = axes
	if o.MaxDepth == 0 {
		o.MaxDepth = 12
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxCells < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return result, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.ExactOperations = budget.operations }()
	// Select one Bezier piece containing the original domain midpoint to build
	// a fixed exact preconditioner. At a C0 knot either one-sided piece suffices.
	d := p.Domain()
	centerU, centerV := d.UMin+(d.UMax-d.UMin)/2, d.VMin+(d.VMax-d.VMin)/2
	centerPatch := p
	for {
		t, u, knot := patchSplitChoice(centerPatch)
		if !knot {
			break
		}
		left, right, e := centerPatch.split(t, u, budget)
		if e != nil {
			result.Pending = append(result.Pending, d)
			return result, e
		}
		x := centerU
		if !u {
			x = centerV
		}
		if x < t {
			centerPatch = left
		} else {
			centerPatch = right
		}
	}
	_, derivative, e := exactSurfaceJet(centerPatch, new(big.Rat).SetFloat64(centerU), new(big.Rat).SetFloat64(centerV), budget)
	if e != nil {
		result.Pending = append(result.Pending, d)
		return result, e
	}
	a, b, c, f := derivative[axes[0]][0], derivative[axes[0]][1], derivative[axes[1]][0], derivative[axes[1]][1]
	det := new(big.Rat).Sub(new(big.Rat).Mul(a, f), new(big.Rat).Mul(b, c))
	if e := budget.check(det); e != nil {
		result.Pending = append(result.Pending, d)
		return result, e
	}
	if det.Sign() == 0 {
		result.Unresolved = append(result.Unresolved, d)
		return result, ErrConvergence
	}
	result.CenterInvertible = true
	inverse := [2][2]*big.Rat{{new(big.Rat).Quo(f, det), new(big.Rat).Quo(new(big.Rat).Neg(b), det)}, {new(big.Rat).Quo(new(big.Rat).Neg(c), det), new(big.Rat).Quo(a, det)}}
	for _, row := range inverse {
		for _, v := range row {
			if e := budget.check(v); e != nil {
				result.Pending = append(result.Pending, d)
				return result, e
			}
		}
	}
	type node struct {
		p     *SurfacePatch
		depth int
	}
	stack := []node{{p, 0}}
	pending := func(current *SurfacePatch) {
		if current != nil {
			result.Pending = append(result.Pending, current.Domain())
		}
		for _, n := range stack {
			result.Pending = append(result.Pending, n.p.Domain())
		}
	}
	for len(stack) > 0 {
		if result.VisitedCells >= o.MaxCells {
			pending(nil)
			return result, ErrConvergence
		}
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		result.VisitedCells++
		t, u, knot := patchSplitChoice(n.p)
		if !knot {
			cell, ok, e := projectedSymmetricCertificate(n.p, axes, inverse, budget)
			if e != nil {
				pending(n.p)
				return result, e
			}
			if ok {
				cell.Depth = n.depth
				result.Certified = append(result.Certified, cell)
				continue
			}
		}
		domain := n.p.Domain()
		if n.depth >= o.MaxDepth || (u && (t == domain.UMin || t == domain.UMax)) || (!u && (t == domain.VMin || t == domain.VMax)) {
			result.Unresolved = append(result.Unresolved, domain)
			continue
		}
		left, right, e := n.p.split(t, u, budget)
		if e != nil {
			pending(n.p)
			return result, e
		}
		stack = append(stack, node{right, n.depth + 1}, node{left, n.depth + 1})
	}
	if len(result.Unresolved) > 0 {
		return result, ErrConvergence
	}
	result.ProvenInjective = true
	result.ProjectionOrientation = det.Sign()
	return result, nil
}

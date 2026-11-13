package geom

import "math/big"

// SurfaceRegularityOptions bounds a knot-aligned regularity covering. Zero
// limits select depth 16, 4096 cells, 2M operations and 16384 coefficient bits.
// A nonzero ReferenceDirection asks for consistent normal orientation against
// that exact stored direction. Zero chooses a signed world component per cell.
type SurfaceRegularityOptions struct {
	ReferenceDirection                                    Vec3
	MaxDepth, MaxCells, MaxOperations, MaxCoefficientBits int
}

// SurfaceNormalCell encloses the scaled normal W^4*(Su cross Sv). Its three
// component ranges define a conservative cone of rays through that box, not a
// normalized spherical cap. Strict component/dot sign proves regularity.
// With a reference direction, Orientation is its exact normal-dot sign;
// otherwise Orientation is the sign of the normal component at Axis.
type SurfaceNormalCell struct {
	Domain                   ParameterRectangle
	ScaledNormal             [3]CertifiedScalarRange
	Orientation, Axis, Depth int
}

// SurfaceRegularity is a conservative parameter-domain covering. Certified
// cells are regular on each one-sided Bezier piece; Unresolved/Pending retain
// unproved regions. Derivative continuity across knot lines is not certified.
// Complete means
// the whole domain is covered by certified cells. ConsistentOrientation is
// meaningful only with a nonzero reference, and requires one sign globally.
// This says nothing about global injectivity, self-intersection or solid validity.
type SurfaceRegularity struct {
	Certified                                  []SurfaceNormalCell
	Unresolved, Pending                        []ParameterRectangle
	Complete, ConsistentOrientation            bool
	Orientation, VisitedCells, ExactOperations int
}

// CertifySurfaceRegularity subdivides exact homogeneous derivative hulls until
// one normal component (or normal-dot-reference) has strict sign. Tangent cone
// hulls containing zero remain unresolved; they are not declared singular.
func CertifySurfaceRegularity(p *SurfacePatch, o SurfaceRegularityOptions) (result SurfaceRegularity, err error) {
	if p == nil || len(p.net) == 0 || !o.ReferenceDirection.IsFinite() {
		return result, ErrInvalid
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 16
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
	reference := o.ReferenceDirection != (Vec3{})
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.ExactOperations = budget.operations }()
	type node struct {
		p     *SurfacePatch
		depth int
	}
	stack := []node{{p, 0}}
	consistent := true
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
			normal, e := patchNormalInterval(n.p, budget)
			if e != nil {
				pending(n.p)
				return result, e
			}
			cell := SurfaceNormalCell{Domain: n.p.Domain(), Axis: -1, Depth: n.depth}
			for axis := 0; axis < 3; axis++ {
				cell.ScaledNormal[axis] = certifiedRange(normal[axis])
				if !reference && cell.Orientation == 0 && normal[axis].sign() != 0 {
					cell.Axis, cell.Orientation = axis, normal[axis].sign()
				}
			}
			if reference {
				sum := exactInterval{new(big.Rat), new(big.Rat)}
				for axis := 0; axis < 3; axis++ {
					d := new(big.Rat).SetFloat64(coordinate(o.ReferenceDirection, axis))
					term, e := intervalMultiply(normal[axis], exactInterval{d, d}, budget)
					if e != nil {
						pending(n.p)
						return result, e
					}
					sum, e = boundedExactInterval(new(big.Rat).Add(sum.lo, term.lo), new(big.Rat).Add(sum.hi, term.hi), budget)
					if e != nil {
						pending(n.p)
						return result, e
					}
				}
				cell.Orientation = sum.sign()
			}
			if cell.Orientation != 0 {
				result.Certified = append(result.Certified, cell)
				if reference {
					if result.Orientation == 0 {
						result.Orientation = cell.Orientation
					} else if result.Orientation != cell.Orientation {
						consistent = false
					}
				}
				continue
			}
		}
		d := n.p.Domain()
		if n.depth >= o.MaxDepth || (u && (t == d.UMin || t == d.UMax)) || (!u && (t == d.VMin || t == d.VMax)) {
			result.Unresolved = append(result.Unresolved, d)
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
	result.Complete = true
	result.ConsistentOrientation = reference && consistent
	return result, nil
}

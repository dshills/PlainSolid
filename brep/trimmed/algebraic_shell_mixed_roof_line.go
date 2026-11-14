package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// MixedRoofLineOptions bounds the combined exact arithmetic and root
// refinements across both original operands. Zero uses 1M operations,
// 4096 coefficient bits and 8192 root subdivisions.
type MixedRoofLineOptions struct {
	geom.SurfacePatchEvaluationOptions
	MaxSubdivisions int
}

// MixedRoofLineEvent records one exposed station and all original supports
// meeting there. Before and After classify the open incident line portions,
// including portions beyond the requested clip at its endpoints.
type MixedRoofLineEvent struct {
	Parameter     *geom.ExactAlgebraicRoot
	Owners        []MixedRoofFaceOwner
	Before, After parametric.Location
}

// MixedRoofLineSpan represents the closure of an open line portion with
// constant material location. Boundary spans have original face Owners.
type MixedRoofLineSpan struct {
	Lower, Upper *geom.ExactAlgebraicRoot
	Location     parametric.Location
	Owners       []MixedRoofFaceOwner
}

// MixedRoofLineSection contains interior and exposed coincident spans plus
// isolated exact boundary events, in original line-parameter units.
type MixedRoofLineSection struct {
	Events                        []MixedRoofLineEvent
	Spans                         []MixedRoofLineSpan
	Start, End                    parametric.Location
	ExactOperations, Subdivisions int
}

type mixedRoofLineWork struct {
	o                        MixedRoofLineOptions
	operations, subdivisions int
}

func (w *mixedRoofLineWork) rat(v *big.Rat) (*big.Rat, error) {
	if w.operations >= w.o.MaxOperations || !mixedRoofFits(v, w.o.MaxCoefficientBits) {
		return nil, geom.ErrConvergence
	}
	w.operations++
	return v, nil
}

func (w *mixedRoofLineWork) rootOptions() (geom.ExactCurvePlaneEventOptions, error) {
	a, b := w.o.MaxOperations-w.operations, w.o.MaxSubdivisions-w.subdivisions
	if a < 1 || b < 1 {
		return geom.ExactCurvePlaneEventOptions{}, geom.ErrConvergence
	}
	return geom.ExactCurvePlaneEventOptions{MaxOperations: a, MaxSubdivisions: b, MaxCoefficientBits: w.o.MaxCoefficientBits}, nil
}

type mixedRoofLinePoly [7]*big.Rat

func mixedRoofEmptyPoly() (p mixedRoofLinePoly) {
	for i := range p {
		p[i] = new(big.Rat)
	}
	return p
}

func (w *mixedRoofLineWork) value(p mixedRoofLinePoly, t *big.Rat) (int, error) {
	z := new(big.Rat)
	degree := len(p) - 1
	for degree > 0 && p[degree].Sign() == 0 {
		degree--
	}
	for i := degree; i >= 0; i-- {
		if _, err := w.rat(z.Mul(z, t)); err != nil {
			return 0, err
		}
		if _, err := w.rat(z.Add(z, p[i])); err != nil {
			return 0, err
		}
	}
	return z.Sign(), nil
}

func (b *mixedRoofBox) linePolynomials(origin, direction [3]*big.Rat, w *mixedRoofLineWork) (out [6]mixedRoofLinePoly, err error) {
	for i := range out {
		out[i] = mixedRoofEmptyPoly()
	}
	d := b.domain
	lo, hi := [2]float64{d.UMin, d.VMin}, [2]float64{d.UMax, d.VMax}
	var powers [2][4][]*big.Rat
	for i := range powers {
		if out[2*i][0], err = w.rat(new(big.Rat).Sub(origin[i], ratInset(lo[i]))); err != nil {
			return out, err
		}
		if out[2*i+1][0], err = w.rat(new(big.Rat).Sub(ratInset(hi[i]), origin[i])); err != nil {
			return out, err
		}
		out[2*i][1].Set(direction[i])
		out[2*i+1][1].Neg(direction[i])
		width, err := w.rat(new(big.Rat).Sub(ratInset(hi[i]), ratInset(lo[i])))
		if err != nil {
			return out, err
		}
		a, err := w.rat(new(big.Rat).Quo(out[2*i][0], width))
		if err != nil {
			return out, err
		}
		e, err := w.rat(new(big.Rat).Quo(direction[i], width))
		if err != nil {
			return out, err
		}
		powers[i][0], powers[i][1] = []*big.Rat{big.NewRat(1, 1)}, []*big.Rat{a, e}
		maxPower := len(b.linePower) - 1
		if i == 1 {
			maxPower = len(b.linePower[0]) - 1
		}
		for k := 2; k <= maxPower; k++ {
			powers[i][k] = make([]*big.Rat, k+1)
			for j := range powers[i][k] {
				powers[i][k][j] = new(big.Rat)
			}
			for j, v := range powers[i][k-1] {
				for shift, a := range powers[i][1] {
					m, err := w.rat(new(big.Rat).Mul(v, a))
					if err != nil {
						return out, err
					}
					dst := powers[i][k][j+shift]
					if _, err := w.rat(dst.Add(dst, m)); err != nil {
						return out, err
					}
				}
			}
		}
	}
	if out[4][0], err = w.rat(new(big.Rat).Sub(origin[2], b.floor)); err != nil {
		return out, err
	}
	out[4][1].Set(direction[2])
	if b.minWeight == nil {
		out[5][0].Neg(origin[2])
		out[5][1].Neg(direction[2])
	}
	for a, row := range b.linePower {
		for k, coeff := range row {
			var weight *big.Rat
			if b.weight != nil {
				weight = b.weight[a][k]
			}
			if coeff.Sign() == 0 && (weight == nil || weight.Sign() == 0) {
				continue
			}
			for i, x := range powers[0][a] {
				for j, y := range powers[1][k] {
					v, err := w.rat(new(big.Rat).Mul(x, y))
					if err != nil {
						return out, err
					}
					if coeff.Sign() != 0 {
						z, err := w.rat(new(big.Rat).Mul(v, coeff))
						if err != nil {
							return out, err
						}
						if _, err := w.rat(out[5][i+j].Add(out[5][i+j], z)); err != nil {
							return out, err
						}
					}
					if weight != nil && weight.Sign() != 0 {
						f, err := w.rat(new(big.Rat).Mul(v, weight))
						if err != nil {
							return out, err
						}
						for shift, z := range [2]*big.Rat{origin[2], direction[2]} {
							product, err := w.rat(new(big.Rat).Mul(f, z))
							if err != nil {
								return out, err
							}
							if _, err := w.rat(out[5][i+j+shift].Sub(out[5][i+j+shift], product)); err != nil {
								return out, err
							}
						}
					}
				}
			}
		}
	}
	return out, nil
}

type mixedRoofCrossing struct {
	source    int
	direction int // Sign immediately after the root.
}

type mixedRoofLineStation struct {
	root     *geom.ExactAlgebraicRoot
	where    []mixedRoofCrossing
	min, max bool
}

func (w *mixedRoofLineWork) compare(ctx context.Context, a, b *geom.ExactAlgebraicRoot) (int, error) {
	ia, ib := a.Interval(), b.Interval()
	if w.operations >= w.o.MaxOperations {
		return 0, geom.ErrConvergence
	}
	w.operations++
	if ia.Upper.Cmp(ib.Lower) < 0 {
		return -1, nil
	}
	if ib.Upper.Cmp(ia.Lower) < 0 {
		return 1, nil
	}
	if ia.Lower.Cmp(ia.Upper) == 0 && ib.Lower.Cmp(ib.Upper) == 0 {
		return ia.Lower.Cmp(ib.Lower), nil
	}
	o, err := w.rootOptions()
	if err != nil {
		return 0, err
	}
	r, err := a.Compare(ctx, b, o)
	w.operations += r.Operations
	w.subdivisions += r.Subdivisions
	if err != nil {
		return 0, err
	}
	if !r.Proven || w.operations > w.o.MaxOperations || w.subdivisions > w.o.MaxSubdivisions {
		return 0, geom.ErrConvergence
	}
	return r.Order, nil
}

// IntersectLineRational sweeps the exact finite line origin+direction*t through
// the two original graph boxes. It composes six inward inequalities per box
// directly with the rational line and isolates the complete degree <=6
// station ledger. Coincident individual support polynomials are retained
// as side, floor, or roof spans; no root is selected by a float tolerance.
func (q *MixedRoofMaterialQuery) IntersectLineRational(ctx context.Context, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o MixedRoofLineOptions) (MixedRoofLineSection, error) {
	var zero MixedRoofLineSection
	var err error
	o.SurfacePatchEvaluationOptions, err = q.evaluationOptions(ctx, o.SurfacePatchEvaluationOptions, origin[0], origin[1], origin[2], direction[0], direction[1], direction[2], minT, maxT)
	if err != nil {
		return zero, err
	}
	if minT.Cmp(maxT) > 0 || direction[0].Sign() == 0 && direction[1].Sign() == 0 && direction[2].Sign() == 0 ||
		o.MaxSubdivisions < 0 || o.MaxSubdivisions > 65536 {
		return zero, geom.ErrInvalid
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	w := mixedRoofLineWork{o: o}
	var faces [12]MixedRoofFaceOwner
	var signs [12]int
	stations := make([]mixedRoofLineStation, 0, 24)
	for i, t := range [2]*big.Rat{minT, maxT} {
		opts, err := w.rootOptions()
		if err != nil {
			return zero, err
		}
		r, n, err := geom.NewExactRationalRoot(ctx, t, opts)
		if err != nil {
			return zero, err
		}
		w.operations += n
		stations = append(stations, mixedRoofLineStation{root: r, min: i == 0, max: i == 1})
	}
	for operand, b := range q.box {
		polys, err := b.linePolynomials(origin, direction, &w)
		if err != nil {
			return zero, err
		}
		for j, f := range [6]int{b.sideFaces[0], b.sideFaces[1], b.sideFaces[2], b.sideFaces[3], b.floorFace, b.roofFace} {
			s := operand*6 + j
			faces[s] = MixedRoofFaceOwner{operand, f}
			p := polys[j]
			degree := len(p) - 1
			for degree >= 0 && p[degree].Sign() == 0 {
				degree--
			}
			if degree < 0 {
				continue
			}
			// The first nonzero derivative determines the signs just to the
			// left and right of the closed clip without choosing a sample.
			left, err := w.leftSign(p, degree, minT)
			if err != nil {
				return zero, err
			}
			signs[s] = left
			if degree == 0 {
				continue
			}
			opts, err := w.rootOptions()
			if err != nil {
				return zero, err
			}
			roots, err := geom.IsolateExactPolynomialRoots(ctx, p[:degree+1], minT, maxT, opts)
			if err != nil {
				return zero, err
			}
			w.operations += roots.Operations
			w.subdivisions += roots.Subdivisions
			if w.operations > o.MaxOperations || w.subdivisions > o.MaxSubdivisions {
				return zero, geom.ErrConvergence
			}
			after := left
			for _, r := range roots.Roots {
				if r.Multiplicity%2 != 0 {
					after = -after
				}
				stations = append(stations, mixedRoofLineStation{root: r.Root, where: []mixedRoofCrossing{{s, after}}})
			}
		}
	}
	// The maximum is 24 stations (two sets of five affine polynomials
	// and one sextic, plus clip ends). Insertion orders and coalesces
	// using proofs that can report errors instead of hiding them in sort.
	ordered := make([]mixedRoofLineStation, 0, len(stations))
	for _, s := range stations {
		merged := false
		i := len(ordered)
		for i > 0 {
			order, err := w.compare(ctx, ordered[i-1].root, s.root)
			if err != nil {
				return zero, err
			}
			if order < 0 {
				break
			}
			if order == 0 {
				p := &ordered[i-1]
				p.where = append(p.where, s.where...)
				p.min, p.max = p.min || s.min, p.max || s.max
				merged = true
				break
			}
			i--
		}
		if !merged {
			ordered = append(ordered, mixedRoofLineStation{})
			copy(ordered[i+1:], ordered[i:])
			ordered[i] = s
		}
	}
	locations := func(signs [12]int) (a [2]parametric.Location) {
		for k := range a {
			a[k] = parametric.Inside
			for _, s := range signs[k*6 : (k+1)*6] {
				if s < 0 {
					a[k] = parametric.Outside
					break
				}
				if s == 0 {
					a[k] = parametric.Boundary
				}
			}
		}
		return a
	}
	type lineState struct {
		loc    parametric.Location
		owners []MixedRoofFaceOwner
	}
	classify := func(signs [12]int) (lineState, error) {
		a := locations(signs)
		loc, err := mixedRoofBooleanLocation(q.op, a)
		s := lineState{loc: loc}
		if err != nil || loc != parametric.Boundary {
			return s, err
		}
		for i, v := range signs {
			if v == 0 && a[i/6] == parametric.Boundary {
				s.owners = append(s.owners, faces[i])
			}
		}
		sort.Slice(s.owners, func(i, j int) bool {
			a, b := s.owners[i], s.owners[j]
			return a.Operand < b.Operand || a.Operand == b.Operand && a.Face < b.Face
		})
		return s, nil
	}
	out := MixedRoofLineSection{}
	var activeStart *geom.ExactAlgebraicRoot
	var active lineState
	closeSpan := func(end *geom.ExactAlgebraicRoot) {
		if activeStart != nil && activeStart != end && active.loc != parametric.Outside {
			out.Spans = append(out.Spans, MixedRoofLineSpan{activeStart, end, active.loc, active.owners})
		}
	}
	for _, s := range ordered {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		before, err := classify(signs)
		if err != nil {
			return zero, err
		}
		at := signs
		for _, hit := range s.where {
			at[hit.source] = 0
		}
		point, err := classify(at)
		if err != nil {
			return zero, err
		}
		for _, hit := range s.where {
			signs[hit.source] = hit.direction
		}
		after, err := classify(signs)
		if err != nil {
			return zero, err
		}
		if s.min {
			out.Start = point.loc
		}
		if s.max {
			out.End = point.loc
		}
		event := false
		if point.loc == parametric.Boundary {
			for _, hit := range s.where {
				for _, owner := range point.owners {
					event = event || owner == faces[hit.source]
				}
			}
		}
		if event {
			out.Events = append(out.Events, MixedRoofLineEvent{s.root, point.owners, before.loc, after.loc})
		}
		if s.min || s.max || event {
			closeSpan(s.root)
			activeStart, active = s.root, after
		}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	out.ExactOperations, out.Subdivisions = w.operations, w.subdivisions
	return out, nil
}

// leftSign uses the leading nonzero Taylor coefficient at t. The degree
// never exceeds six; differentiating in place preserves that exact sign.
func (w *mixedRoofLineWork) leftSign(p mixedRoofLinePoly, degree int, t *big.Rat) (int, error) {
	for order := 0; order <= degree; order++ {
		s, err := w.value(p, t)
		if err != nil {
			return 0, err
		}
		if s != 0 {
			if order%2 != 0 {
				s = -s
			}
			return s, nil
		}
		for i := 0; i < len(p)-1; i++ {
			v, err := w.rat(new(big.Rat).Mul(p[i+1], big.NewRat(int64(i+1), 1)))
			if err != nil {
				return 0, err
			}
			p[i] = v
		}
		p[len(p)-1] = new(big.Rat)
	}
	return 0, geom.ErrConvergence
}

package geom

import (
	"context"
	"math/big"
)

// RationalIsoTubePiece proposes one original proof box and its free chart axis.
// The retained source interval is supplied separately, not rounded into Box.
type RationalIsoTubePiece struct {
	Box      SurfacePairParameterBox
	FreeAxis int
}

// RationalIsoTubeChainOptions defaults to 16 pieces, 256 cells, 16M
// operations, 4096 poles and 16384 coefficient bits. At most 64 pieces are allowed.
type RationalIsoTubeChainOptions struct {
	SurfacePatchOptions
	MaxPieces, MaxCells int
}
type rationalIsoTubePiece struct {
	proof          VerifiedSurfaceTube
	source, free   [2]*big.Rat
	freeIncreasing bool
	endpoints      [2]TubeAlgebraicGraphPointCertificate
	box            SurfacePairParameterBox
}

// RationalIsoTubeChain is a complete, injective iso/plane section in original
// charts. Its disjoint retained source intervals may use overlapping tube boxes
// and different free axes. The exact rational restrictions are immutable and
// must never be discarded when constructing topology or saving a recipe.
// This is not a legacy TubeSegment chain or a shell assembly.
type RationalIsoTubeChain struct {
	a, b       *SurfacePatch
	fixedAxis  int
	fixed      *big.Rat
	pieces     []rationalIsoTubePiece
	operations int
}

// RationalIsoTubePieceDomain returns owned original source/free intervals.
// Endpoints are ordered by increasing source parameter, even if free decreases.
type RationalIsoTubePieceDomain struct {
	Source, Free   ExactParameterInterval
	FreeAxis       int
	FreeIncreasing bool
	Box            SurfacePairParameterBox
	Endpoints      [2]TubeAlgebraicGraphPointCertificate
}

// OriginalSupports returns immutable ordered source/target charts and the
// original fixed source coordinate. The rational value is caller-owned.
func (c *RationalIsoTubeChain) OriginalSupports() (*SurfacePatch, *SurfacePatch, int, *big.Rat, error) {
	if c == nil || c.a == nil || c.b == nil || len(c.pieces) == 0 {
		return nil, nil, 0, nil, ErrInvalid
	}
	return c.a, c.b, c.fixedAxis, new(big.Rat).Set(c.fixed), nil
}

func (c *RationalIsoTubeChain) PieceCount() int {
	if c == nil {
		return 0
	}
	return len(c.pieces)
}
func (c *RationalIsoTubeChain) ExactOperations() int {
	if c == nil {
		return 0
	}
	return c.operations
}
func (c *RationalIsoTubeChain) Piece(i int) (RationalIsoTubePieceDomain, error) {
	if c == nil || i < 0 || i >= len(c.pieces) {
		return RationalIsoTubePieceDomain{}, ErrInvalid
	}
	p := c.pieces[i]
	return RationalIsoTubePieceDomain{Source: ExactParameterInterval{Lower: new(big.Rat).Set(p.source[0]), Upper: new(big.Rat).Set(p.source[1])}, Free: ExactParameterInterval{Lower: new(big.Rat).Set(p.free[0]), Upper: new(big.Rat).Set(p.free[1])}, FreeAxis: p.proof.axis, FreeIncreasing: p.freeIncreasing, Box: p.box, Endpoints: p.endpoints}, nil
}

// MarshalJSON explicitly prevents silently erasing exact restrictions into an
// old floating-domain tube recipe. A versioned reproof codec is not supplied.
func (c *RationalIsoTubeChain) MarshalJSON() ([]byte, error) { return nil, ErrUnrepresentable }

// CertifyRationalIsoTubeChain proves the target is an embedded planar
// trapezoid, that all original contacts reduce to the specified source iso,
// and that exhaustive rational clipping is exactly one positive interval.
// cuts must partition that whole interval, with one freshly proved tube per
// consecutive pair. All source/target coordinates remain inside each tube;
// strict free-coordinate monotonicity proves no retained gap or duplicate arc.
// Interior junctions independently compare all four original roots and model
// point identity. No proposal flag or sampled coverage establishes completeness.
func CertifyRationalIsoTubeChain(ctx context.Context, a, b *SurfacePatch, fixedAxis int, fixed float64, cuts []*big.Rat, proposals []RationalIsoTubePiece, o RationalIsoTubeChainOptions) (result *RationalIsoTubeChain, err error) {
	if ctx == nil || a == nil || b == nil || fixedAxis < 0 || fixedAxis > 1 || !finite(fixed) {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxPieces == 0 {
		o.MaxPieces = 16
	}
	if o.MaxCells == 0 {
		o.MaxCells = 256
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 16000000
	}
	opts, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return nil, e
	}
	if o.MaxPieces < 1 || o.MaxPieces > 64 || o.MaxCells < 1 || o.MaxCells > 4096 || len(proposals) < 1 || len(proposals) > o.MaxPieces || len(cuts) != len(proposals)+1 {
		return nil, ErrInvalid
	}
	if len(a.net) > opts.MaxPoles || len(b.net) > opts.MaxPoles {
		return nil, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	next := func() (SurfacePatchOptions, error) {
		if e := ctx.Err(); e != nil {
			return opts, e
		}
		n := opts.MaxOperations - budget.operations
		if n < 1 {
			return opts, ErrConvergence
		}
		x := opts
		x.MaxOperations = n
		return x, nil
	}
	for i, q := range cuts {
		if q == nil {
			return nil, ErrInvalid
		}
		if e := budget.check(q); e != nil {
			return nil, e
		}
		if i > 0 && q.Cmp(cuts[i-1]) <= 0 {
			return nil, ErrDomain
		}
	}
	child, e := next()
	if e != nil {
		return nil, e
	}
	chart, e := NewTrapezoidSurfaceChart(ctx, b, AffineSurfaceChartOptions{MaxCells: o.MaxCells, MaxOperations: child.MaxOperations, MaxCoefficientBits: child.MaxCoefficientBits})
	if e != nil {
		return nil, e
	}
	budget.operations += chart.ExactOperations()
	child, e = next()
	if e != nil {
		return nil, e
	}
	raw, e := chart.RawSupportPlane(child)
	if e != nil {
		return nil, e
	}
	budget.operations += raw.ExactOperations()
	equation := raw.Coefficients()
	normal := [3]*big.Rat{equation[0], equation[1], equation[2]}
	d := a.Domain()
	lo := [2]*big.Rat{new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin)}
	hi := [2]*big.Rat{new(big.Rat).SetFloat64(d.UMax), new(big.Rat).SetFloat64(d.VMax)}
	var values [3]*big.Rat
	for i, uv := range [3][2]*big.Rat{lo, {hi[0], lo[1]}, {lo[0], hi[1]}} {
		child, e = next()
		if e != nil {
			return nil, e
		}
		jet, e := a.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{MaxOperations: child.MaxOperations, MaxCoefficientBits: child.MaxCoefficientBits})
		budget.operations += jet.ExactOperations
		if e != nil {
			return nil, e
		}
		values[i] = new(big.Rat)
		for j := 0; j < 3; j++ {
			values[i].Add(values[i], new(big.Rat).Mul(normal[j], jet.Point[j]))
			if e := budget.check(values[i]); e != nil {
				return nil, e
			}
		}
	}
	uv := [2]*big.Rat{}
	offset := new(big.Rat).Set(values[0])
	for i := range uv {
		uv[i] = new(big.Rat).Quo(new(big.Rat).Sub(values[i+1], values[0]), new(big.Rat).Sub(hi[i], lo[i]))
		offset.Sub(offset, new(big.Rat).Mul(uv[i], lo[i]))
		if e := budget.check(uv[i]); e != nil {
			return nil, e
		}
	}
	if uv[fixedAxis].Sign() == 0 || uv[1-fixedAxis].Sign() != 0 {
		return nil, ErrConvergence
	}
	exactFixed := new(big.Rat).Quo(new(big.Rat).Neg(new(big.Rat).Add(offset, equation[3])), uv[fixedAxis])
	if exactFixed.Cmp(new(big.Rat).SetFloat64(fixed)) != 0 {
		return nil, ErrDomain
	}
	child, e = next()
	if e != nil {
		return nil, e
	}
	identity, e := CertifySurfaceAffineForm(ctx, a, normal, uv, offset, SurfaceAffineCoordinateOptions{SurfacePatchOptions: child, MaxCells: o.MaxCells})
	budget.operations += identity.ExactOperations
	if e != nil {
		return nil, e
	}
	if !identity.Proven {
		return nil, ErrConvergence
	}
	child, e = next()
	if e != nil {
		return nil, e
	}
	iso, e := a.IsoCurve(fixedAxis, fixed, child)
	if e != nil {
		return nil, e
	}
	budget.operations += iso.ExactOperations()
	child, e = next()
	if e != nil {
		return nil, e
	}
	clip, e := ClipIsoCurveToTrapezoidChart(ctx, iso, chart, SplinePlaneOptions{MaxOperations: child.MaxOperations, MaxCoefficientBits: child.MaxCoefficientBits})
	budget.operations += clip.Operations
	if e != nil {
		return nil, e
	}
	if !clip.Complete || len(clip.Spans) != 1 || len(clip.Contacts) > 0 || len(clip.CoincidentBoundary) > 0 {
		return nil, ErrConvergence
	}
	if cuts[0].Cmp(clip.Spans[0].Start.Parameter) != 0 || cuts[len(cuts)-1].Cmp(clip.Spans[0].End.Parameter) != 0 {
		return nil, ErrDomain
	}
	child, e = next()
	if e != nil {
		return nil, e
	}
	projected, n, e := ProjectIsoCurveToTrapezoidChart(ctx, iso, chart, child)
	budget.operations += n
	if e != nil {
		return nil, e
	}
	// A strictly monotone projected coordinate proves the complete iso image
	// is injective on the retained source interval, independently of tube boxes.
	windowAll := isoRationalEnclosure(cuts[0], cuts[len(cuts)-1])
	child, e = next()
	if e != nil {
		return nil, e
	}
	all, e := projected.Restrict(windowAll.Start, windowAll.End, child)
	if e != nil {
		return nil, e
	}
	budget.operations += all.operations
	child, e = next()
	if e != nil {
		return nil, e
	}
	globalDerivative, e := all.BoundDerivativesContext(ctx, CurveDerivativeBoundOptions{MaxCells: o.MaxCells, MaxOperations: child.MaxOperations, MaxCoefficientBits: child.MaxCoefficientBits})
	budget.operations += globalDerivative.ExactOperations
	if e != nil {
		return nil, e
	}
	monotone := false
	for axis := 0; axis < 2; axis++ {
		pos, neg := len(globalDerivative.Cells) > 0, len(globalDerivative.Cells) > 0
		for _, cell := range globalDerivative.Cells {
			pos = pos && cell.First[axis].Min > 0
			neg = neg && cell.First[axis].Max < 0
		}
		monotone = monotone || pos || neg
	}
	if !monotone {
		return nil, ErrConvergence
	}
	chain := &RationalIsoTubeChain{a: a, b: b, fixedAxis: fixedAxis, fixed: new(big.Rat).Set(exactFixed)}
	for i, proposal := range proposals {
		if proposal.FreeAxis < 0 || proposal.FreeAxis > 3 || proposal.FreeAxis == fixedAxis {
			return nil, ErrInvalid
		}
		child, e = next()
		if e != nil {
			return nil, e
		}
		pa, n, e := a.TrimWithWorkLimit(proposal.Box.A, child.MaxOperations)
		budget.operations += n
		if e != nil {
			return nil, e
		}
		child, e = next()
		if e != nil {
			return nil, e
		}
		pb, n, e := b.TrimWithWorkLimit(proposal.Box.B, child.MaxOperations)
		budget.operations += n
		if e != nil {
			return nil, e
		}
		child, e = next()
		if e != nil {
			return nil, e
		}
		proof, e := CertifySurfacePairTube(pa, pb, SurfacePairTubeOptions{FreeAxis: proposal.FreeAxis, MaxOperations: child.MaxOperations, MaxCoefficientBits: child.MaxCoefficientBits})
		budget.operations += proof.Test.ExactOperations
		if e != nil {
			return nil, e
		}
		if !proof.ProvenBranch {
			return nil, ErrConvergence
		}
		tube := VerifiedSurfaceTube{a: pa, b: pb, axis: proposal.FreeAxis, tangentSign: proof.FreeTangentSign}
		piece := rationalIsoTubePiece{proof: tube, source: [2]*big.Rat{new(big.Rat).Set(cuts[i]), new(big.Rat).Set(cuts[i+1])}, box: proposal.Box}
		window := isoRationalEnclosure(cuts[i], cuts[i+1])
		child, e = next()
		if e != nil {
			return nil, e
		}
		sub, e := projected.Restrict(window.Start, window.End, child)
		if e != nil {
			return nil, e
		}
		budget.operations += sub.operations
		bounds, _ := sub.patch.Bounds()
		ranges := pairParameterRanges(proposal.Box)
		// Dependent source coordinates are exact intervals; target bounds are
		// homogeneous convex hulls over an outward source window. The free axis is
		// handled by exact endpoint order plus the derivative sign proof below.
		sourceRanges := [2][2]*big.Rat{{exactFixed, exactFixed}, {exactFixed, exactFixed}}
		sourceRanges[1-fixedAxis] = piece.source
		for j := 0; j < 2; j++ {
			if j == proposal.FreeAxis {
				continue
			}
			if sourceRanges[j][0].Cmp(new(big.Rat).SetFloat64(ranges[j][0])) <= 0 || sourceRanges[j][1].Cmp(new(big.Rat).SetFloat64(ranges[j][1])) >= 0 {
				return nil, ErrDomain
			}
		}
		targetRanges := [2][2]float64{{bounds.Min.X, bounds.Max.X}, {bounds.Min.Y, bounds.Max.Y}}
		for j := 0; j < 2; j++ {
			if j+2 == proposal.FreeAxis {
				continue
			}
			if targetRanges[j][0] <= ranges[j+2][0] || targetRanges[j][1] >= ranges[j+2][1] {
				return nil, ErrDomain
			}
		}
		increasing := true
		if proposal.FreeAxis >= 2 {
			child, e = next()
			if e != nil {
				return nil, e
			}
			derivatives, e := sub.BoundDerivativesContext(ctx, CurveDerivativeBoundOptions{MaxCells: o.MaxCells, MaxOperations: child.MaxOperations, MaxCoefficientBits: child.MaxCoefficientBits})
			budget.operations += derivatives.ExactOperations
			if e != nil {
				return nil, e
			}
			positive, negative := len(derivatives.Cells) > 0, len(derivatives.Cells) > 0
			for _, cell := range derivatives.Cells {
				positive = positive && cell.First[proposal.FreeAxis-2].Min > 0
				negative = negative && cell.First[proposal.FreeAxis-2].Max < 0
			}
			if !positive && !negative {
				return nil, ErrConvergence
			}
			increasing = positive
		}
		piece.freeIncreasing = increasing
		for end, t := range piece.source {
			target, e := exactCurveRationalPoint(projected.patch, t, budget)
			if e != nil {
				return nil, e
			}
			tuple := [4]*big.Rat{exactFixed, exactFixed, target[0], target[1]}
			tuple[1-fixedAxis] = t
			free := tuple[proposal.FreeAxis]
			if free.Cmp(new(big.Rat).SetFloat64(ranges[proposal.FreeAxis][0])) < 0 || free.Cmp(new(big.Rat).SetFloat64(ranges[proposal.FreeAxis][1])) > 0 {
				return nil, ErrDomain
			}
			index := end
			if !increasing {
				index ^= 1
			}
			piece.free[index] = new(big.Rat).Set(free)
			child, e = next()
			if e != nil {
				return nil, e
			}
			cert, e := tube.EvaluateRationalPlane(ctx, free, TubeAlgebraicGraphEndpointOptions{SurfaceCurveAgreementOptions: SurfaceCurveAgreementOptions{SurfacePatchOptions: child}})
			budget.operations += cert.ExactOperations
			if e != nil {
				return nil, e
			}
			if !cert.Proven {
				return nil, ErrConvergence
			}
			for j, root := range cert.Parameters {
				child, e = next()
				if e != nil {
					return nil, e
				}
				value, e := root.Classify(ctx, ExactCurvePlaneEventOptions{MaxOperations: child.MaxOperations, MaxCoefficientBits: child.MaxCoefficientBits})
				budget.operations += value.Operations
				if e != nil {
					return nil, e
				}
				if value.Kind != ExactRootKindRational || value.Rational.Cmp(tuple[j]) != 0 {
					return nil, ErrConvergence
				}
			}
			piece.endpoints[end] = cert
		}
		if piece.free[0].Cmp(piece.free[1]) >= 0 {
			return nil, ErrConvergence
		}
		if i > 0 {
			left, right := chain.pieces[i-1].endpoints[1], piece.endpoints[0]
			for j := 0; j < 4; j++ {
				child, e = next()
				if e != nil {
					return nil, e
				}
				same, e := left.Parameters[j].Compare(ctx, right.Parameters[j], ExactCurvePlaneEventOptions{MaxOperations: child.MaxOperations, MaxCoefficientBits: child.MaxCoefficientBits})
				budget.operations += same.Operations
				if e != nil {
					return nil, e
				}
				if !same.Proven || same.Order != 0 {
					return nil, ErrConvergence
				}
			}
			child, e = next()
			if e != nil {
				return nil, e
			}
			same, n, e := left.ModelEvent.CompareModelPoint(ctx, right.ModelEvent, ExactCurvePlaneEventOptions{MaxOperations: child.MaxOperations, MaxCoefficientBits: child.MaxCoefficientBits})
			budget.operations += n
			if e != nil {
				return nil, e
			}
			if !same {
				return nil, ErrConvergence
			}
		}
		chain.pieces = append(chain.pieces, piece)
	}
	chain.operations = budget.operations
	return chain, nil
}

// EvaluatePiece evaluates in the retained original free units. A value inside
// the proof box but outside the exact restriction is rejected.
func (c *RationalIsoTubeChain) EvaluatePiece(ctx context.Context, index int, free *big.Rat, o TubeAlgebraicGraphEndpointOptions) (TubeAlgebraicGraphPointCertificate, error) {
	if ctx == nil || c == nil || index < 0 || index >= len(c.pieces) || free == nil {
		return TubeAlgebraicGraphPointCertificate{}, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return TubeAlgebraicGraphPointCertificate{}, e
	}
	opts, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return TubeAlgebraicGraphPointCertificate{}, e
	}
	if free.Num().BitLen() > opts.MaxCoefficientBits || free.Denom().BitLen() > opts.MaxCoefficientBits {
		return TubeAlgebraicGraphPointCertificate{}, ErrConvergence
	}
	p := c.pieces[index]
	if free.Cmp(p.free[0]) < 0 || free.Cmp(p.free[1]) > 0 {
		return TubeAlgebraicGraphPointCertificate{}, ErrDomain
	}
	return p.proof.EvaluateRationalPlane(ctx, free, o)
}

// EnclosePiece uses an outward window only for bounds. The requested exact
// interval must belong to the retained domain; the underlying larger original
// tube already proves existence throughout the outward window.
func (c *RationalIsoTubeChain) EnclosePiece(ctx context.Context, index int, start, end *big.Rat, o SurfaceTubeIntervalOptions) (SurfaceTubeIntervalEnclosure, error) {
	if ctx == nil || c == nil || index < 0 || index >= len(c.pieces) || start == nil || end == nil {
		return SurfaceTubeIntervalEnclosure{}, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return SurfaceTubeIntervalEnclosure{}, e
	}
	bits := o.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	if bits < 64 {
		return SurfaceTubeIntervalEnclosure{}, ErrInvalid
	}
	for _, q := range []*big.Rat{start, end} {
		if q.Num().BitLen() > bits || q.Denom().BitLen() > bits {
			return SurfaceTubeIntervalEnclosure{}, ErrConvergence
		}
	}
	if start.Cmp(end) >= 0 {
		return SurfaceTubeIntervalEnclosure{}, ErrInvalid
	}
	p := c.pieces[index]
	if start.Cmp(p.free[0]) < 0 || end.Cmp(p.free[1]) > 0 {
		return SurfaceTubeIntervalEnclosure{}, ErrDomain
	}
	ranges := pairParameterRanges(p.box)
	window := isoRationalEnclosure(start, end)
	return p.proof.EncloseInterval(ctx, max(ranges[p.proof.axis][0], window.Start), min(ranges[p.proof.axis][1], window.End), o)
}

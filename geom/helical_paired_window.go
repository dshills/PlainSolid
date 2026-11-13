package geom

import (
	"context"
	"math/big"
)

// HelicalWindowCopy identifies one member of the ordered axial lattice:
// Index=Turn*startCount+Start. Physical source turn is ray phase+Turn.
type HelicalWindowCopy struct {
	Index, Turn, Start int
	AxialOffset        *big.Rat
}

// HelicalPairedWindow proves the two neighboring cutters both exist over
// Phase. GapOffsets is the open retained ridge between their closed slices,
// with the shared lead*phase removed. WitnessCutters and WitnessGap are actual
// axial frame coordinates at WitnessPhase. A singleton Phase proves only one
// meridian, not a positive angular window. Outputs are caller-owned.
type HelicalPairedWindow struct {
	Complete, Present, WholeWindow, Singleton bool
	Phase                                     [2]*big.Rat
	Copies                                    [2]HelicalWindowCopy
	Profile                                   HelicalAxialSlice
	GapOffsets                                [2]*big.Rat
	WitnessPhase                              *big.Rat
	WitnessTurns                              [2]*big.Rat
	WitnessCutters                            [2][2]*big.Rat
	WitnessGap                                [2]*big.Rat
	ExactOperations                           int
}

// CertifyPairedWindow clips a canonical physical-turn phase interval [0,1]
// against BOTH neighboring finite sweep windows. At a start-count wrap, the
// second cutter belongs to the next turn and may be absent near a finite end.
// Phase is measured in the surface U convention (the angular ray is h*phase
// turns); no transcendental world-point equality is asserted. The strict
// separation proof on s makes the open gap free of all other lattice copies
// at this radius. It does not claim radial or full-solid ridge containment.
func (s SeparatedHelicalStarts) CertifyPairedWindow(ctx context.Context, radius *big.Rat, leftIndex int, phase [2]*big.Rat, o SurfacePatchEvaluationOptions) (out HelicalPairedWindow, err error) {
	if ctx == nil || s.count < 1 || s.spacing == nil || radius == nil || phase[0] == nil || phase[1] == nil || phase[0].Sign() < 0 || phase[1].Cmp(big.NewRat(1, 1)) > 0 || phase[0].Cmp(phase[1]) > 0 || leftIndex < -1000000 || leftIndex > 1000000 {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 1000000
	}
	if bits == 0 {
		bits = 16384
	}
	if ops < 1 || bits < 64 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.Complete = false
			out.Present = false
		}
	}()
	for _, v := range []*big.Rat{radius, phase[0], phase[1]} {
		if e := b.check(v); e != nil {
			return out, e
		}
	}
	out.Profile, err = exactHelicalPolygonSlice(ctx, s.base.profile, radius, b)
	if err != nil {
		return out, err
	}
	lo, hi := new(big.Rat).Set(phase[0]), new(big.Rat).Set(phase[1])
	lead := new(big.Rat).SetFloat64(s.base.reference.lead)
	for i := 0; i < 2; i++ {
		k := leftIndex + i
		n, j := k/s.count, k%s.count
		if j < 0 {
			j += s.count
			n--
		}
		offset := new(big.Rat).Mul(s.spacing, big.NewRat(int64(k), 1))
		out.Copies[i] = HelicalWindowCopy{k, n, j, offset}
		a := new(big.Rat).Sub(new(big.Rat).SetFloat64(s.base.reference.domain.UMin), big.NewRat(int64(n), 1))
		c := new(big.Rat).Sub(new(big.Rat).SetFloat64(s.base.reference.domain.UMax), big.NewRat(int64(n), 1))
		if a.Cmp(lo) > 0 {
			lo = a
		}
		if c.Cmp(hi) < 0 {
			hi = c
		}
	}
	if lo.Cmp(hi) > 0 || !out.Profile.Present {
		out.Complete = true
		return out, nil
	}
	out.Phase = [2]*big.Rat{lo, hi}
	out.Singleton = lo.Cmp(hi) == 0
	out.WholeWindow = lo.Cmp(phase[0]) == 0 && hi.Cmp(phase[1]) == 0
	out.GapOffsets = [2]*big.Rat{new(big.Rat).Add(out.Profile.Upper, out.Copies[0].AxialOffset), new(big.Rat).Add(out.Profile.Lower, out.Copies[1].AxialOffset)}
	if out.GapOffsets[0].Cmp(out.GapOffsets[1]) >= 0 {
		return out, ErrConvergence
	}
	out.WitnessPhase = new(big.Rat).Quo(new(big.Rat).Add(lo, hi), big.NewRat(2, 1))
	advance := new(big.Rat).Mul(lead, out.WitnessPhase)
	for i := 0; i < 2; i++ {
		out.WitnessTurns[i] = new(big.Rat).Add(out.WitnessPhase, big.NewRat(int64(out.Copies[i].Turn), 1))
		offset := new(big.Rat).Add(advance, out.Copies[i].AxialOffset)
		out.WitnessCutters[i] = [2]*big.Rat{new(big.Rat).Add(offset, out.Profile.Lower), new(big.Rat).Add(offset, out.Profile.Upper)}
	}
	out.WitnessGap = [2]*big.Rat{new(big.Rat).Add(advance, out.GapOffsets[0]), new(big.Rat).Add(advance, out.GapOffsets[1])}
	for _, v := range []*big.Rat{lo, hi, out.WitnessPhase, out.WitnessGap[0], out.WitnessGap[1], out.WitnessCutters[0][0], out.WitnessCutters[1][1]} {
		if e := b.check(v); e != nil {
			return out, e
		}
	}
	out.Complete = true
	out.Present = true
	return out, nil
}

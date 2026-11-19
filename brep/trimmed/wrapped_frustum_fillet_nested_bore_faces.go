//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// FilletedFrustumNestedBoreOptions uses one original baseline at Support's
// precision. Candidate precision doubles at most MaxRefinements times, capped
// at256 bits. Zero limits select32 pairs and6 refinements (range1..32,1..6).
// Exhausting refinement preserves unresolved baseline bounds, never a tie.
type FilletedFrustumNestedBoreOptions struct {
	Support                  FilletedFrustumDistanceOptions
	MaxPairs, MaxRefinements int
}

// FilletedFrustumNestedBoreFaces proves a strictly nearer original torus or
// bore owner, including irrational stock azimuth. Comparison is -1 for torus,
// +1 for bore, meaningful only when Complete. This API never certifies equality.
// It retains two attained candidates only after strict exclusion of every
// other original face. Work counters include all attempted pair refinements.
type FilletedFrustumNestedBoreFaces struct {
	Baseline                                              FilletedFrustumDistance
	Nested                                                *FilletedFrustumNestedToroidWitness
	Bore                                                  *FilletedFrustumBoreWitness
	Lower, Upper                                          *big.Rat
	Tied, Excluded, Unresolved                            []int
	Complete                                              bool
	Comparison, PairChecks, RefinementChecks, MaxBitsUsed int
}

// ClassifyNestedTorusBoreFacesRational bounds the positive nested radicals
// rho=sqrt(S),d=sqrt((rho-M)^2+b^2) with rational outward arithmetic. Each
// candidate must have strict original trim retention, then disjoint squared
// intervals authenticate strict order. No interval overlap becomes a tie.
// Other faces reuse the ONE baseline ledger; there is no budget reset.
func (q FilletedFrustumMaterial) ClassifyNestedTorusBoreFacesRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumNestedBoreOptions) (out FilletedFrustumNestedBoreFaces, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumNestedBoreFaces{}
		}
	}()
	if o.MaxPairs == 0 {
		o.MaxPairs = 32
	}
	if o.MaxRefinements == 0 {
		o.MaxRefinements = 6
	}
	if o.MaxPairs < 1 || o.MaxPairs > 32 || o.MaxRefinements < 1 || o.MaxRefinements > 6 {
		return out, geom.ErrInvalid
	}
	base, e := q.BoundBoundaryDistanceRational(ctx, p, o.Support)
	if e != nil {
		return out, e
	}
	out.Baseline = base
	out.Lower = new(big.Rat).Set(base.Lower)
	out.Upper = new(big.Rat).Set(base.Upper)
	for _, f := range base.PerFace {
		out.Unresolved = append(out.Unresolved, f.Face)
	}
	sort.Ints(out.Unresolved)
	startBits := o.Support.SqrtBits
	if startBits == 0 {
		startBits = 64
	}
	dx, dy := fdSub(p[0], q.center[0]), fdSub(p[1], q.center[1])
	S := fdAdd(fdSquare(dx), fdSquare(dy))
	if S.Sign() <= 0 {
		return out, nil
	}
	for end, circle := range q.fillets.Circles {
		b := fdSub(p[2], circle.Center[1])
		rb := fdMul(circle.Radius, b)
		z0, z1 := q.z[0], circle.Tangency[1]
		if end == 1 {
			z0, z1 = circle.Tangency[1], q.z[1]
		}
		for _, cut := range q.cuts {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			if out.PairChecks >= o.MaxPairs {
				return out, geom.ErrConvergence
			}
			out.PairChecks++
			if p[2].Cmp(cut.z[0]) <= 0 || p[2].Cmp(cut.z[1]) >= 0 {
				continue
			}
			bx, by := fdSub(p[0], cut.center[0]), fdSub(p[1], cut.center[1])
			T := fdAdd(fdSquare(bx), fdSquare(by))
			if T.Sign() <= 0 {
				continue
			}
			bits := startBits
			for step := 0; step < o.MaxRefinements; step++ {
				if e := ctx.Err(); e != nil {
					return out, e
				}
				out.RefinementChecks++
				if bits > out.MaxBitsUsed {
					out.MaxBitsUsed = bits
				}
				// The whole candidate attempt is bounded; advancing precision cannot
				// change either exact source geometry or the unchanged baseline ledger.
				terminal := false
				accepted := func() bool {
					rl, rh := fdSqrt(S, bits)
					al, ah := fdSub(rl, circle.Center[0]), fdSub(rh, circle.Center[0])
					if al.Sign() <= 0 {
						terminal = ah.Sign() <= 0 // Entire enclosure is outside the strict outer branch.
						return false
					}
					dl, _ := fdSqrt(fdAdd(fdSquare(al), fdSquare(b)), bits)
					_, dh := fdSqrt(fdAdd(fdSquare(ah), fdSquare(b)), bits)
					if dl.Sign() <= 0 {
						return false
					}
					zl, zh := new(big.Rat).Quo(rb, dh), new(big.Rat).Quo(rb, dl)
					if rb.Sign() < 0 {
						zl, zh = zh, zl
					}
					zl.Add(zl, circle.Center[1])
					zh.Add(zh, circle.Center[1])
					if zl.Cmp(z0) <= 0 || zh.Cmp(z1) >= 0 {
						terminal = zh.Cmp(z0) <= 0 || zl.Cmp(z1) >= 0 // No higher precision can make this strict interior.
						return false
					}
					torusLo, torusHi := fdSquareRange(fdSub(dl, circle.Radius), fdSub(dh, circle.Radius))
					tl, th := fdSqrt(T, bits)
					boreLo, boreHi := fdSquareRange(fdSub(tl, cut.radius), fdSub(th, cut.radius))
					comparison := 0
					lo, hi := torusLo, torusHi
					if torusHi.Cmp(boreLo) < 0 {
						comparison = -1
					} else if boreHi.Cmp(torusLo) < 0 {
						comparison = 1
						lo, hi = boreLo, boreHi
					} else {
						// Equal singleton intervals prove equality, which this strict-order
						// API deliberately leaves unresolved. Refinement cannot separate it.
						terminal = torusLo.Cmp(torusHi) == 0 && boreLo.Cmp(boreHi) == 0
						return false
					}
					face := q.fillets.Blends[end]
					excluded := []int{}
					for _, f := range base.PerFace {
						if f.Face == face || f.Face == cut.wall {
							continue
						}
						if f.SquaredLower.Cmp(hi) <= 0 {
							return false
						}
						excluded = append(excluded, f.Face)
					}
					if base.Upper.Cmp(lo) < 0 || face == cut.wall {
						err = geom.ErrConvergence
						return false
					}
					out.Nested = &FilletedFrustumNestedToroidWitness{Center: [3]*big.Rat{new(big.Rat).Set(q.center[0]), new(big.Rat).Set(q.center[1]), new(big.Rat).Set(circle.Center[1])}, MajorRadius: new(big.Rat).Set(circle.Center[0]), MinorRadius: new(big.Rat).Set(circle.Radius), HeightOffset: new(big.Rat).Set(b), Azimuth: [2]*big.Rat{new(big.Rat).Set(dx), new(big.Rat).Set(dy)}, AzimuthSquared: new(big.Rat).Set(S), AzimuthRoot: [2]*big.Rat{rl, rh}, MeridianRoot: [2]*big.Rat{dl, dh}, Face: face}
					out.Bore = &FilletedFrustumBoreWitness{Center: [3]*big.Rat{new(big.Rat).Set(cut.center[0]), new(big.Rat).Set(cut.center[1]), new(big.Rat).Set(p[2])}, Numerator: [3]*big.Rat{fdMul(cut.radius, bx), fdMul(cut.radius, by), new(big.Rat)}, Radicand: new(big.Rat).Set(T), Root: [2]*big.Rat{tl, th}, Face: cut.wall}
					if comparison < 0 {
						out.Tied = []int{face}
						excluded = append(excluded, cut.wall)
					} else {
						out.Tied = []int{cut.wall}
						excluded = append(excluded, face)
					}
					sort.Ints(excluded)
					out.Excluded = excluded
					out.Unresolved = nil
					out.Lower = new(big.Rat).Set(lo)
					out.Upper = new(big.Rat).Set(hi)
					out.Comparison = comparison
					out.Complete = true
					return true
				}()
				if err != nil {
					return out, err
				}
				if accepted {
					return out, nil
				}
				if terminal || bits == 256 {
					break
				}
				bits *= 2
				if bits > 256 {
					bits = 256
				}
			}
		}
	}
	return out, nil
}

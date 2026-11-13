package geom

import "context"

// SurfacePairTubeRegion names a uniform graph tube and its free coordinate.
type SurfacePairTubeRegion struct {
	Box      SurfacePairParameterBox
	FreeAxis int
	// Boundary supplies an optional exact boundary-start proposal. The
	// continuation is re-proved against the original patches before use.
	Boundary *SurfaceTubeBoundaryOptions
}

// SurfaceTubeUnionCoverageOptions applies cumulative proof work limits across
// all tubes and outside pieces. MaxPieces bounds closed-box partition growth
// (default 8192); other zero limits match SurfaceTubeCoverageOptions.
type SurfaceTubeUnionCoverageOptions struct{ MaxDepth, MaxCells, MaxPieces, MaxOperations, MaxCoefficientBits int }

// SurfaceTubeUnionCoverage proves every candidate contact belongs to at least
// one verified tube. It does not prove tube connectivity or branch enumeration.
// Exact contact confinement can cover shared boundaries; unproved cases remain unresolved.
type SurfaceTubeUnionCoverage struct {
	Tubes []SurfaceTubeCertificate
	// BoundaryCovered contacts are proved inside the verified union, not excluded.
	BoundaryCovered                                []SurfacePairParameterBox
	OutsideExcluded, Unresolved, Pending           []SurfacePairParameterBox
	Covered                                        bool
	PartitionPieces, VisitedCells, ExactOperations int
}

// CertifySurfacePairCellInTubes subtracts a bounded union of verified tube boxes
// using exact represented parameter cuts. Only the leftover closed slabs need
// contact exclusion. An empty tube list requests exclusion of the whole
// candidate. All geometry refers to the same original surface pair.
func CertifySurfacePairCellInTubes(a, b *SurfacePatch, candidate SurfacePairParameterBox, tubes []SurfacePairTubeRegion, o SurfaceTubeUnionCoverageOptions) (SurfaceTubeUnionCoverage, error) {
	return CertifySurfacePairCellInTubesContext(context.Background(), a, b, candidate, tubes, o)
}

// CertifySurfacePairCellInTubesContext checks for cancellation between bounded
// exact proofs and preserves a covering of the unchecked contact in Pending.
func CertifySurfacePairCellInTubesContext(ctx context.Context, a, b *SurfacePatch, candidate SurfacePairParameterBox, tubes []SurfacePairTubeRegion, o SurfaceTubeUnionCoverageOptions) (result SurfaceTubeUnionCoverage, err error) {
	return certifySurfacePairCellInTubes(ctx, a, b, candidate, tubes, o, nil)
}

func certifySurfacePairCellInTubes(ctx context.Context, a, b *SurfacePatch, candidate SurfacePairParameterBox, tubes []SurfacePairTubeRegion, o SurfaceTubeUnionCoverageOptions, cache *SurfacePairTubeCoverageCache) (result SurfaceTubeUnionCoverage, err error) {
	if ctx == nil || a == nil || b == nil || !candidate.A.Valid() || !candidate.B.Valid() {
		return result, ErrInvalid
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 12
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxPieces == 0 {
		o.MaxPieces = 8192
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxCells < 1 || o.MaxPieces < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return result, ErrInvalid
	}
	validBox := func(box SurfacePairParameterBox) bool {
		for _, item := range []struct {
			p *SurfacePatch
			r ParameterRectangle
		}{{a, box.A}, {b, box.B}} {
			r, d := item.r, item.p.Domain()
			if !r.Valid() || r.UMin < d.UMin || r.UMax > d.UMax || r.VMin < d.VMin || r.VMax > d.VMax {
				return false
			}
		}
		return true
	}
	if !validBox(candidate) {
		return result, ErrDomain
	}
	defer func() {
		if err != nil && !result.Covered && len(result.Unresolved) == 0 && len(result.Pending) == 0 {
			result.Pending = append(result.Pending, candidate)
		}
	}()
	pieces := []SurfacePairParameterBox{candidate}
	for _, tube := range tubes {
		if e := ctx.Err(); e != nil {
			result.Pending = append(result.Pending, pieces...)
			return result, e
		}
		if tube.FreeAxis < 0 || tube.FreeAxis > 3 || !validBox(tube.Box) {
			return result, ErrInvalid
		}
		remaining := o.MaxOperations - result.ExactOperations
		if remaining < 1 {
			return result, ErrConvergence
		}

		key := ""
		if cache != nil {
			key = tubeCoverageKey(tube, o.MaxCoefficientBits)
		}
		entry, hit := cache.lookup(key, remaining)
		if hit {
			result.ExactOperations += entry.work
			result.Tubes = append(result.Tubes, entry.certificate)
		} else {
			before := result.ExactOperations
			budget := &polynomialBudget{maxOperations: remaining, maxBits: o.MaxCoefficientBits}
			pa, e := trimPatchBudget(a, tube.Box.A, budget)
			if e != nil {
				result.ExactOperations += budget.operations
				return result, e
			}
			pb, e := trimPatchBudget(b, tube.Box.B, budget)
			result.ExactOperations += budget.operations
			if e != nil {
				return result, e
			}
			remaining = o.MaxOperations - result.ExactOperations
			if remaining < 1 {
				return result, ErrConvergence
			}
			limits := SurfacePairTubeOptions{FreeAxis: tube.FreeAxis, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits}
			var certificate SurfaceTubeCertificate
			if tube.Boundary == nil {
				certificate, e = CertifySurfacePairTube(pa, pb, limits)
			} else {
				anchor := *tube.Boundary
				anchor.SurfacePairTubeOptions = limits
				certificate, e = CertifyBoundaryAnchoredSurfacePairTube(ctx, pa, pb, anchor)
			}
			result.ExactOperations += certificate.Test.ExactOperations
			result.Tubes = append(result.Tubes, certificate)
			if e != nil {
				return result, e
			}
			if !certificate.ProvenBranch {
				return result, ErrConvergence
			}
			cache.remember(key, tubeCoverageEntry{certificate: certificate, work: result.ExactOperations - before})
		}

		next := []SurfacePairParameterBox{}
		for _, piece := range pieces {
			if e := ctx.Err(); e != nil {
				result.Pending = append(result.Pending, pieces...)
				return result, e
			}
			outside := outsideTubeSlabs(piece, tube.Box)
			if len(outside) > o.MaxPieces-len(next) {
				result.Pending = append(result.Pending, pieces...)
				return result, ErrConvergence
			}
			next = append(next, outside...)
		}
		pieces = next
		result.PartitionPieces = len(pieces)
	}
	proof := SurfaceTubeCoverage{ExactOperations: result.ExactOperations}
	e := ctx.Err()
	if e != nil {
		proof.Pending = append(proof.Pending, pieces...)
	} else {
		e = excludeSurfaceTubeOutsideCovered(ctx, a, b, &proof, pieces, SurfaceTubeCoverageOptions{MaxDepth: o.MaxDepth, MaxCells: o.MaxCells, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}, tubes)
	}
	result.Covered, result.ExactOperations, result.VisitedCells = proof.Covered, proof.ExactOperations, proof.VisitedCells
	result.OutsideExcluded, result.Unresolved, result.Pending = proof.OutsideExcluded, proof.Unresolved, proof.Pending
	result.BoundaryCovered = proof.BoundaryCovered
	return result, e
}

package trimmed

import (
	"context"
	"fmt"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// SplinePairDiagnosticKind distinguishes nonempty evidence from separation.
type SplinePairDiagnosticKind uint8

const (
	SplinePairUnknown SplinePairDiagnosticKind = iota
	SplinePairSeparated
	SplinePairBranches
	SplinePairPointOnlyPendingFan
	SplinePairAffineCornerBranchPendingAssembly
)

// SplinePairDiagnostic is a diagnostic snapshot, never accepted as proof input.
type SplinePairDiagnostic struct {
	Faces    [2]int
	Kind     SplinePairDiagnosticKind
	Branches int
	Reason   string
}
type SplineCornerSectionLedgerOptions struct {
	MaxPairs  int
	Faces     SplineRectangleEmbeddingOptions
	Discovery intersect.SplinePlanePairDiscoveryOptions
	Trace     intersect.SplinePairTraceOptions
}

// SplineCornerSectionLedger inventories every pair of original faces. Unknown
// pairs remain visible even if other pairs have complete branch certificates.
// Neither classified pairs nor the existing local fan authorize assembly.
type SplineCornerSectionLedger struct {
	fan    *SplineCornerFan
	pairs  []SplinePairDiagnostic
	affine []*SplineCornerAffineBranch
}

func (l *SplineCornerSectionLedger) AffineCornerBranches() []*SplineCornerAffineBranch {
	return append([]*SplineCornerAffineBranch(nil), l.affine...)
}
func (l *SplineCornerSectionLedger) Pairs() []SplinePairDiagnostic {
	return append([]SplinePairDiagnostic(nil), l.pairs...)
}
func (l *SplineCornerSectionLedger) PendingAssembly() bool { return l != nil && l.fan != nil }
func (l *SplineCornerSectionLedger) AllPairsClassified() bool {
	if l == nil || len(l.pairs) == 0 {
		return false
	}
	for _, p := range l.pairs {
		if p.Kind == SplinePairUnknown {
			return false
		}
	}
	return true
}

// DiagnoseSplineCornerSections checks all face pairs in deterministic order.
// Point-only status is available solely for the immutable fan's original
// point-contact ledger. Branch status independently certifies whole original
// pair domains; it does not assert that those branches reconcile with the fan.
// Unsupported discovery and exhausted per-proof limits produce Unknown, never
// separation. MaxPairs bounds the full matrix (default64, maximum4096), and
// exceeding it or cancellation returns no partial ledger. Other work limits
// apply independently to each pair; this is intentionally an opt-in diagnostic.
func DiagnoseSplineCornerSections(ctx context.Context, fan *SplineCornerFan, o SplineCornerSectionLedgerOptions) (*SplineCornerSectionLedger, error) {
	if ctx == nil || fan == nil || fan.isolated == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxPairs == 0 {
		o.MaxPairs = 64
	}
	if o.MaxPairs < 1 || o.MaxPairs > 4096 {
		return nil, geom.ErrInvalid
	}
	a, b := fan.anchors[0].solids[0], fan.anchors[0].solids[1]
	if len(a.faces) == 0 || len(b.faces) == 0 || len(a.faces) > o.MaxPairs/len(b.faces) {
		return nil, geom.ErrConvergence
	}
	result := &SplineCornerSectionLedger{fan: fan}
	var faces [2][]splineEmbeddingFace
	var failures [2][]error
	for owner, s := range [2]Solid{a, b} {
		faces[owner] = make([]splineEmbeddingFace, len(s.faces))
		failures[owner] = make([]error, len(s.faces))
		for i, f := range s.faces {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			faces[owner][i], _, failures[owner][i] = s.splineEmbeddingFace(ctx, f, o.Faces)
		}
	}
	for i := range a.faces {
		for j := range b.faces {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			entry := SplinePairDiagnostic{Faces: [2]int{i, j}, Kind: SplinePairUnknown}
			if failures[0][i] != nil || failures[1][j] != nil {
				entry.Reason = fmt.Sprintf("original face proof: %v / %v", failures[0][i], failures[1][j])
				result.pairs = append(result.pairs, entry)
				continue
			}
			if entry.Faces == fan.isolated.faces {
				entry.Kind = SplinePairPointOnlyPendingFan
				result.pairs = append(result.pairs, entry)
				continue
			}
			separation := proveSplinePatchPairDisjoint(ctx, faces[0][i].patch, faces[1][j].patch, o.Faces.Separation)
			if separation == nil {
				entry.Kind = SplinePairSeparated
				result.pairs = append(result.pairs, entry)
				continue
			}
			var patches [2]intersect.Patch
			p, e := intersectSplineFace(a.faces[i], faces[0][i].patch.Domain())
			patches[0] = p
			if e == nil {
				patches[1], e = intersectSplineFace(b.faces[j], faces[1][j].patch.Domain())
			}
			if e == nil {
				proof, err := intersect.CertifyOpenSplinePlanePair(ctx, patches[0], patches[1], intersect.OverlappingGraphTraceOptions{Discovery: o.Discovery, Trace: o.Trace})
				e = err
				if e == nil && proof.Complete && len(proof.Trace.BranchGraph.Branches) > 0 {
					entry.Kind = SplinePairBranches
					entry.Branches = len(proof.Trace.BranchGraph.Branches)
				} else if e == nil {
					e = geom.ErrConvergence
				}
			}
			if entry.Kind == SplinePairUnknown {
				entry.Reason = fmt.Sprintf("separation: %v; branch coverage: %v", separation, e)
			}
			result.pairs = append(result.pairs, entry)
		}
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return result, nil
}

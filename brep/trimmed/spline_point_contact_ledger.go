package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// SplinePointContactLedger retains one nonempty original-face-pair contact.
// PointOnly means the two full original face domains meet at exactly this
// point. PendingFan is always true: no shell continuation, material selection,
// or Boolean assembly permission follows from this evidence. In particular it
// must never be treated as disjointness or as an already reconciled section.
type SplinePointContactLedger struct {
	solids  [2]Solid
	faces   [2]int
	domains [2]geom.ParameterRectangle
	overlap [2]geom.ParameterRectangle
	proof   *geom.GraphCornerContact
}

func (l *SplinePointContactLedger) FacePair() [2]int                           { return l.faces }
func (l *SplinePointContactLedger) Domains() [2]geom.ParameterRectangle        { return l.domains }
func (l *SplinePointContactLedger) OverlapDomains() [2]geom.ParameterRectangle { return l.overlap }
func (l *SplinePointContactLedger) PointOnly() bool                            { return l != nil && l.proof != nil }
func (l *SplinePointContactLedger) PendingFan() bool                           { return l.PointOnly() }
func (l *SplinePointContactLedger) Point() [3]*big.Rat                         { return l.proof.Point() }
func (l *SplinePointContactLedger) Parameters() [2][2]*big.Rat                 { return l.proof.Parameters() }

// Matches binds the immutable original topology instances and face indices;
// geometrically similar but independently rebuilt inputs do not match.
func (l *SplinePointContactLedger) Matches(a, b Solid, pair [2]int) bool {
	if l == nil || l.faces != pair {
		return false
	}
	for i, s := range [2]Solid{a, b} {
		if len(s.faces) == 0 || len(s.edges) == 0 || len(l.solids[i].faces) == 0 || len(l.solids[i].edges) == 0 || &s.faces[0] != &l.solids[i].faces[0] || &s.edges[0] != &l.solids[i].edges[0] {
			return false
		}
	}
	return true
}

// CertifySplineFacePointContact proves one isolated contact, retaining original
// IDs/domains. Both faces must be exact rectangular spline graphs. Exact XY
// clipping proves that portions outside the common projection cannot contact;
// the corner certificate proves the complete overlap. Currently the clipped
// original UV domains/maps must match. Limits apply per bounded subproof.
// This opt-in ledger is deliberately not consumed by any Boolean assembler.
func CertifySplineFacePointContact(ctx context.Context, a, b Solid, pair [2]int, o SplineRectangleEmbeddingOptions) (*SplinePointContactLedger, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	l := &SplinePointContactLedger{solids: [2]Solid{a, b}, faces: pair}
	var patches [2]*geom.SurfacePatch
	for i, s := range l.solids {
		if pair[i] < 0 || pair[i] >= len(s.faces) {
			return nil, geom.ErrInvalid
		}
		face, _, e := s.splineEmbeddingFace(ctx, s.faces[pair[i]], o)
		if e != nil {
			return nil, e
		}
		patches[i] = face.patch
		l.domains[i] = face.patch.Domain()
	}
	overlap, e := geom.ClipAffineXYGraphOverlap(ctx, patches[0], patches[1], o.Patches)
	if e != nil {
		return nil, e
	}
	if overlap.Disjoint {
		return nil, geom.ErrConvergence
	}
	l.overlap = [2]geom.ParameterRectangle{overlap.A, overlap.B}
	for i := range patches {
		patches[i], e = patches[i].Trim(l.overlap[i])
		if e != nil {
			return nil, e
		}
	}
	l.proof, e = geom.CertifyGraphCornerContact(ctx, patches[0], patches[1], o.Patches)
	if e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return l, nil
}

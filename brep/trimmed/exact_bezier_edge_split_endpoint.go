package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

// SplitExactBezierTrimEdgeWithExactEndpoints retains original SplineUV owners
// while opting their endpoint evaluation into exact persisted-source de Boor.
// Original native split/model/UV/vertex proofs, representability and all mismatch
// checks remain mandatory. Only selected owners change mode; failure is atomic.
func (s Solid) SplitExactBezierTrimEdgeWithExactEndpoints(ctx context.Context, edge int, t *big.Rat, o edgecheck.Options) (Solid, EdgeSplit, error) {
	if ctx == nil || edge < 0 || edge >= len(s.edges) || t == nil {
		return Solid{}, EdgeSplit{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, EdgeSplit{}, e
	}
	out := s
	out.faces = s.Faces()
	for fi := range out.faces {
		for _, loop := range append([][]Use{out.faces[fi].Outer}, out.faces[fi].Holes...) {
			for ui, u := range loop {
				if u.Edge != edge {
					continue
				}
				if q, ok := u.Curve.(SplineUV); ok {
					exact, e := q.WithExactEndpoints()
					if e != nil {
						return Solid{}, EdgeSplit{}, e
					}
					loop[ui].Curve = exact
				}
			}
		}
	}
	return out.SplitExactBezierTrimEdge(ctx, edge, t, o)
}

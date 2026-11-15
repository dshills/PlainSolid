package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// Follow the source-ordered boundary network once per loop. Every sample is
// checked against both original UV coordinate identities, not nearest matched.
func polygonOriginalUseSamples(ctx context.Context, f Face, face int, r CertifiedPolygonPatchMesh) ([]PolygonFaceUseSamples, error) {
	var out []PolygonFaceUseSamples
	for li, uses := range append([][]Use{f.Outer}, f.Holes...) {
		if li >= len(r.Loops) {
			return nil, geom.ErrConvergence
		}
		ring := r.Loops[li]
		if len(ring) == 0 {
			return nil, geom.ErrConvergence
		}
		position := 0
		for ui, u := range uses {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			item := PolygonFaceUseSamples{Face: face, Loop: li, Use: ui, Edge: u.Edge, FaceReversed: f.Reversed}
			start := [2]*big.Rat{ratInset(u.UVStart.X), ratInset(u.UVStart.Y)}
			end := [2]*big.Rat{ratInset(u.UVEnd.X), ratInset(u.UVEnd.Y)}
			axis := 0
			if start[0].Cmp(end[0]) == 0 {
				axis = 1
			}
			delta := new(big.Rat).Sub(end[axis], start[axis])
			if delta.Sign() == 0 {
				return nil, geom.ErrDegenerate
			}
			for steps := 0; ; steps++ {
				if steps > len(ring) {
					return nil, geom.ErrConvergence
				}
				id := ring[position%len(ring)]
				uv := r.VertexUV[id]
				p := [2]*big.Rat{ratInset(uv.X), ratInset(uv.Y)}
				t := new(big.Rat).Quo(new(big.Rat).Sub(p[axis], start[axis]), delta)
				if t.Sign() < 0 || t.Cmp(big.NewRat(1, 1)) > 0 {
					return nil, geom.ErrConvergence
				}
				if len(item.Parameters) == 0 && t.Sign() != 0 {
					return nil, geom.ErrConvergence
				}
				if len(item.Parameters) > 0 && t.Cmp(item.Parameters[len(item.Parameters)-1]) <= 0 {
					return nil, geom.ErrConvergence
				}
				for k := 0; k < 2; k++ {
					v := new(big.Rat).Add(start[k], new(big.Rat).Mul(t, new(big.Rat).Sub(end[k], start[k])))
					if v.Cmp(p[k]) != 0 {
						return nil, geom.ErrConvergence
					}
				}
				item.Vertices = append(item.Vertices, id)
				item.Parameters = append(item.Parameters, t)
				if t.Cmp(big.NewRat(1, 1)) == 0 {
					break
				}
				position++
			}
			out = append(out, item)
		}
		if position != len(ring) {
			return nil, geom.ErrConvergence
		}
	}
	return out, nil
}

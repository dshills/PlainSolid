//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// WrappedAnisotropicAnnularSection is an exact horizontal material section.
// Outer and Inner retain each original wall's U basis and exact V parameter.
// InnerReversed directs the inner loop clockwise when viewed from +Z; the
// stored exact curve itself keeps its authored parameterization. Faces/Rim
// index the authenticated bore, never a reconstructed filled operand. Cap=-1
// at nonterminal heights. Different original station grids remain separate.
type WrappedAnisotropicAnnularSection struct {
	Outer, Inner  WrappedAnisotropicStationSection
	InnerReversed bool
}

func (s Solid) SectionWrappedAnisotropicStationBoreRational(ctx context.Context, height *big.Rat, o geom.SurfacePatchOptions) (WrappedAnisotropicAnnularSection, error) {
	var empty WrappedAnisotropicAnnularSection
	if height == nil {
		return empty, geom.ErrInvalid
	}
	p, sources, e := s.validateAnisotropicStationBore(ctx, o)
	if e != nil {
		return empty, e
	}
	if height.Cmp(p.Outer.Heights[0]) < 0 || height.Cmp(p.Outer.Heights[len(p.Outer.Heights)-1]) > 0 {
		return empty, geom.ErrDomain
	}
	var loops [2]WrappedAnisotropicStationSection
	for side, proof := range []WrappedAnisotropicStationProof{p.Outer, p.Inner} {
		walls := p.OuterWalls
		if side == 1 {
			walls = p.InnerWalls
		}
		i := sort.Search(len(walls), func(i int) bool { return height.Cmp(proof.Heights[i+1]) <= 0 })
		profile, e := geom.SectionWrappedAnisotropicRational(ctx, s.faces[walls[i]].Surface.(*geom.WrappedSurfacePatch), height, o)
		if e != nil {
			return empty, e
		}
		q := WrappedAnisotropicStationSection{Profile: profile, Faces: []int{walls[i]}, Cap: -1}
		for j, h := range proof.Heights {
			if height.Cmp(h) != 0 {
				continue
			}
			if j == 0 {
				q.Cap = p.Caps[0]
			} else if j == len(proof.Heights)-1 {
				q.Cap = p.Caps[1]
			} else {
				q.Faces = appendUniqueFace(q.Faces, walls[j])
				q.Profile.Cap = 0
			}
			// Full source authentication already proved the coedge/model pairing.
			// Resolve each rim by its original exact UV occurrence on this band.
			src := sources[side].faces[proof.History.WallFaces[i]]
			dst := s.faces[walls[i]]
			for _, edge := range proof.History.StationRims[j] {
				id := -1
				for _, u := range src.Outer {
					if u.Edge == edge {
						for _, v := range dst.Outer {
							if u.UVStart == v.UVStart && u.UVEnd == v.UVEnd {
								if id >= 0 && id != v.Edge {
									return empty, geom.ErrConvergence
								}
								id = v.Edge
							}
						}
					}
				}
				if id < 0 {
					return empty, geom.ErrConvergence
				}
				q.Rim = append(q.Rim, id)
			}
		}
		if q.Cap >= 0 {
			q.Faces = appendUniqueFace(q.Faces, q.Cap)
		}
		sort.Ints(q.Faces)
		loops[side] = q
	}
	return WrappedAnisotropicAnnularSection{Outer: loops[0], Inner: loops[1], InnerReversed: true}, nil
}

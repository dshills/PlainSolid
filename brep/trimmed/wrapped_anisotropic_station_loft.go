package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// WrappedAnisotropicStationProof authenticates original native weighted walls
// with independent X/Y station scales. Scales are control scales, not inferred
// ellipse radii. All history IDs refer to the supplied native material.
type WrappedAnisotropicStationProof struct {
	Center   [2]*big.Rat
	Heights  []*big.Rat
	Scales   [][2]*big.Rat
	History  WrappedLoftHistory
	Complete bool
}

func buildWrappedAnisotropicBand(ctx context.Context, w *geom.WrappedSurfacePatch, tolerance float64, o geom.SurfacePatchOptions) (Solid, error) {
	p, e := geom.CertifyWrappedAnisotropicEmbedding(ctx, w, o)
	if e != nil {
		return Solid{}, e
	}
	return buildCanonicalWrappedFrustum(ctx, w, tolerance, o, [2]*big.Rat{p.LowerZ, p.UpperZ}, p.QuarterParameters)
}

// NewWrappedAnisotropicStationLoft admits 1..16 independently scaled original
// weighted-quadrant bands. Ordered height slabs prove global nonintersection;
// strictly positive independent scales prove each section simple. Full exact
// shared rim/model-pcurve equality, not only station corners, is mandatory.
// Existing circular/homothetic APIs and certified meshes are not widened.
// Arithmetic quotas apply per bounded band/topology proof pass, at most16 bands;
// exact edge matching retains its own supplied options. Failure is atomic.
func NewWrappedAnisotropicStationLoft(ctx context.Context, bands []*geom.WrappedSurfacePatch, tolerance float64, o geom.SurfacePatchOptions) (Solid, WrappedLoftHistory, error) {
	if ctx == nil || len(bands) < 1 || len(bands) > 16 || !finite(tolerance) || tolerance <= 0 {
		return Solid{}, WrappedLoftHistory{}, geom.ErrInvalid
	}
	var prev geom.WrappedAnisotropicEmbedding
	for i, w := range bands {
		p, e := geom.CertifyWrappedAnisotropicEmbedding(ctx, w, o)
		if e != nil {
			return Solid{}, WrappedLoftHistory{}, e
		}
		if i > 0 {
			if p.LowerZ.Cmp(prev.UpperZ) != 0 {
				return Solid{}, WrappedLoftHistory{}, geom.ErrConvergence
			}
			for axis := 0; axis < 2; axis++ {
				if p.Center[axis].Cmp(prev.Center[axis]) != 0 || p.LowerScale[axis].Cmp(prev.UpperScale[axis]) != 0 {
					return Solid{}, WrappedLoftHistory{}, geom.ErrConvergence
				}
			}
		}
		prev = p
	}
	return joinWrappedStationBandsUsing(ctx, bands, tolerance, o, buildWrappedAnisotropicBand)
}

func (s Solid) ValidateWrappedAnisotropicStationLoft(ctx context.Context, o geom.SurfacePatchOptions) (WrappedAnisotropicStationProof, error) {
	p, _, e := s.validateWrappedAnisotropicStationLoft(ctx, o)
	return p, e
}

func (s Solid) validateWrappedAnisotropicStationLoft(ctx context.Context, o geom.SurfacePatchOptions) (WrappedAnisotropicStationProof, []*geom.WrappedSurfacePatch, error) {
	empty := WrappedAnisotropicStationProof{}
	if ctx == nil {
		return empty, nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, nil, e
	}
	n := len(s.faces) - 2
	if n < 1 || n > 16 || len(s.vertices) != 4*(n+1) || len(s.edges) != 5*n+4 {
		return empty, nil, geom.ErrConvergence
	}
	type wall struct {
		id int
		w  *geom.WrappedSurfacePatch
		p  geom.WrappedAnisotropicEmbedding
	}
	var walls []wall
	var caps []int
	for id, f := range s.faces {
		if len(f.Holes) != 0 {
			return empty, nil, geom.ErrConvergence
		}
		switch w := f.Surface.(type) {
		case *geom.WrappedSurfacePatch:
			if f.Reversed || len(f.Outer) != 10 {
				return empty, nil, geom.ErrConvergence
			}
			p, e := geom.CertifyWrappedAnisotropicEmbedding(ctx, w, o)
			if e != nil {
				return empty, nil, e
			}
			walls = append(walls, wall{id, w, p})
		case geom.PlaneSurface:
			if len(f.Outer) != 4 {
				return empty, nil, geom.ErrConvergence
			}
			caps = append(caps, id)
		default:
			return empty, nil, geom.ErrConvergence
		}
	}
	if len(walls) != n || len(caps) != 2 || s.faces[caps[0]].Reversed == s.faces[caps[1]].Reversed {
		return empty, nil, geom.ErrConvergence
	}
	if !s.faces[caps[0]].Reversed {
		caps[0], caps[1] = caps[1], caps[0]
	}
	sort.Slice(walls, func(i, j int) bool { return walls[i].p.LowerZ.Cmp(walls[j].p.LowerZ) < 0 })
	bands := make([]*geom.WrappedSurfacePatch, n)
	p := WrappedAnisotropicStationProof{Center: walls[0].p.Center, Heights: []*big.Rat{walls[0].p.LowerZ}, Scales: [][2]*big.Rat{walls[0].p.LowerScale}}
	for i, w := range walls {
		bands[i] = w.w
		p.Heights = append(p.Heights, w.p.UpperZ)
		p.Scales = append(p.Scales, w.p.UpperScale)
		p.History.WallFaces = append(p.History.WallFaces, w.id)
	}
	reference, h, e := NewWrappedAnisotropicStationLoft(ctx, bands, s.tolerance, o)
	if e != nil {
		return empty, nil, e
	}
	faceIDs := append(append([]int(nil), p.History.WallFaces...), caps...)
	ids, e := matchWrappedStationLoft(ctx, s, reference, faceIDs, o)
	if e != nil {
		return empty, nil, e
	}
	p.History.CapFaces = [2]int{caps[0], caps[1]}
	p.History.StationRims = h.StationRims
	for i, rim := range h.StationRims {
		for j, id := range rim {
			p.History.StationRims[i][j] = ids[id]
		}
	}
	p.Complete = true
	return p, bands, nil
}

// WrappedAnisotropicStationSection retains exact original source parameters.
// Faces includes both walls at a crease, and the cap at a terminal plane. Rim
// lists the original four edges only when height is exactly a station. Cap is
// -1 away from original terminal caps (internal station disks are not caps).
type WrappedAnisotropicStationSection struct {
	Profile    geom.WrappedAnisotropicSection
	Faces, Rim []int
	Cap        int
}

func (s Solid) SectionWrappedAnisotropicStationRational(ctx context.Context, height *big.Rat, o geom.SurfacePatchOptions) (WrappedAnisotropicStationSection, error) {
	empty := WrappedAnisotropicStationSection{}
	if height == nil {
		return empty, geom.ErrInvalid
	}
	p, bands, e := s.validateWrappedAnisotropicStationLoft(ctx, o)
	if e != nil {
		return empty, e
	}
	if height.Cmp(p.Heights[0]) < 0 || height.Cmp(p.Heights[len(p.Heights)-1]) > 0 {
		return empty, geom.ErrDomain
	}
	i := sort.Search(len(bands), func(i int) bool { return height.Cmp(p.Heights[i+1]) <= 0 })
	profile, e := geom.SectionWrappedAnisotropicRational(ctx, bands[i], height, o)
	if e != nil {
		return empty, e
	}
	out := WrappedAnisotropicStationSection{Profile: profile, Faces: []int{p.History.WallFaces[i]}, Cap: -1}
	for j, h := range p.Heights {
		if height.Cmp(h) != 0 {
			continue
		}
		out.Rim = append([]int(nil), p.History.StationRims[j][:]...)
		if j == 0 {
			out.Cap = p.History.CapFaces[0]
		} else if j == len(p.Heights)-1 {
			out.Cap = p.History.CapFaces[1]
		} else {
			out.Faces = appendUniqueFace(out.Faces, p.History.WallFaces[j])
			out.Profile.Cap = 0
		}
	}
	if out.Cap >= 0 {
		out.Faces = appendUniqueFace(out.Faces, out.Cap)
	}
	sort.Ints(out.Faces)
	return out, nil
}

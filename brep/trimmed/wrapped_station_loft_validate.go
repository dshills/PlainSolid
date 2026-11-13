package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

// WrappedStationLoftProof describes a complete outward circular station
// loft. Geometry is in exact model coordinates and History uses the original
// solid's IDs. Each station is one shared four-edge rim, including at creases.
type WrappedStationLoftProof struct {
	Center   [2]*big.Rat
	Heights  []*big.Rat
	Radii    []*big.Rat
	History  WrappedLoftHistory
	Complete bool
}

// ValidateWrappedCircularStationLoft independently certifies the exact
// canonical band and cap topology, including after native reload or
// reindexing. Each band is an outward circular frustum with matching
// neighboring height, radius and axis. Trim and model curves must retain
// exact agreement. Different subdivisions of the same rim are unsupported.
func (s Solid) ValidateWrappedCircularStationLoft(ctx context.Context, o geom.SurfacePatchOptions) (WrappedStationLoftProof, error) {
	p, _, e := s.validateWrappedCircularStationLoft(ctx, o)
	return p, e
}

func (s Solid) validateWrappedCircularStationLoft(ctx context.Context, o geom.SurfacePatchOptions) (p WrappedStationLoftProof, bands []*geom.WrappedSurfacePatch, err error) {
	if ctx == nil {
		return p, nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return p, nil, e
	}
	n := len(s.faces) - 2
	if n < 1 || n > 127 || len(s.vertices) != 4*(n+1) || len(s.edges) != 5*n+4 {
		return p, nil, geom.ErrConvergence
	}
	type wall struct {
		id int
		w  *geom.WrappedSurfacePatch
		d  geom.WrappedFrustumEmbedding
	}
	var walls []wall
	var caps []int
	for id, f := range s.faces {
		if len(f.Holes) != 0 {
			return p, nil, geom.ErrConvergence
		}
		switch w := f.Surface.(type) {
		case *geom.WrappedSurfacePatch:
			if f.Reversed || len(f.Outer) != 10 {
				return p, nil, geom.ErrConvergence
			}
			d, e := geom.CertifyWrappedFrustumEmbedding(ctx, w, o)
			if e != nil {
				return p, nil, e
			}
			if !d.Circular {
				return p, nil, geom.ErrConvergence
			}
			walls = append(walls, wall{id, w, d})
		case geom.PlaneSurface:
			if len(f.Outer) != 4 {
				return p, nil, geom.ErrConvergence
			}
			caps = append(caps, id)
		default:
			return p, nil, geom.ErrConvergence
		}
	}
	if len(walls) != n || len(caps) != 2 || s.faces[caps[0]].Reversed == s.faces[caps[1]].Reversed {
		return p, nil, geom.ErrConvergence
	}
	if !s.faces[caps[0]].Reversed {
		caps[0], caps[1] = caps[1], caps[0]
	}
	sort.Slice(walls, func(i, j int) bool { return walls[i].d.LowerZ.Cmp(walls[j].d.LowerZ) < 0 })
	a := walls[0].d
	p.Center = [2]*big.Rat{a.CenterX, a.CenterY}
	p.Heights, p.Radii = []*big.Rat{a.LowerZ}, []*big.Rat{a.LowerRadius}
	for _, w := range walls {
		if w.d.CenterX.Cmp(p.Center[0]) != 0 || w.d.CenterY.Cmp(p.Center[1]) != 0 ||
			w.d.LowerZ.Cmp(p.Heights[len(p.Heights)-1]) != 0 ||
			w.d.LowerRadius.Cmp(p.Radii[len(p.Radii)-1]) != 0 {
			return WrappedStationLoftProof{}, nil, geom.ErrConvergence
		}
		p.Heights, p.Radii = append(p.Heights, w.d.UpperZ), append(p.Radii, w.d.UpperRadius)
		p.History.WallFaces = append(p.History.WallFaces, w.id)
		bands = append(bands, w.w)
	}
	p.History.CapFaces = [2]int{caps[0], caps[1]}
	reference, h, e := joinWrappedStationBands(ctx, bands, s.tolerance, o)
	if e != nil {
		return WrappedStationLoftProof{}, nil, e
	}
	faceIDs := append(append([]int(nil), p.History.WallFaces...), caps...)
	edgeIDs, e := matchWrappedStationLoft(ctx, s, reference, faceIDs, o)
	if e != nil {
		return WrappedStationLoftProof{}, nil, e
	}
	p.History.StationRims = h.StationRims
	for i, rim := range h.StationRims {
		for j, id := range rim {
			p.History.StationRims[i][j] = edgeIDs[id]
		}
	}
	p.Complete = true
	return p, bands, nil
}

func matchWrappedStationLoft(ctx context.Context, original, reference Solid, faceIDs []int, o geom.SurfacePatchOptions) ([]int, error) {
	edges, vertices := make([]int, len(reference.edges)), make([]int, len(reference.vertices))
	for i := range edges {
		edges[i] = -1
	}
	for i := range vertices {
		vertices[i] = -1
	}
	reverse := make([]bool, len(edges))
	usedEdges, usedVertices := make([]bool, len(edges)), make([]bool, len(vertices))
	for i, id := range faceIDs {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		want, have := reference.faces[i], original.faces[id]
		if want.Reversed != have.Reversed || want.Surface != have.Surface || len(want.Outer) != len(have.Outer) {
			return nil, geom.ErrConvergence
		}
		shift := -1
		for j, u := range have.Outer {
			if want.Outer[0].UVStart == u.UVStart {
				if shift >= 0 {
					return nil, geom.ErrConvergence
				}
				shift = j
			}
		}
		if shift < 0 {
			return nil, geom.ErrConvergence
		}
		for j, a := range want.Outer {
			b := have.Outer[(j+shift)%len(have.Outer)]
			flip := a.Reversed != b.Reversed
			if a.UVStart != b.UVStart || a.UVEnd != b.UVEnd || (a.Curve == nil) != (b.Curve == nil) {
				return nil, geom.ErrConvergence
			}
			if edges[a.Edge] >= 0 {
				if edges[a.Edge] != b.Edge || reverse[a.Edge] != flip {
					return nil, geom.ErrConvergence
				}
				continue
			}
			if usedEdges[b.Edge] {
				return nil, geom.ErrConvergence
			}
			x, y := reference.edges[a.Edge], original.edges[b.Edge]
			u, ur, e := exactEdgeSplineWithOptions(x, o)
			if e != nil {
				return nil, e
			}
			v, vr, e := exactEdgeSplineWithOptions(y, o)
			if e != nil {
				return nil, e
			}
			if u == nil || v == nil {
				return nil, geom.ErrConvergence
			}
			same, e := geom.CompareExactCurvesContext(ctx, u, v, geom.ExactCurveComparisonOptions{
				NormalizeDomains: true, ReverseB: (ur != vr) != flip,
				MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
			})
			if e != nil {
				return nil, e
			}
			if !same.Complete || !same.Equal {
				return nil, geom.ErrConvergence
			}
			if flip {
				y.Start, y.End = y.End, y.Start
			}
			for _, z := range [][2]int{{x.Start, y.Start}, {x.End, y.End}} {
				k, l := z[0], z[1]
				if vertices[k] >= 0 {
					if vertices[k] != l {
						return nil, geom.ErrConvergence
					}
					continue
				}
				if usedVertices[l] || reference.vertices[k] != original.vertices[l] {
					return nil, geom.ErrConvergence
				}
				vertices[k], usedVertices[l] = l, true
			}
			edges[a.Edge], reverse[a.Edge], usedEdges[b.Edge] = b.Edge, flip, true
		}
	}
	agreement, e := original.ValidateEdgeAgreement(ctx, edgecheck.Options{MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits})
	if e != nil {
		return nil, e
	}
	if !agreement.Complete {
		return nil, geom.ErrConvergence
	}
	for _, u := range agreement.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			return nil, geom.ErrConvergence
		}
	}
	return edges, nil
}

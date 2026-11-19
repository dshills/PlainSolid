//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ProjectiveSplineFaceContactOptions bounds exact trim/support preparation,
// the original contact graph and the binding of its vertices to trim uses.
// Preparation and boundary binding each default to four million operations.
type ProjectiveSplineFaceContactOptions struct {
	Patches                                         geom.SurfacePatchOptions
	Graph                                           geom.ProjectiveSurfaceContactGraphOptions
	MaxPreparationOperations, MaxBoundaryOperations int
}

// ProjectiveSplineFaceCoedgeContact retains the original outer-loop use ID,
// its declared model edge ID, and both original normalized parameters. The
// edge parameter reverses the use clock when the model edge is reversed.
type ProjectiveSplineFaceCoedgeContact struct {
	Loop, Use, Edge             int // Outer loop is 0; holes retain input order from 1.
	UseParameter, EdgeParameter *geom.ExactAlgebraicRoot
}

// ProjectiveSplineFaceVertex binds one vertex of Graph to its incident
// original coedges. A corner retains both coedges on that owner.
type ProjectiveSplineFaceVertex struct {
	Vertex int
	Uses   [2][]ProjectiveSplineFaceCoedgeContact
}

// ProjectiveSplineFaceContacts contains a complete exact section on two
// rectangular trimmed spline faces. Domains are in the original UV
// charts. The graph is empty only if the certified trimmed faces are
// disjoint within their positive-area common projection.
type ProjectiveSplineFaceContacts struct {
	Domains                                   [2]geom.ParameterRectangle
	Graph                                     geom.ProjectiveSurfaceContactGraph
	Vertices                                  []ProjectiveSplineFaceVertex
	PreparationOperations, BoundaryOperations int
	Complete                                  bool
}

// TraceProjectiveSplineFaceContacts restricts each original support to its
// exact four-sided outer wire before tracing the section. Strictly separated
// rectangular holes inside the common projection retain their original
// clockwise uses. Curved trims, coincidence and zero-area projected overlap
// remain outside this subset. Face's edge numbers describe use ownership;
// this query does not independently certify a separate model edge's geometry.
func TraceProjectiveSplineFaceContacts(ctx context.Context, a, b Face, axes [2]int,
	o ProjectiveSplineFaceContactOptions) (r ProjectiveSplineFaceContacts, err error) {
	if ctx == nil {
		return r, geom.ErrInvalid
	}
	if o.MaxPreparationOperations == 0 {
		o.MaxPreparationOperations = 4_000_000
	}
	if o.MaxBoundaryOperations == 0 {
		o.MaxBoundaryOperations = 4_000_000
	}
	if o.MaxPreparationOperations < 1 || o.MaxBoundaryOperations < 1 || o.Patches.MaxOperations < 0 {
		return r, geom.ErrInvalid
	}
	if len(o.Graph.Cover.OriginalHoles[0])+len(o.Graph.Cover.OriginalHoles[1]) != 0 {
		return r, geom.ErrInvalid // Material comes from the original faces.
	}
	faces := [2]Face{a, b}
	var domains [2]geom.ParameterRectangle
	var patches [2]*geom.SurfacePatch
	for i, f := range faces {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		if f.Surface == nil || len(f.Holes) > 32 {
			return r, geom.ErrConvergence
		}
		d, err := projectiveSplineFaceRectangle(f.Outer, false)
		if err != nil {
			return r, err
		}
		domains[i] = d
		var prior []geom.ParameterRectangle
		for _, loop := range f.Holes {
			h, err := projectiveSplineFaceRectangle(loop, true)
			if err != nil {
				return r, err
			}
			if h.UMin <= d.UMin || h.UMax >= d.UMax || h.VMin <= d.VMin || h.VMax >= d.VMax {
				return r, geom.ErrConvergence
			}
			for _, a := range prior {
				if a.UMax >= h.UMin && h.UMax >= a.UMin && a.VMax >= h.VMin && h.VMax >= a.VMin {
					return r, geom.ErrConvergence
				}
			}
			prior = append(prior, h)
			o.Graph.Cover.OriginalHoles[i] = append(o.Graph.Cover.OriginalHoles[i], geom.ExactSurfaceRectangle{
				U: geom.ExactParameterInterval{Lower: new(big.Rat).SetFloat64(h.UMin), Upper: new(big.Rat).SetFloat64(h.UMax)},
				V: geom.ExactParameterInterval{Lower: new(big.Rat).SetFloat64(h.VMin), Upper: new(big.Rat).SetFloat64(h.VMax)},
			})
		}
		limits := o.Patches
		remaining := func() int {
			left := o.MaxPreparationOperations - r.PreparationOperations
			if o.Patches.MaxOperations > 0 {
				left = min(left, o.Patches.MaxOperations)
			}
			return left
		}
		limits.MaxOperations = remaining()
		if limits.MaxOperations < 1 {
			return r, geom.ErrConvergence
		}
		base, pose := parametric.SurfaceMapping(f.Surface)
		p, err := exactSplineSupportPatch(base, limits)
		if err != nil {
			return r, err
		}
		// Reused exact supports can carry historical construction work.
		// Charge only a newly extracted ordinary spline support.
		if _, ok := base.(*geom.BSplineSurface); ok {
			r.PreparationOperations += p.ExactOperations()
		}
		if p.Domain() != d {
			left := remaining()
			if left < 1 {
				return r, geom.ErrConvergence
			}
			q, used, err := p.TrimWithWorkLimit(d, left)
			r.PreparationOperations += used
			if err != nil {
				return r, err
			}
			p = q
		}
		if pose != geom.Identity() {
			left := remaining()
			if left < 1 {
				return r, geom.ErrConvergence
			}
			q, used, err := p.TransformedWithWorkLimit(pose, left)
			r.PreparationOperations += used
			if err != nil {
				return r, err
			}
			p = q
		}
		patches[i] = p
	}
	graph, err := geom.TraceProjectiveSurfaceContacts(ctx, patches[0], patches[1], axes, o.Graph)
	if err != nil {
		return r, err
	}
	vertices := make([]ProjectiveSplineFaceVertex, len(graph.Vertices))
	limits := func() (geom.ExactCurvePlaneEventOptions, error) {
		left := o.MaxBoundaryOperations - r.BoundaryOperations
		if left < 1 {
			return geom.ExactCurvePlaneEventOptions{}, geom.ErrConvergence
		}
		return geom.ExactCurvePlaneEventOptions{MaxOperations: left, MaxCoefficientBits: o.Patches.MaxCoefficientBits}, nil
	}
	one := big.NewRat(1, 1)
	for i, v := range graph.Vertices {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		vertices[i].Vertex = i
		for owner, f := range faces {
			loops := append([][]Use{f.Outer}, f.Holes...)
			for loop, wire := range loops {
				for use, u := range wire {
					start := [2]float64{u.UVStart.X, u.UVStart.Y}
					end := [2]float64{u.UVEnd.X, u.UVEnd.Y}
					fixed := 0
					if start[1] == end[1] {
						fixed = 1
					}
					root := v.OriginalUV[owner][fixed]
					value := new(big.Rat).SetFloat64(start[fixed])
					display := root.Interval()
					if display.Lower.Cmp(value) > 0 || display.Upper.Cmp(value) < 0 {
						continue
					}
					opt, err := limits()
					if err != nil {
						return r, err
					}
					sign, err := root.SignPolynomial(ctx, []*big.Rat{new(big.Rat).Neg(value), one}, opt)
					r.BoundaryOperations += sign.Operations
					if err != nil {
						return r, err
					}
					if sign.Sign != 0 {
						continue
					}
					free := 1 - fixed
					freeRoot := v.OriginalUV[owner][free]
					if loop != 0 {
						// A hole side has a finite extent inside the outer
						// rectangle; equality on its extension is insufficient.
						inside, used, err := projectiveSplineStationOnTrimSegment(ctx, freeRoot, start[free], end[free], limits)
						r.BoundaryOperations += used
						if err != nil {
							return r, err
						}
						if !inside {
							continue
						}
					}
					first := new(big.Rat).SetFloat64(start[free])
					span := new(big.Rat).Sub(new(big.Rat).SetFloat64(end[free]), first)
					scale := new(big.Rat).Inv(span)
					offset := new(big.Rat).Neg(new(big.Rat).Mul(first, scale))
					opt, err = limits()
					if err != nil {
						return r, err
					}
					station, used, err := freeRoot.AffineMap(ctx, offset, scale, opt)
					r.BoundaryOperations += used
					if err != nil {
						return r, err
					}
					edge := station
					if u.Reversed {
						opt, err = limits()
						if err != nil {
							return r, err
						}
						edge, used, err = station.AffineMap(ctx, one, big.NewRat(-1, 1), opt)
						r.BoundaryOperations += used
						if err != nil {
							return r, err
						}
					}
					vertices[i].Uses[owner] = append(vertices[i].Uses[owner], ProjectiveSplineFaceCoedgeContact{
						Loop: loop, Use: use, Edge: u.Edge, UseParameter: station, EdgeParameter: edge})
				}
			}
		}
		if (v.OnOverlapBoundary || v.OnTrimBoundary) && len(vertices[i].Uses[0])+len(vertices[i].Uses[1]) == 0 {
			return r, geom.ErrConvergence
		}
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.Domains, r.Graph, r.Vertices, r.Complete = domains, graph, vertices, true
	return r, nil
}

func projectiveSplineStationOnTrimSegment(ctx context.Context, root *geom.ExactAlgebraicRoot, a, b float64,
	limits func() (geom.ExactCurvePlaneEventOptions, error)) (bool, int, error) {
	var work int
	lo, hi := new(big.Rat).SetFloat64(min(a, b)), new(big.Rat).SetFloat64(max(a, b))
	d := root.Interval()
	if d.Upper.Cmp(lo) < 0 || d.Lower.Cmp(hi) > 0 {
		return false, 0, nil
	}
	for side, cap := range [2]*big.Rat{lo, hi} {
		if side == 0 && d.Lower.Cmp(cap) >= 0 || side == 1 && d.Upper.Cmp(cap) <= 0 {
			continue
		}
		o, err := limits()
		if err != nil {
			return false, work, err
		}
		o.MaxOperations -= work
		if o.MaxOperations < 1 {
			return false, work, geom.ErrConvergence
		}
		proof, err := root.SignPolynomial(ctx, []*big.Rat{new(big.Rat).Neg(cap), big.NewRat(1, 1)}, o)
		work += proof.Operations
		if err != nil {
			return false, work, err
		}
		if side == 0 && proof.Sign < 0 || side == 1 && proof.Sign > 0 {
			return false, work, nil
		}
	}
	return true, work, nil
}

func projectiveSplineFaceRectangle(wire []Use, clockwise bool) (geom.ParameterRectangle, error) {
	if len(wire) != 4 {
		return geom.ParameterRectangle{}, geom.ErrConvergence
	}
	first := wire[0].UVStart
	d := geom.ParameterRectangle{UMin: first.X, UMax: first.X, VMin: first.Y, VMax: first.Y}
	edges := make(map[int]bool, 4)
	for i, use := range wire {
		p := use.UVStart
		if !finite(p.X) || !finite(p.Y) || !finite(use.UVEnd.X) || !finite(use.UVEnd.Y) ||
			use.UVEnd != wire[(i+1)%4].UVStart || use.Edge < 0 || edges[use.Edge] {
			return geom.ParameterRectangle{}, geom.ErrInvalid
		}
		edges[use.Edge] = true
		d.UMin, d.UMax = min(d.UMin, p.X), max(d.UMax, p.X)
		d.VMin, d.VMax = min(d.VMin, p.Y), max(d.VMax, p.Y)
	}
	if !d.Valid() {
		return geom.ParameterRectangle{}, geom.ErrInvalid
	}
	corners := [4]geom.Vec2{{X: d.UMin, Y: d.VMin}, {X: d.UMax, Y: d.VMin}, {X: d.UMax, Y: d.VMax}, {X: d.UMin, Y: d.VMax}}
	direction := 1
	if clockwise {
		direction = -1
	}
	for start, corner := range corners {
		if wire[0].UVStart != corner {
			continue
		}
		for i, use := range wire {
			if use.UVStart != corners[(start+direction*i+4)%4] || use.UVEnd != corners[(start+direction*(i+1)+4)%4] || !exactAffineNativeSourceUse(use) {
				return geom.ParameterRectangle{}, geom.ErrConvergence
			}
		}
		return d, nil
	}
	return geom.ParameterRectangle{}, geom.ErrConvergence
}

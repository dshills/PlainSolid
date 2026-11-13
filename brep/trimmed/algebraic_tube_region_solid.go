package trimmed

import (
	"context"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// Solid projects an exact tube lens into the native trimmed B-rep when
// every section piece uses the complete original tube segment. The exact
// implicit evaluators remain paired and both original supports are retained.
// The native model uses bounded represented vertex witnesses; the lens is
// still authoritative for exact algebraic junction coordinates.
func (l AlgebraicTubeLens) Solid(ctx context.Context, tolerance float64, o edgecheck.Options) (Solid, error) {
	if l.graphs[0] == nil {
		return Solid{}, geom.ErrInvalid
	}
	f := l.Faces()
	var faces [2]Face
	for i, x := range f {
		faces[i] = Face{Surface: x.Support, Reversed: x.Reversed}
	}
	r := tubeRegionSolidRing{cut: l.faces[0].Cut, reversed: [2]bool{f[0].CutReversed, f[1].CutReversed}}
	return assembleTubeRegionSolid(ctx, faces, []tubeRegionSolidRing{r}, tolerance, o)
}

// Solid constructs one native trimmed component, with its original hole
// wires where present. Adjacent regions which touch at a zero-thickness
// rim are deliberately returned separately.
func (r AlgebraicTubeRegions) Solid(ctx context.Context, region int, tolerance float64, o edgecheck.Options) (Solid, error) {
	if region < 0 || region >= len(r.regions) {
		return Solid{}, geom.ErrInvalid
	}
	x := r.regions[region]
	f := r.RegionFaces(region)
	var faces [2]Face
	for i, z := range f {
		faces[i] = Face{Surface: z.Support, Reversed: z.Reversed}
	}
	rings := []tubeRegionSolidRing{{cut: r.partitions[x.Outer].Faces[0].Cut,
		reversed: [2]bool{f[0].OuterReversed, f[1].OuterReversed}}}
	for i, h := range x.Holes {
		rings = append(rings, tubeRegionSolidRing{cut: r.partitions[h].Faces[0].Cut,
			reversed: [2]bool{f[0].HolesReversed[i], f[1].HolesReversed[i]}})
	}
	return assembleTubeRegionSolid(ctx, faces, rings, tolerance, o)
}

type tubeRegionSolidRing struct {
	cut      []AlgebraicTubeFaceCutPiece
	reversed [2]bool
}

func assembleTubeRegionSolid(ctx context.Context, faces [2]Face, rings []tubeRegionSolidRing, tolerance float64, o edgecheck.Options) (Solid, error) {
	if ctx == nil || tolerance <= 0 || !finite(tolerance) {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	var vertices []geom.Vec3
	var edges []parametric.Edge
	for loop, ring := range rings {
		if len(ring.cut) < 3 || len(ring.cut) > 128 {
			return Solid{}, geom.ErrConvergence
		}
		first := len(vertices)
		var uses [2][]Use
		for j, piece := range ring.cut {
			if e := ctx.Err(); e != nil {
				return Solid{}, e
			}
			s := piece.Segment
			for k, r := range []*geom.ExactAlgebraicRoot{piece.FreeStart, piece.FreeEnd} {
				p, e := s.ContainsFreeRoot(ctx, r, geom.ExactCurvePlaneEventOptions{})
				if e != nil {
					return Solid{}, e
				}
				_, _, rev, e := s.ExactFreeDomain()
				if e != nil {
					return Solid{}, e
				}
				side := k
				if rev {
					side ^= 1
				}
				if !p.Proven || !p.Inside || p.BoundaryMask != 1<<side {
					return Solid{}, geom.ErrUnrepresentable
				}
			}
			a, e := s.Evaluate(0)
			if e != nil {
				return Solid{}, e
			}
			vertices = append(vertices, a.Point)
			id := len(edges)
			for chart, trim := range [2]intersect.TubeTrim{s.TrimA(), s.TrimB()} {
				p, e := trim.EvalUV(0)
				if e != nil {
					return Solid{}, e
				}
				q, e := trim.EvalUV(1)
				if e != nil {
					return Solid{}, e
				}
				if p == q {
					return Solid{}, geom.ErrUnrepresentable
				}
				uses[chart] = append(uses[chart], Use{Edge: id, UVStart: p, UVEnd: q, Curve: trim})
				if chart == 0 {
					edges = append(edges, parametric.Edge{Start: first + j, End: first + (j+1)%len(ring.cut),
						Range: parametric.Interval{Max: 1}, Curve: surfaceTrimCurve{faces[0].Surface, p, q, trim}})
				}
			}
		}
		for chart, u := range uses {
			if ring.reversed[chart] {
				out := make([]Use, len(u))
				for i, x := range u {
					x.Reversed = true
					x.UVStart, x.UVEnd = x.UVEnd, x.UVStart
					x.Curve = reversePCurve(x.Curve)
					out[len(u)-1-i] = x
				}
				u = out
			}
			if loop == 0 {
				faces[chart].Outer = u
			} else {
				faces[chart].Holes = append(faces[chart].Holes, u)
			}
		}
	}
	s, e := NewSolidContext(ctx, vertices, edges, faces[:], tolerance)
	if e != nil {
		return Solid{}, e
	}
	p, e := s.ValidateEdgeAgreement(ctx, o)
	if e != nil {
		return Solid{}, e
	}
	if !p.Complete || s.Topology().EulerCharacteristic != 2-2*(len(rings)-1) {
		return Solid{}, geom.ErrConvergence
	}
	return s, nil
}

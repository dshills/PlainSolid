package trimmed

import (
	"context"
	"fmt"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ParabolaMixedCirclesCutComponent identifies the component-local original
// wall, cap and crossing edges. SeparatedEdges and SeparatedVertices refer
// to the entire independent circle on the applicable shell, or contain -1.
type ParabolaMixedCirclesCutComponent struct {
	ParabolaCircleCrossingsCutHistory
	SeparatedEdges    [4]int
	SeparatedVertices [4]int
}

// ParabolaMixedCirclesCutContact records the original node in both
// contacting positive shells. An independent third shell has no contacts.
type ParabolaMixedCirclesCutContact struct {
	ParabolaCircleCrossingsCutContact
	Components [2]int
}

// ParabolaMixedCirclesCut retains the positive exterior and crossing cap.
// The separated circle is either a hole of the exterior or a separate lens.
type ParabolaMixedCirclesCut struct {
	sources    [3]*geom.BSplineSurface
	axis       int
	tolerance  float64
	components []Solid
	history    []ParabolaMixedCirclesCutComponent
	contacts   [2]ParabolaCircleCrossingsCutContact
}

func (r *ParabolaMixedCirclesCut) Components() []Solid {
	if r == nil {
		return nil
	}
	return slices.Clone(r.components)
}

func (r *ParabolaMixedCirclesCut) History() []ParabolaMixedCirclesCutComponent {
	if r == nil {
		return nil
	}
	return slices.Clone(r.history)
}

func (r *ParabolaMixedCirclesCut) Contacts() [2]ParabolaMixedCirclesCutContact {
	var out [2]ParabolaMixedCirclesCutContact
	if r != nil && len(r.components) >= 2 {
		for i, c := range cloneParabolaCrossingCutContacts(r.contacts) {
			out[i] = ParabolaMixedCirclesCutContact{ParabolaCircleCrossingsCutContact: c, Components: [2]int{0, 1}}
		}
	}
	return out
}

// CutParabolaMixedCirclesSplineGraphs subtracts B from A on their exact
// original stock. Only the two proved transverse nodes can be shared by
// distinct retained shells. All positive regions retain each original wire.
func CutParabolaMixedCirclesSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaMixedCirclesCut, error) {
	sources := [3]*geom.BSplineSurface{base, a, b}
	p, err := proveParabolaMixedCirclesPartition(ctx, sources, axis, tolerance, o)
	if err != nil {
		return nil, err
	}
	r := &ParabolaMixedCirclesCut{sources: sources, axis: axis, tolerance: tolerance}
	var histories [2]ParabolaCircleCrossingsCutHistory
	var walls [2]OpenSectionHistory
	evaluate := exactGraphVertexEvaluator(ctx, o.Proof)
	roofs := [2]*geom.BSplineSurface{a, b}
	buildHole := func(vertices []geom.Vec3, edges []parametric.Edge, faces []Face, tolerance float64) (Solid, error) {
		return newMixedCirclesHoledCutSolid(ctx, vertices, edges, faces, tolerance)
	}
	for region, src := range p.regions {
		if src.sign <= 0 {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		body, assembly, err := assembleGraphRegionDifferenceWithHoleBuilder(a, b, src, tolerance, buildHole, evaluate)
		if err != nil {
			return nil, fmt.Errorf("trimmed: mixed parabola-circle Cut region %d: %w", region, err)
		}
		h, err := bindParabolaMixedCirclesCutComponent(ctx, body, assembly, p, region, roofs, o.Proof)
		if err != nil {
			return nil, fmt.Errorf("trimmed: mixed parabola-circle Cut ownership for region %d: %w", region, err)
		}
		if region < 4 {
			i := len(r.components)
			if i >= len(histories) {
				return nil, geom.ErrConvergence
			}
			histories[i], walls[i] = h.ParabolaCircleCrossingsCutHistory, assembly
		}
		r.components = append(r.components, body)
		r.history = append(r.history, h)
	}
	want := 2
	if p.proof.RegionSides[4] > 0 {
		want++
	}
	if len(r.components) != want || histories[0].Region >= 2 || histories[1].Region < 2 ||
		(len(p.regions[histories[0].Region].holes) == 1) != (want == 2) {
		return nil, geom.ErrConvergence
	}
	r.contacts, err = bindParabolaCrossingCutContacts(p.crossing, r.components, histories, walls)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}

// A transposed holed cap can initially fold its chord seeds. Validate
// one finer sampling level of the same original trims before publishing
// the result. This affects only this source-proved circular hole.
func newMixedCirclesHoledCutSolid(ctx context.Context, vertices []geom.Vec3, edges []parametric.Edge,
	faces []Face, tolerance float64) (Solid, error) {
	s, err := newSolidContext(ctx, vertices, edges, faces, tolerance, false)
	if err != nil {
		return Solid{}, err
	}
	_, err = s.refine(ctx, s.minimumLevel, 0, 0)
	if err == errTrimFold && s.minimumLevel == 1 {
		s.minimumLevel = 2
		_, err = s.refine(ctx, s.minimumLevel, 0, 0)
	}
	if err != nil {
		return Solid{}, err
	}
	return s, nil
}

func bindParabolaMixedCirclesCutComponent(ctx context.Context, body Solid, assembly OpenSectionHistory, p parabolaMixedCirclesPartition,
	region int, roofs [2]*geom.BSplineSurface, o geom.GraphPointContactOptions) (h ParabolaMixedCirclesCutComponent, err error) {
	h.SeparatedEdges = [4]int{-1, -1, -1, -1}
	h.SeparatedVertices = [4]int{-1, -1, -1, -1}
	if region == 4 {
		topo := body.Topology()
		if topo.Faces != 2 || topo.Edges != 4 || topo.Vertices != 4 || topo.InnerWires != 0 || len(assembly.WallFaces) != 0 {
			return h, geom.ErrConvergence
		}
		h.ParabolaCircleCrossingsCutHistory = ParabolaCircleCrossingsCutHistory{
			Region: region, RoofFaces: [2]int{assembly.SplineFace, assembly.PlaneFace},
			ParabolaEdges: [3]int{-1, -1, -1}, CircleEdges: [4]int{-1, -1, -1, -1},
			WallFaces: [3]int{-1, -1, -1}, DomainSides: [3]int{-1, -1, -1},
		}
		for owner, id := range h.RoofFaces {
			if err := bindParabolaCirclesRegionFace(ctx, body.faces[id], roofs[owner], owner == 1, p.regions[4], p.proof.RegionWitnesses[4]); err != nil {
				return h, err
			}
		}
		if err := validatePolynomialCutBoundary(ctx, body, assembly, o); err != nil {
			return h, err
		}
	} else {
		h.ParabolaCircleCrossingsCutHistory, err = bindParabolaCircleCrossingsCutComponent(ctx, body, assembly, p.crossing, region, roofs, o)
		if err != nil || len(p.regions[region].holes) == 0 {
			return h, err
		}
	}
	edges, err := bindParabolaCircleCrossingEdges(body, assembly.ContourEdges, p.regions[4].loop)
	if err != nil {
		return h, err
	}
	h.SeparatedEdges = [4]int(edges)
	h.SeparatedVertices, err = bindParabolaMixedCircleRing(body, edges, p)
	if err != nil {
		return h, err
	}
	a, b := body.faces[h.RoofFaces[0]], body.faces[h.RoofFaces[1]]
	loopA, loopB := a.Outer, b.Outer
	if region != 4 {
		loopA, loopB = a.Holes[0], b.Holes[0]
	}
	for _, id := range edges {
		// Cut face B is reversed. Both original pcurves therefore follow
		// the same local wire direction, with opposite shell incidence.
		l := slices.IndexFunc(loopA, func(use Use) bool { return use.Edge == id })
		m := slices.IndexFunc(loopB, func(use Use) bool { return use.Edge == id })
		model, ok := body.edges[id].Curve.(surfaceTrimCurve)
		if l < 0 || m < 0 || loopA[l].Reversed != loopB[m].Reversed || !ok || model.surface != roofs[0] {
			return h, geom.ErrConvergence
		}
	}
	return h, nil
}

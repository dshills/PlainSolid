package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/geom"
)

// ParabolaCircleApexCutHistory binds a positive component to its original
// sector, both roof supports and its original section and stock wall edges.
type ParabolaCircleApexCutHistory struct {
	Region       int
	RoofFaces    [2]int
	SectionEdges []int
	WallFaces    []int
	DomainSides  []int
}

// ParabolaCircleApexCutContact binds the exact shared apex to local component
// vertices and rays ordered by negative and positive free direction.
type ParabolaCircleApexCutContact struct {
	Parameters [2]*big.Rat
	Point      [3]*big.Rat
	Vertices   [2]int
	Rays       [2][2]int
}

// ParabolaCircleApexTangencyCut retains an embedded stock layer and an
// independently embedded positive circular lens. Their closures share only
// the original tangency point.
type ParabolaCircleApexTangencyCut struct {
	sources    [3]*geom.BSplineSurface
	components [2]Solid
	history    [2]ParabolaCircleApexCutHistory
	contact    ParabolaCircleApexCutContact
	axis       int
	tolerance  float64
}

func (r *ParabolaCircleApexTangencyCut) Components() [2]Solid {
	if r == nil {
		return [2]Solid{}
	}
	return r.components
}

func (r *ParabolaCircleApexTangencyCut) History() [2]ParabolaCircleApexCutHistory {
	if r == nil {
		return [2]ParabolaCircleApexCutHistory{}
	}
	out := r.history
	for i, h := range out {
		out[i].SectionEdges = slices.Clone(h.SectionEdges)
		out[i].WallFaces = slices.Clone(h.WallFaces)
		out[i].DomainSides = slices.Clone(h.DomainSides)
	}
	return out
}

func (r *ParabolaCircleApexTangencyCut) Contact() ParabolaCircleApexCutContact {
	if r == nil || r.sources[0] == nil {
		return ParabolaCircleApexCutContact{}
	}
	out := r.contact
	for i, q := range out.Parameters {
		out.Parameters[i] = new(big.Rat).Set(q)
	}
	for i, q := range out.Point {
		out.Point[i] = new(big.Rat).Set(q)
	}
	return out
}

// CutParabolaCircleApexTangencySplineGraphs builds the two positive shells
// when the disk and opposite open region have positive thickness. If the
// enclosing open region is positive instead, its two local material sectors
// touch at the apex and the same-shell pinch returns ErrDegenerate.
func CutParabolaCircleApexTangencySplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface,
	axis int, tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaCircleApexTangencyCut, error) {
	sources := [3]*geom.BSplineSurface{base, a, b}
	p, err := proveParabolaCircleApexPartition(ctx, sources, axis, tolerance, o)
	if err != nil {
		return nil, err
	}
	if p.proof.RegionSides[p.proof.DiskRegion] > 0 {
		return nil, fmt.Errorf("trimmed: parabola-circle Cut enclosing layer pinches at the exact apex: %w", geom.ErrDegenerate)
	}
	regions := [2]int{1 - p.proof.DiskRegion, 2}
	out := &ParabolaCircleApexTangencyCut{sources: sources, axis: axis, tolerance: tolerance,
		contact: ParabolaCircleApexCutContact{Parameters: p.proof.Parameters, Point: p.proof.Point}}
	eval := exactGraphVertexEvaluator(ctx, o.Proof)
	for part, region := range regions {
		if p.regions[region].sign <= 0 {
			return nil, geom.ErrConvergence
		}
		loop := p.regions[region].loop
		body, h, err := assembleOpenSplineGraphPairOnCharts(ctx, a, b, 0, true, tolerance, loop, [2]graphEnvelopeChart{}, nil, eval)
		if err != nil {
			return nil, err
		}
		history, err := bindParabolaCircleApexCutLayer(ctx, body, h, p, region, [2]*geom.BSplineSurface{a, b}, o.Proof)
		if err != nil {
			return nil, fmt.Errorf("trimmed: parabola-circle apex Cut region %d: %w", region, err)
		}
		v, err := exactParabolaCircleCrossingVertex(body, p.proof.Point)
		if err != nil {
			return nil, err
		}
		contactVertex, err := polynomialCutContactVertex(body, h, body.vertices[v])
		if err != nil || contactVertex != v {
			return nil, geom.ErrConvergence
		}
		second := p.proof.ParabolaSecondDerivative
		if region == 2 {
			second = p.proof.CircleSecondDerivative
		}
		rays := [2]int{-1, -1}
		for _, id := range h.ContourEdges {
			e := body.edges[id]
			if e.Start == v || e.End == v {
				sign, err := parabolaCircleApexRay(body, id, v, axis, second)
				if err != nil {
					return nil, err
				}
				i := (sign + 1) / 2
				if rays[i] >= 0 {
					return nil, geom.ErrConvergence
				}
				rays[i] = id
			}
		}
		if rays[0] < 0 || rays[1] < 0 {
			return nil, geom.ErrConvergence
		}
		out.components[part], out.history[part] = body, history
		out.contact.Vertices[part], out.contact.Rays[part] = v, rays
	}
	// A closed disk point in the opposite open region must lie on both
	// the parabola and circle. The exact substituted quartic proves just
	// the apex, where the original vertical interval collapses to a point.
	check := polynomialNodeCutMaterial{components: out.components[:], contacts: []TwoPolynomialNodeComponentContact{
		{Parameters: p.proof.Parameters, Point: p.proof.Point, Components: [2]int{0, 1}, Vertices: out.contact.Vertices},
	}}
	if err := check.validateContactIdentities(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func bindParabolaCircleApexCutLayer(ctx context.Context, s Solid, h OpenSectionHistory, p parabolaCircleApexPartition,
	region int, roofs [2]*geom.BSplineSurface, opts geom.GraphPointContactOptions) (out ParabolaCircleApexCutHistory, err error) {
	topo := s.Topology()
	want, contour, walls := [3]int{5, 10, 7}, 2, 3
	witness := p.proof.CircleCenter
	if region == 2 {
		want, contour, walls = [3]int{2, 4, 4}, 4, 0
	} else {
		axis := p.proof.FreeAxis
		witness = p.proof.Parameters
		ordinate := new(big.Rat).Set(witness[1-axis])
		if region == 1 {
			ordinate.Add(ordinate, big.NewRat(1, 1))
		}
		witness[1-axis] = ordinate.Quo(ordinate, big.NewRat(2, 1))
	}
	if ([3]int{topo.Faces, topo.Edges, topo.Vertices}) != want || topo.InnerWires != 0 ||
		len(h.ContourEdges) != contour || len(h.WallFaces) != walls || len(h.DomainSides) != walls {
		return out, geom.ErrConvergence
	}
	loop := p.regions[region].loop
	caps := [2]int{h.SplineFace, h.PlaneFace}
	for i, id := range caps {
		if err := bindParabolaCircleCrossingFace(ctx, s.faces[id], roofs[i], i == 1, loop, witness); err != nil {
			return out, err
		}
	}
	section, wall := 0, 0
	for i, piece := range loop {
		if piece.side < 0 {
			id := h.ContourEdges[section]
			model, ok := s.edges[id].Curve.(surfaceTrimCurve)
			if !ok || model.surface != roofs[0] || model.a != piece.a || model.b != piece.b ||
				!sameParabolaCircleCrossingTrim(model.curve, piece.c) ||
				s.faces[caps[0]].Outer[i].Edge != id || s.faces[caps[1]].Outer[i].Edge != id {
				return out, geom.ErrConvergence
			}
			section++
			continue
		}
		if h.DomainSides[wall] != piece.side {
			return out, geom.ErrConvergence
		}
		var seen [2]bool
		for _, use := range s.faces[h.WallFaces[wall]].Outer {
			for owner, roof := range roofs {
				t, ok := graphWallSourceTrim(use.Curve, owner == 0, false)
				if ok && t.surface == roof && t.a == piece.a && t.b == piece.b && use.Reversed == (owner == 0) {
					if seen[owner] {
						return out, geom.ErrConvergence
					}
					seen[owner] = true
				}
			}
		}
		if seen != ([2]bool{true, true}) {
			return out, geom.ErrConvergence
		}
		wall++
	}
	if err := validatePolynomialCutBoundary(ctx, s, h, opts); err != nil {
		return out, err
	}
	return ParabolaCircleApexCutHistory{Region: region, RoofFaces: caps, SectionEdges: h.ContourEdges,
		WallFaces: h.WallFaces, DomainSides: h.DomainSides}, nil
}

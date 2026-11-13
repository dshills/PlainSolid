package trimmed

import (
	"context"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type AlgebraicTubeGraphEnvelopeHistory struct {
	TopFaces            [2][]int
	Base                int
	Walls, SectionEdges []int
}

// AssembleAlgebraicTubeGraphEnvelope constructs a native union or
// intersection of strictly positive graph solids over a common base.
// This adapter requires unit identity XY source charts, a strictly signed
// perimeter and complete original tube edges. Disjoint and nested loops
// retain their outer or inner incidence on the original supports, with
// common base, original walls and paired section identity.
func (s AlgebraicTrimmedPairSection) AssembleAlgebraicTubeGraphEnvelope(ctx context.Context,
	base *geom.BSplineSurface, op parametric.BooleanOp, tolerance float64, o AlgebraicTubeGraphBooleanOptions,
	edge edgecheck.Options) (out Solid, h AlgebraicTubeGraphEnvelopeHistory, err error) {
	if ctx == nil || base == nil || s.partitionSource == nil || tolerance <= 0 || !finite(tolerance) ||
		op != parametric.Union && op != parametric.Intersection {
		return out, h, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, h, e
	}
	// A subtraction with exterior allowed supplies the original strict
	// base and perimeter order plus the complete shared region arrangement.
	d, e := s.SubtractAlgebraicTubeGraphSolidsWithExterior(ctx, base, 0, o)
	if e != nil {
		return out, h, e
	}
	r := d.regions
	for _, p := range []*geom.BSplineSurface{base, facespline(s.partitionSource.faces[0]), facespline(s.partitionSource.faces[1])} {
		if e := requireIdentitySplineChart(ctx, p, o.Patch); e != nil {
			return out, h, e
		}
	}
	for _, f := range s.partitionSource.faces {
		if !splineDistanceRectangle(f, geom.ParameterRectangle{UMax: 1, VMax: 1}) {
			return out, h, geom.ErrConvergence
		}
	}
	outside := 1
	if d.exterior {
		outside = 0
	}
	if op == parametric.Intersection {
		outside ^= 1
	}
	pieces := make([]openSectionPiece, 4)
	for i := range pieces {
		pieces[i] = openSectionPiece{a: unitSquare[i], b: unitSquare[(i+1)%4], side: i}
	}
	stock, stockHistory, e := assembleOpenSplineGraphPair(facespline(s.partitionSource.faces[outside]), base, 0, true, tolerance, pieces)
	if e != nil {
		return out, h, e
	}
	vertices, edges, faces := stock.Vertices(), stock.Edges(), stock.Faces()
	// Each exact shared ring is appended once. The first use is the
	// positive UV winding for the inner region; the second is reversed
	// for its immediate parent's hole (or the exterior's hole).
	type sharedRing struct{ use [2][]Use }
	rings := make([]sharedRing, len(r.loops))
	for i, x := range r.loops {
		q := r.partitions[i]
		var local [2]Face
		for c := range local {
			local[c] = Face{Surface: q.Faces[c].Support, Reversed: c != r.regions[i].UpperChart}
		}
		ring := tubeRegionSolidRing{cut: q.Faces[0].Cut, reversed: [2]bool{x.Winding[0] < 0, x.Winding[1] < 0}}
		shell, e := assembleTubeRegionSolid(ctx, local, []tubeRegionSolidRing{ring}, tolerance, edge)
		if e != nil {
			return out, h, e
		}
		vOffset, eOffset := len(vertices), len(edges)
		vertices = append(vertices, shell.vertices...)
		for _, p := range shell.edges {
			p.Start += vOffset
			p.End += vOffset
			edges = append(edges, p)
		}
		for c, f := range shell.faces {
			for _, u := range f.Outer {
				u.Edge += eOffset
				rings[i].use[c] = append(rings[i].use[c], u)
			}
		}
		for j := range shell.edges {
			h.SectionEdges = append(h.SectionEdges, eOffset+j)
		}
	}
	flip := func(in []Use) []Use {
		r := make([]Use, len(in))
		for i, u := range in {
			u.Reversed = !u.Reversed
			u.UVStart, u.UVEnd = u.UVEnd, u.UVStart
			u.Curve = reversePCurve(u.Curve)
			r[len(in)-1-i] = u
		}
		return r
	}
	for i, x := range r.loops {
		if x.Parent < 0 {
			faces[stockHistory.SplineFace].Holes = append(faces[stockHistory.SplineFace].Holes, flip(rings[i].use[outside]))
		}
	}
	h.TopFaces[outside] = []int{stockHistory.SplineFace}
	for i, x := range r.regions {
		c := x.UpperChart
		if op == parametric.Intersection {
			c ^= 1
		}
		f := Face{Surface: facespline(s.partitionSource.faces[c]), Outer: rings[i].use[c]}
		for _, j := range x.Holes {
			f.Holes = append(f.Holes, flip(rings[j].use[c]))
		}
		h.TopFaces[c] = append(h.TopFaces[c], len(faces))
		faces = append(faces, f)
	}
	result, e := NewSolidContext(ctx, vertices, edges, faces, tolerance)
	if e != nil {
		return out, h, e
	}
	p, e := result.ValidateEdgeAgreement(ctx, edge)
	if e != nil {
		return out, h, e
	}
	if !p.Complete || result.Topology().EulerCharacteristic != 2 {
		return out, h, geom.ErrConvergence
	}
	h.Base = stockHistory.PlaneFace
	h.Walls = append([]int(nil), stockHistory.WallFaces...)
	return result, h, nil
}

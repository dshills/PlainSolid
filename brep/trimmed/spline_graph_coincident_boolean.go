package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// CombinePartiallyCoincidentSplineGraphs combines strictly positive shared-base
// graphs when an exact knot-aligned partition contains coincident and separated
// cells. Envelopes retain the original supports and attribute coincident faces
// to both inputs. Subtraction sews exact signed component perimeters and rejects
// pinches, isolated contact vertices, and internal zero seams.
func CombinePartiallyCoincidentSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface,
	op parametric.BooleanOp, tolerance float64, o intersect.SplinePlaneOptions, proof geom.GraphPointContactOptions) (
	Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	if ctx == nil || base == nil || a == nil || b == nil || op > parametric.Difference ||
		!finite(tolerance) || tolerance <= 0 || o.Domain != (geom.ParameterRectangle{}) {
		return Solid{}, h, geom.ErrInvalid
	}
	p, e := prepareSharedBaseGraphContact(ctx, base, a, b, o)
	if e != nil {
		return Solid{}, h, e
	}
	return assemblePartitionedSplineEnvelope(ctx, base, a, b, p[0], p[1], op, tolerance, proof)
}

func assemblePartitionedSplineEnvelope(ctx context.Context, base, a, b *geom.BSplineSurface, ap, bp *geom.SurfacePatch,
	op parametric.BooleanOp, tolerance float64, proof geom.GraphPointContactOptions) (Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	if proof.MaxCells == 0 {
		proof.MaxCells = 64
	}
	if op == parametric.Difference {
		return assemblePartiallyCoincidentGraphDifference(ctx, a, b, ap, bp, tolerance, proof)
	}
	coincidence, e := geom.FindSplineGraphCoincidentComponents(ctx, ap, bp, proof)
	if e != nil {
		return Solid{}, h, e
	}
	p := coincidence.Partition
	if !p.Complete || len(p.Cells) < 2 || len(p.Cells) > 64 {
		return Solid{}, h, geom.ErrConvergence
	}
	regions := make([]graphEnvelopeRegion, 0, len(p.Cells))
	faceByCell := make([]int, len(p.Cells))
	for i, c := range p.Cells {
		faceByCell[i] = -1
		d := c.Domain
		if d.UMin < 0 || d.UMax > 1 || d.VMin < 0 || d.VMax > 1 {
			return Solid{}, h, geom.ErrConvergence
		}
		if c.Coincident {
			continue
		}
		faceByCell[i] = len(regions)
		regions = append(regions, graphEnvelopeRegion{loop: partitionGraphRectangleLoop(d), first: (c.Side > 0) == (op == parametric.Union)})
	}
	if len(coincidence.Components) == 0 || len(regions) == 0 {
		return Solid{}, h, geom.ErrConvergence
	}
	for _, c := range coincidence.Components {
		r := graphEnvelopeRegion{first: true}
		for _, ring := range c.Cycles {
			var loop []openSectionPiece
			for _, x := range ring.Edges {
				a, e := exactGraphPartitionUV(x.Start)
				if e != nil {
					return Solid{}, h, e
				}
				b, e := exactGraphPartitionUV(x.End)
				if e != nil {
					return Solid{}, h, e
				}
				side := -1
				zeroSide := p.ZeroSides[x.ZeroSide]
				if len(zeroSide.Cells) == 1 {
					side = graphPartitionDomainSide(x.Edge)
				}
				loop = append(loop, openSectionPiece{a: a, b: b, side: side})
			}
			if ring.Hole {
				r.holes = append(r.holes, loop)
			} else {
				if len(r.loop) != 0 {
					return Solid{}, h, geom.ErrConvergence
				}
				r.loop = loop
			}
		}
		if len(r.loop) == 0 {
			return Solid{}, h, geom.ErrConvergence
		}
		id := len(regions)
		h.CoincidentFaces = append(h.CoincidentFaces, id)
		for _, i := range c.Cells {
			faceByCell[i] = id
		}
		regions = append(regions, r)
	}
	for _, q := range p.Adjacencies {
		i, j := faceByCell[q.Cells[0]], faceByCell[q.Cells[1]]
		if i < 0 || j < 0 || regions[i].first != regions[j].first && q.ZeroBoundary < 0 {
			return Solid{}, h, geom.ErrConvergence
		}
	}

	out, report, e := assembleSplineEnvelope(ctx, base, a, b, regions, tolerance)
	if e != nil {
		return Solid{}, h, e
	}
	if e := validateExactGraphPartitionEdges(ctx, out, proof); e != nil {
		return Solid{}, h, e
	}
	report.ContactPartition, report.CoincidentFaces, report.CoincidentRegions = &p, h.CoincidentFaces, &coincidence
	report.PartitionFaceByCell = faceByCell
	report.FacesB = append(report.FacesB, h.CoincidentFaces...)
	if report.TopB < 0 {
		report.TopB = h.CoincidentFaces[0]
	}
	contact := make(map[[2]geom.Vec2]bool)
	for _, side := range p.ZeroSides {
		if len(side.Cells) != 2 {
			continue
		}
		var uv [2]geom.Vec2
		for i, variable := range side.Interval {
			fixed, e := geom.RepresentedExactScalar(side.Fixed)
			if e != nil {
				return Solid{}, h, e
			}
			v, e := geom.RepresentedExactScalar(variable)
			if e != nil {
				return Solid{}, h, e
			}
			uv[i] = geom.Vec2{X: fixed, Y: v}
			if side.FixedAxis == 1 {
				uv[i] = geom.Vec2{X: v, Y: fixed}
			}
		}
		contact[uv], contact[[2]geom.Vec2{uv[1], uv[0]}] = true, true
	}
	allGridEdges := report.ContourEdges
	report.ContourEdges = nil
	for _, id := range allGridEdges {
		c, ok := out.edges[id].Curve.(surfaceTrimCurve)
		if !ok {
			return Solid{}, h, geom.ErrConvergence
		}
		if contact[[2]geom.Vec2{c.a, c.b}] {
			report.ContourEdges = append(report.ContourEdges, id)
		} else {
			report.PartitionEdges = append(report.PartitionEdges, id)
		}
	}
	return out, report, nil
}

func validateExactGraphPartitionEdges(ctx context.Context, out Solid, proof geom.GraphPointContactOptions) error {
	r, e := out.ValidateEdgeAgreement(ctx, edgecheck.Options{MaxWork: proof.MaxOperations, MaxBits: proof.MaxCoefficientBits})
	if e != nil {
		return e
	}
	for _, x := range r.Uses {
		if x.Report.Status != edgecheck.Certified || x.Report.SquaredUpperBound == nil || x.Report.SquaredUpperBound.Sign() != 0 {
			return fmt.Errorf("trimmed: partitioned graph face %d edge %d: %s: %w", x.Face, x.Edge, x.Report.Reason, geom.ErrConvergence)
		}
	}
	if !r.Complete {
		return geom.ErrConvergence
	}
	return nil
}

func assemblePartiallyCoincidentGraphDifference(ctx context.Context, a, b *geom.BSplineSurface, ap, bp *geom.SurfacePatch,
	tolerance float64, proof geom.GraphPointContactOptions) (Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	signed, e := geom.FindSplineGraphSignedComponents(ctx, ap, bp, 1, proof)
	if e != nil || !signed.Complete {
		if e == nil {
			e = geom.ErrConvergence
		}
		return Solid{}, h, e
	}
	p := &signed.Partition
	var coincident int
	for _, c := range p.Cells {
		if c.Coincident {
			coincident++
		}
	}
	if coincident == 0 || coincident == len(p.Cells) {
		return Solid{}, h, geom.ErrConvergence
	}
	var regions []openGraphRegion
	for _, c := range signed.Components {
		if len(c.InternalZeroSides) != 0 {
			return Solid{}, h, geom.ErrConvergence
		}
		atZeroSide := make(map[[2]string]bool)
		region := openGraphRegion{sign: 1}
		for _, ring := range c.Cycles {
			loop := make([]openSectionPiece, 0, len(ring.Edges))
			for _, x := range ring.Edges {
				side := -1
				if x.ZeroSide >= 0 {
					atZeroSide[[2]string{x.Start[0].RatString(), x.Start[1].RatString()}] = true
					atZeroSide[[2]string{x.End[0].RatString(), x.End[1].RatString()}] = true
				} else {
					if !x.Exterior || ring.Hole {
						return Solid{}, h, geom.ErrConvergence
					}
					side = graphPartitionDomainSide(x.Edge)
					if side < 0 {
						return Solid{}, h, geom.ErrConvergence
					}
				}
				a, e := exactGraphPartitionUV(x.Start)
				if e != nil {
					return Solid{}, h, e
				}
				b, e := exactGraphPartitionUV(x.End)
				if e != nil {
					return Solid{}, h, e
				}
				loop = append(loop, openSectionPiece{a: a, b: b, side: side})
			}
			if ring.Hole {
				region.holes = append(region.holes, loop)
			} else {
				if len(region.loop) != 0 {
					return Solid{}, h, geom.ErrConvergence
				}
				region.loop = loop
			}
		}
		for _, id := range c.ContactCorners {
			x := p.ZeroCorners[id].Parameters
			if !atZeroSide[[2]string{x[0].RatString(), x[1].RatString()}] {
				return Solid{}, h, geom.ErrConvergence
			}
		}
		if len(region.loop) == 0 {
			return Solid{}, h, geom.ErrConvergence
		}
		regions = append(regions, region)
	}
	out, report, e := combineOpenGraphRegions(ctx, nil, a, b, regions, parametric.Difference, tolerance)
	if e != nil {
		return Solid{}, h, e
	}
	if len(regions) != 0 {
		if e := validateExactGraphPartitionEdges(ctx, out, proof); e != nil {
			return Solid{}, h, e
		}
	}
	report.ContactPartition, report.SignedDifference = p, &signed
	return out, report, nil
}

func partitionGraphRectangleLoop(d geom.ParameterRectangle) []openSectionPiece {
	corners := insetCorners(d)
	boundary := [4]bool{d.VMin == 0, d.UMax == 1, d.VMax == 1, d.UMin == 0}
	loop := make([]openSectionPiece, 4)
	for j, v := range corners {
		side := -1
		if boundary[j] {
			side = j
		}
		loop[j] = openSectionPiece{a: v, b: corners[(j+1)%4], side: side}
	}
	return loop
}

func exactGraphPartitionUV(p [2]*big.Rat) (geom.Vec2, error) {
	u, e := geom.RepresentedExactScalar(p[0])
	if e != nil {
		return geom.Vec2{}, e
	}
	v, e := geom.RepresentedExactScalar(p[1])
	return geom.Vec2{X: u, Y: v}, e
}

func graphPartitionDomainSide(edge geom.SurfaceEdge) int {
	switch edge {
	case geom.SurfaceEdgeVMin:
		return 0
	case geom.SurfaceEdgeUMax:
		return 1
	case geom.SurfaceEdgeVMax:
		return 2
	case geom.SurfaceEdgeUMin:
		return 3
	}
	return -1
}

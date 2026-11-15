package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// SplineGraphBooleanHistory records retained top faces, the shared base,
// and the generated section edges and wall faces. A top face index of -1
// means that input's top was not retained. FacesA/FacesB enumerate all
// retained fragments; TopA/TopB give the first of each, if present.
type SplineGraphBooleanHistory struct {
	TopA, TopB, Base                     int
	FacesA, FacesB                       []int
	ContourEdges, WallFaces, DomainSides []int
	PointContact                         *geom.SplineGraphPointContact
	Ordering                             *geom.SplineGraphOrdering
	SeparableContact                     *geom.SeparableSplineGraphContact
	CoupledQuarticContact                *geom.CoupledQuarticGraphContact
	Saddle                               *geom.QuadraticSaddleSectors
	AxisLine                             *geom.SplineGraphAxisLine
	AxisMultiplicity                     int
	ParallelLines                        *geom.SplineGraphParallelLines
	CrossingLines                        *geom.SplineGraphCrossingLines
	ContactGrid                          *geom.SplineGraphContactGrid
	ObliqueLine                          *geom.SplineGraphObliqueLine
	ParallelOblique                      *geom.SplineGraphParallelObliqueLines
	IntersectingOblique                  *geom.SplineGraphIntersectingObliqueLines
	Ellipse                              *geom.SplineGraphEllipse
	QuadraticEllipse                     *geom.SplineGraphQuadraticEllipse
	RationalEllipseArcs                  *geom.SplineGraphQuadraticEllipseArcs
	DisjointEllipses                     *geom.DiscoveredSplineGraphDisjointEllipses
	ThreeEllipses                        *geom.DiscoveredSplineGraphThreeEllipses
	ContactPartition                     *geom.SplineGraphContactPartition
	CoincidentFaces                      []int
	PartitionFaceByCell                  []int // original knot cell to retained top face (envelopes)
	CoincidentRegions                    *geom.SplineGraphCoincidentComponents
	SignedDifference                     *geom.SplineGraphSignedComponents
	PartitionEdges                       []int // top fragment edges across noncontact grid sides
}

// CombineSplineGraphSolids performs a regularized Boolean on two solids
// between a common base spline and their respective top splines. All three
// use the same identity XY chart. Their spline bases can differ. Both tops
// must be strictly above the base everywhere. The top pair can be strictly
// ordered, identical, or separated by regular transverse sections: open
// and closed sections whose separation and nesting can be proved.
// Difference is A minus B.
func CombineSplineGraphSolids(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp,
	tolerance float64, o intersect.SplinePlaneOptions) (Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	if ctx == nil || base == nil || a == nil || b == nil || op > parametric.Difference ||
		!finite(tolerance) || tolerance <= 0 || o.Domain != (geom.ParameterRectangle{}) {
		return Solid{}, h, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, h, e
	}
	if e := requireIdentitySplineChart(ctx, a, o.Patch); e != nil {
		return Solid{}, h, e
	}
	plane, _ := geom.NewPlane(geom.Vec3{}, geom.Vec3{Z: 1})
	if o.ResidualTolerance == 0 {
		o.ResidualTolerance = tolerance
	}
	for _, s := range []*geom.BSplineSurface{a, b} {
		d, _, _, e := exactGraphPatchesContext(ctx, s, base, o.Patch, o.Domain)
		if e != nil {
			return Solid{}, h, e
		}
		if e = requireGraphAbove(ctx, d, plane, o); e != nil {
			return Solid{}, h, e
		}
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, h, e
	}
	d, ap, bp, e := exactGraphPatchesContext(ctx, a, b, o.Patch, o.Domain)
	if e != nil {
		return Solid{}, h, e
	}
	r, e := d.PlaneBounds(plane)
	if e != nil {
		return Solid{}, h, e
	}
	if r.Relation == geom.PlanePatchCoplanar {
		if op == parametric.Difference {
			return Solid{}, h, nil
		}
		s, w, e := FillSplineGraphPair(a, base, tolerance, o)
		if e != nil {
			return Solid{}, h, e
		}
		return s, SplineGraphBooleanHistory{TopA: w.FaceA, TopB: w.FaceA, Base: w.FaceB, FacesA: []int{w.FaceA}, FacesB: []int{w.FaceA},
			WallFaces: w.WallFaces, DomainSides: w.DomainSides}, nil
	}
	loops, trace, e := intersect.TraceSplineHeightDifferenceContext(ctx, a, b, o)
	if e != nil {
		if errors.Is(e, geom.ErrConvergence) || errors.Is(e, intersect.ErrUnresolved) {
			proof := geom.GraphPointContactOptions{MaxOperations: o.Patch.MaxOperations, MaxCoefficientBits: o.Patch.MaxCoefficientBits}
			q, partitionHistory, partialErr := assemblePartitionedSplineEnvelope(ctx, base, a, b, ap, bp, op, tolerance, proof)
			if partialErr == nil {
				return q, partitionHistory, nil
			}
			for axis := range 2 {
				q, axisHistory, e := assembleAxisLineGraphBoolean(ctx, base, a, b, ap, bp, op, axis, tolerance, o, proof)
				if e == nil {
					return q, axisHistory, nil
				}
				if axisHistory.AxisLine != nil {
					h.AxisLine, h.AxisMultiplicity = axisHistory.AxisLine, axisHistory.AxisMultiplicity
					if errors.Is(e, geom.ErrDegenerate) {
						return Solid{}, h, e
					}
				}
				q, parallel, e := assembleParallelLineGraphBoolean(ctx, base, a, b, ap, bp, op, axis, tolerance,
					geom.GraphFactorAxisLineOptions{GraphPointContactOptions: proof})
				if e == nil {
					return q, parallel, nil
				}
				if parallel.ParallelLines != nil && parallel.ParallelLines.Proven && len(parallel.ParallelLines.Lines) >= 2 {
					h.ParallelLines = parallel.ParallelLines
				}
			}
			if op != parametric.Difference {
				// A discovered saddle is a candidate. The sector constructor
				// proves the entire rectangle before any result is returned.
				candidate, saddleErr := geom.DiscoverQuadraticGraphSaddle(ctx, ap, bp, geom.GraphPointContactDiscoveryOptions{GraphPointContactOptions: proof})
				if saddleErr == nil && candidate.Certificate.Proven {
					s, proof, e := assembleQuadraticSaddleGraph(ctx, base, a, b, ap, bp, candidate.Certificate.Critical.Parameters, op, tolerance, proof)
					if e == nil {
						return s, proof, nil
					}
				}
			}
			q, crossing, crossingErr := assembleCrossingLineGraphBoolean(ctx, base, a, b, ap, bp, op, tolerance,
				geom.GraphCrossingLinesOptions{GraphPointContactOptions: proof})
			if crossingErr == nil {
				return q, crossing, nil
			}
			if crossing.CrossingLines != nil {
				h.CrossingLines = crossing.CrossingLines
				if errors.Is(crossingErr, geom.ErrDegenerate) {
					return Solid{}, h, crossingErr
				}
			}
			q, grid, gridErr := assembleContactGridGraphBoolean(ctx, base, a, b, ap, bp, op, tolerance,
				geom.GraphCrossingLinesOptions{GraphPointContactOptions: proof})
			if gridErr == nil {
				return q, grid, nil
			}
			if grid.ContactGrid != nil {
				h.ContactGrid = grid.ContactGrid
				if errors.Is(gridErr, geom.ErrDegenerate) {
					return Solid{}, h, gridErr
				}
			}
			q, oblique, obliqueErr := assembleObliqueLineGraphBoolean(ctx, base, a, b, ap, bp, op, tolerance,
				geom.GraphFactorAxisLineOptions{GraphPointContactOptions: proof})
			if obliqueErr == nil {
				return q, oblique, nil
			}
			if oblique.ObliqueLine != nil {
				h.ObliqueLine = oblique.ObliqueLine
			}
			q, parallelOblique, parallelObliqueErr := assembleParallelObliqueLineGraphBoolean(ctx, base, a, b, ap, bp, op, tolerance,
				geom.GraphFactorAxisLineOptions{GraphPointContactOptions: proof})
			if parallelObliqueErr == nil {
				return q, parallelOblique, nil
			}
			if parallelOblique.ParallelOblique != nil {
				h.ParallelOblique = parallelOblique.ParallelOblique
			}
			q, obliqueCrossing, obliqueCrossingErr := assembleIntersectingObliqueLineGraphBoolean(ctx, base, a, b, ap, bp, op, tolerance,
				geom.GraphFactorAxisLineOptions{GraphPointContactOptions: proof})
			if obliqueCrossingErr == nil {
				return q, obliqueCrossing, nil
			}
			if obliqueCrossing.IntersectingOblique != nil {
				h.IntersectingOblique = obliqueCrossing.IntersectingOblique
				if errors.Is(obliqueCrossingErr, geom.ErrDegenerate) {
					return Solid{}, h, obliqueCrossingErr
				}
			}
			ellipse, ellipseErr := geom.DiscoverSplineGraphEllipse(ctx, ap, bp, proof)
			if ellipseErr == nil && ellipse.Proven {
				q, elliptical, e := assembleEllipticalContactGraphBoolean(ctx, base, a, b, op, tolerance, ellipse, proof)
				if e == nil {
					return q, elliptical, nil
				}
				h.Ellipse = elliptical.Ellipse
				if errors.Is(e, geom.ErrUnrepresentable) {
					var arcs geom.SplineGraphQuadraticEllipseArcs
					var arcErr error
					if ellipse.Ellipse.RadiusSquared[0].Cmp(ellipse.Ellipse.RadiusSquared[1]) == 0 {
						arcs, arcErr = geom.DiscoverSplineGraphRationalCircleArcs(ctx, ap, bp, geom.RationalCircleArcDiscoveryOptions{GraphPointContactOptions: proof})
					} else {
						arcs, arcErr = geom.DiscoverSplineGraphRationalEllipseArcs(ctx, ap, bp, geom.RationalEllipseArcDiscoveryOptions{GraphPointContactOptions: proof})
					}
					if arcErr == nil {
						q, rational, rationalErr := assembleRationalEllipseGraphBoolean(ctx, base, a, b, op, tolerance, arcs, proof)
						if rationalErr == nil {
							return q, rational, nil
						}
					}
				}
			}
			if !ellipse.Proven {
				rotated, rotatedErr := geom.DiscoverSplineGraphQuadraticEllipse(ctx, ap, bp, proof)
				if rotatedErr == nil && rotated.Proven {
					q, elliptical, e := assembleQuadraticEllipseGraphBoolean(ctx, base, a, b, op, tolerance, rotated, proof)
					if e == nil {
						return q, elliptical, nil
					}
					h.QuadraticEllipse = elliptical.QuadraticEllipse
					if errors.Is(e, geom.ErrUnrepresentable) {
						arcs, arcErr := geom.DiscoverSplineGraphRationalEllipseArcs(ctx, ap, bp, geom.RationalEllipseArcDiscoveryOptions{GraphPointContactOptions: proof})
						if arcErr == nil {
							q, rational, rationalErr := assembleRationalEllipseGraphBoolean(ctx, base, a, b, op, tolerance, arcs, proof)
							if rationalErr == nil {
								return q, rational, nil
							}
						}
					}
				}
			}
			p, discoveryErr := geom.DiscoverSplineGraphPointContact(ctx, ap, bp, geom.GraphPointContactDiscoveryOptions{
				GraphPointContactOptions: geom.GraphPointContactOptions{MaxOperations: o.Patch.MaxOperations,
					MaxCoefficientBits: o.Patch.MaxCoefficientBits},
			})
			if discoveryErr == nil && p.Contact.Proven {
				h.PointContact = &p.Contact
				return combineProvenTangentGraphSolids(base, a, b, p.Contact.Side, op, tolerance, o, h)
			}
			separable, separableErr := geom.DiscoverSeparableSplineGraphPointContact(ctx, ap, bp, geom.SeparableGraphDiscoveryOptions{
				SeparableGraphContactOptions: geom.SeparableGraphContactOptions{
					GraphPointContactOptions: geom.GraphPointContactOptions{MaxOperations: o.Patch.MaxOperations,
						MaxCoefficientBits: o.Patch.MaxCoefficientBits},
				},
			})
			if separableErr == nil && separable.Contact.Proven {
				h.SeparableContact = &separable.Contact
				return combineProvenTangentGraphSolids(base, a, b, separable.Contact.Side, op, tolerance, o, h)
			}
			quartic, quarticErr := geom.DiscoverCoupledQuarticGraphPointContact(ctx, ap, bp, proof)
			if quarticErr == nil && quartic.Proven {
				h.CoupledQuarticContact = &quartic
				return combineProvenTangentGraphSolids(base, a, b, quartic.Side, op, tolerance, o, h)
			}
			order, orderErr := geom.DiscoverSplineGraphOrdering(ctx, ap, bp, geom.GraphPointContactDiscoveryOptions{
				GraphPointContactOptions: geom.GraphPointContactOptions{MaxOperations: o.Patch.MaxOperations,
					MaxCoefficientBits: o.Patch.MaxCoefficientBits},
			})
			if orderErr == nil && order.Ordering.Proven {
				h.Ordering = &order.Ordering
				if op == parametric.Difference {
					if order.Ordering.Side < 0 {
						return Solid{}, h, nil
					}
					return fillProvenOrderedSplineGraphs(a, b, tolerance, h)
				}
				return combineOrderedSplineGraphSolids(base, a, b, order.Ordering.Side, op, tolerance, o, h)
			}
			if err := ctx.Err(); err != nil {
				return Solid{}, h, err
			}
		}
		return Solid{}, h, e
	}
	if len(trace.Isolation.Tangencies) > 0 && len(loops) != 0 {
		return Solid{}, h, fmt.Errorf("trimmed: singular graph Boolean section")
	}
	if len(loops) == 0 {
		sign, e := geom.ClassifySurfacePointPlane(d, plane, .5, .5, geom.SurfacePointSignOptions{})
		if e == nil && sign.Sign == 0 && len(trace.Isolation.Tangencies) != 0 {
			// With a complete trace and only isolated points removed, the
			// rectangle remains connected. Its nonzero sign is constant.
			sign.Sign = trace.Isolation.Tangencies[0].Side
		}
		if e != nil || sign.Sign == 0 {
			if e == nil {
				e = geom.ErrConvergence
			}
			return Solid{}, h, e
		}
		if op == parametric.Difference {
			if sign.Sign < 0 {
				return Solid{}, h, nil
			}
			if len(trace.Isolation.Tangencies) != 0 {
				return Solid{}, h, fmt.Errorf("trimmed: graph difference pinches at an isolated tangent point")
			}
			s, w, e := FillSplineGraphPair(a, b, tolerance, o)
			if e != nil {
				return Solid{}, h, e
			}
			return s, SplineGraphBooleanHistory{TopA: w.FaceA, TopB: w.FaceB, Base: -1, FacesA: []int{w.FaceA}, FacesB: []int{w.FaceB},
				WallFaces: w.WallFaces, DomainSides: w.DomainSides}, nil
		}
		return combineOrderedSplineGraphSolids(base, a, b, sign.Sign, op, tolerance, o, h)
	}
	boundarySign := 0
	if allClosedContours(loops) {
		proof, e := geom.ClassifyPlaneBoundarySigns(d, plane, geom.PlaneBoundarySignOptions{})
		if e != nil {
			return Solid{}, h, e
		}
		if !proof.Complete || proof.StrictSign == 0 {
			return Solid{}, h, geom.ErrConvergence
		}
		boundarySign = proof.StrictSign
	}
	regions, e := openGraphRegions(ctx, loops, boundarySign)
	if e != nil {
		return Solid{}, h, e
	}
	return combineOpenGraphRegions(ctx, base, a, b, regions, op, tolerance)
}

// CombineQuadraticSaddleSplineGraphs computes union or intersection for
// strictly positive shared-base graphs whose exact top difference has one
// certified quadratic saddle over the full unit chart. The four sector
// faces share one topological saddle vertex and four exact model seams.
// The saddle and terminals must have represented exact UV/model coordinates.
// Difference has a pinch between its two positive sectors and is rejected.
func CombineQuadraticSaddleSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, uv [2]*big.Rat,
	op parametric.BooleanOp, tolerance float64, o intersect.SplinePlaneOptions, proof geom.GraphPointContactOptions) (Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	if ctx == nil || base == nil || a == nil || b == nil || op > parametric.Difference ||
		!finite(tolerance) || tolerance <= 0 || o.Domain != (geom.ParameterRectangle{}) {
		return Solid{}, h, geom.ErrInvalid
	}
	if op == parametric.Difference {
		return Solid{}, h, geom.ErrConvergence
	}
	if e := requireIdentitySplineChart(ctx, a, o.Patch); e != nil {
		return Solid{}, h, e
	}
	pl, _ := geom.NewPlane(geom.Vec3{}, geom.Vec3{Z: 1})
	var p [2]*geom.SurfacePatch
	for i, s := range [2]*geom.BSplineSurface{a, b} {
		d, top, _, e := exactGraphPatchesContext(ctx, s, base, o.Patch, o.Domain)
		if e != nil {
			return Solid{}, h, e
		}
		if e = requireGraphAbove(ctx, d, pl, o); e != nil {
			return Solid{}, h, e
		}
		p[i] = top
	}
	return assembleQuadraticSaddleGraph(ctx, base, a, b, p[0], p[1], uv, op, tolerance, proof)
}

func assembleQuadraticSaddleGraph(ctx context.Context, base, a, b *geom.BSplineSurface, ap, bp *geom.SurfacePatch,
	uv [2]*big.Rat, op parametric.BooleanOp, tolerance float64, proof geom.GraphPointContactOptions) (Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	s, e := geom.CertifyQuadraticGraphSaddleSectors(ctx, ap, bp, uv,
		[2][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(1, 1)}, {big.NewRat(0, 1), big.NewRat(1, 1)}}, proof)
	if e != nil {
		return Solid{}, h, e
	}
	center, e := saddleRepresentedUV(uv)
	if e != nil {
		return Solid{}, h, e
	}
	var terminals [4]geom.Vec2
	for i, end := range s.Section.Terminals {
		if !end.Exact {
			return Solid{}, h, geom.ErrUnrepresentable
		}
		terminals[i], e = saddleRepresentedUV(end.Lower)
		if e != nil {
			return Solid{}, h, e
		}
		if _, e = geom.RepresentedExactPoint(end.Point); e != nil {
			return Solid{}, h, e
		}
	}
	p, e := ap.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{
		MaxOperations: proof.MaxOperations, MaxCoefficientBits: proof.MaxCoefficientBits,
	})
	if e != nil {
		return Solid{}, h, e
	}
	if _, e = geom.RepresentedExactPoint(p.Point); e != nil {
		return Solid{}, h, e
	}
	regions := make([]graphEnvelopeRegion, 4)
	for i, sector := range s.Sectors {
		t := sector.PositiveBoundary
		if t.ParameterDirection != 1 || t.StartRay == t.EndRay {
			return Solid{}, h, geom.ErrConvergence
		}
		start, end := terminals[t.StartRay], terminals[t.EndRay]
		arc, e := saddleSquareArc(start, end)
		if e != nil {
			return Solid{}, h, e
		}
		loop := append([]openSectionPiece{{a: center, b: start, side: -1}}, arc...)
		loop = append(loop, openSectionPiece{a: end, b: center, side: -1})
		regions[i] = graphEnvelopeRegion{loop: loop, first: (sector.HeightSide > 0) == (op == parametric.Union)}
	}
	out, h, e := assembleSplineEnvelope(ctx, base, a, b, regions, tolerance)
	if e != nil {
		return Solid{}, SplineGraphBooleanHistory{}, e
	}
	r, e := out.ValidateEdgeAgreement(ctx, edgecheck.Options{MaxWork: proof.MaxOperations, MaxBits: proof.MaxCoefficientBits})
	if e != nil {
		return Solid{}, SplineGraphBooleanHistory{}, e
	}
	for _, u := range r.Uses {
		if u.Report.Status != edgecheck.Certified || u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			return Solid{}, SplineGraphBooleanHistory{}, fmt.Errorf("trimmed: saddle envelope face %d edge %d: %s: %w", u.Face, u.Edge, u.Report.Reason, geom.ErrConvergence)
		}
	}
	if !r.Complete {
		return Solid{}, SplineGraphBooleanHistory{}, geom.ErrConvergence
	}
	h.Saddle = &s
	return out, h, nil
}

func saddleRepresentedUV(p [2]*big.Rat) (geom.Vec2, error) {
	x, e := geom.RepresentedExactScalar(p[0])
	if e != nil {
		return geom.Vec2{}, e
	}
	y, e := geom.RepresentedExactScalar(p[1])
	return geom.Vec2{X: x, Y: y}, e
}

func saddleSquareArc(start, end geom.Vec2) ([]openSectionPiece, error) {
	side := func(p geom.Vec2) int {
		switch {
		case p.Y == 0 && p.X < 1:
			return 0
		case p.X == 1 && p.Y < 1:
			return 1
		case p.Y == 1 && p.X > 0:
			return 2
		case p.X == 0 && p.Y > 0:
			return 3
		default:
			return -1
		}
	}
	before := func(p, q geom.Vec2, axis int) bool {
		switch axis {
		case 0:
			return p.X < q.X && q.Y == 0
		case 1:
			return p.Y < q.Y && q.X == 1
		case 2:
			return p.X > q.X && q.Y == 1
		default:
			return p.Y > q.Y && q.X == 0
		}
	}
	if start == end || side(start) < 0 || side(end) < 0 {
		return nil, geom.ErrConvergence
	}
	var out []openSectionPiece
	for p, n := start, 0; n < 5; n++ {
		j := side(p)
		q := unitSquare[(j+1)%4]
		if before(p, end, j) {
			q = end
		}
		out = append(out, openSectionPiece{a: p, b: q, side: j})
		if q == end {
			return out, nil
		}
		p = q
	}
	return nil, geom.ErrConvergence
}

// CombineTangentSplineGraphSolids combines shared-base graph solids with a
// single proved isolated top contact. The rational UV is only a proposal:
// exact equality, stationarity, graph injectivity and global strict sign
// away from the contact are independently certified. Union and intersection
// keep the appropriate original top. Subtraction is empty if A is lower;
// a nonempty subtraction pinches at the contact and is rejected.
func CombineTangentSplineGraphSolids(ctx context.Context, base, a, b *geom.BSplineSurface, uv [2]*big.Rat,
	op parametric.BooleanOp, tolerance float64, o intersect.SplinePlaneOptions, proof geom.GraphPointContactOptions) (Solid, SplineGraphBooleanHistory, error) {
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	if ctx == nil || base == nil || a == nil || b == nil || op > parametric.Difference ||
		!finite(tolerance) || tolerance <= 0 || o.Domain != (geom.ParameterRectangle{}) {
		return Solid{}, h, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, h, e
	}
	if e := requireIdentitySplineChart(ctx, a, o.Patch); e != nil {
		return Solid{}, h, e
	}
	plane, _ := geom.NewPlane(geom.Vec3{}, geom.Vec3{Z: 1})
	var tops [2]*geom.SurfacePatch
	for i, s := range []*geom.BSplineSurface{a, b} {
		d, top, _, e := exactGraphPatchesContext(ctx, s, base, o.Patch, o.Domain)
		if e == nil {
			e = requireGraphAbove(ctx, d, plane, o)
		}
		if e != nil {
			return Solid{}, h, e
		}
		tops[i] = top
	}
	p, e := geom.CertifySplineGraphPointContact(ctx, tops[0], tops[1], uv, proof)
	if errors.Is(e, geom.ErrConvergence) {
		q, qe := geom.CertifySeparableSplineGraphPointContact(ctx, tops[0], tops[1], uv,
			geom.SeparableGraphContactOptions{GraphPointContactOptions: proof})
		if qe == nil && q.Proven {
			h.SeparableContact = &q
			return combineProvenTangentGraphSolids(base, a, b, q.Side, op, tolerance, o, h)
		}
		quartic, qe := geom.CertifyCoupledQuarticGraphPointContact(ctx, tops[0], tops[1], uv, proof)
		if qe == nil && quartic.Proven {
			h.CoupledQuarticContact = &quartic
			return combineProvenTangentGraphSolids(base, a, b, quartic.Side, op, tolerance, o, h)
		}
		if ctx.Err() != nil {
			return Solid{}, h, ctx.Err()
		}
	}
	h.PointContact = &p
	if e != nil || !p.Proven {
		if e == nil {
			e = geom.ErrConvergence
		}
		return Solid{}, h, e
	}
	return combineProvenTangentGraphSolids(base, a, b, p.Side, op, tolerance, o, h)
}

func combineProvenTangentGraphSolids(base, a, b *geom.BSplineSurface, side int, op parametric.BooleanOp, tolerance float64,
	o intersect.SplinePlaneOptions, h SplineGraphBooleanHistory) (Solid, SplineGraphBooleanHistory, error) {
	if op == parametric.Difference {
		if side < 0 {
			return Solid{}, h, nil
		}
		return Solid{}, h, fmt.Errorf("trimmed: graph difference pinches at its certified point contact: %w", geom.ErrDegenerate)
	}
	return combineOrderedSplineGraphSolids(base, a, b, side, op, tolerance, o, h)
}

func combineOrderedSplineGraphSolids(base, a, b *geom.BSplineSurface, side int, op parametric.BooleanOp, tolerance float64,
	o intersect.SplinePlaneOptions, h SplineGraphBooleanHistory) (Solid, SplineGraphBooleanHistory, error) {
	x, y := a, true
	if (side > 0) != (op == parametric.Union) {
		x, y = b, false
	}
	s, w, e := FillSplineGraphPair(x, base, tolerance, o)
	if e != nil {
		return Solid{}, h, e
	}
	if y {
		h.TopA = w.FaceA
		h.FacesA = []int{w.FaceA}
	} else {
		h.TopB = w.FaceA
		h.FacesB = []int{w.FaceA}
	}
	h.Base, h.WallFaces, h.DomainSides = w.FaceB, w.WallFaces, w.DomainSides
	return s, h, nil
}

// Both tops already have the same identity XY chart. A strict global
// ordering proof can replace the isolation normally used to fill the gap.
func fillProvenOrderedSplineGraphs(a, b *geom.BSplineSurface, tolerance float64, h SplineGraphBooleanHistory) (Solid, SplineGraphBooleanHistory, error) {
	pieces := make([]openSectionPiece, len(unitSquare))
	for i := range pieces {
		pieces[i] = openSectionPiece{a: unitSquare[i], b: unitSquare[(i+1)%len(unitSquare)], side: i}
	}
	s, p, e := assembleOpenSplineGraphPair(a, b, 0, true, tolerance, pieces)
	if e != nil {
		return Solid{}, h, e
	}
	h.TopA, h.TopB = p.SplineFace, p.PlaneFace
	h.FacesA, h.FacesB = []int{h.TopA}, []int{h.TopB}
	h.WallFaces, h.DomainSides = p.WallFaces, p.DomainSides
	return s, h, nil
}

func allClosedContours(loops []intersect.CertifiedContour) bool {
	for _, c := range loops {
		if !c.Closed {
			return false
		}
	}
	return true
}

func requireGraphAbove(ctx context.Context, d *geom.SurfacePatch, pl geom.Plane, o intersect.SplinePlaneOptions) error {
	r, e := d.PlaneBounds(pl)
	if e != nil || r.Relation == geom.PlanePatchAbove {
		return e
	}
	if r.Relation != geom.PlanePatchUnresolved {
		return fmt.Errorf("trimmed: graph top is not strictly above the common base")
	}
	trace, e := geom.TraceSurfacePlaneContext(ctx, d, pl, o.Trace)
	if e != nil {
		return e
	}
	if !trace.Complete || len(trace.Contours) != 0 || len(trace.Isolation.Tangencies) != 0 || len(trace.Isolation.Coplanar) != 0 {
		return fmt.Errorf("trimmed: graph top is not strictly above the common base")
	}
	q, e := geom.ClassifySurfacePointPlane(d, pl, .5, .5, geom.SurfacePointSignOptions{})
	if e != nil {
		return e
	}
	if q.Sign != 1 {
		return fmt.Errorf("trimmed: graph top is not strictly above the common base")
	}
	return nil
}

type graphEnvelopeRegion struct {
	loop  []openSectionPiece
	holes [][]openSectionPiece
	first bool
}

func combineOpenGraphRegions(ctx context.Context, base, a, b *geom.BSplineSurface, regions []openGraphRegion, op parametric.BooleanOp, tolerance float64) (Solid, SplineGraphBooleanHistory, error) {
	if op != parametric.Difference {
		r := make([]graphEnvelopeRegion, len(regions))
		for i, x := range regions {
			r[i] = graphEnvelopeRegion{x.loop, x.holes, (x.sign > 0) == (op == parametric.Union)}
		}
		return assembleSplineEnvelope(ctx, base, a, b, r, tolerance)
	}
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}
	var parts []PlacedSolid
	faceOffset, edgeOffset := 0, 0
	for _, r := range regions {
		if r.sign <= 0 {
			continue
		}
		s, local, e := assembleGraphRegionDifference(a, b, r, tolerance)
		if e != nil {
			return Solid{}, h, e
		}
		for _, id := range local.ContourEdges {
			h.ContourEdges = append(h.ContourEdges, id+edgeOffset)
		}
		for _, id := range local.WallFaces {
			h.WallFaces = append(h.WallFaces, id+faceOffset)
		}
		h.DomainSides = append(h.DomainSides, local.DomainSides...)
		h.FacesA = append(h.FacesA, local.SplineFace+faceOffset)
		h.FacesB = append(h.FacesB, local.PlaneFace+faceOffset)
		faceOffset += len(s.faces)
		edgeOffset += len(s.edges)
		parts = append(parts, PlacedSolid{Solid: s, Transform: geom.Identity()})
	}
	if len(parts) == 0 {
		return Solid{}, h, nil
	}
	if len(h.FacesA) > 0 {
		h.TopA = h.FacesA[0]
	}
	if len(h.FacesB) > 0 {
		h.TopB = h.FacesB[0]
	}
	s, e := AssembleShells(ctx, parts, 32)
	return s, h, e
}

func assembleSplineEnvelope(ctx context.Context, base, a, b *geom.BSplineSurface,
	regions []graphEnvelopeRegion, tolerance float64) (Solid, SplineGraphBooleanHistory, error) {
	bad := func(reason string) (Solid, SplineGraphBooleanHistory, error) {
		return Solid{}, SplineGraphBooleanHistory{}, fmt.Errorf("trimmed: spline envelope: %s", reason)
	}
	type arc struct {
		openSectionPiece
		surface *geom.BSplineSurface
	}
	var perimeter []arc
	for _, region := range regions {
		for _, x := range region.loop {
			if x.side < 0 {
				continue
			}
			s := a
			if !region.first {
				s = b
			}
			perimeter = append(perimeter, arc{x, s})
		}
	}
	if len(perimeter) < 4 || len(perimeter) > 36 {
		return bad("incomplete perimeter partition")
	}
	// The two portions of the original square perimeter are independently
	// counterclockwise. Assemble their pieces by exact endpoint identity.
	index := make(map[geom.Vec2]int, len(perimeter))
	for i, x := range perimeter {
		if _, exists := index[x.a]; exists {
			return bad("ambiguous perimeter")
		}
		index[x.a] = i
	}
	var outer []arc
	p := unitSquare[0]
	for range len(perimeter) {
		i, ok := index[p]
		if !ok {
			return bad("disconnected perimeter")
		}
		delete(index, p)
		outer = append(outer, perimeter[i])
		p = perimeter[i].b
	}
	if p != unitSquare[0] {
		return bad("unclosed perimeter")
	}
	h := SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: len(regions)}
	faces := make([]Face, len(regions)+1)
	faces[h.Base] = Face{Surface: base, Reversed: true}
	var vertices []geom.Vec3
	var edges []parametric.Edge
	top, low := map[geom.Vec2]int{}, map[geom.Vec2]int{}
	vertex := func(ids map[geom.Vec2]int, x openSectionPiece, surface *geom.BSplineSurface) (int, error) {
		uv := x.a
		p, e := graphTrimStartDisplay(ctx, surface, x)
		if e != nil {
			return 0, e
		}
		if i, ok := ids[uv]; ok {
			if !closePoint(vertices[i], p, tolerance) {
				return 0, fmt.Errorf("trimmed: graph junction exceeds tolerance")
			}
			return i, nil
		}
		i := len(vertices)
		vertices = append(vertices, p)
		ids[uv] = i
		return i, nil
	}
	edge := func(i, j int, c geom.Curve, length float64) int {
		id := len(edges)
		edges = append(edges, parametric.Edge{Start: i, End: j, Curve: c, Range: parametric.Interval{Max: length}})
		return id
	}
	model := func(ids map[geom.Vec2]int, surface *geom.BSplineSurface, x openSectionPiece) (int, error) {
		i, e := vertex(ids, x, surface)
		if e != nil {
			return 0, e
		}
		j, e := vertex(ids, openSectionPiece{a: x.b, b: x.a, c: reversePCurve(x.c)}, surface)
		if e != nil {
			return 0, e
		}
		return edge(i, j, surfaceTrimCurve{surface, x.a, x.b, x.c}, 1), nil
	}
	section := make(map[[2]geom.Vec2]int)
	for i, region := range regions {
		s := a
		if region.first {
			h.FacesA = append(h.FacesA, i)
		} else {
			s = b
			h.FacesB = append(h.FacesB, i)
		}
		faces[i].Surface = s
		for _, ring := range append([][]openSectionPiece{region.loop}, region.holes...) {
			for _, x := range ring {
				if x.side >= 0 {
					continue
				}
				if e := ctx.Err(); e != nil {
					return Solid{}, h, e
				}
				id, ok := section[[2]geom.Vec2{x.b, x.a}]
				if ok {
					if _, e := vertex(top, x, s); e != nil {
						return Solid{}, h, e
					}
				} else {
					var e error
					id, e = model(top, s, x)
					if e != nil {
						return Solid{}, h, e
					}
					section[[2]geom.Vec2{x.a, x.b}] = id
					h.ContourEdges = append(h.ContourEdges, id)
				}
			}
		}
	}
	if len(h.FacesA) > 0 {
		h.TopA = h.FacesA[0]
	}
	if len(h.FacesB) > 0 {
		h.TopB = h.FacesB[0]
	}
	topEdges, lowEdges, uprights := map[geom.Vec2]int{}, map[geom.Vec2]int{}, map[geom.Vec2]int{}
	for _, x := range outer {
		id, e := model(top, x.surface, x.openSectionPiece)
		if e != nil {
			return Solid{}, h, e
		}
		topEdges[x.a] = id
		id, e = model(low, base, x.openSectionPiece)
		if e != nil {
			return Solid{}, h, e
		}
		lowEdges[x.a] = id
		faces[h.Base].Outer = append(faces[h.Base].Outer, Use{Edge: id, UVStart: x.a, UVEnd: x.b})
		tp, bp := vertices[top[x.a]], vertices[low[x.a]]
		if tp.Z <= bp.Z {
			return bad("wall collapsed at vertex")
		}
		d := tp.Sub(bp)
		c, e := geom.NewLine(bp, d)
		if e != nil {
			return Solid{}, h, e
		}
		uprights[x.a] = edge(low[x.a], top[x.a], c, d.Length())
	}
	for i, r := range regions {
		for j, ring := range append([][]openSectionPiece{r.loop}, r.holes...) {
			var uses []Use
			for _, x := range ring {
				id, reversed := topEdges[x.a], false
				if x.side < 0 {
					var ok bool
					id, ok = section[[2]geom.Vec2{x.a, x.b}]
					if !ok {
						id, reversed = section[[2]geom.Vec2{x.b, x.a}], true
					}
				}
				uses = append(uses, Use{Edge: id, Reversed: reversed, UVStart: x.a, UVEnd: x.b, Curve: x.c})
			}
			if j == 0 {
				faces[i].Outer = uses
			} else {
				faces[i].Holes = append(faces[i].Holes, uses)
			}
		}
	}
	for _, x := range outer {
		dir := geom.Vec3{X: x.b.X - x.a.X, Y: x.b.Y - x.a.Y}
		length := dir.Length()
		startHeight := vertices[low[x.a]].Z
		frame, e := geom.NewFrame(geom.Vec3{X: x.a.X, Y: x.a.Y, Z: startHeight}, dir, dir.Cross(geom.Vec3{Z: 1}))
		if e != nil {
			return Solid{}, h, e
		}
		wall, e := geom.NewPlaneSurface(frame)
		if e != nil {
			return Solid{}, h, e
		}
		lo := geom.Vec2{}
		hi := geom.Vec2{X: length, Y: vertices[low[x.b]].Z - startHeight}
		tr := geom.Vec2{X: length, Y: vertices[top[x.b]].Z - startHeight}
		tl := geom.Vec2{Y: vertices[top[x.a]].Z - startHeight}
		lc := wallTrim{base, x.a, x.b, length, startHeight, lo.Y, hi.Y}
		tc := wallTrim{x.surface, x.a, x.b, length, startHeight, tl.Y, tr.Y}
		faces = append(faces, Face{Surface: wall, Outer: []Use{
			{Edge: lowEdges[x.a], UVStart: lo, UVEnd: hi, Curve: lc},
			{Edge: uprights[x.b], UVStart: hi, UVEnd: tr},
			{Edge: topEdges[x.a], Reversed: true, UVStart: tr, UVEnd: tl, Curve: reverseTrim{tc}},
			{Edge: uprights[x.a], Reversed: true, UVStart: tl, UVEnd: lo},
		}})
		h.WallFaces = append(h.WallFaces, len(faces)-1)
		h.DomainSides = append(h.DomainSides, x.side)
	}
	s, e := NewSolidContext(ctx, vertices, edges, faces, tolerance)
	if e != nil {
		return Solid{}, SplineGraphBooleanHistory{}, e
	}
	return s, h, nil
}

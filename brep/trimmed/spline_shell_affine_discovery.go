package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/geom"
	"plainsolid/intersect"
)

func discoverAffineSplineOpenSections(ctx context.Context, a, b Solid, ap, bp []splineEmbeddingFace,
	pairs [][2]int, o SplineShellOpenOptions) ([]SplineShellOpenSection, error) {
	if len(pairs) == 0 || len(pairs) > 64 {
		return nil, geom.ErrConvergence
	}
	var out []SplineShellOpenSection
	charts := map[[2]int]*geom.AffineSurfaceChart{}
	for _, pair := range pairs {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		patches := [2]*geom.SurfacePatch{ap[pair[0]].patch, bp[pair[1]].patch}
		faces := [2]Face{a.faces[pair[0]], b.faces[pair[1]]}
		var ps [2]intersect.Patch
		var cs [2]*geom.AffineSurfaceChart
		for k, p := range patches {
			var e error
			ps[k], e = intersectSplineFace(faces[k], p.Domain())
			if e != nil {
				return nil, geom.ErrConvergence
			}
			key := [2]int{k, pair[k]}
			c := charts[key]
			if c == nil {
				chartPatch := p
				if o.verticalQuadraticWarp != 0 {
					chartPatch, e = p.RemoveVerticalQuadratic(ctx, o.verticalQuadraticWarp, o.Trace.Seeds.Patches)
					if e != nil {
						return nil, e
					}
				}
				c, e = geom.NewAffineSurfaceChart(ctx, chartPatch, geom.AffineSurfaceChartOptions{
					MaxOperations: o.Trace.Coincidence.MaxOperations, MaxCoefficientBits: o.Trace.Coincidence.MaxCoefficientBits,
				})
				if e != nil {
					if errors.Is(e, geom.ErrDegenerate) {
						e = geom.ErrConvergence
					}
					return nil, e
				}
				charts[key] = c
			}
			cs[k] = c
		}
		section, e := proposeAffineSplineSection(ctx, pair, ps, patches, cs, o)
		if e != nil {
			return nil, e
		}
		out = append(out, section)
	}
	return out, nil
}

func proposeAffineSplineSection(ctx context.Context, pair [2]int, ps [2]intersect.Patch, patches [2]*geom.SurfacePatch,
	charts [2]*geom.AffineSurfaceChart, o SplineShellOpenOptions) (SplineShellOpenSection, error) {
	var out SplineShellOpenSection
	r, e := geom.IntersectAffineSurfaceCharts(ctx, charts[0], charts[1], geom.AffineChartIntersectionOptions{
		MaxOperations: o.Endpoints.MaxOperations, MaxCoefficientBits: o.Endpoints.MaxCoefficientBits,
	})
	if e != nil {
		return out, e
	}
	if !r.Complete || r.Kind != geom.AffineChartsSegment {
		return out, geom.ErrConvergence
	}
	return proposeSplineSectionFromExactEnds(ctx, pair, ps, patches, r.Endpoints, o)
}

func proposeSplineSectionFromExactEnds(ctx context.Context, pair [2]int, ps [2]intersect.Patch, patches [2]*geom.SurfacePatch,
	points [2]geom.AffineChartIntersectionEndpoint, o SplineShellOpenOptions) (SplineShellOpenSection, error) {
	var out SplineShellOpenSection
	ends := [2]SplineShellOpenEndpoint{}
	coords := [2][4]float64{}
	sideFor := func(mask uint8) (geom.SurfaceEdge, bool) {
		if mask == 0 || mask&(mask-1) != 0 {
			return 0, false
		}
		side := geom.SurfaceEdge(0)
		for mask != 1 {
			side++
			mask >>= 1
		}
		return side, true
	}
	for i, v := range points {
		mask, operand := v.BoundaryA, 0
		if mask == 0 {
			mask, operand = v.BoundaryB, 1
		}
		side, ok := sideFor(uint8(mask))
		if !ok {
			return out, geom.ErrConvergence
		}
		hint := [4]*big.Rat{v.A[0], v.A[1], v.B[0], v.B[1]}
		for j, x := range hint {
			f, e := geom.RepresentedExactScalar(x)
			if e != nil {
				return out, geom.ErrUnrepresentable
			}
			coords[i][j] = f
		}
		ends[i] = SplineShellOpenEndpoint{Operand: operand, Edge: SplineTubeEndpoint{Side: side, Hint: hint}}
		if operand == 0 && v.BoundaryB != 0 {
			b, ok := sideFor(uint8(v.BoundaryB))
			if !ok {
				return out, geom.ErrConvergence
			}
			ends[i].OtherSide = &b
		}
	}
	var seeds intersect.SplineSeedReport
	for i, x := range ends {
		axis := 2 * x.Operand
		if x.Edge.Side >= geom.SurfaceEdgeVMin {
			axis++
		}
		start, end := coords[i][axis], coords[i^1][axis]
		m := new(big.Rat).Add(ratInset(start), ratInset(end))
		mid, e := geom.RepresentedExactScalar(m.Quo(m, big.NewRat(2, 1)))
		if e != nil || mid == start || mid == end {
			return out, geom.ErrUnrepresentable
		}
		d := [2]geom.ParameterRectangle{patches[0].Domain(), patches[1].Domain()}
		// The root segment is affine in this free coordinate. For any
		// dependent axis, its values on this half lie between the exact
		// endpoint and midpoint. Restrict broadly around that range to
		// avoid unrelated internal knots in the two supporting charts.
		for k := range 4 {
			if k == axis {
				continue
			}
			x, y := coords[i][k], coords[i^1][k]
			r := new(big.Rat).Add(ratInset(x), ratInset(y))
			m, e := geom.RepresentedExactScalar(r.Quo(r, big.NewRat(2, 1)))
			if e != nil {
				return out, geom.ErrUnrepresentable
			}
			lo, hi := min(x, m), max(x, m)
			if k%2 == 0 {
				lo, hi = affineTubePadding(lo, hi, d[k/2].UMin, d[k/2].UMax)
				d[k/2].UMin, d[k/2].UMax = lo, hi
			} else {
				lo, hi = affineTubePadding(lo, hi, d[k/2].VMin, d[k/2].VMax)
				d[k/2].VMin, d[k/2].VMax = lo, hi
			}
		}
		if axis%2 == 0 {
			d[x.Operand].UMin, d[x.Operand].UMax = min(start, mid), max(start, mid)
		} else {
			d[x.Operand].VMin, d[x.Operand].VMax = min(start, mid), max(start, mid)
		}
		a, e := patches[0].Trim(d[0])
		if e != nil {
			return out, e
		}
		b, e := patches[1].Trim(d[1])
		if e != nil {
			return out, e
		}
		options := o.Trace.Seeds.TubeOptions
		options.FreeAxis = axis
		var p geom.SurfaceTubeCertificate
		var anchor *intersect.TubeBoundaryAnchor
		if x.OtherSide == nil {
			p, e = geom.CertifySurfacePairTube(a, b, options)
		} else {
			anchor = &intersect.TubeBoundaryAnchor{AtUpper: start > mid, Parameters: coords[i]}
			p, e = geom.CertifyBoundaryAnchoredSurfacePairTube(ctx, a, b, geom.SurfaceTubeBoundaryOptions{
				SurfacePairTubeOptions: options, AtUpper: anchor.AtUpper, Anchor: x.Edge.Hint,
			})
		}
		if errors.Is(e, geom.ErrDomain) {
			return out, geom.ErrConvergence
		}
		if e != nil {
			return out, e
		}
		if !p.ProvenBranch {
			return out, geom.ErrConvergence
		}
		seeds.Attempts = append(seeds.Attempts, intersect.SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: d[0], DomainB: d[1]}, Tube: &p, BoundaryAnchor: anchor})
	}
	graph, e := intersect.JoinSplineTubeBranches(ctx, ps[0], ps[1], seeds, o.Trace.Branches)
	if e != nil {
		return out, e
	}
	if len(graph.Branches) != 1 || len(graph.Branches[0].Cells) != 2 || graph.Branches[0].Closed || graph.Branches[0].StartUnresolved || graph.Branches[0].EndUnresolved {
		return out, geom.ErrConvergence
	}
	if graph.Branches[0].Cells[0].Cell == 1 {
		ends[0], ends[1] = ends[1], ends[0]
	}
	return SplineShellOpenSection{FaceA: pair[0], FaceB: pair[1], TubeCells: seeds, Start: ends[0], End: ends[1]}, nil
}

func affineTubePadding(lo, hi, domainLo, domainHi float64) (float64, float64) {
	return domainLo/2 + lo/2, domainHi/2 + hi/2
}

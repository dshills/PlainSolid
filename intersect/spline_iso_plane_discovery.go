package intersect

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
)

// discoverIsoPlaneSections is a sufficient fallback for nonlinear plane-chart
// projections: a complete plane trace must consist of exact constant-coordinate
// sections. Each whole iso is rechecked against the plane and clipped exactly.
func discoverIsoPlaneSections(ctx context.Context, a, b *geom.SurfacePatch, o SplinePlanePairDiscoveryOptions) (r SplineOpenPlaneDiscovery, err error) {
	for swapped, pair := range [2][2]*geom.SurfacePatch{{a, b}, {b, a}} {
		source, plane := pair[0], pair[1]
		pl, e := exactAffineSplineRawPlane(ctx, plane, o)
		if e != nil {
			if errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrDegenerate) || errors.Is(e, geom.ErrUnrepresentable) {
				continue
			}
			return r, e
		}
		trace, e := geom.TraceSurfaceRawPlaneContext(ctx, source, pl, o.Plane)
		if e != nil {
			if errors.Is(e, geom.ErrConvergence) {
				continue
			}
			return r, e
		}
		if !trace.Complete || len(trace.Isolation.Arcs) == 0 || len(trace.Isolation.Coplanar) > 0 || len(trace.Isolation.Tangencies) > 0 {
			continue
		}
		chart, e := geom.NewAffineSurfaceChart(ctx, plane, geom.AffineSurfaceChartOptions{MaxOperations: o.Tubes.MaxOperations, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
		if e != nil {
			return r, e
		}
		type iso struct {
			axis  int
			fixed float64
		}
		var sections []iso
		keys := map[string]bool{}
		supported := true
		for _, arc := range trace.Isolation.Arcs {
			p, e := arc.ExactEndpoint(0, geom.ArcExactEndpointOptions{})
			q, f := arc.ExactEndpoint(1, geom.ArcExactEndpointOptions{})
			if e != nil || f != nil {
				supported = false
				break
			}
			axis := arc.Certificate.FixedParameterAxis
			if p.Parameters[axis].Cmp(q.Parameters[axis]) != 0 {
				supported = false
				break
			}
			fixed, e := geom.RepresentedExactScalar(p.Parameters[axis])
			if e != nil {
				supported = false
				break
			}
			key := string(rune('0'+axis)) + p.Parameters[axis].RatString()
			if !keys[key] {
				keys[key] = true
				sections = append(sections, iso{axis, fixed})
			}
		}
		if !supported {
			continue
		}
		local := SplineOpenPlaneDiscovery{}
		for _, section := range sections {
			curve, e := source.IsoCurve(section.axis, section.fixed, geom.SurfacePatchOptions{MaxOperations: o.Tubes.MaxOperations, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
			if e != nil {
				return r, e
			}
			clipped, e := geom.ClipIsoCurveToAffineChart(ctx, curve, chart, geom.SplinePlaneOptions{MaxOperations: o.Tubes.MaxOperations, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
			if e != nil {
				if errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrDomain) {
					supported = false
					break
				}
				return r, e
			}
			if !clipped.Complete || len(clipped.Contacts) > 0 || len(clipped.CoincidentBoundary) > 0 {
				supported = false
				break
			}
			for _, span := range clipped.Spans {
				if span.Start.TangentBoundary || span.End.TangentBoundary {
					supported = false
					break
				}
				tubes, e := geom.DiscoverSurfaceIsoPlaneRationalTubes(ctx, source, plane, section.axis, section.fixed, span.Start.Parameter, span.End.Parameter, o.Tubes)
				if e != nil {
					if errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrDomain) || errors.Is(e, geom.ErrDegenerate) {
						supported = false
						break
					}
					return r, e
				}
				if !tubes.Complete || len(local.Seeds.Attempts)+len(tubes.Tubes) > o.MaxTubes {
					supported = false
					break
				}
				for _, t := range tubes.Tubes {
					cell := geom.SurfaceSurfaceCell{DomainA: t.Box.A, DomainB: t.Box.B}
					axis := t.FreeAxis
					if swapped == 1 {
						cell.DomainA, cell.DomainB = cell.DomainB, cell.DomainA
						axis = (axis + 2) % 4
					}
					pa, e := a.Trim(cell.DomainA)
					if e != nil {
						return r, e
					}
					pb, e := b.Trim(cell.DomainB)
					if e != nil {
						return r, e
					}
					proof, e := geom.CertifySurfacePairTube(pa, pb, geom.SurfacePairTubeOptions{FreeAxis: axis, MaxOperations: o.Tubes.MaxOperations, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
					if e != nil || !proof.ProvenBranch {
						supported = false
						break
					}
					local.Seeds.Attempts = append(local.Seeds.Attempts, SplineSeedCell{Cell: cell, Tube: &proof})
				}
				if !supported {
					break
				}
				for end, ep := range []geom.AffineChartCurveEndpoint{span.Start, span.End} {
					uv := [2]*big.Rat{}
					uv[section.axis] = new(big.Rat).SetFloat64(section.fixed)
					uv[1-section.axis] = ep.Parameter
					hint := [4]*big.Rat{uv[0], uv[1], ep.UV[0], ep.UV[1]}
					if swapped == 1 {
						hint = [4]*big.Rat{ep.UV[0], ep.UV[1], uv[0], uv[1]}
					}
					terminal, e := certifyIsoDiscoveryTerminal(ctx, a, b, local.Seeds.Attempts, hint, o)
					if e != nil {
						if !errors.Is(e, ErrUnresolved) {
							return r, e
						}
						supported = false
						break
					}
					terminal.Contour, terminal.End = local.OpenContours, end
					local.Terminals = append(local.Terminals, terminal)
				}
				if !supported {
					break
				}
				local.OpenContours++
			}
			if !supported {
				break
			}
		}
		if supported && local.OpenContours > 0 {
			return local, nil
		}
	}
	return r, ErrUnresolved
}

func certifyIsoDiscoveryTerminal(ctx context.Context, a, b *geom.SurfacePatch, seeds []SplineSeedCell, hint [4]*big.Rat, o SplinePlanePairDiscoveryOptions) (SplineOpenPlaneTerminal, error) {
	for i, seed := range seeds {
		axis := seed.Tube.FreeAxis
		d := seed.Cell.DomainA
		if axis >= 2 {
			d = seed.Cell.DomainB
		}
		lo, hi := d.UMin, d.UMax
		if axis%2 == 1 {
			lo, hi = d.VMin, d.VMax
		}
		upper := hint[axis].Cmp(new(big.Rat).SetFloat64(hi)) == 0
		if !upper && hint[axis].Cmp(new(big.Rat).SetFloat64(lo)) != 0 {
			continue
		}
		pa, e := a.Trim(seed.Cell.DomainA)
		if e != nil {
			return SplineOpenPlaneTerminal{}, e
		}
		pb, e := b.Trim(seed.Cell.DomainB)
		if e != nil {
			return SplineOpenPlaneTerminal{}, e
		}
		tube, e := geom.NewVerifiedSurfaceTube(pa, pb, geom.SurfacePairTubeOptions{FreeAxis: axis, MaxOperations: o.Tubes.MaxOperations, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
		if e != nil {
			return SplineOpenPlaneTerminal{}, e
		}
		cert, e := tube.CertifyEndpoint(ctx, upper, hint, geom.TubeEndpointOptions{MaxOperations: o.Tubes.MaxOperations, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
		if e != nil {
			if errors.Is(e, geom.ErrDomain) || errors.Is(e, geom.ErrConvergence) {
				continue
			}
			return SplineOpenPlaneTerminal{}, e
		}
		result := SplineOpenPlaneTerminal{Attempt: i, Upper: upper, Certificate: cert}
		for j, p := range cert.Parameters {
			result.Parameters[j] = new(big.Rat).Set(p)
		}
		for j, p := range cert.Point {
			result.Point[j] = new(big.Rat).Set(p)
		}
		return result, nil
	}
	return SplineOpenPlaneTerminal{}, ErrUnresolved
}

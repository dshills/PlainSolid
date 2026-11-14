package intersect

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/geom"
)

// SplinePlanePairDiscoveryOptions bounds an automatic transverse branch
// fallback for one affine spline plane and one spline patch; the opt-in
// trapezoid route uses a proved rational inverse for complete iso sections. Arc endpoints
// must have represented free coordinates. A later independent tube covering
// still validates the branch set against both original charts.
type SplinePlanePairDiscoveryOptions struct {
	// TrapezoidCharts enables the exact rational inverse iso fallback for planar nonlinear charts.
	TrapezoidCharts bool
	Enabled         bool // lets TraceSplinePair try this fallback after general discovery
	Plane           geom.SurfaceTraceOptions
	Tubes           geom.SurfaceArcTubeDiscoveryOptions
	MaxArcs         int
	MaxTubes        int
	// UCuts/VCuts optionally add exact proof-only knot lines on the
	// nonplanar chart before tracing. Tubes still use the original inputs.
	UCuts, VCuts []float64
}

// DiscoverSplinePlanePairTubes converts an exact spline-plane trace into
// candidate tubes on the original A/B charts. Affine support planes retain
// exact determinant coefficients; unit-normal rounding is not used. The result
// is a candidate report for CertifySplinePairBranches.
func DiscoverSplinePlanePairTubes(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, o SplinePlanePairDiscoveryOptions) (SplineSeedReport, error) {
	if o.TrapezoidCharts {
		var open SplineOpenPlaneDiscovery
		return discoverSplinePlanePairTubes(ctx, a, b, p, o, &open, false)
	}
	return discoverSplinePlanePairTubes(ctx, a, b, p, o, nil, false)
}

func discoverSplinePlanePairTubes(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, o SplinePlanePairDiscoveryOptions, open *SplineOpenPlaneDiscovery, allowClosed bool) (SplineSeedReport, error) {
	var out SplineSeedReport
	if o.MaxArcs == 0 {
		o.MaxArcs = 64
	}
	if o.MaxTubes == 0 {
		o.MaxTubes = 256
	}
	if ctx == nil || !a.valid() || !b.valid() || o.MaxArcs < 1 || o.MaxArcs > 4096 || o.MaxTubes < 1 || o.MaxTubes > 4096 || len(o.UCuts)+len(o.VCuts) > 64 {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	pa, e := exactSplinePatch(a, p)
	if e != nil {
		return out, e
	}
	pb, e := exactSplinePatch(b, p)
	if e != nil {
		return out, e
	}
	for swapped, pair := range [2][2]*geom.SurfacePatch{{pa, pb}, {pb, pa}} {
		x, y := pair[0], pair[1]
		plane, e := exactAffineSplineRawPlane(ctx, y, o)
		if e != nil {
			if errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrDegenerate) || errors.Is(e, geom.ErrUnrepresentable) {
				continue
			}
			return out, e
		}
		tracePatch := x
		if open != nil {
			chart, e := geom.NewAffineSurfaceChart(ctx, y, geom.AffineSurfaceChartOptions{MaxOperations: o.Tubes.MaxOperations, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
			if e != nil {
				return out, e
			}

			o.Tubes.AffinePlaneFreeAxes = false
			clip, clipError := geom.ClipSurfaceAffineProjection(ctx, x, chart, p)
			if clipError == nil {
				if clip.Empty {
					continue
				}
				tracePatch, e = x.Trim(clip.Domain)
				if e != nil {
					return out, e
				}
				o.Tubes.AffinePlaneFreeAxes = true
			} else if !errors.Is(clipError, geom.ErrConvergence) && !errors.Is(clipError, geom.ErrUnrepresentable) && !errors.Is(clipError, geom.ErrDegenerate) {
				return out, clipError
			}
			// Unsupported global projection retains the original untrimmed
			// trace proposal. Tube and exact terminal proofs remain mandatory.

		}
		for axis, cuts := range [2][]float64{o.UCuts, o.VCuts} {
			for _, t := range cuts {
				if open != nil {
					d := tracePatch.Domain()
					lo, hi := d.UMin, d.UMax
					if axis == 1 {
						lo, hi = d.VMin, d.VMax
					}
					if t <= lo || t >= hi {
						continue
					}
				}
				if e := ctx.Err(); e != nil {
					return out, e
				}
				if axis == 0 {
					tracePatch, e = tracePatch.InsertUKnot(t, 1, p)
				} else {
					tracePatch, e = tracePatch.InsertVKnot(t, 1, p)
				}
				if e != nil {
					return out, e
				}
			}
		}
		r, e := geom.TraceSurfaceRawPlaneContext(ctx, tracePatch, plane, o.Plane)
		if e != nil && !errors.Is(e, geom.ErrConvergence) {
			return out, e
		}
		if !r.Complete || len(r.Isolation.Coplanar) != 0 || len(r.Isolation.Tangencies) != 0 || len(r.Isolation.Arcs) > o.MaxArcs {
			continue
		}
		var local []SplineSeedCell
		complete := true
		for _, contour := range r.Contours {
			for _, ref := range contour.Arcs {
				arc := r.Isolation.Arcs[ref.Index]
				axis := 1 - arc.Certificate.FixedParameterAxis
				coord := func(p geom.Vec2) float64 {
					if axis == 0 {
						return p.X
					}
					return p.Y
				}
				lo, hi := arc.Endpoints[0].Parameters, arc.Endpoints[1].Parameters
				start, end := coord(lo.Min), coord(hi.Min)
				if start != coord(lo.Max) || end != coord(hi.Max) {
					x, e := arc.ExactEndpoint(0, geom.ArcExactEndpointOptions{})
					y, f := arc.ExactEndpoint(1, geom.ArcExactEndpointOptions{})
					if e == nil && f == nil {
						start, e = geom.RepresentedExactScalar(x.FreeParameter)
						end, f = geom.RepresentedExactScalar(y.FreeParameter)
						if e != nil || f != nil {
							complete = false
							break
						}
					} else {
						complete = false
						break
					}
				}
				if start == end {
					complete = false
					break
				}
				q, e := geom.DiscoverSurfacePlaneArcTubes(ctx, arc, x, y, min(start, end), max(start, end), o.Tubes)
				if e != nil && !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrInvalid) && !errors.Is(e, geom.ErrDomain) {
					return out, e
				}
				if !q.Complete || len(local)+len(q.Tubes) > o.MaxTubes || (open != nil && len(q.Excluded) > 0) {
					complete = false
					break
				}
				for _, t := range q.Tubes {
					cell := geom.SurfaceSurfaceCell{DomainA: t.Box.A, DomainB: t.Box.B}
					free := t.FreeAxis
					if swapped == 1 {
						cell.DomainA, cell.DomainB = cell.DomainB, cell.DomainA
						free = (free + 2) % 4
					}
					qa, e := pa.Trim(cell.DomainA)
					if e != nil {
						return out, e
					}
					qb, e := pb.Trim(cell.DomainB)
					if e != nil {
						return out, e
					}
					proof, e := geom.CertifySurfacePairTube(qa, qb, geom.SurfacePairTubeOptions{FreeAxis: free,
						MaxOperations: o.Tubes.MaxOperations, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
					if e != nil || !proof.ProvenBranch {
						if e == nil {
							e = ErrUnresolved
						}
						return out, e
					}
					local = append(local, SplineSeedCell{Cell: cell, Tube: &proof})
				}
			}
			if !complete {
				break
			}
		}
		if complete {
			if open != nil {
				terminals, count, terminalError := certifyOpenPlaneTerminals(ctx, r, local, pa, pb, y, swapped, o, allowClosed)
				if terminalError != nil {
					if errors.Is(terminalError, geom.ErrConvergence) || errors.Is(terminalError, geom.ErrDomain) || errors.Is(terminalError, ErrUnresolved) {
						continue
					}
					return out, terminalError
				}
				open.Terminals, open.OpenContours = terminals, count
				open.ClosedContours = len(r.Contours) - count
			}
			out.Attempts = local
			return out, nil
		}
	}
	if open != nil {
		fallback, e := discoverIsoPlaneSections(ctx, pa, pb, o)
		if e == nil {
			*open = fallback
			return fallback.Seeds, nil
		}
		if !errors.Is(e, ErrUnresolved) {
			return out, e
		}
	}
	return out, ErrUnresolved
}

func exactAffineSplinePlane(ctx context.Context, p *geom.SurfacePatch, o SplinePlanePairDiscoveryOptions) (geom.Plane, error) {
	chart, e := geom.NewAffineSurfaceChart(ctx, p, geom.AffineSurfaceChartOptions{MaxCells: o.MaxArcs,
		MaxOperations: o.Tubes.MaxOperations, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
	if e != nil {
		return geom.Plane{}, e
	}
	d := chart.Domain()
	point := func(u, v float64) (geom.Vec3, error) {
		x, e := chart.EvalExact(new(big.Rat).SetFloat64(u), new(big.Rat).SetFloat64(v))
		if e != nil {
			return geom.Vec3{}, e
		}
		return geom.RepresentedExactPoint(x)
	}
	a, e := point(d.UMin, d.VMin)
	if e != nil {
		return geom.Plane{}, e
	}
	b, e := point(d.UMax, d.VMin)
	if e != nil {
		return geom.Plane{}, e
	}
	c, e := point(d.UMin, d.VMax)
	if e != nil {
		return geom.Plane{}, e
	}
	pl, e := geom.NewPlane(a, b.Sub(a).Cross(c.Sub(a)))
	if e != nil {
		return geom.Plane{}, geom.ErrUnrepresentable
	}
	q, e := p.PlaneBounds(pl)
	if e != nil {
		return geom.Plane{}, e
	}
	if q.Relation != geom.PlanePatchCoplanar {
		return geom.Plane{}, geom.ErrUnrepresentable
	}
	return pl, nil
}

func exactAffineSplineRawPlane(ctx context.Context, p *geom.SurfacePatch, o SplinePlanePairDiscoveryOptions) (geom.RawSupportPlane, error) {
	chart, e := geom.NewAffineSurfaceChart(ctx, p, geom.AffineSurfaceChartOptions{MaxCells: o.MaxArcs, MaxOperations: o.Tubes.MaxOperations, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
	if e != nil {
		return geom.RawSupportPlane{}, e
	}
	return chart.RawSupportPlane(geom.SurfacePatchOptions{MaxOperations: o.Tubes.MaxOperations, MaxCoefficientBits: o.Tubes.MaxCoefficientBits})
}

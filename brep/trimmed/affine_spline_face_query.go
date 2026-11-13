package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// AffineSplineFaceQueryOptions bounds each independent support/trim proof.
type AffineSplineFaceQueryOptions struct {
	Patch geom.SurfacePatchOptions
	Chart geom.AffineSurfaceChartOptions
	Trim  trimvalidate.Options
}

// AffineSplineFaceQuery binds an exact affine spline support and certified
// finite trim region. It does not certify shared model edges or a closed shell.
// Native positive-weight quadratic/cubic/quartic trims and straight uses are
// supported when their entire control hull lies in the support rectangle.
type AffineSplineFaceQuery struct {
	face  Face
	chart *geom.AffineSurfaceChart
	plane [4]*big.Rat
	trim  trimvalidate.Options
}

// NewAffineSplineFaceQuery rechecks the stored mapped spline, original trim
// topology and support-domain containment. No mesh or rounded normal is used.
func NewAffineSplineFaceQuery(ctx context.Context, f Face, o AffineSplineFaceQueryOptions) (*AffineSplineFaceQuery, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	opts, e := o.Trim.WithDefaults()
	if e != nil {
		return nil, e
	}
	p, e := exactFaceSupportPatch(f, o.Patch)
	if e != nil {
		return nil, e
	}
	chart, e := geom.NewAffineSurfaceChart(ctx, p, o.Chart)
	if e != nil {
		return nil, e
	}
	domain := chart.Domain()
	bounds := [2][2]*big.Rat{{ratInset(domain.UMin), ratInset(domain.UMax)}, {ratInset(domain.VMin), ratInset(domain.VMax)}}
	count := 0
	for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
		for _, use := range loop {
			if !uvFinite(use.UVStart) || !uvFinite(use.UVEnd) {
				return nil, geom.ErrInvalid
			}
			count++
			if count > opts.MaxEdges {
				return nil, geom.ErrConvergence
			}
			var controls [][3]*big.Rat
			switch q := use.Curve.(type) {
			case nil:
				controls = [][3]*big.Rat{{ratInset(use.UVStart.X), ratInset(use.UVStart.Y), big.NewRat(1, 1)}, {ratInset(use.UVEnd.X), ratInset(use.UVEnd.Y), big.NewRat(1, 1)}}
			case ExactQuadraticUV:
				h := q.HomogeneousControls()
				controls = h[:]
			case *ExactQuadraticUV:
				if q == nil {
					return nil, geom.ErrInvalid
				}
				h := q.HomogeneousControls()
				controls = h[:]
			case ExactBezierUV:
				controls = q.HomogeneousControls()
			case *ExactBezierUV:
				if q == nil {
					return nil, geom.ErrInvalid
				}
				controls = q.HomogeneousControls()
			default:
				return nil, geom.ErrConvergence
			}
			for _, h := range controls {
				for _, v := range h {
					if v == nil {
						return nil, geom.ErrInvalid
					}
					if v.Num().BitLen() > opts.MaxBits || v.Denom().BitLen() > opts.MaxBits {
						return nil, geom.ErrConvergence
					}
				}
				if h[2].Sign() <= 0 {
					return nil, geom.ErrInvalid
				}
				for axis := 0; axis < 2; axis++ {
					for side := 0; side < 2; side++ {
						v := new(big.Rat).Mul(bounds[axis][side], h[2])
						if v.Num().BitLen() > opts.MaxBits || v.Denom().BitLen() > opts.MaxBits {
							return nil, geom.ErrConvergence
						}
						cmp := h[axis].Cmp(v)
						if side == 0 && cmp < 0 || side == 1 && cmp > 0 {
							return nil, geom.ErrDomain
						}
					}
				}
			}
		}
	}
	proof, e := ValidateFaceTrimLoops(ctx, f, opts)
	if e != nil {
		return nil, e
	}
	if proof.Status != trimvalidate.Certified {
		return nil, geom.ErrConvergence
	}
	raw, e := chart.RawSupportPlane(o.Patch)
	if e != nil {
		return nil, e
	}
	plane := raw.Coefficients()
	if f.Reversed {
		for _, v := range plane {
			v.Neg(v)
		}
	}
	snapshot := cloneFace(f)
	for _, loop := range append([][]Use{snapshot.Outer}, snapshot.Holes...) {
		for i := range loop {
			switch q := loop[i].Curve.(type) {
			case *ExactQuadraticUV:
				loop[i].Curve = *q
			case *ExactBezierUV:
				loop[i].Curve = *q
			}
		}
	}
	return &AffineSplineFaceQuery{face: snapshot, chart: chart, plane: plane, trim: opts}, nil
}

// Plane returns caller-owned exact coefficients N·XYZ+D=0. N has the face's
// source orientation (including Reversed) and is deliberately not unit length.
func (q *AffineSplineFaceQuery) Plane() (out [4]*big.Rat) {
	if q == nil {
		return
	}
	for i, v := range q.plane {
		out[i] = new(big.Rat).Set(v)
	}
	return
}

type AffineSplineFacePoint struct {
	Proven, OnPlane bool
	Location        trimvalidate.Location
	UV              [2]*big.Rat
}

// ClassifyRationalPoint inverts exact model coordinates without rounding. An
// off-plane point is Outside with OnPlane=false and no UV claim. On-plane UV is
// returned even outside the support rectangle. Failure publishes no point proof.
func (q *AffineSplineFaceQuery) ClassifyRationalPoint(ctx context.Context, point [3]*big.Rat, o geom.AffineChartInverseOptions) (AffineSplineFacePoint, error) {
	var zero AffineSplineFacePoint
	if q == nil {
		return zero, geom.ErrInvalid
	}
	inverse, e := q.chart.InvertExactContext(ctx, point, o)
	if e != nil {
		return zero, e
	}
	out := AffineSplineFacePoint{Proven: true, OnPlane: inverse.OnPlane, Location: trimvalidate.Outside}
	if !inverse.OnPlane {
		return out, nil
	}
	out.UV = [2]*big.Rat{inverse.U, inverse.V}
	if !inverse.Inside {
		return out, nil
	}
	location, proof, e := LocateRationalInFaceTrim(ctx, q.face, out.UV, q.trim)
	if e != nil {
		return zero, e
	}
	if proof.Status != trimvalidate.Certified || location == trimvalidate.Unknown {
		return zero, geom.ErrConvergence
	}
	out.Location = location
	return out, nil
}

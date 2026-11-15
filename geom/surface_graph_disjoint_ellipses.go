package geom

import "context"

// SplineGraphDisjointEllipses proves exactly two closed transverse contours on
// the complete original common-XY domain. Each lies strictly inside one of two
// closed U slabs. In particular their shared separator has no contact. This is
// a coverage certificate, not an assertion that the original roofs are smooth
// at their knots. Returned proposal data is not accepted as authority by native
// constructors, which independently reconstruct the proof.
type SplineGraphDisjointEllipses struct {
	Complete        bool
	PartitionAxis   int // 0 is original U; 1 is original V.
	Domain          ParameterRectangle
	Contours        [2]SplineGraphEllipse
	ExactOperations int
}

// CertifySplineGraphDisjointEllipses certifies both complete original slabs,
// including their shared boundary. splitU is an exact represented original
// parameter. Both quotient signs must agree outside their respective contours.
// All restriction and contact work is charged cumulatively; failure is atomic.
func CertifySplineGraphDisjointEllipses(ctx context.Context, a, c *SurfacePatch, splitU float64, hints [2]GraphEllipseHint, o GraphPointContactOptions) (r SplineGraphDisjointEllipses, err error) {
	return CertifySplineGraphDisjointEllipsesOnAxis(ctx, a, c, 0, splitU, hints, o)
}

// CertifySplineGraphDisjointEllipsesOnAxis proves the same complete original coverage along
// axis 0 (U) or 1 (V), without transposing or replacing either original chart.
func CertifySplineGraphDisjointEllipsesOnAxis(ctx context.Context, a, c *SurfacePatch, axis int, splitU float64, hints [2]GraphEllipseHint, o GraphPointContactOptions) (r SplineGraphDisjointEllipses, err error) {
	used := 0
	defer func() {
		if err != nil {
			r = SplineGraphDisjointEllipses{}
		}
		r.ExactOperations = used
	}()
	if ctx == nil || a == nil || c == nil || axis < 0 || axis > 1 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 {
		return r, ErrInvalid
	}
	d := a.Domain()
	if !d.Valid() || d != c.Domain() || !(splitU > [2]float64{d.UMin, d.VMin}[axis] && splitU < [2]float64{d.UMax, d.VMax}[axis]) {
		return r, ErrInvalid
	}
	var originals [2]*SurfacePatch
	for i, p := range []*SurfacePatch{a, c} {
		if used >= o.MaxOperations {
			return r, ErrConvergence
		}
		q, e := p.WithLimits(SurfacePatchOptions{MaxOperations: o.MaxOperations - used, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return r, e
		}
		used += q.ExactOperations()
		originals[i] = q
	}
	cells := 0
	for i := 0; i < 2; i++ {
		slab := d
		if axis == 0 {
			if i == 0 {
				slab.UMax = splitU
			} else {
				slab.UMin = splitU
			}
		} else {
			if i == 0 {
				slab.VMax = splitU
			} else {
				slab.VMin = splitU
			}
		}
		var p [2]*SurfacePatch
		for j, original := range originals {
			if e := ctx.Err(); e != nil {
				return r, e
			}
			if used >= o.MaxOperations {
				return r, ErrConvergence
			}
			q, work, e := original.TrimWithWorkLimit(slab, o.MaxOperations-used)
			used += work
			if e != nil {
				return r, e
			}
			p[j] = q
		}
		if used >= o.MaxOperations || cells >= o.MaxCells {
			return r, ErrConvergence
		}
		proof := o
		proof.MaxOperations -= used
		proof.MaxCells -= cells
		q, e := CertifySplineGraphEllipse(ctx, p[0], p[1], hints[i], proof)
		used += q.ExactOperations
		if e != nil {
			return r, e
		}
		if !q.Proven || q.Domain != slab {
			return r, ErrConvergence
		}
		cells += len(q.CertifiedCells)
		r.Contours[i] = q
	}
	if r.Contours[0].OutsideSide != r.Contours[1].OutsideSide || r.Contours[0].InsideSide != r.Contours[1].InsideSide {
		return r, ErrConvergence
	}
	r.PartitionAxis = axis
	r.Complete = true
	r.Domain = d
	return r, nil
}

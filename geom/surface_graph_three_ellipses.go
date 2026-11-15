package geom

import "context"

// SplineGraphThreeEllipses proves exactly three closed transverse contours on
// the complete original common-XY domain. Each lies strictly inside one of three
// closed U slabs. In particular each shared separator has no contact. This is
// a coverage certificate, not an assertion that the original roofs are smooth
// at their knots. Returned proposal data is not accepted as authority by native
// constructors, which independently reconstruct the proof.
type SplineGraphThreeEllipses struct {
	Complete        bool
	PartitionAxis   int // 0 is original U; 1 is original V.
	Domain          ParameterRectangle
	Contours        [3]SplineGraphEllipse
	ExactOperations int
}

// CertifySplineGraphThreeEllipses certifies all three complete original slabs,
// including both shared boundaries. splitU contains exact represented original
// parameters. All quotient signs must agree outside their respective contours.
// All restriction and contact work is charged cumulatively; failure is atomic.
func CertifySplineGraphThreeEllipses(ctx context.Context, a, c *SurfacePatch, splitU [2]float64, hints [3]GraphEllipseHint, o GraphPointContactOptions) (r SplineGraphThreeEllipses, err error) {
	return CertifySplineGraphThreeEllipsesOnAxis(ctx, a, c, 0, splitU, hints, o)
}

// CertifySplineGraphThreeEllipsesOnAxis proves the same complete original coverage along
// axis 0 (U) or 1 (V), without transposing or replacing either original chart.
func CertifySplineGraphThreeEllipsesOnAxis(ctx context.Context, a, c *SurfacePatch, axis int, splitU [2]float64, hints [3]GraphEllipseHint, o GraphPointContactOptions) (r SplineGraphThreeEllipses, err error) {
	used := 0
	defer func() {
		if err != nil {
			r = SplineGraphThreeEllipses{}
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
	if !d.Valid() || d != c.Domain() || !(splitU[0] > [2]float64{d.UMin, d.VMin}[axis] && splitU[0] < splitU[1] && splitU[1] < [2]float64{d.UMax, d.VMax}[axis]) {
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
	for i := 0; i < 3; i++ {
		slab := d
		if axis == 0 {
			if i > 0 {
				slab.UMin = splitU[i-1]
			}
			if i < 2 {
				slab.UMax = splitU[i]
			}
		} else {
			if i > 0 {
				slab.VMin = splitU[i-1]
			}
			if i < 2 {
				slab.VMax = splitU[i]
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
	for i := 1; i < 3; i++ {
		if r.Contours[0].OutsideSide != r.Contours[i].OutsideSide || r.Contours[0].InsideSide != r.Contours[i].InsideSide {
			return r, ErrConvergence
		}
	}
	r.PartitionAxis = axis
	r.Complete = true
	r.Domain = d
	return r, nil
}

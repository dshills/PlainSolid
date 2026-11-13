package geom

import "fmt"

// DegreeElevationOptions bounds output storage. Zero MaxPoles selects 1,000,000.
type DegreeElevationOptions struct{ MaxPoles int }

// ElevateDegree returns the same rational curve at targetDegree (up to float64
// construction error). Clamped continuous splines are converted to piecewise
// Bezier form, elevated in homogeneous coordinates, and joined with interior
// multiplicity targetDegree. Geometry retains its original smoothness, but the
// resulting knot representation declares only C0 continuity. This operation
// does not attempt knot removal. targetDegree must be in [Degree(),64].
func (c *BSpline) ElevateDegree(targetDegree int) (*BSpline, error) {
	return c.ElevateDegreeWithOptions(targetDegree, DegreeElevationOptions{})
}
func (c *BSpline) ElevateDegreeWithOptions(targetDegree int, options DegreeElevationOptions) (*BSpline, error) {
	if c == nil || len(c.poles) == 0 || targetDegree < c.degree || targetDegree > 64 {
		return nil, ErrInvalid
	}
	limit := options.MaxPoles
	if limit == 0 {
		limit = 1000000
	}
	if limit < 2 {
		return nil, ErrInvalid
	}
	if len(c.poles) > limit {
		return nil, ErrConvergence
	}
	if targetDegree == c.degree {
		return NewBSpline(c.degree, c.poles, c.knots, c.weights)
	}
	if !projectionKnotsSupported(c.degree, len(c.poles), c.knots) {
		return nil, fmt.Errorf("%w: elevation requires clamped continuous knots", ErrInvalid)
	}
	a, b := c.Domain()
	breaks := []float64{a}
	for _, k := range c.knots {
		if k > breaks[len(breaks)-1] && k < b {
			breaks = append(breaks, k)
		}
	}
	breaks = append(breaks, b)
	spans := len(breaks) - 1
	if spans > (limit-1)/targetDegree {
		return nil, ErrConvergence
	}
	remaining := toHom(c.poles, c.weights)
	knots := c.knots
	output := make([]homogeneous, 0, spans*targetDegree+1)
	newKnots := make([]float64, 0, spans*targetDegree+targetDegree+2)
	for i := 0; i <= targetDegree; i++ {
		newKnots = append(newKnots, a)
	}
	for span := 0; span < spans; span++ {
		piece := remaining
		if span < spans-1 {
			left, right, _, rightKnots := splitHomogeneous(remaining, knots, c.degree, breaks[span+1])
			piece, remaining, knots = left, right, rightKnots
		}
		for degree := c.degree; degree < targetDegree; degree++ {
			elevated := make([]homogeneous, degree+2)
			elevated[0], elevated[degree+1] = piece[0], piece[degree]
			for i := 1; i <= degree; i++ {
				alpha := float64(i) / float64(degree+1)
				for j := 0; j < 4; j++ {
					elevated[i][j] = alpha*piece[i-1][j] + (1-alpha)*piece[i][j]
				}
			}
			piece = elevated
		}
		if span == 0 {
			output = append(output, piece...)
		} else {
			output = append(output, piece[1:]...)
		}
		multiplicity := targetDegree
		if span == spans-1 {
			multiplicity++
		}
		for i := 0; i < multiplicity; i++ {
			newKnots = append(newKnots, breaks[span+1])
		}
	}
	poles := make([]Vec3, len(output))
	weights := make([]float64, len(output))
	for i, h := range output {
		p, e := projectHom(h)
		if e != nil {
			return nil, e
		}
		poles[i], weights[i] = p, h[3]
	}
	return NewBSpline(targetDegree, poles, newKnots, weights)
}

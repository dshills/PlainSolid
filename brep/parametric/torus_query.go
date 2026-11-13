package parametric

import (
	"math"

	"plainsolid/geom"
)

// Ring describes the solid enclosed by a ring torus. Its major radius must
// exceed its positive minor radius.
type Ring struct {
	Frame        geom.Frame
	Major, Minor float64
}

func (r Ring) valid() bool {
	return r.Frame.ZAxis().Length() > 0 && r.Minor > 0 && r.Major > r.Minor && finite(r.Major+r.Minor)
}

// Classify tests the signed tube distance at a point.
func (r Ring) Classify(p geom.Vec3, tolerance float64) (Location, error) {
	if !r.valid() || !p.IsFinite() || tolerance < 0 || !finite(tolerance) {
		return Outside, geom.ErrInvalid
	}
	x := r.Frame.ToLocal(p)
	d := math.Hypot(math.Hypot(x.X, x.Y)-r.Major, x.Z) - r.Minor
	if !finite(d) {
		return Outside, geom.ErrInvalid
	}
	if math.Abs(d) <= tolerance {
		return Boundary, nil
	}
	if d < 0 {
		return Inside, nil
	}
	return Outside, nil
}

// Ray returns all isolated contacts with the nonnegative ray segment,
// including tangencies. Parameter distance is in model units.
func (r Ring) Ray(origin, direction geom.Vec3, maxDistance, tolerance float64) (geom.LineTorusResult, error) {
	if !r.valid() || maxDistance <= 0 || math.IsNaN(maxDistance) || tolerance <= 0 || !finite(tolerance) {
		return geom.LineTorusResult{}, geom.ErrInvalid
	}
	s, e := geom.NewTorusSurface(r.Frame, r.Major, r.Minor)
	if e != nil {
		return geom.LineTorusResult{}, e
	}
	l, e := geom.NewLine(origin, direction)
	if e != nil {
		return geom.LineTorusResult{}, e
	}
	return geom.IntersectLineTorus(l, s, geom.LineTorusOptions{
		UseParameterRange: true, MinParameter: 0, MaxParameter: maxDistance, DistanceTolerance: tolerance,
	})
}

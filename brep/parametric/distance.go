package parametric

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

// CylinderRegion describes a finite right circular cylinder. Its frame
// origin is the base center and its axis points toward the upper cap.
type CylinderRegion struct {
	Frame          geom.Frame
	Radius, Height float64
}

func (c CylinderRegion) signedDistance(p geom.Vec3) (float64, geom.Vec3, error) {
	if !p.IsFinite() || c.Radius <= 0 || c.Height <= 0 || !finite(c.Radius) || !finite(c.Height) || c.Frame.ZAxis().Length() == 0 {
		return 0, geom.Vec3{}, geom.ErrInvalid
	}
	x := c.Frame.ToLocal(p)
	r := math.Hypot(x.X, x.Y)
	// Signed distance to the product of a closed disk and a closed interval.
	a, b := r-c.Radius, math.Max(-x.Z, x.Z-c.Height)
	d := math.Hypot(math.Max(a, 0), math.Max(b, 0)) + math.Min(math.Max(a, b), 0)
	if !x.IsFinite() || !finite(d) {
		return 0, geom.Vec3{}, geom.ErrInvalid
	}
	return d, x, nil
}

func (c CylinderRegion) Classify(p geom.Vec3, tolerance float64) (Location, error) {
	if tolerance < 0 || !finite(tolerance) {
		return Outside, geom.ErrInvalid
	}
	d, _, e := c.signedDistance(p)
	if e != nil {
		return Outside, e
	}
	if math.Abs(d) <= tolerance {
		return Boundary, nil
	}
	if d < 0 {
		return Inside, nil
	}
	return Outside, nil
}

// BoundaryProjection reports a signed distance to a selected nearest point.
// If Unique is false, equally near points exist; a deterministic representative
// is returned.
type BoundaryProjection struct {
	Point          geom.Vec3
	SignedDistance float64
	Unique         bool
}

func (c CylinderRegion) Project(p geom.Vec3) (BoundaryProjection, error) {
	d, x, e := c.signedDistance(p)
	if e != nil {
		return BoundaryProjection{}, e
	}
	r := math.Hypot(x.X, x.Y)
	u := geom.Vec3{X: 1}
	if r > 0 {
		u = geom.Vec3{X: x.X / r, Y: x.Y / r}
	}
	y := u.Scale(math.Min(r, c.Radius))
	y.Z = math.Max(0, math.Min(c.Height, x.Z))
	unique := true
	if d < 0 {
		a, b, c0 := c.Radius-r, x.Z, c.Height-x.Z
		m := math.Min(a, math.Min(b, c0))
		count := 0
		for _, q := range []float64{a, b, c0} {
			if q == m {
				count++
			}
		}
		unique = count == 1
		switch {
		case a == m:
			y = u.Scale(c.Radius)
			y.Z = x.Z
			unique = unique && r > 0
		case b == m:
			y.Z = 0
		default:
			y.Z = c.Height
		}
	}
	q := c.Frame.ToWorld(y)
	if !q.IsFinite() {
		return BoundaryProjection{}, fmt.Errorf("parametric: cylinder projection overflow")
	}
	return BoundaryProjection{q, d, unique}, nil
}

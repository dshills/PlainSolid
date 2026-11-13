package parametric

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

type Location uint8

const (
	Outside Location = iota
	Boundary
	Inside
)

type BallProjection struct {
	Point          geom.Vec3
	SignedDistance float64
	Unique         bool
}

func (b Ball) valid() bool { return b.Center.IsFinite() && b.Radius > 0 && finite(b.Radius) }

// Project returns the nearest boundary point and signed distance (negative
// inside). At the center any boundary point is nearest; +X is returned.
func (b Ball) Project(p geom.Vec3) (BallProjection, error) {
	if !b.valid() || !p.IsFinite() {
		return BallProjection{}, geom.ErrInvalid
	}
	d := p.Sub(b.Center)
	length := d.Length()
	if !finite(length) {
		return BallProjection{}, geom.ErrInvalid
	}
	dir := geom.Vec3{X: 1}
	if length != 0 {
		var e error
		dir, e = d.Unit()
		if e != nil {
			return BallProjection{}, e
		}
	}
	point := b.Center.Add(dir.Scale(b.Radius))
	if !point.IsFinite() || point == b.Center {
		return BallProjection{}, fmt.Errorf("parametric: ball boundary outside numerical range")
	}
	return BallProjection{point, length - b.Radius, length != 0}, nil
}

// Classify uses a world-space distance tolerance around the ball boundary.
func (b Ball) Classify(p geom.Vec3, tolerance float64) (Location, error) {
	if !b.valid() || !p.IsFinite() || tolerance < 0 || !finite(tolerance) {
		return Outside, geom.ErrInvalid
	}
	d := p.Sub(b.Center).Length()
	if !finite(d) {
		return Outside, geom.ErrInvalid
	}
	d -= b.Radius
	if math.Abs(d) <= tolerance {
		return Boundary, nil
	}
	if d < 0 {
		return Inside, nil
	}
	return Outside, nil
}

// DistanceTo returns minimum Euclidean distance between two closed balls,
// including zero for overlap or tangency.
func (b Ball) DistanceTo(c Ball) (float64, error) {
	if !b.valid() || !c.valid() {
		return 0, geom.ErrInvalid
	}
	d := b.Center.Sub(c.Center).Length()
	if !finite(d) {
		return 0, geom.ErrInvalid
	}
	if d <= b.Radius {
		return 0, nil
	}
	return math.Max(0, d-b.Radius-c.Radius), nil
}

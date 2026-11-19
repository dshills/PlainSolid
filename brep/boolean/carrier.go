package boolean

import (
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// family names a kind of carrier. It is the key of the surface-pair table
// of section 7.4 and of the curve table of section 7.5.
type family uint8

const (
	planeFamily family = iota
	lineFamily
)

// surface is the unbounded carrier of a face.
type surface interface {
	family() family
	// offset is the signed distance from the carrier to p, positive on
	// the side its normal points to.
	offset(p geom.Vec3) float64
	// normal is the unit carrier normal at the point nearest p.
	normal(p geom.Vec3) geom.Vec3
	// chart is the planar domain the carrier's faces are split in.
	chart() chart
	// value is the surface a face of the result stores.
	value() (geom.Surface, error)
}

// curve is the unbounded carrier of an edge, with a parameter that
// increases along it.
type curve interface {
	family() family
	at(t float64) geom.Vec3
	// tangent is the unit direction of increasing parameter at t.
	tangent(t float64) geom.Vec3
	// param is the parameter of the point of the carrier nearest p.
	param(p geom.Vec3) float64
	// flipped is the same carrier with its parameter reversed.
	flipped() curve
	// value is the curve and range an edge of the result stores between
	// two vertices that lie on the carrier within tolerance (B6).
	value(from, to geom.Vec3) (geom.Curve, parametric.Interval, error)
}

// leg is a directed part of a curve carrier, from parameter t0 to t1.
type leg struct {
	c      curve
	t0, t1 float64
}

// back is the same part walked the other way.
func (l leg) back() leg { return leg{l.c, l.t1, l.t0} }

// off is the distance from p to an unbounded curve carrier.
func off(c curve, p geom.Vec3) float64 { return p.Distance(c.at(c.param(p))) }

// plane is the carrier of a planar face. Its normal is the frame's z axis
// and its chart coordinates are the frame's x and y.
type plane struct{ frame geom.Frame }

func (s plane) family() family               { return planeFamily }
func (s plane) offset(p geom.Vec3) float64   { return p.Sub(s.frame.Origin()).Dot(s.frame.ZAxis()) }
func (s plane) normal(geom.Vec3) geom.Vec3   { return s.frame.ZAxis() }
func (s plane) chart() chart                 { return planeChart(s) }
func (s plane) value() (geom.Surface, error) { return geom.NewPlaneSurface(s.frame) }

// line is the carrier of a straight edge; its parameter is distance.
type line struct{ origin, dir geom.Vec3 }

func (c line) family() family            { return lineFamily }
func (c line) at(t float64) geom.Vec3    { return c.origin.Add(c.dir.Scale(t)) }
func (c line) tangent(float64) geom.Vec3 { return c.dir }
func (c line) param(p geom.Vec3) float64 { return p.Sub(c.origin).Dot(c.dir) }
func (c line) flipped() curve            { return line{c.origin, c.dir.Scale(-1)} }
func (c line) value(from, to geom.Vec3) (geom.Curve, parametric.Interval, error) {
	l, err := geom.NewLine(from, to.Sub(from))
	return l, parametric.Interval{Max: to.Distance(from)}, err
}

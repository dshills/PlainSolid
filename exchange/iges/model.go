// Package iges reads and writes a bounded ASCII IGES geometry subset: Cartesian
// points (116), line segments
// (110), constant-Z circular arcs (100), rational B-spline curves (126), and tensor-product surfaces (128).
// Referenced affine placements are supported; topology and assemblies are not.
package iges

import (
	"errors"
	"math"
	"plainsolid/geom"
)

// Limits bounds input records, scalar parameters and decoded control points.
// Zero fields select 64 MiB, one million records, 100,000 entities, four million
// values and one million control points. Input is buffered before parsing.
type Limits struct {
	MaxBytes                                             int64
	MaxRecords, MaxEntities, MaxValues, MaxControlPoints int
}

func (l Limits) defaults() (Limits, error) {
	if l.MaxBytes == 0 {
		l.MaxBytes = 64 << 20
	}
	if l.MaxRecords == 0 {
		l.MaxRecords = 1_000_000
	}
	if l.MaxEntities == 0 {
		l.MaxEntities = 100_000
	}
	if l.MaxValues == 0 {
		l.MaxValues = 4_000_000
	}
	if l.MaxControlPoints == 0 {
		l.MaxControlPoints = 1_000_000
	}
	if l.MaxBytes < 1 || l.MaxBytes > 1<<30 || l.MaxRecords < 1 || l.MaxEntities < 1 || l.MaxValues < 1 || l.MaxControlPoints < 1 {
		return l, errors.New("iges: invalid limits")
	}
	return l, nil
}

// Model stores raw file coordinates. LengthUnit is meters per coordinate unit;
// coordinates are not rescaled. Only model-space scale 1 is supported.
type Model struct {
	LengthUnit       float64
	Entities         []Entity
	CurvesOnSurfaces []CurveOnSurface
	TrimmedSurfaces  []TrimmedSurface
}

// Entity contains exactly one point, curve, surface or bounded direct-curve composite.
// Label/subscript preserve directory labels.
// Form is 0 for lines/arcs and 0..5 for the B-spline classification hint.
type Entity struct {
	// Retains source provenance: generic import may round weight normalization,
	// but an exact curve-on-surface proof must reject that alteration.
	sourceWeightsRounded bool
	// Transform maps definition-space curve coordinates to model space; nil is identity.
	Transform       *geom.Affine
	Label           string
	Subscript, Form int
	// Point is an independent type-116 Cartesian datum, not a curve.
	Point     *geom.Vec3
	Line      *Line
	Arc       *Arc
	Spline    *Spline
	Surface   *Surface
	Composite *CompositeCurve
}

// CompositeCurve contains 4–64 direct curve entity indices. Nested composites are unsupported.
type CompositeCurve struct{ Curves []int }

type Line struct{ Start, End geom.Vec3 }

// Arc is counterclockwise around +Z from Start to End. Equal endpoints denote a
// complete circle. Start/End contain XY coordinates; Center.Z is the plane.
type Arc struct {
	Center     geom.Vec3
	Start, End geom.Vec2
}

// Spline retains the IGES trimmed parameter interval and informational flags.
// Periodic does not change evaluation into wrapped extrapolation.
type Spline struct {
	Curve                                *geom.BSpline
	Start, End                           float64
	Planar, Closed, Polynomial, Periodic bool
	Normal                               geom.Vec3
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
func (a Arc) parameters() (float64, float64, float64, error) {
	if !a.Center.IsFinite() || !finite(a.Start.X) || !finite(a.Start.Y) || !finite(a.End.X) || !finite(a.End.Y) {
		return 0, 0, 0, errors.New("iges: nonfinite arc")
	}
	sx, sy, ex, ey := a.Start.X-a.Center.X, a.Start.Y-a.Center.Y, a.End.X-a.Center.X, a.End.Y-a.Center.Y
	r := math.Hypot(sx, sy)
	endRadius := math.Hypot(ex, ey)
	if !finite(r) || r == 0 || !finite(endRadius) || math.Abs(endRadius/r-1) > 1e-10 {
		return 0, 0, 0, errors.New("iges: inconsistent arc radius")
	}
	start, end := math.Atan2(sy, sx), math.Atan2(ey, ex)
	sweep := end - start
	if a.Start == a.End {
		sweep = 2 * math.Pi
	} else {
		if sweep == 0 {
			return 0, 0, 0, errors.New("iges: arc sweep below numeric resolution")
		}
		if sweep < 0 {
			sweep += 2 * math.Pi
		}
		if sweep == 2*math.Pi {
			return 0, 0, 0, errors.New("iges: arc sweep indistinguishable from full circle")
		}
	}
	return r, start, sweep, nil
}
func (a Arc) Sweep() (float64, error) { _, _, s, e := a.parameters(); return s, e }

// Eval evaluates the arc at a normalized parameter in [0,1].
func (a Arc) Eval(t float64) (geom.Vec3, error) {
	r, start, sweep, e := a.parameters()
	if e != nil {
		return geom.Vec3{}, e
	}
	if !finite(t) || t < 0 || t > 1 {
		return geom.Vec3{}, errors.New("iges: arc parameter out of range")
	}
	if t == 0 {
		return geom.Vec3{X: a.Start.X, Y: a.Start.Y, Z: a.Center.Z}, nil
	}
	if t == 1 {
		return geom.Vec3{X: a.End.X, Y: a.End.Y, Z: a.Center.Z}, nil
	}
	p := geom.Vec3{X: a.Center.X + r*math.Cos(start+t*sweep), Y: a.Center.Y + r*math.Sin(start+t*sweep), Z: a.Center.Z}
	if !p.IsFinite() {
		return geom.Vec3{}, errors.New("iges: arc evaluation overflow")
	}
	return p, nil
}
func (s Spline) Validate() error {
	if s.Curve == nil {
		return errors.New("iges: nil spline")
	}
	lo, hi := s.Curve.Domain()
	if !finite(lo) || !finite(hi) || !finite(s.Start) || !finite(s.End) || s.Start < lo || s.End > hi || s.Start >= s.End || !s.Normal.IsFinite() {
		return errors.New("iges: invalid spline interval/normal")
	}
	poles, weights := s.Curve.Poles(), s.Curve.Weights()
	if s.Polynomial {
		for _, w := range weights {
			if w != weights[0] {
				return errors.New("iges: inconsistent polynomial flag")
			}
		}
	}
	if s.Planar {
		n, e := s.Normal.Unit()
		if e != nil || math.Abs(s.Normal.Length()-1) > 1e-8 {
			return errors.New("iges: invalid planar spline normal")
		}
		origin := poles[0]
		scale := 0.
		for _, p := range poles {
			scale = math.Max(scale, p.Sub(origin).Length())
		}
		if !finite(scale) {
			return errors.New("iges: planar spline arithmetic overflow")
		}
		for _, p := range poles {
			d := p.Sub(origin)
			if scale > 0 {
				d = geom.Vec3{X: d.X / scale, Y: d.Y / scale, Z: d.Z / scale}
			}
			// IGES writers may serialize the informational unit normal with
			// fewer digits than the geometry. Use the same angular precision
			// as the unit-length check; trim correspondence remains exact.
			if math.Abs(d.Dot(n)) > 1e-8 {
				return errors.New("iges: nonplanar spline controls")
			}
		}
	}
	return nil
}
func (e Entity) validate() error {
	if e.Transform != nil {
		if _, err := e.Transform.Inverse(); err != nil {
			return errors.New("iges: invalid transformation matrix")
		}
	}
	n := 0
	if e.Composite != nil {
		n++
		if (len(e.Composite.Curves) < 4 || len(e.Composite.Curves) > MaxOrthogonalVertices || len(e.Composite.Curves)%2 != 0) || e.Transform != nil || e.Form != 0 {
			return errors.New("iges: unsupported composite curve")
		}
	}
	if e.Point != nil {
		n++
		if !e.Point.IsFinite() || e.Form != 0 {
			return errors.New("iges: invalid point")
		}
	}
	if e.Line != nil {
		n++
		if !e.Line.Start.IsFinite() || !e.Line.End.IsFinite() || e.Line.Start == e.Line.End || !e.Line.End.Sub(e.Line.Start).IsFinite() {
			return errors.New("iges: invalid line")
		}
		if e.Form != 0 {
			return errors.New("iges: only bounded line segments supported")
		}
	}
	if e.Arc != nil {
		n++
		if e.Form != 0 {
			return errors.New("iges: invalid arc form")
		}
		if _, _, _, err := e.Arc.parameters(); err != nil {
			return err
		}
	}
	if e.Spline != nil {
		n++
		if e.Form < 0 || e.Form > 5 {
			return errors.New("iges: invalid spline form")
		}
		if err := e.Spline.Validate(); err != nil {
			return err
		}
	}
	if e.Surface != nil {
		n++
		if e.Form < 0 || e.Form > 9 {
			return errors.New("iges: invalid surface form")
		}
		if err := e.Surface.Validate(); err != nil {
			return err
		}
	}
	if n != 1 {
		return errors.New("iges: exactly one geometry per entity required")
	}
	if len(e.Label) > 8 || e.Subscript < 0 || e.Subscript > 99999999 {
		return errors.New("iges: invalid directory label/subscript")
	}
	for _, c := range []byte(e.Label) {
		if c < 32 || c > 126 {
			return errors.New("iges: only ASCII directory labels supported")
		}
	}
	return nil
}

// Eval evaluates an entity in model coordinates at a normalized parameter [0,1].
// Points return their constant coordinate throughout [0,1].
// For splines this maps onto the stored trimming interval before transformation.
func (e Entity) Eval(t float64) (geom.Vec3, error) {
	if err := e.validate(); err != nil {
		return geom.Vec3{}, err
	}
	if !finite(t) || t < 0 || t > 1 {
		return geom.Vec3{}, errors.New("iges: entity parameter out of range")
	}
	var p geom.Vec3
	var err error
	switch {
	case e.Point != nil:
		p = *e.Point
	case e.Line != nil:
		p = e.Line.Start.Scale(1 - t).Add(e.Line.End.Scale(t))
	case e.Arc != nil:
		p, err = e.Arc.Eval(t)
	case e.Spline != nil:
		p, err = e.Spline.Curve.Eval(e.Spline.Start*(1-t) + e.Spline.End*t)
	default:
		return geom.Vec3{}, errors.New("iges: use EvalSurface for surfaces")
	}
	if err != nil {
		return geom.Vec3{}, err
	}
	if e.Transform != nil {
		p = e.Transform.Apply(p)
	}
	if !p.IsFinite() {
		return geom.Vec3{}, errors.New("iges: transformed coordinate overflow")
	}
	return p, nil
}
func (e Entity) bounds() (geom.Box, error) {
	var points []geom.Vec3
	switch {
	case e.Point != nil:
		points = []geom.Vec3{*e.Point}
	case e.Line != nil:
		points = []geom.Vec3{e.Line.Start, e.Line.End}
	case e.Arc != nil:
		r, _, _, err := e.Arc.parameters()
		if err != nil {
			return geom.Box{}, err
		}
		c := e.Arc.Center
		points = []geom.Vec3{{X: c.X - r, Y: c.Y - r, Z: c.Z}, {X: c.X + r, Y: c.Y + r, Z: c.Z}}
	case e.Spline != nil:
		points = e.Spline.Curve.Poles()
	case e.Surface != nil:
		points = e.Surface.Geometry.Data().Poles
	}
	b, err := geom.Bounds(points)
	if err != nil {
		return geom.Box{}, err
	}
	if e.Transform != nil {
		return e.Transform.ApplyBox(b)
	}
	return b, nil
}

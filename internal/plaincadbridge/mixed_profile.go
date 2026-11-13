package plaincadbridge

import (
	"context"
	"fmt"
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// mixedProfileRequest describes proved analytic sketch families, never sampled
// arc polygons. Coordinates are in the supplied orthonormal sketch frame.
type mixedProfileRequest struct {
	Min         []number              `json:"min"`
	Max         []number              `json:"max"`
	Radii       []number              `json:"radii"`
	Kind        string                `json:"kind"`
	Origin      *vector               `json:"origin"`
	Normal      *vector               `json:"normal"`
	XDirection  *vector               `json:"xDirection"`
	Center      []number              `json:"center"`
	Right       []number              `json:"right"`
	Radius      number                `json:"radius"`
	StartAngle  number                `json:"startAngle"`
	Sweep       number                `json:"sweep"`
	InnerRadius number                `json:"innerRadius"`
	Height      number                `json:"height"`
	Outer       [][]number            `json:"outer"`
	Circles     []mixedProfileCircle  `json:"circles"`
	Segments    []mixedProfileSegment `json:"segments"`
}
type mixedProfileCircle struct {
	Center []number `json:"center"`
	Radius number   `json:"radius"`
}

type mixedProfileSegment struct {
	Start      []number `json:"start"`
	End        []number `json:"end"`
	Center     []number `json:"center"`
	Radius     number   `json:"radius"`
	StartAngle number   `json:"startAngle"`
	Sweep      number   `json:"sweep"`
}

func mixedPoint(p []number) (geom.Vec2, error) {
	if len(p) != 2 || math.IsNaN(float64(p[0])) || math.IsInf(float64(p[0]), 0) || math.IsNaN(float64(p[1])) || math.IsInf(float64(p[1]), 0) {
		return geom.Vec2{}, geom.ErrInvalid
	}
	return geom.Vec2{X: float64(p[0]), Y: float64(p[1])}, nil
}
func newMixedProfile(q mixedProfileRequest) (trimmed.Solid, float64, float64, error) {
	fail := func(e error) (trimmed.Solid, float64, float64, error) {
		return trimmed.Solid{}, 0, 0, fmt.Errorf("mixed profile extrusion: %w", e)
	}
	if q.Origin == nil || q.Normal == nil || q.XDirection == nil {
		return fail(geom.ErrInvalid)
	}
	if len(q.Circles) > 16 || len(q.Outer) > 64 {
		return fail(&failureError{"LIMIT", "mixed profile exceeds 16 circular holes or 64 polygon vertices"})
	}
	f, e := geom.NewFrame(q.Origin.vec(), q.XDirection.vec(), q.Normal.vec())
	if e != nil {
		return fail(e)
	}
	// The request is a rigid sketch frame, not a skew coordinate transform.
	if f.XAxis() != q.XDirection.vec() || f.ZAxis() != q.Normal.vec() {
		return fail(&failureError{"UNSUPPORTED", "mixed profile requires a represented orthonormal frame"})
	}
	holes := make([]trimmed.ExtrusionCircle, len(q.Circles))
	for i, c := range q.Circles {
		p, e := mixedPoint(c.Center)
		if e != nil {
			return fail(e)
		}
		holes[i] = trimmed.ExtrusionCircle{Center: p, Radius: float64(c.Radius)}
	}
	h, r := float64(q.Height), float64(q.Radius)
	if q.Kind != "capsule" && q.InnerRadius != 0 || q.Kind != "circularSegment" && (q.StartAngle != 0 || q.Sweep != 0) {
		return fail(geom.ErrDomain)
	}
	if q.Kind != "roundedRectangle" && (len(q.Min) != 0 || len(q.Max) != 0 || len(q.Radii) != 0) {
		return fail(geom.ErrDomain)
	}
	var s trimmed.Solid
	var area, perimeter float64
	switch q.Kind {
	case "roundedRectangle":
		if len(q.Center) != 0 || len(q.Right) != 0 || len(q.Outer) != 0 || r != 0 || len(q.Radii) != 4 {
			return fail(geom.ErrDomain)
		}
		minimum, err := mixedPoint(q.Min)
		if err != nil {
			return fail(err)
		}
		maximum, err := mixedPoint(q.Max)
		if err != nil {
			return fail(err)
		}
		profile := trimmed.RoundedRectangle{Min: minimum, Max: maximum}
		area = (maximum.X - minimum.X) * (maximum.Y - minimum.Y)
		perimeter = 2 * ((maximum.X - minimum.X) + (maximum.Y - minimum.Y))
		for i, v := range q.Radii {
			radius := float64(v)
			profile.Radii[i] = radius
			area -= (1 - math.Pi/4) * radius * radius
			perimeter += (math.Pi/2 - 2) * radius
		}
		s, e = trimmed.ExtrudeRoundedRectangle(context.Background(), f, profile, holes, nil, h)
	case "semicircle", "circularSegment", "capsule", "disk":
		if len(q.Outer) != 0 || (q.Kind != "disk" && len(holes) != 0) || (q.Kind != "capsule" && len(q.Right) != 0) {
			return fail(geom.ErrDomain)
		}
		c, pointErr := mixedPoint(q.Center)
		if pointErr != nil {
			return fail(pointErr)
		}
		switch q.Kind {
		case "semicircle":
			s, e = trimmed.ExtrudeSemicircle(context.Background(), f, c, r, h)
			area = math.Pi * r * r / 2
			perimeter = (math.Pi + 2) * r
		case "circularSegment":
			sweep := math.Abs(float64(q.Sweep))
			s, e = trimmed.ExtrudeCircularSegment(context.Background(), f, c, r, float64(q.StartAngle), float64(q.Sweep), h)
			area = r * r * (sweep - math.Sin(sweep)) / 2
			perimeter = r * (sweep + 2*math.Sin(sweep/2))
		case "capsule":
			right, err := mixedPoint(q.Right)
			if err != nil {
				return fail(err)
			}
			if q.InnerRadius == 0 {
				s, e = trimmed.ExtrudeCapsule(context.Background(), f, c, right, r, h)
			} else {
				s, e = trimmed.ExtrudeCapsuleRing(context.Background(), f, c, right, r, float64(q.InnerRadius), h)
			}
			d := right.X - c.X
			area = math.Pi*r*r + 2*r*d
			perimeter = 2*math.Pi*r + 2*d
			if q.InnerRadius != 0 {
				inner := float64(q.InnerRadius)
				area -= math.Pi*inner*inner + 2*inner*d
				perimeter += 2*math.Pi*inner + 2*d
			}
		case "disk":
			s, e = trimmed.ExtrudeDiskWithHoles(context.Background(), f, trimmed.ExtrusionCircle{Center: c, Radius: r}, holes, nil, h)
			area = math.Pi * r * r
			perimeter = 2 * math.Pi * r
		}
	case "polygon":
		if len(q.Center) != 0 || len(q.Right) != 0 || r != 0 {
			return fail(geom.ErrDomain)
		}
		p := make([]geom.Vec2, len(q.Outer))
		for i, v := range q.Outer {
			p[i], e = mixedPoint(v)
			if e != nil {
				return fail(e)
			}
		}
		s, e = trimmed.ExtrudePolygonWithHoles(context.Background(), f, p, holes, nil, h)
		if len(p) > 0 {
			anchor := p[0]
			for i, a := range p {
				b := p[(i+1)%len(p)]
				area += (a.X-anchor.X)*(b.Y-anchor.Y) - (a.Y-anchor.Y)*(b.X-anchor.X)
				perimeter += math.Hypot(b.X-a.X, b.Y-a.Y)
			}
			area = math.Abs(area) / 2
		}
	default:
		return fail(&failureError{"UNSUPPORTED", "unknown analytic mixed profile family"})
	}
	if e != nil {
		return fail(e)
	}
	for _, hole := range holes {
		area -= math.Pi * hole.Radius * hole.Radius
		perimeter += 2 * math.Pi * hole.Radius
	}
	volume, surface := area*math.Abs(h), 2*area+perimeter*math.Abs(h)
	if volume <= 0 || surface <= 0 || math.IsInf(volume, 0) || math.IsInf(surface, 0) || math.IsNaN(volume) || math.IsNaN(surface) {
		return fail(geom.ErrInvalid)
	}
	return s, volume, surface, nil
}

// The support function uses the actual arc and chord rather than the
// complete cylindrical support. The frame vectors follow rigid placement.
type circularSegmentSweep struct {
	origin, x, y, z geom.Vec3
	radius, height  float64
	start, sweep    float64
}

func (s circularSegmentSweep) bounds() (geom.Box, error) {
	start, end := s.start, s.start+s.sweep
	loAngle, hiAngle := min(start, end), max(start, end)
	maximum := func(x, y float64) float64 {
		angle := math.Atan2(y, x)
		angle += 2 * math.Pi * math.Ceil((loAngle-angle)/(2*math.Pi))
		if angle <= hiAngle {
			return s.radius * math.Hypot(x, y)
		}
		if start == 0 && s.sweep == math.Pi {
			return s.radius * math.Abs(x)
		}
		return s.radius * max(x*math.Cos(start)+y*math.Sin(start), x*math.Cos(end)+y*math.Sin(end))
	}
	o, x, y, z := coordinates(s.origin), coordinates(s.x), coordinates(s.y), coordinates(s.z)
	var lo, hi vector
	for i := range o {
		span := z[i] * s.height
		lo[i] = o[i] - maximum(-x[i], -y[i]) + min(0, span)
		hi[i] = o[i] + maximum(x[i], y[i]) + max(0, span)
	}
	return geom.Bounds([]geom.Vec3{lo.vec(), hi.vec()})
}

func (b *Bridge) mixedExtrude(p mixedProfileRequest) (Shape, error) {
	if p.Kind == "arcLoop" {
		return b.extrudeArcLoop(p)
	}
	if len(p.Segments) != 0 {
		return Shape{}, geom.ErrDomain
	}
	s, volume, area, err := newMixedProfile(p)
	if err != nil {
		return Shape{}, err
	}
	body := &analyticBody{solid: s, volume: volume, area: area}
	if p.Kind == "polygon" {
		// The constructor proved each circular hole strictly inside this
		// straight convex outer wire. Every support extreme is therefore at
		// one of the actual outer vertices on either extrusion endpoint.
		frame, err := geom.NewFrame(p.Origin.vec(), p.XDirection.vec(), p.Normal.vec())
		if err != nil {
			return Shape{}, err
		}
		vertices := s.Vertices()
		for _, xy := range p.Outer {
			point, err := mixedPoint(xy)
			if err != nil {
				return Shape{}, err
			}
			for _, z := range []float64{0, float64(p.Height)} {
				v := frame.ToWorld(geom.Vec3{X: point.X, Y: point.Y, Z: z})
				retained := false
				for _, actual := range vertices {
					if actual == v {
						retained = true
						break
					}
				}
				if !retained {
					return Shape{}, fmt.Errorf("polygon outer extremum is not retained: %w", geom.ErrDomain)
				}
				body.polygonExtrema = append(body.polygonExtrema, v)
			}
		}
	}
	if p.Kind == "roundedRectangle" && len(p.Circles) == 0 {
		frame, err := geom.NewFrame(p.Origin.vec(), p.XDirection.vec(), p.Normal.vec())
		if err != nil {
			return Shape{}, err
		}
		lo, _ := mixedPoint(p.Min)
		hi, _ := mixedPoint(p.Max)
		profile := trimmed.RoundedRectangle{Min: lo, Max: hi}
		for i, radius := range p.Radii {
			profile.Radii[i] = float64(radius)
		}
		body.rounded = &roundedStock{frame: frame, profile: profile, height: float64(p.Height)}
	}
	if p.Kind == "disk" {
		// newMixedProfile validated this exact frame and every loop above.
		f, _ := geom.NewFrame(p.Origin.vec(), p.XDirection.vec(), p.Normal.vec())
		center, _ := mixedPoint(p.Center)
		height, radius := float64(p.Height), float64(p.Radius)
		disk := &step.DiskHoleExtrusionBody{
			Name: "PlainSolid", Frame: f, Height: height,
			Outer: trimmed.ExtrusionCircle{Center: center, Radius: radius},
			Holes: make([]trimmed.ExtrusionCircle, len(p.Circles)),
		}
		for i, circle := range p.Circles {
			hole, _ := mixedPoint(circle.Center)
			disk.Holes[i] = trimmed.ExtrusionCircle{Center: hole, Radius: float64(circle.Radius)}
		}
		body.stepDisk = disk
		if len(disk.Holes) == 1 && center == disk.Holes[0].Center {
			origin := f.ToWorld(geom.Vec3{X: center.X, Y: center.Y, Z: min(0, height)})
			frame, err := geom.NewFrame(origin, f.XAxis(), f.ZAxis())
			if err != nil {
				return Shape{}, err
			}
			body.sector = &radialSector{frame: frame, inner: disk.Holes[0].Radius,
				radius: radius, height: math.Abs(height), angle: 2 * math.Pi}
		}
	}
	if p.Kind == "polygon" && len(p.Circles) == 1 {
		body.threadStock = rectangularThreadStock(p)
	}
	if p.Kind == "semicircle" || p.Kind == "circularSegment" {
		f, err := geom.NewFrame(p.Origin.vec(), p.XDirection.vec(), p.Normal.vec())
		if err != nil {
			return Shape{}, err
		}
		center, err := mixedPoint(p.Center)
		if err != nil {
			return Shape{}, err
		}
		start, sweep := math.Remainder(float64(p.StartAngle), 2*math.Pi), float64(p.Sweep)
		if p.Kind == "semicircle" {
			sweep = math.Pi
		}
		body.segment = &circularSegmentSweep{
			f.ToWorld(geom.Vec3{X: center.X, Y: center.Y}), f.XAxis(), f.YAxis(), f.ZAxis(),
			float64(p.Radius), float64(p.Height), start, sweep,
		}
	}
	return b.putAnalyticBody(body)
}

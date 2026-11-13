package plaincadbridge

import (
	"errors"
	"math"
	"strconv"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// cylinderBody retains an analytic closed cylinder, not its display triangles.
// Frame.ZAxis points in the extrusion direction and height is positive.
type cylinderBody struct {
	solid          parametric.Solid
	frame          geom.Frame
	radius, height float64
}

func (b *Bridge) circle(origin, normal geom.Vec3, radius, height float64) (Shape, error) {
	if radius <= 1e-7 || height == 0 || !finite(radius) || !finite(height) {
		return Shape{}, errors.New("circle requires radius greater than 1e-7 and nonzero finite height")
	}
	if height < 0 {
		normal = normal.Scale(-1)
		height = -height
	}
	frame, err := geom.FrameFromNormal(origin, normal)
	if err != nil {
		return Shape{}, err
	}
	c, err := newCylinder(frame, radius, height)
	if err != nil {
		return Shape{}, err
	}
	return b.putCylinder(c)
}
func newCylinder(frame geom.Frame, radius, height float64) (*cylinderBody, error) {
	s, err := parametric.Cylinder(frame, radius, height)
	if err != nil {
		return nil, err
	}
	return &cylinderBody{s, frame, radius, height}, nil
}
func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
func (b *Bridge) publish(s stored) (Shape, error) {
	if len(b.shapes) >= MaxHandles {
		return Shape{}, &failureError{"LIMIT", "dispose shapes before allocating more than 256 handles"}
	}
	b.next++
	s.result.Handle = "ps:" + strconv.FormatUint(b.next, 10)
	b.shapes[s.result.Handle] = s
	return s.result, nil
}
func (b *Bridge) putCylinder(c *cylinderBody) (Shape, error) {
	bounds, err := c.solid.Bounds()
	if err != nil {
		return Shape{}, err
	}
	// These measures are analytic properties of the retained circle and axial interval.
	volume := math.Pi * c.radius * c.radius * c.height
	area := 2 * math.Pi * c.radius * (c.radius + c.height)
	if !finite(volume) || !finite(area) || volume <= 0 {
		return Shape{}, errors.New("cylinder measures exceed numeric range")
	}
	return b.publish(stored{cylinder: c, result: Shape{GeometryAssertions: Assertions{true, volume, area, 1}, Bounds: Bounds{coordinates(bounds.Min), coordinates(bounds.Max)}}})
}

type optionalNumber struct {
	value number
	set   bool
}

func (n *optionalNumber) UnmarshalJSON(data []byte) error {
	if err := n.value.UnmarshalJSON(data); err != nil {
		return err
	}
	n.set = true
	return nil
}
func tessellationOptions(q request) (parametric.TessellationOptions, error) {
	d, a := .05, .2
	if q.LinearDeflection.set {
		d = float64(q.LinearDeflection.value)
	}
	if q.AngularDeflection.set {
		a = float64(q.AngularDeflection.value)
	}
	if !finite(d) || d <= 0 || !finite(a) || a <= 0 || a > math.Pi {
		return parametric.TessellationOptions{}, errors.New("mesh requires positive deflection and angular deflection at most pi")
	}
	// PlainCAD's angular deflection limits normal turning across a facet.
	// The kernel option limits sampled normals relative to the facet normal.
	// Half the requested turn bounds any pair of those sampled normals by the
	// triangle inequality. This remains sampled, not a surface-wide certificate.
	facetAngle := a / 2
	if facetAngle == 0 {
		return parametric.TessellationOptions{}, parametric.ErrBudget
	}
	if facetAngle*2 > a {
		facetAngle = math.Nextafter(facetAngle, 0)
	}
	return parametric.TessellationOptions{Deflection: d, Angle: facetAngle, MaxSubdivisions: 256, MaxTriangles: 1000000}, nil
}
func (s stored) tessellate(q request) (mesh.Mesh, error) {
	o, err := tessellationOptions(q)
	if err != nil {
		return mesh.Mesh{}, err
	}
	if s.analytic != nil {
		m, err := s.analytic.tessellate(o)
		if err == nil && s.analytic.helical && o.Deflection == .5 && o.Angle == .1 && s.analytic.helicalMesh == nil {
			s.analytic.helicalMesh = &m
		}
		return m, err
	}
	if s.treated != nil {
		return s.treated.tessellate(o)
	}
	if s.bore != nil {
		return s.tessellateBore(o)
	}
	if s.cylinder != nil {
		// The compact axial mesh keeps circular volume accurate without
		// subdividing the straight generators and the planar cap interiors.
		o.Angle = min(o.Angle, .025)
		c := s.cylinder
		p := coaxialBody{frame: c.frame, spans: []coaxialSpan{{0, c.height, c.radius}}}
		return p.tessellate(o)
	}
	return s.solid.Tessellate(), nil
}
func rigid(m geom.Affine) error {
	if _, err := m.Inverse(); err != nil {
		return err
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			dot := 0.
			for k := 0; k < 3; k++ {
				dot += m[k][i] * m[k][j]
			}
			want := 0.
			if i == j {
				want = 1
			}
			if !finite(dot) || math.Abs(dot-want) > 1e-12 {
				return &failureError{"UNSUPPORTED", "curved shape placement requires a rigid transform"}
			}
		}
	}
	return nil
}
func (c *cylinderBody) transformed(m geom.Affine) (*cylinderBody, error) {
	if err := rigid(m); err != nil {
		return nil, err
	}
	f, err := geom.NewFrame(m.Apply(c.frame.Origin()), m.ApplyVector(c.frame.XAxis()), m.ApplyVector(c.frame.ZAxis()))
	if err != nil {
		return nil, err
	}
	return newCylinder(f, c.radius, c.height)
}
func (c *cylinderBody) planarFace(origin, normal geom.Vec3) (PlanarFace, error) {
	n, err := normal.Unit()
	if err != nil || !origin.IsFinite() {
		return PlanarFace{}, errors.New("invalid planar reference")
	}
	found := -1
	var f geom.Frame
	outward := geom.Vec3{}
	for i, face := range c.solid.Faces() {
		frame, ok := parametric.PolarPlaneFrame(face.Surface)
		if !ok {
			continue
		}
		if math.Abs(math.Abs(frame.ZAxis().Dot(n))-1) > 1e-9 || math.Abs(frame.Origin().Sub(origin).Dot(frame.ZAxis())) > 1e-7 {
			continue
		}
		if found >= 0 {
			return PlanarFace{}, errors.New("planar face reference is ambiguous")
		}
		found, f = i, frame
		outward = frame.ZAxis().Scale(-1)
		if face.Reversed {
			outward = frame.ZAxis()
		}
	}
	if found < 0 {
		return PlanarFace{}, errors.New("planar face reference no longer exists")
	}
	area := math.Pi * c.radius * c.radius
	if area <= 1e-14 {
		return PlanarFace{}, errors.New("planar face has no measurable area")
	}
	return PlanarFace{found, coordinates(f.Origin()), coordinates(outward), area}, nil
}

package parametric

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

var circleDomain = Interval{0, 2 * math.Pi}
var normalized = Interval{0, 1}

type patch struct {
	face      Face
	corners   [4]int
	collapsed [4]bool
}

func build(vertices []geom.Vec3, size float64, patches ...patch) (Solid, error) {
	tolerance := size * 1e-12
	if tolerance == 0 || !finite(tolerance) {
		return Solid{}, fmt.Errorf("parametric: primitive scale outside supported range")
	}
	return buildPatches(vertices, tolerance, patches...)
}

func buildPatches(vertices []geom.Vec3, tolerance float64, patches ...patch) (Solid, error) {
	faces := make([]Face, len(patches))
	var edges []Edge
	for i, p := range patches {
		faces[i] = p.face
		for j, use := range p.face.Boundary {
			if use.Edge < len(edges) {
				continue
			}
			if use.Edge != len(edges) || use.Reversed {
				return Solid{}, fmt.Errorf("parametric: invalid primitive edge layout")
			}
			edge := Edge{Start: p.corners[j], End: p.corners[(j+1)%4], Range: normalized}
			if !p.collapsed[j] {
				edge.Curve = iso{p.face, j}
			}
			edges = append(edges, edge)
		}
	}
	return NewSolid(vertices, edges, faces, tolerance)
}

// Sphere constructs a latitude-longitude spherical boundary with one shared
// seam and collapsed edges at the two poles. The exact surface is retained.
func Sphere(center geom.Vec3, radius float64) (Solid, error) {
	frame, err := geom.FrameFromNormal(center, geom.Vec3{Z: 1})
	if err != nil {
		return Solid{}, err
	}
	surface, err := geom.NewSphereSurface(frame, radius)
	if err != nil {
		return Solid{}, err
	}
	return build([]geom.Vec3{center.Add(geom.Vec3{Z: -radius}), center.Add(geom.Vec3{Z: radius})}, radius,
		patch{Face{Surface: surface, U: circleDomain, V: Interval{-math.Pi / 2, math.Pi / 2},
			Boundary: [4]Use{{0, false}, {1, false}, {2, false}, {1, true}}},
			[4]int{0, 0, 1, 1}, [4]bool{true, false, true, false}})
}

// Torus constructs a ring torus from one surface with two periodic seams.
func Torus(frame geom.Frame, major, minor float64) (Solid, error) {
	surface, err := geom.NewTorusSurface(frame, major, minor)
	if err != nil {
		return Solid{}, err
	}
	v, err := surface.Eval(0, 0)
	if err != nil {
		return Solid{}, err
	}
	return build([]geom.Vec3{v}, major+minor, patch{
		face: Face{Surface: surface, U: circleDomain, V: circleDomain, Boundary: [4]Use{{0, false}, {1, false}, {0, true}, {1, true}}}})
}

// Cylinder constructs a cylinder along the frame's positive Z axis. The frame
// origin is the center of the bottom cap.
func Cylinder(frame geom.Frame, radius, height float64) (Solid, error) {
	surface, err := geom.NewCylinderSurface(frame, radius)
	if err != nil {
		return Solid{}, err
	}
	if height <= 0 || !finite(height) {
		return Solid{}, geom.ErrInvalid
	}
	bottom, top := frame.Origin(), frame.ToWorld(geom.Vec3{Z: height})
	topFrame, err := geom.NewFrame(top, frame.XAxis(), frame.ZAxis())
	if err != nil {
		return Solid{}, err
	}
	return build([]geom.Vec3{frame.ToWorld(geom.Vec3{X: radius}), frame.ToWorld(geom.Vec3{X: radius, Z: height}), bottom, top}, math.Max(radius, height),
		patch{face: Face{Surface: surface, U: circleDomain, V: Interval{0, height}, Boundary: [4]Use{{0, false}, {1, false}, {2, false}, {1, true}}}, corners: [4]int{0, 0, 1, 1}},
		patch{Face{Surface: disk{frame}, U: circleDomain, V: Interval{0, radius}, Boundary: [4]Use{{3, false}, {4, false}, {0, true}, {4, true}}},
			[4]int{2, 2, 0, 0}, [4]bool{true, false, false, false}},
		patch{Face{Surface: disk{topFrame}, U: circleDomain, V: Interval{0, radius}, Boundary: [4]Use{{5, false}, {6, false}, {2, false}, {6, true}}, Reversed: true},
			[4]int{3, 3, 1, 1}, [4]bool{true, false, false, false}})
}

// Cone constructs a cone whose apex is the frame origin and whose base is
// height units along +Z. HalfAngle is in radians.
func Cone(frame geom.Frame, halfAngle, height float64) (Solid, error) {
	surface, err := geom.NewConeSurface(frame, halfAngle)
	if err != nil {
		return Solid{}, err
	}
	if height <= 0 || !finite(height) {
		return Solid{}, geom.ErrInvalid
	}
	radius := height * math.Tan(halfAngle)
	if radius <= 0 || !finite(radius) {
		return Solid{}, geom.ErrInvalid
	}
	top := frame.ToWorld(geom.Vec3{Z: height})
	topFrame, err := geom.NewFrame(top, frame.XAxis(), frame.ZAxis())
	if err != nil {
		return Solid{}, err
	}
	return build([]geom.Vec3{frame.Origin(), frame.ToWorld(geom.Vec3{X: radius, Z: height}), top}, math.Max(radius, height),
		patch{Face{Surface: surface, U: circleDomain, V: Interval{0, height}, Boundary: [4]Use{{0, false}, {1, false}, {2, false}, {1, true}}},
			[4]int{0, 0, 1, 1}, [4]bool{true, false, false, false}},
		patch{Face{Surface: disk{topFrame}, U: circleDomain, V: Interval{0, radius}, Boundary: [4]Use{{3, false}, {4, false}, {2, false}, {4, true}}, Reversed: true},
			[4]int{2, 2, 1, 1}, [4]bool{true, false, false, false}})
}

// disk is a polar parameterization of a planar cap. du × dv points along -Z.
type disk struct{ frame geom.Frame }

// PolarPlaneFrame returns the frame of the built-in polar plane chart
// (angle, radius). Its chart normal points along -Z.
func PolarPlaneFrame(s geom.Surface) (geom.Frame, bool) {
	d, ok := s.(disk)
	return d.frame, ok
}

func (d disk) Eval(u, v float64) (geom.Vec3, error) {
	if !finite(u) || !finite(v) || v < 0 {
		return geom.Vec3{}, geom.ErrDomain
	}
	p := d.frame.ToWorld(geom.Vec3{X: v * math.Cos(u), Y: v * math.Sin(u)})
	if !p.IsFinite() {
		return geom.Vec3{}, geom.ErrInvalid
	}
	return p, nil
}
func (d disk) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	if _, err := d.Eval(u, v); err != nil {
		return geom.Vec3{}, geom.Vec3{}, err
	}
	x, y := d.frame.XAxis(), d.frame.YAxis()
	return x.Scale(-v * math.Sin(u)).Add(y.Scale(v * math.Cos(u))), x.Scale(math.Cos(u)).Add(y.Scale(math.Sin(u))), nil
}

// TorusElbow sweeps a circular section along a circular arc, with planar
// end caps. The arc starts along the frame's X axis and advances toward Y.
// Angle must be strictly between zero and 2*pi radians.
func TorusElbow(frame geom.Frame, major, minor, angle float64) (Solid, error) {
	if angle <= 0 || angle >= 2*math.Pi || !finite(angle) {
		return Solid{}, geom.ErrInvalid
	}
	surface, err := geom.NewTorusSurface(frame, major, minor)
	if err != nil {
		return Solid{}, err
	}
	if math.Min(angle, 2*math.Pi-angle)*major <= 1e-11*(major+minor) {
		return Solid{}, fmt.Errorf("parametric: unresolved elbow ends")
	}
	domain := Interval{0, angle}
	capFrame := func(u float64) (geom.Frame, error) {
		radial := frame.XAxis().Scale(math.Cos(u)).Add(frame.YAxis().Scale(math.Sin(u)))
		return geom.NewFrame(frame.Origin().Add(radial.Scale(major)), radial, radial.Cross(frame.ZAxis()))
	}
	bottom, err := capFrame(0)
	if err != nil {
		return Solid{}, err
	}
	top, err := capFrame(angle)
	if err != nil {
		return Solid{}, err
	}
	start, err := surface.Eval(0, 0)
	if err != nil {
		return Solid{}, err
	}
	end, err := surface.Eval(angle, 0)
	if err != nil {
		return Solid{}, err
	}
	return build([]geom.Vec3{start, end, bottom.Origin(), top.Origin()}, major+minor,
		patch{face: Face{Surface: surface, U: domain, V: circleDomain, Boundary: [4]Use{{0, false}, {1, false}, {0, true}, {2, false}}}, corners: [4]int{0, 1, 1, 0}},
		patch{Face{Surface: disk{bottom}, U: circleDomain, V: Interval{0, minor}, Boundary: [4]Use{{3, false}, {4, false}, {2, false}, {4, true}}, Reversed: true},
			[4]int{2, 2, 0, 0}, [4]bool{true, false, false, false}},
		patch{Face{Surface: disk{top}, U: circleDomain, V: Interval{0, minor}, Boundary: [4]Use{{5, false}, {6, false}, {1, true}, {6, true}}},
			[4]int{3, 3, 1, 1}, [4]bool{true, false, false, false}})
}

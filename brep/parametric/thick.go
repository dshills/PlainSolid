package parametric

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

// Tube is a concentric thick-walled cylinder with planar annular end caps.
func Tube(frame geom.Frame, outerRadius, height, thickness float64) (Solid, error) {
	inner := outerRadius - thickness
	if !finite(outerRadius) || !finite(height) || !finite(thickness) || inner <= 0 || inner >= outerRadius || height <= 0 {
		return Solid{}, geom.ErrInvalid
	}
	return CombineParallelCylinders(frame, DiskProfile{Radius: outerRadius}, DiskProfile{Radius: inner}, height, Difference, math.Max(height, outerRadius)*1e-12)
}

// HollowTorus constructs a closed toroidal shell with constant wall thickness,
// measured in the minor radial direction. Its two boundaries have opposite
// orientations; the inner cavity remains closed.
func HollowTorus(frame geom.Frame, major, minor, thickness float64) (Solid, error) {
	if !finite(thickness) || thickness <= 0 || minor-thickness <= 0 || minor-thickness >= minor {
		return Solid{}, geom.ErrInvalid
	}
	out, e := Torus(frame, major, minor)
	if e != nil {
		return Solid{}, e
	}
	in, e := Torus(frame, major, minor-thickness)
	if e != nil {
		return Solid{}, e
	}
	if thickness <= math.Max(out.tolerance, in.tolerance) {
		return Solid{}, fmt.Errorf("parametric: unresolved toroidal wall thickness")
	}
	for i := range in.faces {
		in.faces[i].Reversed = !in.faces[i].Reversed
	}
	return joinIndependent(out, in)
}

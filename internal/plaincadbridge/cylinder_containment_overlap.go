package plaincadbridge

import (
	"errors"
	"plainsolid/brep"
)

// containedPlanarCylinderVolume proves full planar material containment before
// reporting its volume. A convex cylinder contains the convex hull of all
// boundary vertices, hence every point of a validated bounded planar solid.
// The existing cardinal-frame predicate checks represented radial squares and
// both axial caps exactly. No tolerance or display sample decides containment.
func containedPlanarCylinderVolume(c *cylinderBody, s brep.Solid) (float64, bool, error) {
	if c == nil || !finite(c.radius) || !finite(c.height) || c.radius <= 0 || c.height <= 0 {
		return 0, false, errors.New("invalid containing cylinder")
	}
	if len(s.Vertices()) == 0 || len(s.Faces()) == 0 {
		return 0, false, errors.New("containment overlap requires nonempty planar material")
	}
	if e := s.Validate(); e != nil {
		return 0, false, e
	}
	if !cylinderContainsPlanar(c, s) {
		return 0, false, nil
	}
	p, e := s.MassProperties()
	if e != nil {
		return 0, false, e
	}
	if !finite(p.Volume) || p.Volume <= 0 {
		return 0, false, errors.New("containment overlap requires positive planar volume")
	}
	return p.Volume, true, nil
}

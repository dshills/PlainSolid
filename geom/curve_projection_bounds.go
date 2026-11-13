package geom

// ProjectionBounds returns the outward bounds of the exact curve over its
// original parameter interval. It uses its immutable positive control hull.
func (c *SurfaceIsoCurve) ProjectionBounds() (Box, error) {
	if c == nil || c.patch == nil {
		return Box{}, ErrInvalid
	}
	return c.patch.Bounds()
}

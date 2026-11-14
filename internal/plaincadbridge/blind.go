package plaincadbridge

import (
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func stopBore(s trimmed.Solid, h trimmed.BoreHistory, axis geom.Vec3, delta float64) (trimmed.Solid, int, error) {
	return trimmed.StopPerpendicularBore(s, h, axis, delta)
}

package plaincadbridge

import (
	"plainsolid/brep/blend"
	"plainsolid/geom"
)

// This file is the adapter of the edge treatment of a general Boolean
// result (specs/general-edge-treatment). Only a general result takes
// this path; every other stock is treated as before.

// generalEdgeTolerance holds the tolerances of a reference: the bridge's
// point tolerance (nearPoint) and the angular tolerance of the general
// Boolean.
var generalEdgeTolerance = blend.Tolerance{Point: 1e-7, Angular: generalOptions.Angular}

// generalEdgeAvailable is edgeAvailable on a general result (R11): the
// reference resolves to one chain of straight edges between two planes
// that are not parallel.
func generalEdgeAvailable(s stored, start, end geom.Vec3) bool {
	solid, err := generalOperand(s)
	if err != nil {
		return false
	}
	_, err = blend.Locate(solid, start, end, generalEdgeTolerance)
	return err == nil
}

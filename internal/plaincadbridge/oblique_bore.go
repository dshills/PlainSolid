package plaincadbridge

import (
	"math"
	"math/big"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type obliqueBore struct {
	removedVolume, wallArea float64
	openingAreas            [2]float64
}

// newObliqueThroughBore handles one through cutter on an exact axis-aligned
// rectangular stock. The slab intersections locate the actual opening faces.
// BoreCylinderAlong then proves both full ellipses are strictly interior to
// those faces and constructs the closed analytic boundary.
func newObliqueThroughBore(base brep.Solid, tool *cylinderBody) (*boreBody, bool, error) {
	if tool == nil {
		return nil, false, nil
	}
	direction := tool.frame.ZAxis()
	nonzero := 0
	for _, component := range coordinates(direction) {
		if component != 0 {
			nonzero++
		}
	}
	if nonzero < 2 {
		return nil, false, nil
	}
	frame, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		return nil, false, err
	}
	if _, ok := orthogonalPrism(base, frame); !ok {
		return nil, false, nil
	}
	fail := func(message string) (*boreBody, bool, error) {
		return nil, true, &failureError{"UNSUPPORTED", message}
	}

	axis, err := direction.Unit()
	if err != nil {
		return nil, true, err
	}
	var entry, exit geom.Plane
	cut := trimmed.CylinderCut{Entry: -1, Exit: -1, AxisPoint: tool.frame.Origin(), Direction: direction, Radius: tool.radius}
	start, end := math.Inf(-1), math.Inf(1)
	for i := range base.Faces() {
		plane, err := base.FacePlane(i)
		if err != nil {
			return nil, true, err
		}
		denominator := plane.Normal().Dot(axis)
		if denominator == 0 {
			if plane.SignedDistance(cut.AxisPoint) >= 0 {
				return fail("oblique cylinder axis does not cross the stock interior")
			}
			continue
		}
		t := -plane.SignedDistance(cut.AxisPoint) / denominator
		if !finite(t) {
			return fail("oblique cylinder support intersection exceeds numeric range")
		}
		if denominator < 0 && t > start {
			cut.Entry, entry, start = i, plane, t
		} else if denominator > 0 && t < end {
			cut.Exit, exit, end = i, plane, t
		}
	}

	length := end - start
	if cut.Entry < 0 || cut.Exit < 0 || !finite(length) || length <= 0 {
		return fail("oblique cylinder axis does not cross the stock interior")
	}
	if !toolCapClearsOpening(tool, entry, 0) || !toolCapClearsOpening(tool, exit, tool.height) {
		return fail("finite oblique cylinder does not cover both entire stock openings")
	}
	body, err := buildObliqueThroughBore(base, tool, cut, axis, entry, exit, length)
	return body, true, err
}

func buildObliqueThroughBore(base brep.Solid, tool *cylinderBody, cut trimmed.CylinderCut, axis geom.Vec3, entry, exit geom.Plane, length float64) (*boreBody, error) {
	solid, _, err := trimmed.BoreCylinderAlong(base, cut.Entry, cut.Exit, cut.AxisPoint, cut.Direction, cut.Radius, 1e-9)
	if err != nil {
		return nil, unsupportedEdge("oblique through bore requires strictly interior elliptical openings: " + err.Error())
	}

	diskArea := math.Pi * tool.radius * tool.radius
	proof := &obliqueBore{
		removedVolume: diskArea * length,
		wallArea:      2 * math.Pi * tool.radius * length,
		openingAreas: [2]float64{
			diskArea / math.Abs(entry.Normal().Dot(axis)),
			diskArea / math.Abs(exit.Normal().Dot(axis)),
		},
	}
	if !finite(proof.removedVolume) || !finite(proof.wallArea) ||
		!finite(proof.openingAreas[0]) || !finite(proof.openingAreas[1]) {
		return nil, unsupportedEdge("oblique cylinder measures exceed numeric range")
	}
	return &boreBody{
		solid: solid, base: base, cylinders: []*cylinderBody{tool}, cuts: []trimmed.CylinderCut{cut},
		spans: []float64{length}, blind: []bool{false}, oblique: proof,
	}, nil
}

// toolCapClearsOpening proves the entire finite tool cap lies in the outer
// halfspace of the stock face. The signed center distance must be at least
// the projected disk support. Square only after proving a nonnegative center
// distance, and compare exact rationals of the represented frame to avoid
// rounding a missing generator into a through cut.
func toolCapClearsOpening(tool *cylinderBody, plane geom.Plane, station float64) bool {
	normal := coordinates(plane.Normal())
	toolOrigin, planeOrigin := coordinates(tool.frame.Origin()), coordinates(plane.Origin())
	axis, xAxis, yAxis := coordinates(tool.frame.ZAxis()), coordinates(tool.frame.XAxis()), coordinates(tool.frame.YAxis())
	rat := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	var center, x, y big.Rat
	for i, n := range normal {
		coordinate := new(big.Rat).Mul(rat(axis[i]), rat(station))
		coordinate.Add(coordinate, rat(toolOrigin[i]))
		coordinate.Sub(coordinate, rat(planeOrigin[i]))
		center.Add(&center, coordinate.Mul(coordinate, rat(n)))
		x.Add(&x, new(big.Rat).Mul(rat(xAxis[i]), rat(n)))
		y.Add(&y, new(big.Rat).Mul(rat(yAxis[i]), rat(n)))
	}
	if center.Sign() < 0 {
		return false
	}
	projectedSupportSquared := new(big.Rat).Add(x.Mul(&x, &x), y.Mul(&y, &y))
	projectedSupportSquared.Mul(projectedSupportSquared, new(big.Rat).Mul(rat(tool.radius), rat(tool.radius)))
	return center.Mul(&center, &center).Cmp(projectedSupportSquared) >= 0
}

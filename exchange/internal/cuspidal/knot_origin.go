package cuspidal

import (
	"math/big"
	"slices"

	"plainsolid/brep/trimmed"
)

// KnotOrigin names the native cusp and both of its original roof owners.
type KnotOrigin struct {
	Vertex           int
	RoofFaces, Germs [2]int
}

// HasSharedLongitudinalKnot checks both exact roof pcurves in the boundary
// proof. The cusp must lie on an interior C0 knot of each original chart,
// with both of its outgoing germs entering the same adjacent cell of each.
func (o KnotOrigin) HasSharedLongitudinalKnot(b trimmed.ExactBezierBoundary) bool {
	for _, roof := range o.RoofFaces {
		f := b.Faces[roof]
		if f.Spline == nil || len(f.Loops) != 1 {
			return false
		}
		loop := f.Loops[0]
		var starts [2][2]*big.Rat
		var sides [2][2]int
		for i, germ := range o.Germs {
			use := slices.IndexFunc(loop, func(u trimmed.ExactBezierBoundaryUse) bool { return u.Edge == germ })
			if use < 0 {
				return false
			}
			control := loop[use].UV.Control
			if len(control) < 3 {
				return false
			}
			first, step := 0, 1
			e := b.Edges[germ]
			if e.End == o.Vertex {
				first, step = len(control)-1, -1
			} else if e.Start != o.Vertex {
				return false
			}
			for axis := range 2 {
				at := func(index int) *big.Rat {
					row := control[first+index*step]
					return new(big.Rat).Quo(row[axis], row[3])
				}
				starts[i][axis] = at(0)
				if at(1).Cmp(starts[i][axis]) == 0 {
					sides[i][axis] = at(2).Cmp(starts[i][axis])
				}
			}
		}
		d := f.Spline.Data()
		hasKnot := false
		for axis, knots := range [2][]float64{d.KnotsU, d.KnotsV} {
			if sides[0][axis] == 0 || sides[0][axis] != sides[1][axis] || starts[0][axis].Cmp(starts[1][axis]) != 0 {
				continue
			}
			station, exact := starts[0][axis].Float64()
			degree := [2]int{d.DegreeU, d.DegreeV}[axis]
			if exact && hasInteriorC0Knot(knots, station, degree) {
				hasKnot = true
			}
		}
		if !hasKnot {
			return false
		}
	}
	return true
}

func hasInteriorC0Knot(knots []float64, station float64, degree int) bool {
	if degree < 1 || station <= knots[degree] || station >= knots[len(knots)-degree-1] {
		return false
	}
	start, found := slices.BinarySearch(knots, station)
	if !found {
		return false
	}
	remaining := knots[start:]
	return remaining[degree-1] == station && remaining[degree] > station
}

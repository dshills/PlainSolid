package csg

import (
	"fmt"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"sort"
)

// SectionKind gives the exact affine dimension of a convex plane section.
type SectionKind uint8

const (
	SectionEmpty SectionKind = iota
	SectionPoint
	SectionSegment
	SectionArea
)

// SectionResult contains zero, one, two, or an ordered convex polygon of points.
// Area polygons are counterclockwise when viewed from the positive-normal side;
// the first point is not repeated. Classification and hull predicates are exact
// for the represented coordinates and plane. Points and Area are rounded float64;
// an area that cannot be represented as finite positive float64 is an error.
type SectionResult struct {
	Kind   SectionKind
	Points []geom.Vec3
	Area   float64
}

// SplitResult partitions a solid into normal.Dot(p-plane.Origin()) <= 0 and >= 0.
// Tangent cuts preserve the original solid on its occupied side and leave the
// other side empty; Section still reports the face, edge, or vertex contact.
type SplitResult struct {
	Negative, Positive brep.Solid
	Section            SectionResult
}

type sectionPoint [3]*big.Rat
type sectionData struct {
	result             SectionResult
	positive, negative bool
}

func representedPoint(p geom.Vec3) sectionPoint {
	return sectionPoint{rational(p.X), rational(p.Y), rational(p.Z)}
}
func exactPlaneDistance(normal, origin, p geom.Vec3) *big.Rat {
	return new(big.Rat).Add(new(big.Rat).Add(product(rational(normal.X), difference(rational(p.X), rational(origin.X))), product(rational(normal.Y), difference(rational(p.Y), rational(origin.Y)))), product(rational(normal.Z), difference(rational(p.Z), rational(origin.Z))))
}
func sectionOrient(a, b, c sectionPoint, axis int) *big.Rat {
	u, v := (axis+1)%3, (axis+2)%3
	return difference(product(difference(b[u], a[u]), difference(c[v], a[v])), product(difference(b[v], a[v]), difference(c[u], a[u])))
}

// Section intersects a convex planar solid with an infinite plane. Tangencies
// are returned explicitly as point, segment, or area sections. No distance-based
// snapping is used for contact classification. Tolerance is used only to validate
// the convex operand, consistently with the other convex CSG operations.
func Section(s brep.Solid, cut geom.Plane, tolerance float64) (SectionResult, error) {
	if _, _, err := featurePlanes(s, tolerance); err != nil {
		return SectionResult{}, err
	}
	data, err := section(s, cut)
	return data.result, err
}
func section(s brep.Solid, cut geom.Plane) (sectionData, error) {
	var data sectionData
	normal, origin := cut.Normal(), cut.Origin()
	if !normal.IsFinite() || !origin.IsFinite() || normal.Length() == 0 {
		return data, fmt.Errorf("csg: invalid section plane")
	}
	vertices := s.Vertices()
	distances := make([]*big.Rat, len(vertices))
	var points []sectionPoint
	for i, p := range vertices {
		distances[i] = exactPlaneDistance(normal, origin, p)
		switch distances[i].Sign() {
		case -1:
			data.negative = true
		case 1:
			data.positive = true
		case 0:
			points = append(points, representedPoint(p))
		}
	}
	for _, edge := range s.Edges() {
		a, b := edge.Start, edge.End
		if distances[a].Sign()*distances[b].Sign() >= 0 {
			continue
		}
		ratio := new(big.Rat).Quo(distances[a], difference(distances[a], distances[b]))
		p, q := representedPoint(vertices[a]), representedPoint(vertices[b])
		var intersection sectionPoint
		for axis := 0; axis < 3; axis++ {
			intersection[axis] = new(big.Rat).Add(p[axis], product(difference(q[axis], p[axis]), ratio))
		}
		points = append(points, intersection)
	}
	if len(points) == 0 {
		return data, nil
	}
	components := [3]float64{normal.X, normal.Y, normal.Z}
	axis := 0
	for i := 1; i < 3; i++ {
		if math.Abs(components[i]) > math.Abs(components[axis]) {
			axis = i
		}
	}
	u, v := (axis+1)%3, (axis+2)%3
	sort.SliceStable(points, func(i, j int) bool {
		comparison := points[i][u].Cmp(points[j][u])
		if comparison != 0 {
			return comparison < 0
		}
		return points[i][v].Cmp(points[j][v]) < 0
	})
	unique := make([]sectionPoint, 0, len(points))
	for _, p := range points {
		if len(unique) == 0 || p[u].Cmp(unique[len(unique)-1][u]) != 0 || p[v].Cmp(unique[len(unique)-1][v]) != 0 {
			unique = append(unique, p)
		}
	}
	var hull []sectionPoint
	if len(unique) <= 2 {
		hull = unique
	} else {
		lower := make([]sectionPoint, 0, len(unique))
		for _, p := range unique {
			for len(lower) >= 2 && sectionOrient(lower[len(lower)-2], lower[len(lower)-1], p, axis).Sign() <= 0 {
				lower = lower[:len(lower)-1]
			}
			lower = append(lower, p)
		}
		upper := make([]sectionPoint, 0, len(unique))
		for i := len(unique) - 1; i >= 0; i-- {
			p := unique[i]
			for len(upper) >= 2 && sectionOrient(upper[len(upper)-2], upper[len(upper)-1], p, axis).Sign() <= 0 {
				upper = upper[:len(upper)-1]
			}
			upper = append(upper, p)
		}
		hull = append(lower[:len(lower)-1], upper[:len(upper)-1]...)
	}
	switch len(hull) {
	case 1:
		data.result.Kind = SectionPoint
	case 2:
		data.result.Kind = SectionSegment
	default:
		data.result.Kind = SectionArea
	}
	if data.result.Kind == SectionArea {
		twiceArea := new(big.Rat)
		for i := 1; i+1 < len(hull); i++ {
			twiceArea.Add(twiceArea, sectionOrient(hull[0], hull[i], hull[i+1], axis))
		}
		projected, _ := new(big.Rat).Quo(twiceArea, big.NewRat(2, 1)).Float64()
		data.result.Area = math.Abs(projected) * normal.Length() / math.Abs(components[axis])
		if data.result.Area == 0 || math.IsNaN(data.result.Area) || math.IsInf(data.result.Area, 0) {
			return sectionData{}, fmt.Errorf("%w: section area is outside float64 range", ErrUnsupported)
		}
		if components[axis] < 0 {
			for i, j := 0, len(hull)-1; i < j; i, j = i+1, j-1 {
				hull[i], hull[j] = hull[j], hull[i]
			}
		}
	}
	data.result.Points = make([]geom.Vec3, len(hull))
	for i, p := range hull {
		x, _ := p[0].Float64()
		y, _ := p[1].Float64()
		z, _ := p[2].Float64()
		data.result.Points[i] = geom.Vec3{X: x, Y: y, Z: z}
		if !data.result.Points[i].IsFinite() {
			return sectionData{}, fmt.Errorf("%w: section coordinate conversion failed", ErrUnsupported)
		}
	}
	return data, nil
}

// Split cuts a convex planar solid into two closed convex solids and returns the
// common section. Proper cuts consume one extra supporting plane. A cut creating
// an unresolved thin part, an unrepresentable plane offset, or failed boundary
// reconstruction returns an error rather than silently treating it as tangency.
func Split(s brep.Solid, cut geom.Plane, tolerance float64) (SplitResult, error) {
	var result SplitResult
	planes, origin, err := featurePlanes(s, tolerance)
	if err != nil {
		return result, err
	}
	data, err := section(s, cut)
	if err != nil {
		return result, err
	}
	result.Section = data.result
	if !data.positive {
		result.Negative = s
		return result, nil
	}
	if !data.negative {
		result.Positive = s
		return result, nil
	}
	if data.result.Kind != SectionArea {
		return SplitResult{}, fmt.Errorf("%w: transverse cut has no area section", ErrUnsupported)
	}
	if len(planes)+1 > MaxPlanes {
		return SplitResult{}, fmt.Errorf("csg: split supporting-plane limit exceeded (%d)", MaxPlanes)
	}
	normal := cut.Normal()
	exactOffset := exactPlaneDistance(normal, origin, cut.Origin())
	offset, _ := exactOffset.Float64()
	if math.IsNaN(offset) || math.IsInf(offset, 0) || new(big.Rat).Abs(difference(exactOffset, rational(offset))).Cmp(rational(tolerance)) > 0 {
		return SplitResult{}, fmt.Errorf("%w: cut-plane offset cannot be represented within tolerance", ErrUnsupported)
	}
	negative := append(append([]plane(nil), planes...), plane{n: normal, d: offset})
	positive := append(append([]plane(nil), planes...), plane{n: normal.Scale(-1), d: -offset})
	result.Negative, err = reconstruct(negative, origin, tolerance)
	if err != nil {
		return SplitResult{}, fmt.Errorf("%w: negative split boundary: %v", ErrUnsupported, err)
	}
	result.Positive, err = reconstruct(positive, origin, tolerance)
	if err != nil {
		return SplitResult{}, fmt.Errorf("%w: positive split boundary: %v", ErrUnsupported, err)
	}
	if len(result.Negative.Vertices()) == 0 || len(result.Positive.Vertices()) == 0 {
		return SplitResult{}, fmt.Errorf("%w: split creates an unresolved thin part", ErrUnsupported)
	}
	source := s.Tessellate()
	volume := source.SignedVolume()
	sum := result.Negative.Tessellate().SignedVolume() + result.Positive.Tessellate().SignedVolume()
	bound := math.Max(math.Abs(volume)*1e-10, 8*tolerance*source.Area())
	if math.IsNaN(sum) || math.IsInf(sum, 0) || math.IsNaN(bound) || math.IsInf(bound, 0) || math.Abs(sum-volume) > bound {
		return SplitResult{}, fmt.Errorf("%w: split volume conservation failed", ErrUnsupported)
	}
	return result, nil
}

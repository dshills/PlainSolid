package csg

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/heal"
	"reflect"
	"sort"
)

// ErrUnsupported denotes a boundary event this planar Boolean implementation
// cannot resolve within its stated geometric and work limits.
var ErrUnsupported = errors.New("csg: unsupported or unresolved boundary event")

const maxBooleanFragments = 512
const maxBooleanSplitChecks = 2000000

type fragment struct {
	points []geom.Vec3
	plane  plane
}

// Union constructs a regularized boundary union of two convex planar solids.
// Individual source face loops must also be convex; inner wires are unsupported.
// Disjoint/contained operands, transverse intersections, and exactly coplanar
// face overlap are supported. Common same-oriented exterior area is owned by A;
// opposite-oriented shared faces are removed. Edge/vertex-only unions and
// near-coplanar or near-plane ambiguities remain explicit errors. Results may use
// triangular planar faces to make all split edges conforming; topology IDs are regenerated.
// This is a bounded floating-boundary algorithm with exact clipping ratios and
// exact mesh collision verification, not a general curved or nonconvex Boolean.
func Union(a, b brep.Solid, tolerance float64) (brep.Solid, error) {
	return boundaryBoolean(a, b, tolerance, false)
}

// Difference constructs the regularized boundary of a minus b for convex planar
// operands under Union's restrictions. The output may be nonconvex or contain an
// inward cavity shell; that does not make it a valid input to convex-only APIs.
func Difference(a, b brep.Solid, tolerance float64) (brep.Solid, error) {
	return boundaryBoolean(a, b, tolerance, true)
}

func boundaryBoolean(a, b brep.Solid, tolerance float64, subtract bool) (brep.Solid, error) {
	if math.IsNaN(tolerance) || math.IsInf(tolerance, 0) || tolerance <= 0 {
		return brep.Solid{}, fmt.Errorf("csg: tolerance must be finite and positive")
	}
	av, bv := a.Vertices(), b.Vertices()
	origin := geom.Vec3{}
	if len(av) > 0 {
		origin = av[0]
	} else if len(bv) > 0 {
		origin = bv[0]
	}
	var pa, pb []plane
	var err error
	if len(av) > 0 {
		pa, err = convexPlanes(a, origin, tolerance)
		if err != nil {
			return brep.Solid{}, err
		}
	}
	if len(bv) > 0 {
		pb, err = convexPlanes(b, origin, tolerance)
		if err != nil {
			return brep.Solid{}, err
		}
	}
	if len(av) == 0 {
		if subtract {
			return brep.Solid{}, nil
		}
		return b, nil
	}
	if len(bv) == 0 {
		return a, nil
	}
	if reflect.DeepEqual(av, bv) && reflect.DeepEqual(a.Edges(), b.Edges()) && reflect.DeepEqual(a.Faces(), b.Faces()) {
		if subtract {
			return brep.Solid{}, nil
		}
		return a, nil
	}
	ca, err := boundaryPlanes(a, pa, origin)
	if err != nil {
		return brep.Solid{}, err
	}
	cb, err := boundaryPlanes(b, pb, origin)
	if err != nil {
		return brep.Solid{}, err
	}
	if strictlyContained(av, cb, origin, tolerance) {
		if subtract {
			return brep.Solid{}, nil
		}
		return b, nil
	}
	if strictlyContained(bv, ca, origin, tolerance) {
		if subtract {
			return sewSolids(a, b.Reversed())
		}
		return a, nil
	}
	if exactContained(av, cb) {
		if subtract {
			return brep.Solid{}, nil
		}
		return b, nil
	}
	if !subtract && exactContained(bv, ca) {
		return a, nil
	}
	overlap, err := Intersect(a, b, tolerance)
	if err != nil {
		return brep.Solid{}, fmt.Errorf("%w: intersection preflight: %v", ErrUnsupported, err)
	}
	emptyOverlap := len(overlap.Vertices()) == 0
	sharedArea := false
	if emptyOverlap {
		contact, e := collision.MeshIntersections(a.Tessellate(), b.Tessellate(), collision.Options{})
		if e != nil {
			return brep.Solid{}, fmt.Errorf("%w: contact classification work limit", ErrUnsupported)
		}
		if len(contact.Contacts) == 0 {
			if possibleContained(av, pb, origin, tolerance) || possibleContained(bv, pa, origin, tolerance) {
				return brep.Solid{}, fmt.Errorf("%w: unresolved near-boundary containment", ErrUnsupported)
			}
			if subtract {
				return a, nil
			}
			return sewSolids(a, b)
		}
		for _, c := range contact.Contacts {
			if c.Kind == collision.Crossing {
				return brep.Solid{}, fmt.Errorf("%w: intersection collapsed below tolerance", ErrUnsupported)
			}
			if c.Kind == collision.CoplanarOverlap {
				sharedArea = true
			}
		}
	}
	relations := make([][]int, len(ca))
	opposing := false
	for i, p := range ca {
		relations[i] = make([]int, len(cb))
		for j, q := range cb {
			relation := coplanarRelation(p, q)
			if relation != 0 && (!exactSupports(p, av) || !exactSupports(q, bv)) {
				return brep.Solid{}, fmt.Errorf("%w: coincident plane is not an exact convex support", ErrUnsupported)
			}
			relations[i][j] = relation
			if relation < 0 {
				opposing = true
			}
			if relation == 0 && p.plane.n.Cross(q.plane.n).Length() <= angularTolerance {
				delta := p.plane.d - q.plane.d
				if p.plane.n.Dot(q.plane.n) < 0 {
					delta = p.plane.d + q.plane.d
				}
				if math.Abs(delta) <= tolerance {
					return brep.Solid{}, fmt.Errorf("%w: near-coplanar supporting faces are not exactly coincident", ErrUnsupported)
				}
			}
		}
	}
	if emptyOverlap {
		if !opposing || !sharedArea {
			return brep.Solid{}, fmt.Errorf("%w: edge/vertex-only or unresolved operand contact", ErrUnsupported)
		}
		if subtract {
			return a, nil
		}
	}
	fa, err := solidFragments(a, pa, origin)
	if err != nil {
		return brep.Solid{}, err
	}
	fb, err := solidFragments(b, pb, origin)
	if err != nil {
		return brep.Solid{}, err
	}
	var boundary []fragment
	appendFragment := func(f fragment) error {
		if len(f.points) < 3 {
			return nil
		}
		boundary = append(boundary, f)
		if len(boundary) > maxBooleanFragments {
			return fmt.Errorf("%w: boundary-fragment work limit", ErrUnsupported)
		}
		return nil
	}
	for index, face := range fa {
		owned := false
		for _, relation := range relations[index] {
			if !subtract && relation == 1 || subtract && relation == -1 {
				owned = true
				break
			}
		}
		if owned {
			if err = appendFragment(face); err != nil {
				return brep.Solid{}, err
			}
			continue
		}
		outside, err := outsideFragments(face, cb, tolerance)
		if err != nil {
			return brep.Solid{}, err
		}
		for _, part := range outside {
			if err = appendFragment(part); err != nil {
				return brep.Solid{}, err
			}
		}
	}
	for index, face := range fb {
		if subtract {
			coincident := false
			for i := range ca {
				if relations[i][index] != 0 {
					coincident = true
					break
				}
			}
			if coincident {
				continue
			}
			part := face.points
			for _, p := range ca {
				part, err = clipPolygon(part, p, true, tolerance)
				if err != nil {
					return brep.Solid{}, err
				}
				if len(part) < 3 {
					break
				}
			}
			if len(part) >= 3 {
				for i, j := 0, len(part)-1; i < j; i, j = i+1, j-1 {
					part[i], part[j] = part[j], part[i]
				}
				if err = appendFragment(fragment{part, plane{n: face.plane.n.Scale(-1), d: -face.plane.d}}); err != nil {
					return brep.Solid{}, err
				}
			}
		} else {
			outside, err := outsideFragments(face, ca, tolerance)
			if err != nil {
				return brep.Solid{}, err
			}
			for _, part := range outside {
				if err = appendFragment(part); err != nil {
					return brep.Solid{}, err
				}
			}
		}
	}
	result, err := assembleFragments(boundary, tolerance)
	if err != nil {
		return brep.Solid{}, err
	}
	result, err = result.Transformed(geom.Translation(origin))
	if err != nil {
		return brep.Solid{}, fmt.Errorf("%w: world-coordinate reconstruction: %v", ErrUnsupported, err)
	}
	return verifyBoundary(result)
}
func planeValue(p plane, v geom.Vec3) *big.Rat {
	return difference(new(big.Rat).Add(new(big.Rat).Add(product(rational(p.n.X), rational(v.X)), product(rational(p.n.Y), rational(v.Y))), product(rational(p.n.Z), rational(v.Z))), rational(p.d))
}
func strictlyContained(vertices []geom.Vec3, planes []clippingPlane, origin geom.Vec3, tolerance float64) bool {
	for _, p := range planes {
		limit := new(big.Rat).Neg(p.limit(tolerance))
		for _, v := range vertices {
			var value *big.Rat
			if p.exact {
				value = difference(coefficientDot(p.normal, v), p.globalOffset)
			} else {
				local := v.Sub(origin)
				if !local.IsFinite() {
					return false
				}
				value = planeValue(p.plane, local)
			}
			if value.Cmp(limit) >= 0 {
				return false
			}
		}
	}
	return true
}

func solidFragments(s brep.Solid, planes []plane, origin geom.Vec3) ([]fragment, error) {
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	result := make([]fragment, len(faces))
	for f, face := range faces {
		if len(face.Holes) > 0 {
			return nil, fmt.Errorf("%w: convex operand has inner face wires", ErrUnsupported)
		}
		result[f].plane = planes[f]
		for _, u := range face.Boundary {
			id := edges[u.Edge].Start
			if u.Reversed {
				id = edges[u.Edge].End
			}
			p := vertices[id].Sub(origin)
			if !p.IsFinite() {
				return nil, fmt.Errorf("%w: local coordinates overflow", ErrUnsupported)
			}
			result[f].points = append(result[f].points, p)
		}
	}
	return result, nil
}
func outsideFragments(face fragment, planes []clippingPlane, tolerance float64) ([]fragment, error) {
	pending := face.points
	var result []fragment
	for _, p := range planes {
		outside, err := clipPolygon(pending, p, false, tolerance)
		if err != nil {
			return nil, err
		}
		if len(outside) >= 3 {
			result = append(result, fragment{outside, face.plane})
		}
		pending, err = clipPolygon(pending, p, true, tolerance)
		if err != nil {
			return nil, err
		}
		if len(pending) < 3 {
			break
		}
	}
	return result, nil
}
func clipPolygon(input []geom.Vec3, p clippingPlane, inside bool, tolerance float64) ([]geom.Vec3, error) {
	if len(input) < 3 {
		return nil, nil
	}
	values := make([]*big.Rat, len(input))
	positive, negative := 0, 0
	limit := p.limit(tolerance)
	for i, v := range input {
		values[i] = p.value(v)
		sign := values[i].Sign()
		if sign > 0 {
			positive++
		} else if sign < 0 {
			negative++
		}
		if sign != 0 && new(big.Rat).Abs(values[i]).Cmp(limit) <= 0 {
			return nil, fmt.Errorf("%w: clipping event lies within tolerance of a plane", ErrUnsupported)
		}
	}
	if positive == 0 {
		if inside {
			return append([]geom.Vec3(nil), input...), nil
		}
		return nil, nil
	}
	if negative == 0 {
		if !inside {
			return append([]geom.Vec3(nil), input...), nil
		}
		return nil, nil
	}
	keep := func(value *big.Rat) bool {
		if inside {
			return value.Sign() <= 0
		}
		return value.Sign() >= 0
	}
	var output []geom.Vec3
	previous := len(input) - 1
	for current, v := range input {
		pin, cin := keep(values[previous]), keep(values[current])
		if pin != cin {
			ratio := new(big.Rat).Quo(values[previous], difference(values[previous], values[current]))
			a := input[previous]
			coordinate := func(x, y float64) float64 {
				value := new(big.Rat).Add(rational(x), product(difference(rational(y), rational(x)), ratio))
				f, _ := value.Float64()
				return f
			}
			output = append(output, geom.Vec3{X: coordinate(a.X, v.X), Y: coordinate(a.Y, v.Y), Z: coordinate(a.Z, v.Z)})
		}
		if cin {
			output = append(output, v)
		}
		previous = current
	}
	cleaned := make([]geom.Vec3, 0, len(output))
	for _, v := range output {
		if len(cleaned) == 0 || cleaned[len(cleaned)-1] != v {
			cleaned = append(cleaned, v)
		}
	}
	if len(cleaned) > 1 && cleaned[0] == cleaned[len(cleaned)-1] {
		cleaned = cleaned[:len(cleaned)-1]
	}
	if len(cleaned) < 3 {
		return nil, nil
	}
	return cleaned, nil
}

func assembleFragments(fragments []fragment, tolerance float64) (brep.Solid, error) {
	if len(fragments) == 0 {
		return brep.Solid{}, nil
	}
	faces := make([]heal.Face, len(fragments))
	for i, f := range fragments {
		faces[i].Outer = f.points
	}
	sewn, report, err := heal.Sew(faces, heal.Options{Tolerance: tolerance})
	if err == nil {
		return sewn, nil
	}
	if errors.Is(err, heal.ErrLimit) || errors.Is(err, heal.ErrNonManifold) || report.InputVertices == 0 || len(report.VertexMap) != len(fragments) {
		return brep.Solid{}, fmt.Errorf("%w: fragment welding: %v", ErrUnsupported, err)
	}
	// The remap is useful even for open or collinear boundary fragments. Split
	// every edge at every welded boundary vertex, then fan triangulate the convex
	// fragment so collinear split points remain topologically conforming.
	var triangles []heal.Face
	checks := 0
	for f, mapping := range report.VertexMap {
		ids := mapping.Outer
		seen := map[int]bool{}
		for _, id := range ids {
			if seen[id] {
				return brep.Solid{}, fmt.Errorf("%w: welding collapses a fragment", ErrUnsupported)
			}
			seen[id] = true
			if math.Abs(fragments[f].plane.n.Dot(report.Vertices[id])-fragments[f].plane.d) > 2*tolerance {
				return brep.Solid{}, fmt.Errorf("%w: weld displacement changes a face plane", ErrUnsupported)
			}
		}
		var boundary []int
		for i, a := range ids {
			b := ids[(i+1)%len(ids)]
			start, end := report.Vertices[a], report.Vertices[b]
			type split struct {
				id int
				t  float64
			}
			splits := []split{{a, 0}}
			for id, p := range report.Vertices {
				if id == a || id == b {
					continue
				}
				checks++
				if checks > maxBooleanSplitChecks {
					return brep.Solid{}, fmt.Errorf("%w: edge-split work limit", ErrUnsupported)
				}
				nearest, t, err := geom.ClosestOnSegment(p, start, end)
				if err != nil {
					return brep.Solid{}, fmt.Errorf("%w: unstable edge projection", ErrUnsupported)
				}
				if t > 0 && t < 1 && p.Sub(nearest).Length() <= tolerance {
					splits = append(splits, split{id, t})
				}
			}
			sort.Slice(splits, func(i, j int) bool { return splits[i].t < splits[j].t })
			for _, s := range splits {
				boundary = append(boundary, s.id)
			}
		}
		center := geom.Vec3{}
		for _, id := range ids {
			center = center.Add(report.Vertices[id].Scale(1 / float64(len(ids))))
		}
		if !center.IsFinite() {
			return brep.Solid{}, fmt.Errorf("%w: fragment center overflows", ErrUnsupported)
		}
		for i, a := range boundary {
			b := boundary[(i+1)%len(boundary)]
			triangles = append(triangles, heal.Face{Outer: []geom.Vec3{report.Vertices[a], report.Vertices[b], center}})
			if len(triangles) > heal.MaxFaces {
				return brep.Solid{}, fmt.Errorf("%w: triangulated boundary work limit", ErrUnsupported)
			}
		}
	}
	sewn, _, err = heal.Sew(triangles, heal.Options{})
	if err != nil {
		return brep.Solid{}, fmt.Errorf("%w: conforming boundary reconstruction: %v", ErrUnsupported, err)
	}
	return sewn, nil
}
func sewSolids(a, b brep.Solid) (brep.Solid, error) {
	var faces []heal.Face
	for _, s := range []brep.Solid{a, b} {
		vertices, edges := s.Vertices(), s.Edges()
		for _, face := range s.Faces() {
			f := heal.Face{}
			for loop, uses := range append([][]brep.EdgeUse{face.Boundary}, face.Holes...) {
				var points []geom.Vec3
				for _, u := range uses {
					id := edges[u.Edge].Start
					if u.Reversed {
						id = edges[u.Edge].End
					}
					points = append(points, vertices[id])
				}
				if loop == 0 {
					f.Outer = points
				} else {
					f.Holes = append(f.Holes, points)
				}
			}
			faces = append(faces, f)
		}
	}
	result, _, err := heal.Sew(faces, heal.Options{})
	if err != nil {
		return brep.Solid{}, fmt.Errorf("%w: shell assembly: %v", ErrUnsupported, err)
	}
	return verifyBoundary(result)
}
func verifyBoundary(s brep.Solid) (brep.Solid, error) {
	report, err := collision.SelfIntersections(s.Tessellate(), collision.Options{MaxPairs: 200000, MaxContacts: 1})
	if err != nil || !report.Complete || len(report.Contacts) > 0 {
		return brep.Solid{}, fmt.Errorf("%w: boundary collision validation failed (%v, %d contacts)", ErrUnsupported, err, len(report.Contacts))
	}
	return s, nil
}

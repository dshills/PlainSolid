package plaincadbridge

import (
	"errors"
	"math"
	"math/big"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// boreBody preserves the planar source and each exact analytic cylindrical tool.
// The specialized constructor proves strict radial containment and convex stock.
// The ordinary path uses perpendicular opening planes. The single oblique
// through path retains its separate elliptical opening and wall measures.
type boreBody struct {
	solid        trimmed.Solid
	base         brep.Solid
	cylinders    []*cylinderBody
	cuts         []trimmed.CylinderCut
	spans        []float64
	blind        []bool
	floors       []int
	union        *trimmed.CylinderUnionHistory
	layered      *trimmed.LayeredBoreHistory
	crossed      *trimmed.BoxCrossedBoresResult
	partialSkew  bool
	blindCrossed *blindCrossedGeometry
	oblique      *obliqueBore
}

func exactDotDifference(a, b, n geom.Vec3) int {
	x, y, z := []float64{a.X, a.Y, a.Z}, []float64{b.X, b.Y, b.Z}, []float64{n.X, n.Y, n.Z}
	sum := new(big.Rat)
	for i := 0; i < 3; i++ {
		d := new(big.Rat).Sub(new(big.Rat).SetFloat64(x[i]), new(big.Rat).SetFloat64(y[i]))
		sum.Add(sum, new(big.Rat).Mul(d, new(big.Rat).SetFloat64(z[i])))
	}
	return sum.Sign()
}

// boreSpan requires a face's exact represented tangent vectors to be orthogonal
// to the cylinder direction. Consequently every point of each circular tool cap
// has the same axial support-plane side, not just the central ray.
func boreSpan(base brep.Solid, c *cylinderBody, through bool) (trimmed.CylinderCut, float64, float64, error) {
	unsupported := func(message string) (trimmed.CylinderCut, float64, float64, error) {
		return trimmed.CylinderCut{}, 0, 0, &failureError{"UNSUPPORTED", message}
	}
	m, owners := base.Tessellate(), base.TriangleFaces()
	faces := base.Faces()
	first := make([][3]int, len(faces))
	seen := make([]bool, len(faces))
	for i, f := range owners {
		if !seen[f] {
			first[f] = m.Triangles[i]
			seen[f] = true
		}
	}
	entry, exit := -1, -1
	start, end := 0., 0.
	axis := c.frame.ZAxis()
	origin := c.frame.Origin()
	if exactDotDifference(c.frame.XAxis(), geom.Vec3{}, axis) != 0 || exactDotDifference(c.frame.YAxis(), geom.Vec3{}, axis) != 0 {
		return unsupported("represented cylinder radial axes do not prove exact cap parallelism")
	}
	for i, t := range first {
		if !seen[i] {
			return unsupported("cylinder cut requires finite planar faces")
		}
		a, b, d := m.Vertices[t[0]], m.Vertices[t[1]], m.Vertices[t[2]]
		if exactDotDifference(b, a, axis) != 0 || exactDotDifference(d, a, axis) != 0 {
			continue
		}
		p, err := base.FacePlane(i)
		if err != nil {
			return trimmed.CylinderCut{}, 0, 0, err
		}
		dot := p.Normal().Dot(axis)
		parameter := p.Origin().Sub(origin).Dot(p.Normal()) / dot
		if !finite(parameter) {
			return unsupported("cylinder support intersection exceeds numeric range")
		}
		if dot < 0 {
			if entry >= 0 {
				return unsupported("multiple possible cylindrical entry planes")
			}
			entry, start = i, parameter
		} else {
			if exit >= 0 {
				return unsupported("multiple possible cylindrical exit planes")
			}
			exit, end = i, parameter
		}
	}
	if entry < 0 || exit < 0 || end <= start {
		return unsupported("cylinder cut requires two parallel perpendicular opening planes")
	}
	if through {
		finish := origin.Add(axis.Scale(c.height))
		if !finish.IsFinite() {
			return unsupported("cylinder endpoint exceeds numeric range")
		}
		for _, v := range []struct {
			face  int
			point geom.Vec3
		}{{entry, origin}, {exit, finish}} {
			t := first[v.face]
			side, err := geom.Orient3D(m.Vertices[t[0]], m.Vertices[t[1]], m.Vertices[t[2]], v.point)
			if err != nil {
				return trimmed.CylinderCut{}, 0, 0, err
			}
			if side > 0 {
				return unsupported("finite cylinder does not cover both opening planes; blind or partial cuts are unsupported")
			}
		}
	}
	return trimmed.CylinderCut{Entry: entry, Exit: exit, AxisPoint: origin, Direction: axis, Radius: c.radius}, start, end, nil
}

// sameCardinalBoreLine proves equality of both represented transverse coordinates.
func sameCardinalBoreLine(a, b *cylinderBody) bool {
	if !cardinalFrame(a.frame) || !cardinalFrame(b.frame) {
		return false
	}
	axis := a.frame.ZAxis()
	return axis.Cross(b.frame.ZAxis()) == (geom.Vec3{}) &&
		axis.Cross(a.frame.Origin().Sub(b.frame.Origin())) == (geom.Vec3{})
}

func newBore(base brep.Solid, cylinders []*cylinderBody) (*boreBody, error) {
	if len(cylinders) == 1 {
		if body, handled, err := newObliqueThroughBore(base, cylinders[0]); handled || err != nil {
			return body, err
		}
	}
	if len(cylinders) > 32 {
		return nil, &failureError{"LIMIT", "at most 32 cylindrical tools may be retained"}
	}
	cylinders = append([]*cylinderBody(nil), cylinders...)
	cuts := make([]trimmed.CylinderCut, len(cylinders))
	spans := make([]float64, len(cylinders))
	blind := make([]bool, len(cylinders))
	floors := make([]int, len(cylinders))
	deltas := make([]float64, len(cylinders))
	for i, c := range cylinders {
		cut, start, end, err := boreSpan(base, c, true)
		if err != nil {
			cut, start, end, err = boreSpan(base, c, false)
			if err != nil {
				return nil, err
			}
			finish := c.frame.Origin().Add(c.frame.ZAxis().Scale(c.height))
			entrySide, entryErr := capSide(base, cut.Entry, c.frame.Origin())
			exitSide, exitErr := capSide(base, cut.Exit, finish)
			if entryErr != nil {
				return nil, entryErr
			}
			if exitErr != nil {
				return nil, exitErr
			}

			// Negative CAD sweeps may encode the same finite tool with a positive
			// axis and its origin inside stock. Canonicalize an exit-open pocket to
			// entry-open geometry only after exact support-side classification.
			if entrySide > 0 && exitSide <= 0 {
				frame, e := geom.NewFrame(finish, c.frame.XAxis(), c.frame.ZAxis().Scale(-1))
				if e != nil {
					return nil, e
				}
				if frame.XAxis() != c.frame.XAxis() || frame.YAxis() != c.frame.YAxis().Scale(-1) || frame.ZAxis() != c.frame.ZAxis().Scale(-1) {
					return nil, &failureError{"UNSUPPORTED", "finite cylinder reversal cannot preserve represented radial frame"}
				}
				reversed, e := newCylinder(frame, c.radius, c.height)
				if e != nil {
					return nil, e
				}
				if reversed.frame.Origin().Add(reversed.frame.ZAxis().Scale(reversed.height)) != c.frame.Origin() {
					return nil, &failureError{"UNSUPPORTED", "finite cylinder reversal cannot preserve represented endpoints"}
				}
				c = reversed
				cylinders[i] = c
				cut, start, end, err = boreSpan(base, c, false)
				if err != nil {
					return nil, err
				}
				finish = c.frame.Origin().Add(c.frame.ZAxis().Scale(c.height))
				entrySide, entryErr = capSide(base, cut.Entry, c.frame.Origin())
				exitSide, exitErr = capSide(base, cut.Exit, finish)
				if entryErr != nil {
					return nil, entryErr
				}
				if exitErr != nil {
					return nil, exitErr
				}
			}
			if !finish.IsFinite() || entrySide > 0 || exitSide <= 0 || c.height-start <= 1e-9 || end-c.height <= 1e-9 {
				return nil, &failureError{"UNSUPPORTED", "finite cylinder is not a measurable entry-open blind pocket or through bore"}
			}
			blind[i] = true
			deltas[i] = end - c.height
		}
		cuts[i], spans[i] = cut, end-start-deltas[i]
	}
	s, history, err := trimmed.BoreCylinders(base, cuts, 1e-9)
	if err != nil {
		pair, pairCuts, pairSpans, pairBlind := retainedCrossedToolPair(cylinders, cuts, spans, blind)
		if result, handled, err := newBlindCrossedBore(base, pair, pairCuts, pairSpans, pairBlind); handled {
			return result, err
		}
		if result, handled, err := newCrossedBore(base, pair, pairCuts, pairSpans, pairBlind); handled {
			return result, err
		}
		return newUnionBore(base, cylinders, cuts, blind)
	}
	for i := range cylinders {
		if blind[i] {
			s, floors[i], err = stopBore(s, history[i], cuts[i].Direction, deltas[i])
			if err != nil {
				return nil, err
			}
		}
	}
	return &boreBody{solid: s, base: base, cylinders: cylinders, cuts: cuts, spans: spans, blind: blind, floors: floors}, nil
}
func (body *boreBody) measures() (float64, float64, error) {
	p, err := body.base.MassProperties()
	if err != nil {
		return 0, 0, err
	}
	volume, area := p.Volume, p.SurfaceArea
	if h := body.oblique; h != nil {
		volume -= h.removedVolume
		area += h.wallArea - h.openingAreas[0] - h.openingAreas[1]
		if !finite(volume) || !finite(area) || volume <= 0 || area <= 0 {
			return 0, 0, errors.New("invalid oblique through-bore material measures")
		}
		return volume, area, nil
	}
	if h := body.crossed; h != nil {
		return volume - h.RemovedVolume, area + h.SurfaceAreaDelta, nil
	}
	if h := body.layered; h != nil {
		volume -= h.RemovedVolume
		area += h.SurfaceAreaDelta
		if !finite(volume) || !finite(area) || volume <= 0 || area <= 0 {
			return 0, 0, errors.New("invalid layered-cylinder measures")
		}
		return volume, area, nil
	}
	if u := body.union; u != nil {
		volume -= u.Area * u.Depth
		area += u.Perimeter * u.Depth
		if !u.Blind {
			area -= 2 * u.Area
		}
		if !finite(volume) || !finite(area) || volume <= 0 || area <= 0 {
			return 0, 0, errors.New("invalid cylindrical union measures")
		}
		return volume, area, nil
	}
	for i, c := range body.cylinders {
		volume -= math.Pi * c.radius * c.radius * body.spans[i]
		area += 2 * math.Pi * c.radius * (body.spans[i] - c.radius)
		if body.blind[i] {
			area += 2 * math.Pi * c.radius * c.radius
		}
	}
	if !finite(volume) || !finite(area) || volume <= 0 || area <= 0 {
		return 0, 0, errors.New("invalid cylindrical-cut material measures")
	}
	return volume, area, nil
}
func (b *Bridge) putBore(body *boreBody) (Shape, error) {
	volume, area, err := body.measures()
	if err != nil {
		return Shape{}, err
	}
	bounds, err := body.base.Bounds()
	if err != nil {
		return Shape{}, err
	}
	return b.publish(stored{bore: body, result: Shape{GeometryAssertions: Assertions{true, volume, area, 1}, Bounds: Bounds{coordinates(bounds.Min), coordinates(bounds.Max)}}})
}
func (body *boreBody) transformed(m geom.Affine) (*boreBody, error) {
	if err := rigid(m); err != nil {
		return nil, err
	}
	base, err := body.base.Transformed(m)
	if err != nil {
		return nil, err
	}
	cylinders := make([]*cylinderBody, len(body.cylinders))
	for i, c := range body.cylinders {
		cylinders[i], err = c.transformed(m)
		if err != nil {
			return nil, err
		}
	}
	return newBore(base, cylinders)
}
func (body *boreBody) planarFace(origin, normal geom.Vec3) (PlanarFace, error) {
	if h := body.layered; h != nil {
		return body.cutPlanarFace(origin, normal, h.Entry, h.Exit, h.EntryRemovedArea, h.ExitRemovedArea, h.FloorFaces, h.FloorAreas)
	}
	if body.union != nil {
		return body.unionPlanarFace(origin, normal)
	}
	n, err := normal.Unit()
	if err != nil || !origin.IsFinite() {
		return PlanarFace{}, errors.New("invalid planar reference")
	}
	var floor *PlanarFace
	for i, blind := range body.blind {
		if !blind {
			continue
		}
		c := body.cylinders[i]
		p := c.frame.Origin().Add(c.frame.ZAxis().Scale(c.height))
		axis := c.frame.ZAxis()
		if math.Abs(math.Abs(axis.Dot(n))-1) <= 1e-9 && math.Abs(p.Sub(origin).Dot(axis)) <= 1e-7 {
			if floor != nil {
				return PlanarFace{}, errors.New("planar face reference is ambiguous")
			}
			f := PlanarFace{body.floors[i], coordinates(p), coordinates(axis.Scale(-1)), math.Pi * c.radius * c.radius}
			floor = &f
		}
	}
	if floor != nil {
		if _, err := matchingPlanarFace(body.base, origin, normal); err == nil {
			return PlanarFace{}, errors.New("planar face reference is ambiguous")
		}
		return *floor, nil
	}

	// BoreCylinders retains every original planar face and introduces strictly
	// interior circular holes; it neither removes nor merges original supports.
	f, err := matchingPlanarFace(body.base, origin, normal)
	if err != nil {
		return PlanarFace{}, err
	}
	baseFace := f.Face
	if body.crossed != nil {
		// The crossed constructor orders the box faces in its own frame.
		found := false
		for i, face := range body.solid.Faces() {
			plane, ok := face.Surface.(geom.PlaneSurface)
			if ok && math.Abs(plane.Frame().ZAxis().Dot(n)) == 1 &&
				math.Abs(origin.Sub(plane.Frame().Origin()).Dot(n)) <= 1e-7 {
				f.Face, found = i, true
				break
			}
		}
		if !found {
			return PlanarFace{}, errors.New("retained planar support mismatch")
		}
	}
	if _, ok := body.solid.Faces()[f.Face].Surface.(geom.PlaneSurface); !ok {
		return PlanarFace{}, errors.New("retained planar support mismatch")
	}
	if h := body.oblique; h != nil {
		if body.cuts[0].Entry == baseFace {
			f.Area -= h.openingAreas[0]
		}
		if body.cuts[0].Exit == baseFace {
			f.Area -= h.openingAreas[1]
		}
	} else {
		for i, c := range body.cylinders {
			if body.cuts[i].Entry == baseFace || (!body.blind[i] && body.cuts[i].Exit == baseFace) {
				f.Area -= math.Pi * c.radius * c.radius
			}
		}
	}
	if f.Area <= 1e-14 {
		return PlanarFace{}, errors.New("planar face has no measurable retained area")
	}
	return f, nil
}
func (b *Bridge) curvedBoolean(a, c stored, op string) (any, error) {
	if op == "cut" && boundsSeparated(a.result.Bounds, c.result.Bounds) {
		return nil, &failureError{"UNSUPPORTED", "Boolean cut removed no volume. Move the tool into the target body."}
	}
	if op == "common" && a.cylinder != nil && c.cylinder == nil {
		a, c = c, a
	}
	if c.cylinder == nil || a.cylinder != nil {
		return nil, &failureError{"UNSUPPORTED", "curved Boolean combination is outside the finite entry-open cylinder-cut domain"}
	}
	base := a.solid
	var cylinders []*cylinderBody
	if a.bore != nil {
		base = a.bore.base
		cylinders = a.bore.cylinders
	}
	if op == "cut" {
		body, err := newBore(base, append(append([]*cylinderBody(nil), cylinders...), c.cylinder))
		if err != nil {
			return nil, err
		}
		volume, _, err := body.measures()
		if err != nil {
			return nil, err
		}
		if !removesMaterial(a.result.GeometryAssertions.Volume, volume) {
			return nil, &failureError{"UNSUPPORTED", "cylindrical cut removed no material"}
		}
		return b.putBore(body)
	}
	if op == "common" && boundsSeparated(a.result.Bounds, c.result.Bounds) {
		return b.put(brep.Solid{})
	}
	if op == "common" {
		for _, old := range cylinders {
			oldBounds, err := old.solid.Bounds()
			if err != nil {
				return nil, err
			}
			if !boundsSeparated(Bounds{coordinates(oldBounds.Min), coordinates(oldBounds.Max)}, c.result.Bounds) {
				return nil, &failureError{"UNSUPPORTED", "cylinder overlap with an existing bore is unresolved"}
			}
		}
		cut, start, end, err := boreSpan(base, c.cylinder, false)
		if err != nil {
			return nil, err
		}
		// This construction validates the full radial footprint independently from
		// finite axial clipping, so an intersecting side wall cannot be ignored.
		if _, _, err := trimmed.BoreCylinderAlong(base, cut.Entry, cut.Exit, cut.AxisPoint, cut.Direction, cut.Radius, 1e-9); err != nil {
			return nil, &failureError{"UNSUPPORTED", "cylinder overlap requires a strictly contained radial footprint: " + err.Error()}
		}
		lo, hi := math.Max(0, start), math.Min(c.cylinder.height, end)
		if hi <= lo {
			return b.put(brep.Solid{})
		}
		return b.circle(c.cylinder.frame.Origin().Add(c.cylinder.frame.ZAxis().Scale(lo)), c.cylinder.frame.ZAxis(), c.cylinder.radius, hi-lo)
	}
	return nil, &failureError{"UNSUPPORTED", "curved Boolean combination is outside the finite entry-open cylinder-cut domain"}
}

// Existing shape bounds enclose the original analytic geometry. Strictly
// separated intervals prove a miss without assuming unsupported intersections.
func boundsSeparated(a, b Bounds) bool {
	for i := 0; i < 3; i++ {
		if a.Max[i] < b.Min[i] || b.Max[i] < a.Min[i] {
			return true
		}
	}
	return false
}

func capSide(base brep.Solid, face int, p geom.Vec3) (int, error) {
	m := base.Tessellate()
	for i, f := range base.TriangleFaces() {
		if f == face {
			t := m.Triangles[i]
			side, err := geom.Orient3D(m.Vertices[t[0]], m.Vertices[t[1]], m.Vertices[t[2]], p)
			if err != nil {
				return 0, err
			}
			return int(side), nil
		}
	}
	return 0, errors.New("missing cylinder support face")
}

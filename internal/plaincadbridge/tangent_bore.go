package plaincadbridge

import (
	"errors"
	"math"
	"math/big"

	"plainsolid/geom"
)

// tangentBlindBore retains the regularized box difference in its circular
// chart. The contact generator has four incident faces, so the shape cannot
// be stored as a closed manifold B-rep or used as a fabrication solid.
type tangentBlindBore struct {
	placement geom.Affine
	box       geom.Box // bore entry is Z=0; the tangent side is Y=-radius
	radius    float64
	depth     float64
}

func tangentBlindBoreRecipe(stock, tool stored) (*tangentBlindBore, bool) {
	c := tool.cylinder
	if !stock.planar() || c == nil || !cardinalFrame(c.frame) {
		return nil, false
	}
	vertices := stock.solid.Vertices()
	if len(vertices) != 8 || len(stock.solid.Edges()) != 12 || len(stock.solid.Faces()) != 6 {
		return nil, false
	}
	axis := c.frame.ZAxis()
	for _, end := range []bool{false, true} {
		origin, inward := c.frame.Origin(), axis
		if end {
			origin, inward = c.frame.ToWorld(geom.Vec3{Z: c.height}), axis.Scale(-1)
			if !exactAxisDistance(origin.Dot(axis), c.frame.Origin().Dot(axis), c.height) {
				continue
			}
		}
		for _, x := range []geom.Vec3{c.frame.XAxis(), c.frame.YAxis(), c.frame.XAxis().Scale(-1), c.frame.YAxis().Scale(-1)} {
			frame, err := geom.NewFrame(origin, x, inward)
			if err != nil {
				continue
			}
			local := make([]geom.Vec3, len(vertices))
			roundTrip := true
			for i, p := range vertices {
				local[i] = frame.ToLocal(p)
				roundTrip = roundTrip && frame.ToWorld(local[i]) == p
			}
			box, err := geom.Bounds(local)
			r := c.radius
			if err != nil || !roundTrip || box.Min.Z != 0 || box.Max.Z <= c.height ||
				box.Min.Y != -r || box.Max.Y <= r || box.Min.X >= -r || box.Max.X <= r {
				continue
			}
			if !exactTangentSide(vertices, frame, r) || !rectangularFaces(stock.solid, frame, box.Min, box.Max) {
				continue
			}
			return &tangentBlindBore{frameMap(frame), box, r, c.height}, true
		}
	}
	return nil, false
}

// Float subtraction can round a small clearance or overlap to the same
// local ordinate as the radius. Prove the contact on the represented input.
func exactTangentSide(vertices []geom.Vec3, frame geom.Frame, radius float64) bool {
	n, origin := frame.YAxis(), frame.Origin()
	side := math.Inf(1)
	for _, p := range vertices {
		side = min(side, p.Dot(n))
	}
	return exactAxisDistance(origin.Dot(n), side, radius)
}

func exactAxisDistance(high, low, distance float64) bool {
	if !finite(high) || !finite(low) {
		return false
	}
	var a, b, d big.Rat
	a.SetFloat64(high)
	b.SetFloat64(low)
	d.SetFloat64(distance)
	return a.Sub(&a, &b).Cmp(&d) == 0
}

func (h *tangentBlindBore) removed() float64 { return math.Pi * h.radius * h.radius * h.depth }

func (b *Bridge) putTangentBlindBore(h *tangentBlindBore) (Shape, error) {
	var corners []geom.Vec3
	for _, x := range []float64{h.box.Min.X, h.box.Max.X} {
		for _, y := range []float64{h.box.Min.Y, h.box.Max.Y} {
			for _, z := range []float64{0, h.box.Max.Z} {
				corners = append(corners, h.placement.Apply(geom.Vec3{X: x, Y: y, Z: z}))
			}
		}
	}
	bounds, err := geom.Bounds(corners)
	if err != nil {
		return Shape{}, err
	}
	d := h.box.Max.Sub(h.box.Min)
	volume := d.X*d.Y*d.Z - h.removed()
	area := 2*(d.X*d.Y+d.Y*d.Z+d.Z*d.X) + 2*math.Pi*h.radius*h.depth
	if !finite(volume) || !finite(area) || volume <= 0 || area <= 0 {
		return Shape{}, geom.ErrUnrepresentable
	}
	if !removesMaterial(d.X*d.Y*d.Z, volume) {
		return Shape{}, unsupportedEdge("Boolean cut removed no volume")
	}
	// Qualify the contact before publishing a handle, including after placement.
	o, err := tessellationOptions(request{})
	if err != nil {
		return Shape{}, err
	}
	if _, err := h.tessellate(o); err != nil {
		return Shape{}, err
	}
	result := Shape{GeometryAssertions: Assertions{true, volume, area, 1}, Bounds: Bounds{coordinates(bounds.Min), coordinates(bounds.Max)}}
	return b.publish(stored{tangent: h, result: result})
}

func (h *tangentBlindBore) planarFace(origin, normal geom.Vec3) (PlanarFace, error) {
	n, err := normal.Unit()
	if err != nil || !origin.IsFinite() {
		return PlanarFace{}, geom.ErrInvalid
	}
	lo, hi := h.box.Min, h.box.Max
	d := hi.Sub(lo)
	disk := math.Pi * h.radius * h.radius
	faces := []struct {
		point, normal geom.Vec3
		area          float64
	}{
		{geom.Vec3{X: lo.X}, geom.Vec3{X: -1}, d.Y * d.Z},
		{geom.Vec3{X: hi.X}, geom.Vec3{X: 1}, d.Y * d.Z},
		{geom.Vec3{Y: lo.Y}, geom.Vec3{Y: -1}, d.X * d.Z},
		{geom.Vec3{Y: hi.Y}, geom.Vec3{Y: 1}, d.X * d.Z},
		{geom.Vec3{}, geom.Vec3{Z: -1}, d.X*d.Y - disk},
		{geom.Vec3{Z: hi.Z}, geom.Vec3{Z: 1}, d.X * d.Y},
		{geom.Vec3{Z: h.depth}, geom.Vec3{Z: -1}, disk},
	}
	var result PlanarFace
	found := false
	for i, face := range faces {
		p, outward := h.placement.Apply(face.point), h.placement.ApplyVector(face.normal)
		if math.Abs(math.Abs(outward.Dot(n))-1) > 1e-9 || math.Abs(p.Sub(origin).Dot(outward)) > 1e-7 {
			continue
		}
		if found {
			return PlanarFace{}, errors.New("planar face reference is ambiguous")
		}
		result, found = PlanarFace{i, coordinates(p), coordinates(outward), face.area}, true
	}
	if !found {
		return PlanarFace{}, errors.New("planar face reference no longer exists")
	}
	return result, nil
}

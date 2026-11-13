package scene

import (
	"fmt"
	"sort"

	"plainsolid/geom"
	"plainsolid/mesh"
	"plainsolid/query"
)

// Item describes an instance. Asset names can be reused by different items.
type Item struct {
	ID, Asset string
	Transform geom.Affine
}

type instance struct {
	id           string
	world, local geom.Affine
	index        *query.Index
}

// Scene keeps immutable copies of the input meshes and instances. It can
// safely be queried concurrently.
type Scene struct{ items []instance }

// New copies each referenced mesh once. IDs must be unique and nonempty.
// Transform maps asset coordinates into world coordinates.
func New(assets map[string]mesh.Mesh, items []Item) (*Scene, error) {
	s := &Scene{}
	seen := map[string]bool{}
	indices := map[string]*query.Index{}
	for _, item := range items {
		if item.ID == "" || seen[item.ID] {
			return nil, fmt.Errorf("scene: empty or duplicate ID %q", item.ID)
		}
		seen[item.ID] = true
		index, ok := indices[item.Asset]
		if !ok {
			m, ok := assets[item.Asset]
			if !ok || item.Asset == "" {
				return nil, fmt.Errorf("scene: missing asset %q", item.Asset)
			}
			var err error
			index, err = query.New(m)
			if err != nil {
				return nil, fmt.Errorf("scene: asset %q: %w", item.Asset, err)
			}
			indices[item.Asset] = index
		}
		inverse, err := item.Transform.Inverse()
		if err != nil {
			return nil, fmt.Errorf("scene: instance %q: %w", item.ID, err)
		}
		s.items = append(s.items, instance{item.ID, item.Transform, inverse, index})
	}
	sort.Slice(s.items, func(i, j int) bool { return s.items[i].id < s.items[j].id })
	return s, nil
}

// PickResult identifies the hit triangle and its barycentric coordinates in the
// original asset. Point is in world coordinates; Distance starts at the near
// plane rather than the camera eye.
type PickResult struct {
	ID          string
	Triangle    int
	Point       geom.Vec3
	Barycentric geom.Vec3
	Distance    float64
}

func (s *Scene) Pick(camera Camera, screen geom.Vec2) (PickResult, bool, error) {
	if s == nil {
		return PickResult{}, false, fmt.Errorf("scene: nil scene")
	}
	r, err := camera.Ray(screen)
	if err != nil {
		return PickResult{}, false, err
	}
	best := PickResult{Distance: r.Length}
	found := false
	for _, item := range s.items {
		dir := item.local.ApplyVector(r.Direction)
		scale := dir.Length()
		length := best.Distance * scale
		if scale <= 0 || !finite(scale) || !finite(length) {
			return PickResult{}, false, fmt.Errorf("scene: invalid local ray for %q", item.id)
		}
		if length == 0 {
			if best.Distance != 0 {
				return PickResult{}, false, fmt.Errorf("scene: local ray underflow for %q", item.id)
			}
			break
		}
		h, ok, err := item.index.Ray(item.local.Apply(r.Origin), dir, length)
		if err != nil {
			return PickResult{}, false, err
		}
		if !ok {
			continue
		}
		distance := h.Distance / scale
		point := item.world.Apply(h.Point)
		if !point.IsFinite() || !finite(distance) {
			return PickResult{}, false, fmt.Errorf("scene: world hit outside float64 range")
		}
		if distance > r.Length || found && distance >= best.Distance {
			continue
		}
		best = PickResult{item.id, h.Triangle, point, h.Barycentric, distance}
		found = true
	}
	return best, found, nil
}

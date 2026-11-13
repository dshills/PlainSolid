package document

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"plainsolid/brep"
	"plainsolid/geom"
)

// FeatureFace is a stable semantic reference to a supported generated face.
// It is valid only in the feature graph that defines Feature. Transforms
// and cylinder bores preserve the face indices of their inputs.
type FeatureFace struct {
	Feature string
	Role    string
}

// ResolveWrappedStationLoftFace resolves lower-cap, upper-cap or wall:<i>
// on a station loft feature. A wall index identifies the band between
// station i and i+1. Radius/height edits and placement transforms preserve
// these roles; inserting or deleting a station changes later band indices.
func (s State) ResolveWrappedStationLoftFace(ref FeatureFace) (int, error) {
	seen := map[string]bool{}
	for id := ref.Feature; id != ""; {
		if seen[id] {
			return 0, fmt.Errorf("document: feature cycle at %q", id)
		}
		seen[id] = true
		f, ok := s.Features[id]
		if !ok {
			return 0, fmt.Errorf("document: missing feature %q", id)
		}
		switch f.Kind {
		case TransformFeature:
			if _, e := f.Transform.Inverse(); e != nil {
				return 0, e
			}
			id = f.Input
		case WrappedStationLoftFeature:
			n := len(f.Stations) - 1
			if n < 1 || n > 127 {
				return 0, fmt.Errorf("document: invalid station loft")
			}
			switch ref.Role {
			case "lower-cap":
				return n, nil
			case "upper-cap":
				return n + 1, nil
			}
			if role, ok := strings.CutPrefix(ref.Role, "wall:"); ok {
				i, e := strconv.Atoi(role)
				if e == nil && i >= 0 && i < n && strconv.Itoa(i) == role {
					return i, nil
				}
			}
			return 0, fmt.Errorf("document: unknown station loft face role %q", ref.Role)
		default:
			return 0, fmt.Errorf("document: feature kind %q has no station loft face roles", f.Kind)
		}
	}
	return 0, fmt.Errorf("document: empty feature reference")
}

// ResolveBoxFace resolves one of x-min, x-max, y-min, y-max, z-min or z-max
// from a box feature (possibly passed through transforms or a bore). It uses
// the untransformed generator's orientation to assign names and retains the
// same face index under size edits and nonsingular affine transforms.
func (s State) ResolveBoxFace(ref FeatureFace) (int, error) {
	roles := map[string]geom.Vec3{
		"x-min": {X: -1}, "x-max": {X: 1}, "y-min": {Y: -1}, "y-max": {Y: 1}, "z-min": {Z: -1}, "z-max": {Z: 1},
	}
	want, ok := roles[ref.Role]
	if !ok {
		return 0, fmt.Errorf("document: unknown box face role %q", ref.Role)
	}
	id := ref.Feature
	seen := map[string]bool{}
	for id != "" {
		if seen[id] {
			return 0, fmt.Errorf("document: feature cycle at %q", id)
		}
		seen[id] = true
		f, ok := s.Features[id]
		if !ok {
			return 0, fmt.Errorf("document: missing feature %q", id)
		}
		switch f.Kind {
		case TransformFeature:
			if _, e := f.Transform.Inverse(); e != nil {
				return 0, e
			}
			id = f.Input
		case CylinderBoreFeature:
			id = f.Input
		case BoxFeature:
			if f.Input != "" {
				return 0, fmt.Errorf("document: box cannot have an input")
			}
			b, e := brep.Box(f.Min, f.Max)
			if e != nil {
				return 0, e
			}
			for i := range b.Faces() {
				p, e := b.FacePlane(i)
				if e != nil {
					return 0, e
				}
				if p.Normal().Dot(want) > 0.999999999999 {
					return i, nil
				}
			}
			return 0, fmt.Errorf("document: missing semantic face")
		default:
			return 0, fmt.Errorf("document: feature kind %q has no stable box face roles", f.Kind)
		}
	}
	return 0, fmt.Errorf("document: empty feature reference")
}

// BoxFaceRoles returns the accepted semantic roles in sorted order.
func BoxFaceRoles() []string {
	a := []string{"x-min", "x-max", "y-min", "y-max", "z-min", "z-max"}
	sort.Strings(a)
	return a
}

// ResolveWrappedCylinderFace resolves wall, lower-cap and upper-cap on
// supported cylinder features. A fillet also has lower-blend or upper-blend
// when its corresponding radius is positive. Transform features preserve
// the assigned face index.
func (s State) ResolveWrappedCylinderFace(ref FeatureFace) (int, error) {
	roles := map[string]int{"wall": 0, "lower-cap": 1, "upper-cap": 2, "lower-blend": 3, "upper-blend": 4}
	role, ok := roles[ref.Role]
	if !ok {
		return 0, fmt.Errorf("document: unknown wrapped cylinder face role %q", ref.Role)
	}
	seen := map[string]bool{}
	for id := ref.Feature; id != ""; {
		if seen[id] {
			return 0, fmt.Errorf("document: feature cycle at %q", id)
		}
		seen[id] = true
		f, ok := s.Features[id]
		if !ok {
			return 0, fmt.Errorf("document: missing feature %q", id)
		}
		switch f.Kind {
		case WrappedCylinderFeature, WrappedDraftFeature, WrappedBoreFeature, WrappedFrustumOffsetFeature:
			if role > 2 {
				return 0, fmt.Errorf("document: wrapped cylinder feature has no blend")
			}
			return role, nil
		case WrappedCylinderOffsetFeature:
			// The result may gain a blend at either formerly sharp rim.
			// Resolve the evaluated geometry instead of inferring its
			// topology from the sign of the offset or from the input order.
			state, e := s.EvaluateFeatures(context.Background())
			if e != nil {
				return 0, e
			}
			solid := state.Trimmed[id]
			if solid.Topology().Faces == 3 {
				if role > 2 {
					return 0, fmt.Errorf("document: wrapped cylinder offset has no blend")
				}
				w, proof, e := solid.ValidateCanonicalWrappedCylinder(context.Background(), geom.SurfacePatchOptions{})
				if e != nil {
					return 0, e
				}
				z := proof.LowerZ
				if role == 2 {
					z = proof.UpperZ
				}
				for i, face := range solid.Faces() {
					if role == 0 && face.Surface == w {
						return i, nil
					}
					if plane, ok := face.Surface.(geom.PlaneSurface); role != 0 && ok {
						if new(big.Rat).SetFloat64(plane.Frame().Origin().Z).Cmp(z) == 0 {
							return i, nil
						}
					}
				}
				return 0, geom.ErrConvergence
			}
			p, e := solid.ValidateWrappedCylinderRimFillet(context.Background(), geom.SurfacePatchOptions{})
			if e != nil {
				return 0, e
			}
			index := [5]int{p.Lateral, p.Caps[0], p.Caps[1], p.Blends[0], p.Blends[1]}[role]
			if index < 0 {
				return 0, fmt.Errorf("document: selected rim is sharp")
			}
			return index, nil
		case WrappedCylinderFilletFeature:
			if math.IsNaN(f.Radius) || math.IsNaN(f.Distance) || math.IsInf(f.Radius, 0) || math.IsInf(f.Distance, 0) ||
				f.Radius < 0 || f.Distance < 0 || f.Radius == 0 && f.Distance == 0 {
				return 0, fmt.Errorf("document: invalid wrapped cylinder fillet radii")
			}
			if role == 0 {
				return role, nil
			}
			if role == 3 {
				if f.Radius == 0 {
					return 0, fmt.Errorf("document: lower rim is sharp")
				}
				return 1, nil
			}
			if role == 4 {
				if f.Distance == 0 {
					return 0, fmt.Errorf("document: upper rim is sharp")
				}
				if f.Radius > 0 {
					return 2, nil
				}
				return 1, nil
			}
			n := 0
			for _, r := range []float64{f.Radius, f.Distance} {
				if r > 0 {
					n++
				}
			}
			return role + n, nil
		case TransformFeature:
			if _, e := f.Transform.Inverse(); e != nil {
				return 0, e
			}
			id = f.Input
		default:
			return 0, fmt.Errorf("document: feature kind %q has no wrapped cylinder face roles", f.Kind)
		}
	}
	return 0, fmt.Errorf("document: empty feature reference")
}

// ResolveWrappedStationBoreFace resolves lower-cap, upper-cap,
// outer-wall:<index> and inner-wall:<index>. Band order follows exact
// model heights. Transform recipes preserve the resulting face indices.
func (s State) ResolveWrappedStationBoreFace(ref FeatureFace) (int, error) {
	seen := make(map[string]bool)
	for id := ref.Feature; id != ""; {
		if seen[id] {
			return 0, fmt.Errorf("document: feature cycle at %q", id)
		}
		seen[id] = true
		f, ok := s.Features[id]
		if !ok {
			return 0, fmt.Errorf("document: missing feature %q", id)
		}
		if f.Kind == TransformFeature {
			if _, e := f.Transform.Inverse(); e != nil {
				return 0, e
			}
			id = f.Input
			continue
		}
		if f.Kind != WrappedStationBoreFeature {
			return 0, fmt.Errorf("document: feature kind %q has no station bore roles", f.Kind)
		}
		ctx := context.Background()
		state, e := s.EvaluateFeatures(ctx)
		if e != nil {
			return 0, e
		}
		p, e := state.Trimmed[id].ValidateWrappedCircularStationBore(ctx, geom.SurfacePatchOptions{})
		if e != nil {
			return 0, e
		}
		switch ref.Role {
		case "lower-cap":
			return p.Caps[0], nil
		case "upper-cap":
			return p.Caps[1], nil
		}
		for i, prefix := range []string{"outer-wall:", "inner-wall:"} {
			if rest, ok := strings.CutPrefix(ref.Role, prefix); ok {
				n, e := strconv.Atoi(rest)
				ids := [2][]int{p.OuterWalls, p.InnerWalls}[i]
				if e == nil && strconv.Itoa(n) == rest && n >= 0 && n < len(ids) {
					return ids[n], nil
				}
			}
		}
		return 0, fmt.Errorf("document: invalid station bore face role %q", ref.Role)
	}
	return 0, fmt.Errorf("document: empty feature reference")
}

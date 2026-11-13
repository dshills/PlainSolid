package document

import (
	"context"
	"fmt"
	"sort"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/csg"
	"plainsolid/csg/draft"
	sketchfeature "plainsolid/csg/feature"
	"plainsolid/csg/fillet"
	"plainsolid/csg/offset"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

// FeatureKind selects an exact construction supported by the document evaluator.
type FeatureKind string

const (
	PlanarProtrusionFeature      FeatureKind = "planar-protrusion"
	PlanarPocketFeature          FeatureKind = "planar-pocket"
	PlanarThroughPocketFeature   FeatureKind = "planar-through-pocket"
	PlanarToFaceFeature          FeatureKind = "planar-to-face"
	PlanarRevolveFeature         FeatureKind = "planar-revolve"
	LongCapChainFeature          FeatureKind = "long-cap-chain"
	HelicalGrooveFeature         FeatureKind = "helical-groove"
	HelicalPairPartFeature       FeatureKind = "helical-pair-part"
	BoxFeature                   FeatureKind = "box"
	PrismFeature                 FeatureKind = "prism"
	SphereFeature                FeatureKind = "sphere"
	RoundedBoxFeature            FeatureKind = "rounded-box"
	PlanarOffsetFeature          FeatureKind = "planar-offset"
	TransformFeature             FeatureKind = "transform"
	BooleanFeature               FeatureKind = "planar-boolean"
	CylinderBoreFeature          FeatureKind = "cylinder-bore"
	PrismFilletFeature           FeatureKind = "prism-corner-fillet"
	WrappedCylinderFeature       FeatureKind = "wrapped-cylinder"
	WrappedDraftFeature          FeatureKind = "wrapped-draft"
	WrappedBoreFeature           FeatureKind = "wrapped-bore"
	WrappedStationLoftFeature    FeatureKind = "wrapped-station-loft"
	WrappedFrustumOffsetFeature  FeatureKind = "wrapped-frustum-offset"
	WrappedCylinderFilletFeature FeatureKind = "wrapped-cylinder-fillet"
	WrappedCylinderOffsetFeature FeatureKind = "wrapped-cylinder-offset"
	WrappedStationBoreFeature    FeatureKind = "wrapped-station-bore"
)

// Feature describes a reproducible construction. Asset IDs stay stable across
// edits. Input is required for offset, transform and Boolean. Boolean also
// requires Tool and uses Operation. Box uses Min and Max;
// Prism uses Profile, Holes and Offset; Sphere uses Center and Radius;
// RoundedBox uses Min, Max and Radius; planar offset uses Distance and
// Tolerance; transform uses Transform. A bore uses a box input (possibly
// transformed), EntryRole, ExitRole, Center, Direction, Radius and Tolerance.
// Center and Direction are in the input asset's coordinates. A zero Direction
// selects the inward normal of the entry face. A prism corner fillet uses a
// counterclockwise horizontal Profile and a vertical Offset. Radius rounds
// every vertical edge, or CornerRadii selects individual edges and radii.
// Planar protrusion/pocket uses a named box face through Input and EntryRole,
// an interior world-space Profile and full translation Offset; holes unsupported.
// PlanarToFace uses Input, EntryRole, Profile, Holes and Direction; its
// stable target role is resolved against the regenerated box or placed box.
// PlanarRevolve uses a world-space Profile, Center as a point on the directed
// axis, Direction as the axis, and Distance as the signed angle in radians.
// LongCapChain uses its explicit LongCapChainRecipe field.
// HelicalGroove uses HelicalGroove; HelicalPairPart uses HelicalPair.
// A wrapped cylinder uses Center as the lower cap center, Radius and Offset.Z
// as height. A wrapped draft uses a canonical cylinder Input and the lower
// and upper radial changes in Radius and Distance. A wrapped bore uses two
// canonical cylinder assets, Input and Tool. A station loft uses Center.X/Y
// and ordered model-space Stations. A frustum offset uses Input and signed
// Distance for the lateral normal offset, with caps at the shifted rims.
// A wrapped cylinder fillet uses Input and the lower/upper radii in
// Radius/Distance; zero leaves the corresponding rim sharp. A complete
// wrapped cylinder offset uses Input and the signed Distance. A station
// bore uses coaxial station lofts in Input and Tool.
type Feature struct {
	Kind                        FeatureKind
	Input                       string `json:",omitempty"`
	Min, Max, Center, Offset    geom.Vec3
	Radius, Distance, Tolerance float64
	Profile                     []geom.Vec3   `json:",omitempty"`
	Holes                       [][]geom.Vec3 `json:",omitempty"`
	Transform                   geom.Affine
	Tool                        string         `json:",omitempty"`
	Operation                   poly.Operation `json:",omitempty"`
	EntryRole, ExitRole         string         `json:",omitempty"`
	Direction                   geom.Vec3
	CornerRadii                 []fillet.CornerRadius        `json:",omitempty"`
	Stations                    []trimmed.WrappedLoftStation `json:",omitempty"`
	LongCapChain                *LongCapChainRecipe          `json:",omitempty"`
	HelicalGroove               *HelicalGrooveRecipe         `json:",omitempty"`
	HelicalPair                 *HelicalPairPartRecipe       `json:",omitempty"`
}

func (f Feature) clone() Feature {
	f.Profile = append([]geom.Vec3(nil), f.Profile...)
	f.CornerRadii = append([]fillet.CornerRadius(nil), f.CornerRadii...)
	f.Stations = append([]trimmed.WrappedLoftStation(nil), f.Stations...)
	if f.LongCapChain != nil {
		recipe := *f.LongCapChain
		f.LongCapChain = &recipe
	}
	if f.HelicalGroove != nil {
		recipe := f.HelicalGroove.clone()
		f.HelicalGroove = &recipe
	}
	if f.HelicalPair != nil {
		recipe := *f.HelicalPair
		recipe.Shaft, recipe.Nut = recipe.Shaft.clone(), recipe.Nut.clone()
		f.HelicalPair = &recipe
	}
	h := make([][]geom.Vec3, len(f.Holes))
	for i, x := range f.Holes {
		h[i] = append([]geom.Vec3(nil), x...)
	}
	if len(h) > 0 {
		f.Holes = h
	}
	return f
}

// EvaluateFeatures returns a copy of the state with all feature assets
// materialized and the recipes removed. Materialized assets retain feature
// IDs. Evaluation is deterministic and atomic. Invalid inputs, cycles, missing
// dependencies and operation errors return no partially evaluated state.
func (s State) EvaluateFeatures(ctx context.Context) (State, error) {
	if ctx == nil {
		return State{}, fmt.Errorf("document: nil evaluation context")
	}
	if e := ctx.Err(); e != nil {
		return State{}, e
	}
	if len(s.Features) > 10000 {
		return State{}, fmt.Errorf("document: feature count limit exceeded")
	}
	result := clone(s)
	result.Features = nil
	if result.Planar == nil {
		result.Planar = map[string]brep.Solid{}
	}
	if result.Parametric == nil {
		result.Parametric = map[string]parametric.Solid{}
	}
	if result.Trimmed == nil {
		result.Trimmed = map[string]trimmed.Solid{}
	}
	ids := make([]string, 0, len(s.Features))
	for id := range s.Features {
		if _, ok := s.Assets[id]; ok {
			return State{}, fmt.Errorf("document: feature %q conflicts with an asset", id)
		}
		if _, ok := s.Planar[id]; ok {
			return State{}, fmt.Errorf("document: feature %q conflicts with an asset", id)
		}
		if _, ok := s.Parametric[id]; ok {
			return State{}, fmt.Errorf("document: feature %q conflicts with an asset", id)
		}
		if _, ok := s.Trimmed[id]; ok {
			return State{}, fmt.Errorf("document: feature %q conflicts with an asset", id)
		}
		if id == "" {
			return State{}, fmt.Errorf("document: empty feature ID")
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	marks := map[string]uint8{}
	type item struct {
		name     string
		expanded bool
	}
	for _, id := range ids {
		stack := []item{{name: id}}
		for len(stack) > 0 {
			if e := ctx.Err(); e != nil {
				return State{}, e
			}
			x := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			name := x.name
			f, ok := s.Features[name]
			if !ok || marks[name] == 2 {
				continue
			}
			if !x.expanded {
				if marks[name] == 1 {
					return State{}, fmt.Errorf("document: feature cycle at %q", name)
				}
				marks[name] = 1
				stack = append(stack, item{name, true})
				needsInput := f.Kind == PlanarToFaceFeature || f.Kind == PlanarThroughPocketFeature || f.Kind == PlanarProtrusionFeature || f.Kind == PlanarPocketFeature || f.Kind == TransformFeature || f.Kind == PlanarOffsetFeature || f.Kind == BooleanFeature || f.Kind == CylinderBoreFeature ||
					f.Kind == WrappedDraftFeature || f.Kind == WrappedBoreFeature || f.Kind == WrappedFrustumOffsetFeature ||
					f.Kind == WrappedCylinderFilletFeature || f.Kind == WrappedCylinderOffsetFeature || f.Kind == WrappedStationBoreFeature
				needsTool := f.Kind == BooleanFeature || f.Kind == WrappedBoreFeature || f.Kind == WrappedStationBoreFeature
				if (f.Input != "") != needsInput || (f.Tool != "") != needsTool {
					return State{}, fmt.Errorf("document: invalid input for feature %q", name)
				}
				if f.Tool != "" {
					stack = append(stack, item{name: f.Tool})
				}
				if f.Input != "" {
					stack = append(stack, item{name: f.Input})
				}
				continue
			}

			var p brep.Solid
			var q parametric.Solid
			var r trimmed.Solid
			var e error
			curved, uv := false, false
			switch f.Kind {
			case BoxFeature:
				p, e = brep.Box(f.Min, f.Max)
			case PrismFeature:
				p, e = brep.PrismWithHoles(f.Profile, f.Holes, f.Offset)
			case SphereFeature:
				curved = true
				q, e = parametric.Sphere(f.Center, f.Radius)
			case RoundedBoxFeature:
				curved = true
				q, e = parametric.RoundedBox(f.Min, f.Max, f.Radius)
			case WrappedCylinderFeature, WrappedDraftFeature, WrappedBoreFeature, WrappedStationLoftFeature, WrappedFrustumOffsetFeature, WrappedCylinderFilletFeature, WrappedCylinderOffsetFeature, WrappedStationBoreFeature:
				uv = true
				r, e = evaluateWrappedFeature(ctx, result.Trimmed, f)
			case PrismFilletFeature:
				uv = true
				if len(f.Profile) < 3 || len(f.Profile) > 128 || len(f.Holes) != 0 || f.Offset.X != 0 || f.Offset.Y != 0 || f.Offset.Z <= 0 {
					e = fmt.Errorf("prism corner fillet needs a positive vertical extrusion and no holes")
					break
				}
				z := f.Profile[0].Z
				p := make([]geom.Vec2, len(f.Profile))
				for i, x := range f.Profile {
					if x.Z != z {
						e = fmt.Errorf("prism corner fillet needs a horizontal profile")
						break
					}
					p[i] = geom.Vec2{X: x.X, Y: x.Y}
				}
				if e == nil {
					if len(f.CornerRadii) == 0 {
						var x fillet.Result
						x, e = fillet.Prism(ctx, p, z, z+f.Offset.Z, f.Radius, fillet.Options{Tolerance: f.Tolerance})
						r = x.Solid
					} else if f.Radius != 0 {
						e = fmt.Errorf("prism corner fillet requires either Radius or CornerRadii")
					} else {
						var x fillet.SelectiveResult
						x, e = fillet.PrismCorners(ctx, p, z, z+f.Offset.Z, f.CornerRadii, fillet.Options{Tolerance: f.Tolerance})
						r = x.Solid
					}
				}
			case PlanarToFaceFeature:
				target, ok := result.Planar[f.Input]
				if !ok {
					e = fmt.Errorf("to-face extrusion requires planar target %q", f.Input)
					break
				}
				var face int
				face, e = s.ResolveBoxFace(FeatureFace{f.Input, f.EntryRole})
				if e == nil {
					p, e = brep.PrismToFace(ctx, f.Profile, f.Holes, f.Direction, target, face)
				}
			case PlanarRevolveFeature:
				uv = true
				if len(f.Holes) != 0 {
					e = fmt.Errorf("model-space polygon revolve does not accept sketch holes")
					break
				}
				r, e = trimmed.RevolvePlanarPolygon(ctx, f.Profile, f.Center, f.Direction, f.Distance)
			case LongCapChainFeature:
				uv = true
				r, e = evaluateLongCapChain(ctx, f)
			case HelicalGrooveFeature, HelicalPairPartFeature:
				uv = true
				r, e = evaluateHelicalFeature(ctx, f)
			case PlanarProtrusionFeature, PlanarPocketFeature, PlanarThroughPocketFeature:
				a, ok := result.Planar[f.Input]
				if !ok {
					return State{}, fmt.Errorf("document: feature %q needs planar input", name)
				}
				if len(f.Holes) != 0 || (f.Kind != PlanarThroughPocketFeature && f.ExitRole != "") {
					e = fmt.Errorf("planar sketch feature supports no holes or exit role")
					break
				}
				var selected int
				selected, e = s.ResolveBoxFace(FeatureFace{f.Input, f.EntryRole})
				if e != nil {
					break
				}
				kind := sketchfeature.Protrusion
				if f.Kind == PlanarPocketFeature {
					kind = sketchfeature.Pocket
				}
				var x sketchfeature.Result
				if f.Kind == PlanarThroughPocketFeature {
					var exit int
					exit, e = s.ResolveBoxFace(FeatureFace{f.Input, f.ExitRole})
					if e == nil {
						x, e = sketchfeature.ThroughPocket(ctx, a, selected, exit, f.Profile, f.Offset, sketchfeature.Options{})
					}
				} else {
					x, e = sketchfeature.Extrude(ctx, a, selected, f.Profile, f.Offset, kind, sketchfeature.Options{})
				}
				p = x.Solid
			case PlanarOffsetFeature:
				a, ok := result.Planar[f.Input]
				if !ok {
					return State{}, fmt.Errorf("document: feature %q needs planar input %q", name, f.Input)
				}
				p, e = csg.Offset(a, f.Distance, f.Tolerance)
			case BooleanFeature:
				a, ok := result.Planar[f.Input]
				b, hasTool := result.Planar[f.Tool]
				if !ok || !hasTool {
					return State{}, fmt.Errorf("document: feature %q requires two planar assets", name)
				}
				p, e = poly.Combine(ctx, a, b, f.Operation, poly.Options{})
			case CylinderBoreFeature:
				uv = true
				a, ok := result.Planar[f.Input]
				if !ok {
					return State{}, fmt.Errorf("document: feature %q needs planar input %q", name, f.Input)
				}
				var entry, exit int
				entry, e = s.ResolveBoxFace(FeatureFace{f.Input, f.EntryRole})
				if e != nil {
					break
				}
				exit, e = s.ResolveBoxFace(FeatureFace{f.Input, f.ExitRole})
				if e != nil {
					break
				}
				if f.Direction == (geom.Vec3{}) {
					r, _, e = trimmed.BoreCylinder(a, entry, exit, f.Center, f.Radius, f.Tolerance)
				} else {
					r, _, e = trimmed.BoreCylinderAlong(a, entry, exit, f.Center, f.Direction, f.Radius, f.Tolerance)
				}
			case TransformFeature:
				if a, ok := result.Planar[f.Input]; ok {
					p, e = a.Transformed(f.Transform)
				} else if a, ok := result.Parametric[f.Input]; ok {
					curved = true
					q, e = a.Transformed(f.Transform)
				} else if a, ok := result.Trimmed[f.Input]; ok {
					uv = true
					r, e = a.Transformed(f.Transform)
				} else {
					return State{}, fmt.Errorf("document: feature %q needs exact input %q", name, f.Input)
				}
			default:
				return State{}, fmt.Errorf("document: unsupported feature kind %q", f.Kind)
			}
			if e != nil {
				return State{}, fmt.Errorf("document: feature %q: %w", name, e)
			}
			if uv {
				result.Trimmed[name] = r
			} else if curved {
				result.Parametric[name] = q
			} else {
				result.Planar[name] = p
			}
			marks[name] = 2
		}
	}
	return result, nil
}

func evaluateWrappedFeature(ctx context.Context, assets map[string]trimmed.Solid, f Feature) (trimmed.Solid, error) {
	o := geom.SurfacePatchOptions{}
	tol := f.Tolerance
	if tol == 0 {
		tol = 1e-9
	}
	if f.Kind == WrappedCylinderFeature {
		if f.Offset.X != 0 || f.Offset.Y != 0 {
			return trimmed.Solid{}, fmt.Errorf("wrapped cylinder requires a vertical height")
		}
		w, e := geom.NewCanonicalWrappedCylinder(ctx, f.Center, f.Radius, f.Offset.Z, o)
		if e != nil {
			return trimmed.Solid{}, e
		}
		s, _, e := trimmed.NewWrappedCylinder(ctx, w, tol, o)
		return s, e
	}
	if f.Kind == WrappedStationLoftFeature {
		if !f.Center.IsFinite() || f.Center.Z != 0 {
			return trimmed.Solid{}, fmt.Errorf("document: station loft center uses XY; stations give absolute Z")
		}
		s, _, e := trimmed.LoftWrappedCircularStations(ctx, geom.Vec2{X: f.Center.X, Y: f.Center.Y}, f.Stations, tol, o)
		return s, e
	}
	input, ok := assets[f.Input]
	if !ok {
		return trimmed.Solid{}, fmt.Errorf("wrapped feature requires a canonical trimmed input")
	}
	if f.Kind == WrappedFrustumOffsetFeature {
		w, _, e := input.ValidateCanonicalWrappedFrustum(ctx, o)
		if e != nil {
			return trimmed.Solid{}, e
		}
		s, _, e := offset.WrappedCircularFrustum(ctx, w, f.Distance, tol, o)
		return s, e
	}
	if f.Kind == WrappedStationBoreFeature {
		tool, ok := assets[f.Tool]
		if !ok {
			return trimmed.Solid{}, fmt.Errorf("wrapped station bore requires a circular station loft tool")
		}
		s, h, e := trimmed.CombineCoaxialWrappedStationLofts(ctx, input, tool, parametric.Difference, o)
		if e != nil {
			return trimmed.Solid{}, e
		}
		if len(h.InnerBands) == 0 {
			return trimmed.Solid{}, geom.ErrDomain
		}
		return s, nil
	}
	if f.Kind == WrappedCylinderOffsetFeature {
		s, _, e := offset.WrappedRoundedCylinder(ctx, input, f.Distance, o)
		return s, e
	}
	if f.Kind == WrappedCylinderFilletFeature {
		s, _, e := fillet.WrappedCylinderRims(ctx, input, f.Radius, f.Distance, o)
		return s, e
	}
	w, _, e := input.ValidateCanonicalWrappedCylinder(ctx, o)
	if e != nil {
		return trimmed.Solid{}, e
	}
	if f.Kind == WrappedDraftFeature {
		s, _, e := draft.WrappedFrustum(ctx, w, f.Radius, f.Distance, tol, o)
		return s, e
	}
	tool, ok := assets[f.Tool]
	if !ok {
		return trimmed.Solid{}, fmt.Errorf("wrapped bore requires a canonical trimmed cylinder tool")
	}
	t, _, e := tool.ValidateCanonicalWrappedCylinder(ctx, o)
	if e != nil {
		return trimmed.Solid{}, e
	}
	s, _, e := trimmed.NewWrappedCylinderBore(ctx, w, t, tol, o)
	return s, e
}

package document

import (
	"context"
	"errors"
	"fmt"

	"plainsolid/csg/poly"
	"plainsolid/geom"
)

// NamedFaceStatus is the result of regenerating a semantic source face in a
// downstream planar feature. A split after subtraction can yield several
// boundary faces; an absent face is different from an invalid reference.
type NamedFaceStatus uint8

const (
	NamedFaceDeleted NamedFaceStatus = iota
	NamedFaceUnique
	NamedFaceMultiple
)

// PlanarFaceHistory is an address in the document feature graph, not a stored
// topology index. Source must identify an authored box and one of BoxFaceRoles;
// Target is that box or a downstream planar transform, Boolean, seam join,
// or inset join.
type PlanarFaceHistory struct {
	Source FeatureFace
	Target string
}

// ResolvedPlanarFace identifies an output boundary face of the evaluated
// target. Reversed is relative to the material side of the original source
// after transporting it through the feature chain.
type ResolvedPlanarFace struct {
	Index    int
	Reversed bool
}

// ResolvedPlanarFaceHistory is a complete set of surviving faces under the
// planar Boolean's owning-source policy. A single polygonal source may map to
// several conforming polygons. Face indices are valid for one evaluation.
type ResolvedPlanarFaceHistory struct {
	Status NamedFaceStatus
	Faces  []ResolvedPlanarFace
}

// ErrUnrelatedFeature is reported when a target does not descend from the
// referenced authored box. A genuine dependent Boolean can delete its face.
var ErrUnrelatedFeature = errors.New("document: target does not descend from named feature")

// ResolvePlanarFaceHistory evaluates and traces every owned fragment of one
// semantic box face through planar Booleans, joins and transforms. It never chooses
// a fragment by proximity, normal tolerance or its previous index. Coincident
// Boolean faces are attributed only to the operand that owns the result.
func (s State) ResolvePlanarFaceHistory(ctx context.Context, ref PlanarFaceHistory) (ResolvedPlanarFaceHistory, error) {
	var zero ResolvedPlanarFaceHistory
	if ctx == nil || ref.Source.Feature == "" || ref.Target == "" {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if f, ok := s.Features[ref.Source.Feature]; !ok || f.Kind != BoxFeature {
		return zero, fmt.Errorf("document: named source must be an authored box: %w", geom.ErrUnrepresentable)
	}
	original, err := s.ResolveBoxFace(ref.Source)
	if err != nil {
		return zero, err
	}
	if _, ok := s.Features[ref.Target]; !ok {
		return zero, fmt.Errorf("document: unknown target feature %q", ref.Target)
	}
	type lineage struct {
		related bool
		faces   map[int]bool // present index -> reversed
		err     error
	}
	cache := map[string]lineage{
		ref.Source.Feature: {related: true, faces: map[int]bool{original: false}},
	}
	observer := featureFaceObserver{
		needsHistory: func(f Feature) bool {
			a, b := cache[f.Input], cache[f.Tool]
			return (a.related || b.related) && a.err == nil && b.err == nil
		},
		record: func(id string, f Feature, geometry State, history poly.TopologyHistory) error {
			if id == ref.Source.Feature {
				return nil
			}
			a, b := cache[f.Input], cache[f.Tool]
			if !a.related && !b.related {
				return nil
			}
			next := lineage{related: true, err: errors.Join(a.err, b.err)}
			defer func() { cache[id] = next }()
			if next.err != nil {
				return nil
			}
			switch f.Kind {
			case TransformFeature:
				// Solid.Transformed preserves face order and adjusts winding
				// for reflected material before history is propagated.
				if in, iok := geometry.Planar[f.Input]; iok {
					if out, ook := geometry.Planar[id]; ook && len(in.Faces()) == len(out.Faces()) {
						next.faces = a.faces
						break
					}
				}
				next.err = geom.ErrUnrepresentable
			case BooleanFeature, PlanarSeamJoinFeature, PlanarInsetJoinFeature:
				sources := history.Faces
				if len(sources) != len(geometry.Planar[id].Faces()) {
					next.err = geom.ErrUnrepresentable
					break
				}
				prior := [2]lineage{a, b}
				next.faces = make(map[int]bool)
				for index, src := range sources {
					if src.Operand > poly.OperandB {
						next.err = geom.ErrUnrepresentable
						break
					}
					if reversed, ok := prior[src.Operand].faces[src.Face]; ok {
						next.faces[index] = reversed != src.Reversed
					}
				}
			default:
				next.err = fmt.Errorf("document: no planar face ownership through feature %q: %w", id, geom.ErrUnrepresentable)
			}
			return nil
		},
	}
	evaluated, err := s.evaluateFeatures(ctx, &observer)
	if err != nil {
		return zero, err
	}
	mapping := cache[ref.Target]
	if mapping.err != nil {
		return zero, mapping.err
	}
	if !mapping.related {
		return zero, ErrUnrelatedFeature
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	out := ResolvedPlanarFaceHistory{Status: NamedFaceDeleted}
	if len(mapping.faces) != 0 {
		out.Status = NamedFaceUnique
		if len(mapping.faces) > 1 {
			out.Status = NamedFaceMultiple
		}
		for index := range evaluated.Planar[ref.Target].Faces() {
			if reversed, ok := mapping.faces[index]; ok {
				out.Faces = append(out.Faces, ResolvedPlanarFace{Index: index, Reversed: reversed})
			}
		}
	}
	return out, nil
}

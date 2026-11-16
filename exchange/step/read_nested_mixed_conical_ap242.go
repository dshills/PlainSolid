package step

import (
	"context"
	"io"
	"math/big"

	"plainsolid/geom"
)

// NestedMixedConicalAP242Assembly retains the original two local products,
// a shared assembly-only definition, and the proved expanded occurrence paths.
type NestedMixedConicalAP242Assembly struct {
	Assembly    Assembly
	Group       AssemblyProduct
	Parts       map[int64]MixedConicalAP242Part
	Occurrences []ExactNAUOOccurrence
	Instances   []NestedMixedConicalAP242Instance
}

// NestedMixedConicalAP242Instance applies the exact composed pose to
// coordinates of its original product. Path stores the outer and inner
// original NAUO entity IDs in traversal order.
type NestedMixedConicalAP242Instance struct {
	ProductID int64
	Path      [2]int64
	pose      mappedRationalBasis
}

// Matrix returns an independent copy of the exact world pose.
func (u NestedMixedConicalAP242Instance) Matrix() [3][4]*big.Rat {
	return (ExactMappedOccurrence{pose: u.pose}).Matrix()
}

// ApplyExact returns the exact world coordinates of a represented local point.
func (u NestedMixedConicalAP242Instance) ApplyExact(p geom.Vec3) ([3]*big.Rat, error) {
	return (ExactMappedOccurrence{pose: u.pose}).ApplyExact(p)
}

// ReadNestedMixedConicalAP242Assembly admits a root with two to eight
// occurrences of one assembly-only group and up to eight original material
// links in that group. Both local solid families and all placement records
// are independently authenticated. Expanded worlds reuse local topology.
func ReadNestedMixedConicalAP242Assembly(ctx context.Context, r io.Reader, l Limits, o geom.SurfacePatchOptions) (NestedMixedConicalAP242Assembly, error) {
	var zero NestedMixedConicalAP242Assembly
	m, err := readMixedConicalAP242Assembly(ctx, r, l, o, true)
	if err != nil {
		return zero, err
	}
	var outer, inner []ExactNAUOOccurrence
	for _, u := range m.Occurrences {
		if u.ChildID == m.group.DefinitionID {
			outer = append(outer, u)
		} else {
			inner = append(inner, u)
		}
	}
	world := make([]NestedMixedConicalAP242Instance, 0, len(outer)*len(inner))
	for _, parent := range outer {
		for _, child := range inner {
			if err := ctx.Err(); err != nil {
				return zero, err
			}
			world = append(world, NestedMixedConicalAP242Instance{
				ProductID: child.ChildID, Path: [2]int64{parent.NAUOID, child.NAUOID},
				pose: composeMappedPose(parent.pose, child.pose),
			})
		}
	}
	return NestedMixedConicalAP242Assembly{
		Assembly: m.Assembly, Group: m.group, Parts: m.Parts, Occurrences: m.Occurrences, Instances: world,
	}, nil
}

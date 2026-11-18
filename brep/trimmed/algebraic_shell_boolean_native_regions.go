//go:build research

package trimmed

import (
	"context"
	"fmt"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

// NativeTubeRegionComponents materializes all retained two-support regions of
// a proved algebraic Boolean, including a mixture of disks and regions with
// holes. Every wire must be a whole original closed section, paired once on
// the same original supports. Original shell edges and partial wires remain
// unsupported. Components follow original outer-branch order; holes follow
// original branch order. Nothing is published on a failed component.
//
// The Boolean proof remains authoritative for exact algebraic junctions and
// material separation; native vertices are represented witnesses. Persist the
// original Boolean recipe to reprove this result, not an asserted component list.
// Limits are 256 pieces per ring and 512 edges in aggregate; legacy adapters
// retain 128 pieces per ring. Broader native serialization is not implied, and
// existing persistence limits remain unchanged.
func (s AlgebraicShellBoolean) NativeTubeRegionComponents(ctx context.Context, tolerance float64, o edgecheck.Options) ([]Solid, error) {
	if tolerance <= 0 || !finite(tolerance) {
		return nil, geom.ErrInvalid
	}
	layouts, e := s.nativeTubeRegions(ctx)
	if e != nil {
		return nil, e
	}
	var result []Solid
	for _, layout := range layouts {
		var rings []tubeRegionSolidRing
		for i, branch := range layout.Branches {
			ring, e := s.nativeWholeTubeRingBounded(ctx, branch, layout.Uses[i], 256)
			if e != nil {
				return nil, e
			}
			rings = append(rings, ring)
		}
		component, e := assembleTubeRegionSolidBounded(ctx, layout.Faces, rings, tolerance, o, 256)
		if e != nil {
			return nil, fmt.Errorf("trimmed: tube region outer branch %d: %w", layout.Branches[0], e)
		}
		result = append(result, component)
	}
	return result, nil
}

type nativeTubeRegionLayout struct {
	Owners   [2]int
	Faces    [2]Face
	Branches []int // outer then original-branch-ordered holes
	Uses     [][2][]AlgebraicShellBooleanUse
}

func (s AlgebraicShellBoolean) nativeTubeRegions(ctx context.Context) ([]nativeTubeRegionLayout, error) {
	if ctx == nil || s.source == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if len(s.faces) == 0 {
		return nil, nil
	}
	if len(s.faces) > 32 || len(s.source.branches) > 16 {
		return nil, geom.ErrConvergence
	}
	// Validate every original edge occurrence before consulting private indexing.
	totalUses := 0
	branchOf := func(uses []AlgebraicShellBooleanUse) (int, error) {
		if len(uses) < 3 || len(uses) > 256 {
			return -1, geom.ErrConvergence
		}
		totalUses += len(uses)
		if totalUses > 1024 {
			return -1, geom.ErrConvergence
		}
		branch := -1
		for _, use := range uses {
			if use.Edge < 0 || use.Edge >= len(s.index) {
				return -1, geom.ErrInvalid
			}
			edge := s.index[use.Edge]
			if edge.OriginalSubdivisionEdge >= 0 || edge.SectionPiece < 0 || edge.SectionPiece >= len(s.parts[0].Section) {
				return -1, geom.ErrConvergence
			}
			q := s.parts[0].Section[edge.SectionPiece].Branch
			if q < 0 || q >= len(s.source.branches) || branch >= 0 && branch != q {
				return -1, geom.ErrConvergence
			}
			branch = q
		}
		return branch, nil
	}
	owners := make([][2]int, len(s.source.branches))
	for i := range owners {
		owners[i] = [2]int{-1, -1}
	}
	holes := make([]map[int][]AlgebraicShellBooleanUse, len(s.faces))
	for i, f := range s.faces {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if f.Operand < 0 || f.Operand > 1 || len(f.Holes) > 8 {
			return nil, geom.ErrConvergence
		}
		branch, e := branchOf(f.Outer)
		if e != nil {
			return nil, e
		}
		if owners[branch][f.Operand] >= 0 {
			return nil, geom.ErrConvergence
		}
		owners[branch][f.Operand] = i
		holes[i] = make(map[int][]AlgebraicShellBooleanUse)
		for _, loop := range f.Holes {
			h, e := branchOf(loop)
			if e != nil {
				return nil, e
			}
			if h == branch || holes[i][h] != nil {
				return nil, geom.ErrConvergence
			}
			holes[i][h] = loop
		}
	}
	used := make([]bool, len(owners))
	var result []nativeTubeRegionLayout
	for outer, ids := range owners {
		if ids == [2]int{-1, -1} {
			continue
		}
		if ids[0] < 0 || ids[1] < 0 || used[outer] {
			return nil, geom.ErrConvergence
		}
		if len(holes[ids[0]]) != len(holes[ids[1]]) {
			return nil, geom.ErrConvergence
		}
		pair := s.source.branches[outer].Faces
		var faces [2]Face
		var loops [2][]AlgebraicShellBooleanUse
		for operand, id := range ids {
			f := s.faces[id]
			if f.Original != pair[operand] {
				return nil, geom.ErrConvergence
			}
			orig, e := s.OriginalFace(id)
			if e != nil {
				return nil, e
			}
			faces[operand] = Face{Surface: orig.Surface, Reversed: f.Reversed}
			loops[operand] = f.Outer
		}
		_, e := s.nativeWholeTubeRingOrder(ctx, outer, loops, 256)
		if e != nil {
			return nil, e
		}
		component := nativeTubeRegionLayout{Owners: ids, Faces: faces, Branches: []int{outer}, Uses: [][2][]AlgebraicShellBooleanUse{loops}}
		used[outer] = true
		for h := range owners {
			left, has := holes[ids[0]][h]
			if !has {
				continue
			}
			right, has := holes[ids[1]][h]
			if !has || used[h] || owners[h] != [2]int{-1, -1} || s.source.branches[h].Faces != pair {
				return nil, geom.ErrConvergence
			}
			_, e := s.nativeWholeTubeRingOrder(ctx, h, [2][]AlgebraicShellBooleanUse{left, right}, 256)
			if e != nil {
				return nil, e
			}
			component.Branches = append(component.Branches, h)
			component.Uses = append(component.Uses, [2][]AlgebraicShellBooleanUse{left, right})
			used[h] = true
		}
		result = append(result, component)
	}
	return result, nil
}

package step

import (
	"context"
	"io"
	"math/big"
	"slices"

	"plainsolid/geom"
)

const (
	maxMixedConicalAP242OccurrenceDepth = 8
	maxMixedConicalAP242Instances       = 256
	maxMixedConicalAP242PoseBits        = 16 << 10
)

// MixedConicalAP242DAGLimits bounds the original STEP parser, occurrence
// path length and expanded leaf count independently. Zero selects the
// defaults of eight relationships per path and 256 leaf instances.
type MixedConicalAP242DAGLimits struct {
	Source                               Limits
	MaxOccurrenceDepth, MaxLeafInstances int
}

// MixedConicalAP242DAG retains one rooted acyclic product graph, the two
// proved local parts and their expanded rational world occurrences.
// Assembly-only products are identified by their absence from Parts.
type MixedConicalAP242DAG struct {
	Assembly    Assembly
	Parts       map[int64]MixedConicalAP242Part
	Occurrences []ExactNAUOOccurrence
	Instances   []RationalHierarchyInstance
}

// ReadMixedConicalAP242DAG strictly reads one AP242 assembly with up to 16
// definitions, 64 original occurrences and the usual two conical materials.
// Both leaves retain their original topology in file units. Original NAUO
// IDs identify each path in source sibling order, and every composed pose
// remains rational with numerator and denominator sizes bounded at 16K bits.
func ReadMixedConicalAP242DAG(ctx context.Context, r io.Reader, l MixedConicalAP242DAGLimits, o geom.SurfacePatchOptions) (MixedConicalAP242DAG, error) {
	var zero MixedConicalAP242DAG
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	l, err := mixedConicalAP242DAGLimits(l)
	if err != nil {
		return zero, err
	}
	h, err := readMixedConicalAP242HierarchyMode(r, l.Source, mixedConicalAP242DAG)
	if err != nil {
		return zero, err
	}
	worlds, err := mixedConicalAP242DAGWorlds(ctx, h.assembly.Roots[0], h.records.occurrences, l)
	if err != nil {
		return zero, err
	}
	m, err := readMixedConicalAP242Materials(ctx, h, l.Source, o)
	if err != nil {
		return zero, err
	}
	return MixedConicalAP242DAG{Assembly: m.Assembly, Parts: m.Parts, Occurrences: m.Occurrences, Instances: worlds}, nil
}

func mixedConicalAP242DAGLimits(l MixedConicalAP242DAGLimits) (MixedConicalAP242DAGLimits, error) {
	if l.MaxOccurrenceDepth == 0 {
		l.MaxOccurrenceDepth = maxMixedConicalAP242OccurrenceDepth
	}
	if l.MaxLeafInstances == 0 {
		l.MaxLeafInstances = maxMixedConicalAP242Instances
	}
	if l.MaxOccurrenceDepth < 1 || l.MaxOccurrenceDepth > maxMixedConicalAP242OccurrenceDepth ||
		l.MaxLeafInstances < 1 || l.MaxLeafInstances > maxMixedConicalAP242Instances {
		return l, geom.ErrInvalid
	}
	sourceLimits, err := occurrenceColorSourceLimits(l.Source)
	if err != nil {
		return l, err
	}
	l.Source = sourceLimits
	return l, nil
}

// ReadAssembly already proves a rooted acyclic graph and all referenced
// products. Only assembly definitions may own uses, and each one must have
// children. In particular a representation of a solid always stays a leaf.
func mixedConicalAP242DAGUseIndex(a Assembly, layout mixedConicalAP242Layout) (map[int64]AssemblyOccurrence, error) {
	uses := make(map[int64]AssemblyOccurrence, len(a.Occurrences))
	parents := map[int64]bool{}
	for _, u := range a.Occurrences {
		if layout.parts[u.Parent] != 0 {
			return nil, errMixedConicalAP242
		}
		parents[u.Parent] = true
		uses[u.ID] = u
	}
	for _, p := range a.Products {
		if layout.parts[p.DefinitionID] == 0 && !parents[p.DefinitionID] {
			return nil, errMixedConicalAP242
		}
	}
	return uses, nil
}

type mixedConicalAP242BranchSize struct {
	depth, leaves int
}

// Count by definition before expanding paths. Each incoming use contributes
// its child's whole count, even when the child's subtree was memoized.
func mixedConicalAP242DAGWorlds(ctx context.Context, root int64, uses []ExactNAUOOccurrence, l MixedConicalAP242DAGLimits) ([]RationalHierarchyInstance, error) {
	children := map[int64][]ExactNAUOOccurrence{}
	for _, u := range uses {
		children[u.ParentID] = append(children[u.ParentID], u)
	}
	sizes := map[int64]mixedConicalAP242BranchSize{}
	var size func(int64) (mixedConicalAP242BranchSize, error)
	size = func(id int64) (mixedConicalAP242BranchSize, error) {
		if err := ctx.Err(); err != nil {
			return mixedConicalAP242BranchSize{}, err
		}
		if n, ok := sizes[id]; ok {
			return n, nil
		}
		var n mixedConicalAP242BranchSize
		if len(children[id]) == 0 {
			n.leaves = 1
		}
		for _, u := range children[id] {
			child, err := size(u.ChildID)
			if err != nil {
				return n, err
			}
			n.depth = max(n.depth, child.depth+1)
			if n.depth > l.MaxOccurrenceDepth || child.leaves > l.MaxLeafInstances-n.leaves {
				return n, geom.ErrConvergence
			}
			n.leaves += child.leaves
		}
		sizes[id] = n
		return n, nil
	}
	n, err := size(root)
	if err != nil {
		return nil, err
	}
	out := make([]RationalHierarchyInstance, 0, n.leaves)
	path := make([]int64, 0, n.depth)
	var walk func(int64, mappedRationalBasis) error
	walk = func(id int64, pose mappedRationalBasis) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(children[id]) == 0 {
			out = append(out, RationalHierarchyInstance{ProductID: id, Path: slices.Clone(path), pose: pose})
			return nil
		}
		for _, u := range children[id] {
			next := u.pose
			if len(path) != 0 {
				next = composeMappedPose(pose, next)
			}
			for _, col := range [][3]*big.Rat{next.X, next.Y, next.Z, next.Origin} {
				for _, q := range col {
					if q.Num().BitLen() > maxMixedConicalAP242PoseBits || q.Denom().BitLen() > maxMixedConicalAP242PoseBits {
						return geom.ErrConvergence
					}
				}
			}
			path = append(path, u.NAUOID)
			if err := walk(u.ChildID, next); err != nil {
				return err
			}
			path = path[:len(path)-1]
		}
		return nil
	}
	if err := walk(root, mappedRationalBasis{}); err != nil {
		return nil, err
	}
	return out, nil
}

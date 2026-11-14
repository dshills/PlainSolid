package step

import (
	"context"
	"io"
	"plainsolid/geom"
	"sort"
)

// IndependentAnalyticHierarchy retains three shared local materials and a
// bounded proper signed-coordinate product DAG. Each expanded Instance has its
// complete original NAUO path and exact represented root-space transform.
type IndependentAnalyticHierarchy IndependentAnalyticDAGAssembly

// IndependentAnalyticHierarchyModel supplies nil Source for assembly-only
// groups and authenticated independent sources for the three local materials.
type IndependentAnalyticHierarchyModel IndependentTorusAssemblyModel

// ReadIndependentAnalyticHierarchy permits reachable assembly-only groups with
// at most 16 products, 64 source occurrences, eight occurrence edges per path,
// and 64 expanded material leaves. One planar, cylinder and torus local product
// are required; the torus is reused at least twice. Cycles and unused groups fail.
func ReadIndependentAnalyticHierarchy(ctx context.Context, r io.Reader, l Limits, tol float64) (IndependentAnalyticHierarchy, error) {
	m, e := readIndependentTorusAssembly(ctx, r, l, tol, true, true, true)
	if e != nil {
		return IndependentAnalyticHierarchy{}, e
	}
	paths, e := independentAnalyticHierarchyPaths(ctx, m)
	if e != nil {
		return IndependentAnalyticHierarchy{}, e
	}
	return IndependentAnalyticHierarchy{Assembly: m.Assembly, Products: m.Products, Instances: paths}, nil
}

// WriteIndependentAnalyticHierarchy preserves shared products in standard STEP
// relationships, then re-proves the staged graph and all expanded paths.
func WriteIndependentAnalyticHierarchy(ctx context.Context, w io.Writer, m IndependentAnalyticHierarchyModel, l Limits, tol float64) error {
	return writeIndependentTorusAssembly(ctx, w, IndependentTorusAssemblyModel(m), l, tol, true, true, true)
}

// WriteIndependentAnalyticHierarchySource retains the original entity IDs and
// original source graph after re-proving its geometry and occurrence ownership.
func WriteIndependentAnalyticHierarchySource(ctx context.Context, w io.Writer, source *File, l Limits, tol float64) error {
	return writeIndependentTorusAssemblySource(ctx, w, source, l, tol, true, true, true)
}

func independentAnalyticHierarchyPaths(ctx context.Context, m IndependentTorusAssembly, sphereMode ...bool) ([]IndependentAnalyticDAGInstance, error) {
	bad := func() ([]IndependentAnalyticDAGInstance, error) { return nil, geom.ErrInvalid }
	a := m.Assembly
	sphere := len(sphereMode) >= 1 && sphereMode[0]
	frustum := len(sphereMode) == 2 && sphereMode[1]
	if len(a.Products) < 4 || len(a.Products) > 16 || len(a.Roots) != 1 || len(a.Occurrences) > 64 || (len(m.Products) != 3 && !(sphere && len(m.Products) == 4) && !(frustum && len(m.Products) == 5)) {
		return bad()
	}
	nodes := map[int64]bool{}
	var torus int64
	for _, p := range a.Products {
		if nodes[p.DefinitionID] {
			return bad()
		}
		nodes[p.DefinitionID] = true
		if q, ok := m.Products[p.DefinitionID]; ok && q.Torus != nil {
			if torus != 0 {
				return bad()
			}
			torus = p.DefinitionID
		}
	}
	root := a.Roots[0]
	if !nodes[root] || m.Products[root].SourceBodyID != 0 || torus == 0 {
		return bad()
	}
	children := map[int64][]AssemblyOccurrence{}
	ids := map[int64]bool{}
	for _, u := range a.Occurrences {
		if !nodes[u.Parent] || !nodes[u.Child] || u.ID <= 0 || ids[u.ID] || !planarSplineRigid(u.Transform) || m.Products[u.Parent].SourceBodyID != 0 {
			return bad()
		}
		ids[u.ID] = true
		children[u.Parent] = append(children[u.Parent], u)
	}
	for id := range nodes {
		if m.Products[id].SourceBodyID == 0 && len(children[id]) == 0 {
			return bad()
		}
	}
	active, seen := map[int64]bool{}, map[int64]bool{}
	uses := map[int64]int{}
	paths := []IndependentAnalyticDAGInstance{}
	visits := 0
	var walk func(int64, geom.Affine, []int64) error
	walk = func(id int64, pose geom.Affine, path []int64) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		visits++
		if visits > 513 || len(path) > 8 || active[id] {
			return geom.ErrInvalid
		}
		seen[id] = true
		if _, ok := m.Products[id]; ok {
			if len(path) == 0 || len(paths) >= 64 {
				return geom.ErrInvalid
			}
			uses[id]++
			paths = append(paths, IndependentAnalyticDAGInstance{ProductID: id, Path: append([]int64(nil), path...), Transform: pose})
			return nil
		}
		active[id] = true
		for _, u := range children[id] {
			next, e := independentAnalyticDAGCompose(pose, u.Transform)
			if e != nil {
				return e
			}
			p := append(append([]int64(nil), path...), u.ID)
			if e = walk(u.Child, next, p); e != nil {
				return e
			}
		}
		delete(active, id)
		return nil
	}
	if e := walk(root, geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}, nil); e != nil {
		return nil, e
	}
	if len(seen) != len(nodes) || uses[torus] < 2 {
		return bad()
	}
	for id := range m.Products {
		if uses[id] == 0 {
			return bad()
		}
	}
	sort.Slice(paths, func(i, j int) bool {
		for k := 0; k < len(paths[i].Path) && k < len(paths[j].Path); k++ {
			if paths[i].Path[k] != paths[j].Path[k] {
				return paths[i].Path[k] < paths[j].Path[k]
			}
		}
		return len(paths[i].Path) < len(paths[j].Path)
	})
	return paths, nil
}

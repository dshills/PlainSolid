package step

import (
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// IndependentAnalyticDAGInstance retains one expanded leaf path through original
// NAUO IDs. Transform maps the unchanged local material into root coordinates.
type IndependentAnalyticDAGInstance struct {
	ProductID int64
	Path      []int64
	Transform geom.Affine
}

// IndependentAnalyticDAGAssembly has an assembly-only root, one planar leaf,
// and one analytic group reused twice. That group owns one cylinder occurrence
// and one torus occurrence. Materials are stored once; five leaf paths are explicit.
type IndependentAnalyticDAGAssembly struct {
	Assembly  Assembly
	Products  map[int64]IndependentExactMaterial
	Instances []IndependentAnalyticDAGInstance
}
type IndependentAnalyticDAGAssemblyModel IndependentTorusAssemblyModel

// ReadIndependentAnalyticDAGAssembly proves the original two-level graph and
// every local material, then exposes exact composed source occurrence paths.
func ReadIndependentAnalyticDAGAssembly(ctx context.Context, r io.Reader, l Limits, tol float64) (IndependentAnalyticDAGAssembly, error) {
	m, e := readIndependentTorusAssembly(ctx, r, l, tol, true, true)
	if e != nil {
		return IndependentAnalyticDAGAssembly{}, e
	}
	paths, e := independentAnalyticDAGPaths(m)
	if e != nil {
		return IndependentAnalyticDAGAssembly{}, e
	}
	return IndependentAnalyticDAGAssembly{Assembly: m.Assembly, Products: m.Products, Instances: paths}, nil
}

// WriteIndependentAnalyticDAGAssembly authors standard product relationships
// and re-proves the complete staged DAG before output.
func WriteIndependentAnalyticDAGAssembly(ctx context.Context, w io.Writer, m IndependentAnalyticDAGAssemblyModel, l Limits, tol float64) error {
	return writeIndependentTorusAssembly(ctx, w, IndependentTorusAssemblyModel(m), l, tol, true, true)
}

// WriteIndependentAnalyticDAGAssemblySource preserves accepted original IDs
// and declarations after complete graph and material reproof.
func WriteIndependentAnalyticDAGAssemblySource(ctx context.Context, w io.Writer, source *File, l Limits, tol float64) error {
	return writeIndependentTorusAssemblySource(ctx, w, source, l, tol, true, true)
}

func independentAnalyticDAGPaths(m IndependentTorusAssembly) ([]IndependentAnalyticDAGInstance, error) {
	bad := func() ([]IndependentAnalyticDAGInstance, error) { return nil, geom.ErrInvalid }
	a := m.Assembly
	if len(a.Products) != 5 || len(a.Roots) != 1 || len(a.Occurrences) != 5 || len(m.Products) != 3 {
		return bad()
	}
	root := a.Roots[0]
	var group, planar, torus, cylinder int64
	for _, p := range a.Products {
		if p.DefinitionID == root {
			if m.Products[root].SourceBodyID != 0 {
				return bad()
			}
			continue
		}
		b, ok := m.Products[p.DefinitionID]
		if !ok {
			if group != 0 {
				return bad()
			}
			group = p.DefinitionID
			continue
		}
		switch {
		case b.Planar != nil:
			if planar != 0 {
				return bad()
			}
			planar = p.DefinitionID
		case b.Torus != nil:
			if torus != 0 {
				return bad()
			}
			torus = p.DefinitionID
		case b.Cylinder != nil:
			if cylinder != 0 {
				return bad()
			}
			cylinder = p.DefinitionID
		default:
			return bad()
		}
	}
	if group == 0 || planar == 0 || torus == 0 || cylinder == 0 {
		return bad()
	}
	var groupUses []AssemblyOccurrence
	var planarUse *AssemblyOccurrence
	children := map[int64]AssemblyOccurrence{}
	for _, u := range a.Occurrences {
		if !planarSplineRigid(u.Transform) {
			return bad()
		}
		switch {
		case u.Parent == root && u.Child == group:
			groupUses = append(groupUses, u)
		case u.Parent == root && u.Child == planar:
			if planarUse != nil {
				return bad()
			}
			v := u
			planarUse = &v
		case u.Parent == group && (u.Child == torus || u.Child == cylinder):
			if _, ok := children[u.Child]; ok {
				return bad()
			}
			children[u.Child] = u
		default:
			return bad()
		}
	}
	if len(groupUses) != 2 || planarUse == nil || len(children) != 2 {
		return bad()
	}
	paths := []IndependentAnalyticDAGInstance{{ProductID: planar, Path: []int64{planarUse.ID}, Transform: planarUse.Transform}}
	for _, g := range groupUses {
		for _, id := range []int64{torus, cylinder} {
			u := children[id]
			pose, e := independentAnalyticDAGCompose(g.Transform, u.Transform)
			if e != nil {
				return nil, e
			}
			paths = append(paths, IndependentAnalyticDAGInstance{ProductID: id, Path: []int64{g.ID, u.ID}, Transform: pose})
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

// Both factors already have exact represented signed-coordinate frames. The
// rational sum also proves the composed translation, rather than rounding it.
func independentAnalyticDAGCompose(a, b geom.Affine) (geom.Affine, error) {
	var out geom.Affine
	if !planarSplineRigid(a) || !planarSplineRigid(b) {
		return out, geom.ErrInvalid
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 4; j++ {
			q := new(big.Rat)
			if j == 3 {
				q.SetFloat64(a[i][3])
			}
			for k := 0; k < 3; k++ {
				q.Add(q, new(big.Rat).Mul(new(big.Rat).SetFloat64(a[i][k]), new(big.Rat).SetFloat64(b[k][j])))
			}
			v, exact := q.Float64()
			if !exact || !finiteNumber(v) {
				return geom.Affine{}, geom.ErrUnrepresentable
			}
			out[i][j] = v
		}
	}
	if !planarSplineRigid(out) {
		return geom.Affine{}, geom.ErrInvalid
	}
	return out, nil
}

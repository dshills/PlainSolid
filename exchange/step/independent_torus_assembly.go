package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"plainsolid/geom"
)

// IndependentTorusAssembly retains two local products (one planar and one exact
// model-only ring torus) under a single flat assembly root. Occurrences are
// proper signed-coordinate rigid poses. Local materials are never baked.
type IndependentTorusAssembly struct {
	Assembly Assembly
	Products map[int64]IndependentExactMaterial
}

// IndependentTorusAssemblyPart supplies a previously authenticated independent
// product source. Assembly-only nodes have nil Source; each model API determines
// its permitted node topology.
type IndependentTorusAssemblyPart struct {
	ID, Name string
	Source   *File
}
type IndependentTorusAssemblyModel struct {
	Products    []IndependentTorusAssemblyPart
	Occurrences []AssemblyInstance
	LengthUnit  float64
}

// ReadIndependentTorusAssembly proves the explicit flat two-product assembly,
// exclusive local geometry, exact frame declarations, and complete local BReps.
// Rounded angular pcurves, nested groups and other material families fail.
func ReadIndependentTorusAssembly(ctx context.Context, r io.Reader, l Limits, tol float64) (IndependentTorusAssembly, error) {
	return readIndependentTorusAssembly(ctx, r, l, tol, false)
}
func readIndependentTorusAssembly(ctx context.Context, r io.Reader, l Limits, tol float64, cylinder bool, dagMode ...bool) (IndependentTorusAssembly, error) {
	products := 3
	if cylinder {
		products = 4
	}
	dag := cylinder && len(dagMode) >= 1 && dagMode[0]
	hierarchy := dag && len(dagMode) >= 2 && dagMode[1]
	sphere := hierarchy && len(dagMode) >= 3 && dagMode[2]
	frustum := sphere && len(dagMode) == 4 && dagMode[3]
	if dag {
		products = 5
	}
	bad := func() (IndependentTorusAssembly, error) {
		return IndependentTorusAssembly{}, geom.ErrInvalid
	}
	if ctx == nil || r == nil || !finiteNumber(tol) || tol <= 0 {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return IndependentTorusAssembly{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, int64(1<<20))
	l.MaxEntities = min(l.MaxEntities, 4096)
	l.MaxValues = min(l.MaxValues, 65536)
	l.MaxDepth = min(l.MaxDepth, 16)
	a, e := ReadAssembly(r, l)
	if e != nil {
		return IndependentTorusAssembly{}, e
	}
	if hierarchy {
		products = len(a.Products)
		if products < 4 || products > 16 || len(a.Occurrences) > 64 {
			return bad()
		}
	}
	if len(a.Products) != products || len(a.Roots) != 1 || len(a.Occurrences) < products || (!hierarchy && len(a.Occurrences) > 16) {
		return bad()
	}
	f := a.Source
	out := IndependentTorusAssembly{Assembly: a, Products: map[int64]IndependentExactMaterial{}}
	owned := map[int64]int64{}
	geometry := map[int64]bool{}
	roots := []int64{}
	var prefix string
	var havePrefix bool
	nf, nv, nu, nt, np, nc, ns, nfr := 0, 0, 0, 0, 0, 0, 0, 0
	for _, p := range a.Products {
		if e = ctx.Err(); e != nil {
			return IndependentTorusAssembly{}, e
		}
		if len(p.Representations) != 1 {
			return bad()
		}
		kind := "ADVANCED_BREP_SHAPE_REPRESENTATION"
		node := p.DefinitionID == a.Roots[0]
		if dag {
			if _, err := simple(f, p.Representations[0], "SHAPE_REPRESENTATION", 3); err == nil {
				node = true
			}
		}
		if node {
			kind = "SHAPE_REPRESENTATION"
		}
		rep, e := simple(f, p.Representations[0], kind, 3)
		if e != nil {
			return bad()
		}
		u, e := nauoSIPrefix(f, rep.Args[2].Ref)
		if e != nil || (u != "$" && u != "MILLI") {
			return bad()
		}
		if havePrefix && u != prefix {
			return bad()
		}
		prefix = u
		havePrefix = true
		ids, advanced, e := a.productItems(p.DefinitionID)
		if e != nil {
			return bad()
		}
		if node {
			if len(ids) != 0 {
				return bad()
			}
		} else {
			if len(ids) != 1 || !advanced[0] {
				return bad()
			}
			closure, e := nauoClosure(f, ids, l.MaxDepth)
			if e != nil {
				return bad()
			}
			for id := range closure {
				if owned[id] != 0 {
					return bad()
				}
				owned[id] = p.DefinitionID
				geometry[id] = true
			}
			viewIDs, e := nauoClosure(f, []int64{p.Representations[0]}, l.MaxDepth)
			if e != nil {
				return bad()
			}
			view := &File{Header: f.Header, Entities: map[int64]Entity{}}
			for _, id := range f.Order {
				if viewIDs[id] {
					view.Order = append(view.Order, id)
					view.Entities[id] = f.Entities[id]
				}
			}
			m, e := readIndependentFrustumOrMaterial(ctx, view, l, tol, cylinder, sphere, frustum)
			if e != nil {
				return IndependentTorusAssembly{}, fmt.Errorf("step: assembly product %d: %w", p.DefinitionID, e)
			}
			if m.Frustum != nil && frustum {
				nfr++
			} else if m.Torus != nil {
				nt++
			} else if m.Cylinder != nil && cylinder {
				nc++
			} else if m.Sphere != nil && sphere {
				ns++
			} else if m.Planar != nil {
				np++
			} else {
				return bad()
			}
			m.SourceProductID = p.ProductID
			m.SourceDefinitionID = p.DefinitionID
			m.SourceRepresentationID = p.Representations[0]
			m.ProductName = p.Name
			if m.LengthUnit != a.LengthUnit {
				return bad()
			}
			nf += len(m.Solid.Faces())
			nv += len(m.Solid.Vertices())
			for _, face := range m.Solid.Faces() {
				nu += len(face.Outer)
				for _, loop := range face.Holes {
					nu += len(loop)
				}
			}
			if nf > l.MaxFaces || nv > l.MaxVertices || nu > l.MaxEdgeUses {
				return bad()
			}
			out.Products[p.DefinitionID] = m
		}
		items, e := listRefs(rep.Args[1], 1)
		if e != nil {
			return bad()
		}
		for _, id := range items {
			en := f.Entities[id]
			if len(en.Records) == 1 && en.Records[0].Type == "AXIS2_PLACEMENT_3D" {
				b, e := mappedRationalFrame(f, id)
				if e != nil {
					return bad()
				}
				if _, e = independentTorusAssemblyPose(b); e != nil {
					return bad()
				}

				c, e := nauoClosure(f, []int64{id}, l.MaxDepth)
				if e != nil {
					return bad()
				}
				for x := range c {
					if owned[x] != 0 {
						return bad()
					}
					geometry[x] = true
				}
			}
		}
	}
	if (sphere && len(out.Products) > l.MaxSolids) || nt != 1 || np != 1 || (cylinder && nc != 1) || (!cylinder && nc != 0) || ns > 1 || (!sphere && ns != 0) || (frustum && nfr != 1) || (!frustum && nfr != 0) {
		return bad()
	}
	if !dag {
		uses := map[int64]int{}
		for _, u := range a.Occurrences {
			if u.Parent != a.Roots[0] || out.Products[u.Child].SourceBodyID == 0 || !planarSplineRigid(u.Transform) {
				return bad()
			}
			uses[u.Child]++
		}
		for id, p := range out.Products {
			if uses[id] == 0 || (p.Torus != nil && uses[id] < 2) {
				return bad()
			}
		}
	}
	metadata := map[string]bool{}
	for _, k := range []string{"APPLICATION_CONTEXT", "APPLICATION_PROTOCOL_DEFINITION", "PRODUCT_CONTEXT", "PRODUCT_DEFINITION_CONTEXT", "PRODUCT", "PRODUCT_DEFINITION_FORMATION", "PRODUCT_DEFINITION", "PRODUCT_DEFINITION_SHAPE", "SHAPE_DEFINITION_REPRESENTATION", "SHAPE_REPRESENTATION", "ADVANCED_BREP_SHAPE_REPRESENTATION", "NEXT_ASSEMBLY_USAGE_OCCURRENCE", "ITEM_DEFINED_TRANSFORMATION", "REPRESENTATION_RELATIONSHIP", "REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION", "SHAPE_REPRESENTATION_RELATIONSHIP", "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION", "GEOMETRIC_REPRESENTATION_CONTEXT", "GLOBAL_UNIT_ASSIGNED_CONTEXT", "REPRESENTATION_CONTEXT", "LENGTH_UNIT", "NAMED_UNIT", "SI_UNIT", "PLANE_ANGLE_UNIT", "SOLID_ANGLE_UNIT", "GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT", "UNCERTAINTY_MEASURE_WITH_UNIT"} {
		metadata[k] = true
	}
	counts := map[string]int{}
	for _, id := range f.Order {
		for _, rec := range f.Entities[id].Records {
			counts[rec.Type]++
			if rec.Type == "SHAPE_DEFINITION_REPRESENTATION" {
				if len(rec.Args) != 2 {
					return bad()
				}
				shape, e := simple(f, rec.Args[0].Ref, "PRODUCT_DEFINITION_SHAPE", 3)
				if e != nil {
					return bad()
				}
				if m, ok := out.Products[shape.Args[2].Ref]; ok {
					m.SourceRootID = id
					out.Products[shape.Args[2].Ref] = m
				}
			}
			if rec.Type == "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION" {
				if len(rec.Args) != 2 {
					return bad()
				}
				shape, e := simple(f, rec.Args[1].Ref, "PRODUCT_DEFINITION_SHAPE", 3)
				if e != nil {
					return bad()
				}
				var tid int64
				for _, r := range f.Entities[rec.Args[0].Ref].Records {
					if r.Type == "REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION" {
						if len(r.Args) != 1 {
							return bad()
						}
						tid = r.Args[0].Ref
					}
				}
				tr, e := simple(f, tid, "ITEM_DEFINED_TRANSFORMATION", 4)
				if e != nil {
					return bad()
				}
				from, e := mappedRationalFrame(f, tr.Args[2].Ref)
				if e != nil {
					return bad()
				}
				to, e := mappedRationalFrame(f, tr.Args[3].Ref)
				if e != nil {
					return bad()
				}
				exact, e := independentTorusAssemblyPose(composeMappedPose(to, inverseMappedPose(from)))
				if e != nil {
					return bad()
				}
				found := false
				for _, u := range a.Occurrences {
					if u.ID == shape.Args[2].Ref {
						found = true
						if u.Transform != exact {
							return bad()
						}
					}
				}
				if !found {
					return bad()
				}
			}
			if !geometry[id] && !metadata[rec.Type] {
				return bad()
			}
			if rec.Type == "SHAPE_DEFINITION_REPRESENTATION" || rec.Type == "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION" || rec.Type == "APPLICATION_PROTOCOL_DEFINITION" {
				roots = append(roots, id)
			}
		}
	}
	if counts["APPLICATION_PROTOCOL_DEFINITION"] != 1 || counts["SHAPE_DEFINITION_REPRESENTATION"] != products || counts["CONTEXT_DEPENDENT_SHAPE_REPRESENTATION"] != len(a.Occurrences) {
		return bad()
	}
	all, e := nauoClosure(f, roots, l.MaxDepth)
	if e != nil || len(all) != len(f.Entities) {
		return bad()
	}
	if hierarchy {
		if _, e = independentAnalyticHierarchyPaths(ctx, out, sphere, frustum); e != nil {
			return IndependentTorusAssembly{}, e
		}
	} else if dag {
		if _, e = independentAnalyticDAGPaths(out); e != nil {
			return bad()
		}
	}
	return out, nil
}

// WriteIndependentTorusAssembly stages authenticated independent source bodies
// into standard NAUO/CDSR products, then re-proves the complete result atomically.
func WriteIndependentTorusAssembly(ctx context.Context, w io.Writer, m IndependentTorusAssemblyModel, l Limits, tol float64) error {
	return writeIndependentTorusAssembly(ctx, w, m, l, tol, false)
}
func writeIndependentTorusAssembly(ctx context.Context, w io.Writer, m IndependentTorusAssemblyModel, l Limits, tol float64, cylinder bool, dagMode ...bool) error {
	products := 3
	if cylinder {
		products = 4
	}
	dag := cylinder && len(dagMode) >= 1 && dagMode[0]
	hierarchy := dag && len(dagMode) >= 2 && dagMode[1]
	sphere := hierarchy && len(dagMode) >= 3 && dagMode[2]
	frustum := sphere && len(dagMode) == 4 && dagMode[3]
	if dag {
		products = 5
	}
	if hierarchy {
		products = len(m.Products)
		if products < 4 || products > 16 || len(m.Occurrences) > 64 {
			return geom.ErrInvalid
		}
	}
	if ctx == nil || w == nil || len(m.Products) != products || (!hierarchy && len(m.Occurrences) > 16) {
		return geom.ErrInvalid
	}
	graph := AssemblyModel{LengthUnit: m.LengthUnit, Occurrences: m.Occurrences}
	counts := make([]int, products)
	for _, u := range m.Occurrences {
		if !planarSplineRigid(u.Transform) {
			return geom.ErrInvalid
		}
	}
	sources := make([]*File, products)
	ids := make([]int64, products)
	for i, p := range m.Products {
		graph.Products = append(graph.Products, AssemblyPart{ID: p.ID, Name: p.Name})
		if p.Source == nil {
			continue
		}
		records := 4
		if frustum {
			records = 7
		}
		if !independentNativeSourceRecordBound(p.Source, 1024, records) {
			return geom.ErrInvalid
		}
		raw, e := mappedPlanarSourceBytes(p.Source)
		if e != nil {
			return e
		}
		c, e := stageIndependentFrustumMaterial(ctx, p.Source, raw, l, tol, frustum)
		if e != nil {
			return fmt.Errorf("stage %s: %w", p.ID, e)
		}
		if len(c.Members) != 1 || c.LengthUnit != m.LengthUnit || (c.Members[0].Torus == nil && c.Members[0].Planar == nil && !(cylinder && c.Members[0].Cylinder != nil) && !(sphere && c.Members[0].Sphere != nil) && !(frustum && c.Members[0].Frustum != nil)) {
			return fmt.Errorf("stage %s: family/unit mismatch", p.ID)
		}
		sources[i] = c.Source
		ids[i] = c.Members[0].SourceBodyID
		counts[i] = 1
	}
	var b bytes.Buffer
	e := writeAssemblyGeometry(&b, graph, counts, func(i int) (*File, []int64, error) { return sources[i], []int64{ids[i]}, nil })
	if e != nil {
		return e
	}
	if _, e = readIndependentTorusAssembly(ctx, bytes.NewReader(b.Bytes()), l, tol, cylinder, dag, hierarchy, sphere, frustum); e != nil {
		return fmt.Errorf("staged assembly reproof: %w", e)
	}
	n, e := w.Write(b.Bytes())
	if e == nil && n != b.Len() {
		e = io.ErrShortWrite
	}
	return e
}

func independentTorusAssemblyPose(b mappedRationalBasis) (geom.Affine, error) {
	var pose geom.Affine
	for i := 0; i < 3; i++ {
		for j, v := range [4]*big.Rat{b.X[i], b.Y[i], b.Z[i], b.Origin[i]} {
			x, exact := v.Float64()
			if !exact {
				return pose, geom.ErrUnrepresentable
			}
			pose[i][j] = x
		}
	}
	if !planarSplineRigid(pose) {
		return pose, geom.ErrInvalid
	}
	return pose, nil
}

// WriteIndependentTorusAssemblySource re-proves and re-emits an accepted source
// graph without remapping its product, occurrence or local seam IDs.
func WriteIndependentTorusAssemblySource(ctx context.Context, w io.Writer, source *File, l Limits, tol float64) error {
	return writeIndependentTorusAssemblySource(ctx, w, source, l, tol, false)
}
func writeIndependentTorusAssemblySource(ctx context.Context, w io.Writer, source *File, l Limits, tol float64, cylinder bool, dagMode ...bool) error {
	dag := cylinder && len(dagMode) >= 1 && dagMode[0]
	hierarchy := dag && len(dagMode) >= 2 && dagMode[1]
	sphere := hierarchy && len(dagMode) >= 3 && dagMode[2]
	frustum := sphere && len(dagMode) == 4 && dagMode[3]
	records := 4
	if frustum {
		records = 7
	}
	if ctx == nil || w == nil || !independentNativeSourceRecordBound(source, 4096, records) {
		return geom.ErrInvalid
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	if _, e = readIndependentTorusAssembly(ctx, bytes.NewReader(raw), l, tol, cylinder, dag, hierarchy, sphere, frustum); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}

package step

import (
	"bytes"
	"fmt"
	"io"
)

// WrappedMaterialAssemblyPart contains certified local bodies shared by explicit
// occurrences. An assembly-only product has no Bodies.
type WrappedMaterialAssemblyPart struct {
	ID, Name string
	Bodies   []WrappedMaterialBody
}
type WrappedMaterialAssemblyModel struct {
	Products    []WrappedMaterialAssemblyPart
	Occurrences []AssemblyInstance
	LengthUnit  float64
}
type WrappedMaterialAssembly struct {
	Assembly      Assembly
	ProductBodies map[int64][]WrappedMaterialBody
}

// WriteWrappedMaterialAssembly retains products and proper rigid placements.
// Local payloads are authoritative and pass the mixed material proof before any
// output. This API neither merges touching bodies nor infers occurrences.
func WriteWrappedMaterialAssembly(w io.Writer, m WrappedMaterialAssemblyModel) error {
	graph := AssemblyModel{LengthUnit: m.LengthUnit, Occurrences: m.Occurrences}
	counts := make([]int, len(m.Products))
	for i, p := range m.Products {
		graph.Products = append(graph.Products, AssemblyPart{ID: p.ID, Name: p.Name})
		counts[i] = len(p.Bodies)
	}
	var staged bytes.Buffer
	e := writeAssemblyGeometry(&staged, graph, counts, func(i int) (*File, []int64, error) {
		var b bytes.Buffer
		if e := WriteWrappedMaterials(&b, WrappedMaterialModel{Bodies: m.Products[i].Bodies, LengthUnit: m.LengthUnit}); e != nil {
			return nil, nil, e
		}
		f, e := Parse(&b, Limits{})
		if e != nil {
			return nil, nil, e
		}
		ids := []int64{}
		for _, id := range f.Order {
			en := f.Entities[id]
			if len(en.Records) == 1 && (en.Records[0].Type == "MANIFOLD_SOLID_BREP" || en.Records[0].Type == "BREP_WITH_VOIDS") {
				ids = append(ids, id)
			}
		}
		return f, ids, nil
	})
	if e != nil {
		return e
	}
	if _, e = ReadWrappedMaterialAssembly(bytes.NewReader(staged.Bytes()), Limits{}, WrappedSurfaceOptions{}); e != nil {
		return e
	}
	if w == nil {
		return fmt.Errorf("step: nil mixed assembly writer")
	}
	n, e := w.Write(staged.Bytes())
	if e == nil && n != staged.Len() {
		e = io.ErrShortWrite
	}
	return e
}

// ReadWrappedMaterialAssembly validates local geometry once per product. All
// bodies enter one proof view, so ownership and resource limits remain global.
// Occurrence transforms are retained separately and never applied to supports.
func ReadWrappedMaterialAssembly(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedMaterialAssembly, error) {
	fail := func() (WrappedMaterialAssembly, error) {
		return WrappedMaterialAssembly{}, fmt.Errorf("step: unsupported mixed material assembly ownership")
	}
	var e error
	l, e = l.defaults()
	if e != nil {
		return WrappedMaterialAssembly{}, e
	}
	a, e := ReadAssembly(r, l)
	if e != nil {
		return WrappedMaterialAssembly{}, e
	}
	owner := map[int64]int64{}
	reps := map[int64]bool{}
	for _, p := range a.Products {
		ids, advanced, e := a.productItems(p.DefinitionID)
		if e != nil {
			return WrappedMaterialAssembly{}, e
		}
		for i, id := range ids {
			if !advanced[i] || owner[id] != 0 {
				return fail()
			}
			owner[id] = p.DefinitionID
		}
		if len(ids) > 0 {
			for _, id := range p.Representations {
				if _, e := simple(a.Source, id, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3); e == nil {
					reps[id] = true
				}
			}
		}
	}
	if len(owner) == 0 || len(owner) > l.MaxSolids {
		return fail()
	}
	view := &File{Entities: map[int64]Entity{}, Header: a.Source.Header}
	active := map[int64]bool{}
	var copyID func(int64, int) error
	copyID = func(id int64, depth int) error {
		if active[id] || depth >= l.MaxDepth {
			return fmt.Errorf("step: cyclic/deep assembly body")
		}
		if _, ok := view.Entities[id]; ok {
			return nil
		}
		en, ok := a.Source.Entities[id]
		if !ok {
			return fmt.Errorf("step: missing body dependency")
		}
		active[id] = true
		var visit func(Value) error
		visit = func(v Value) error {
			if v.Kind == Reference {
				return copyID(v.Ref, depth+1)
			}
			for _, x := range v.Items {
				if e := visit(x); e != nil {
					return e
				}
			}
			return nil
		}
		for _, rec := range en.Records {
			for _, v := range rec.Args {
				if e := visit(v); e != nil {
					return e
				}
			}
		}
		delete(active, id)
		view.Entities[id] = en
		return nil
	}
	for _, id := range a.Source.Order {
		if reps[id] {
			if e := copyID(id, 0); e != nil {
				return WrappedMaterialAssembly{}, e
			}
		}
	}
	points := 0
	for _, id := range a.Source.Order {
		en := a.Source.Entities[id]
		_, reachable := view.Entities[id]
		for _, rec := range en.Records {
			if rec.Type == "CARTESIAN_POINT" {
				points++
			}
			switch rec.Type {
			case "MANIFOLD_SOLID_BREP", "BREP_WITH_VOIDS":
				if owner[id] == 0 {
					return fail()
				}
			case "ADVANCED_FACE", "CLOSED_SHELL", "ORIENTED_CLOSED_SHELL", "B_SPLINE_SURFACE_WITH_KNOTS", "EDGE_CURVE", "VERTEX_POINT", "FACE_BOUND", "FACE_OUTER_BOUND", "EDGE_LOOP", "ORIENTED_EDGE":
				if !reachable {
					return fail()
				}
			}
		}
		if reachable {
			view.Order = append(view.Order, id)
		}
	}
	if points > l.MaxVertices {
		return fail()
	}
	m, e := ReadWrappedMaterials(bytes.NewReader(serializeWrappedProofFile(view)), l, o)
	if e != nil {
		return WrappedMaterialAssembly{}, e
	}
	if m.LengthUnit != a.LengthUnit || len(m.Bodies) != len(owner) {
		return fail()
	}
	out := WrappedMaterialAssembly{Assembly: a, ProductBodies: map[int64][]WrappedMaterialBody{}}
	for _, p := range a.Products {
		out.ProductBodies[p.DefinitionID] = nil
	}
	for _, b := range m.Bodies {
		p := owner[b.SourceBodyID]
		if p == 0 {
			return fail()
		}
		out.ProductBodies[p] = append(out.ProductBodies[p], b)
	}
	return out, nil
}

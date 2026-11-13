package step

import (
	"bytes"
	"fmt"
	"io"
	"plainsolid/geom"
)

// SplineLensAssemblyPart contains certified local bodies shared by explicit
// occurrences. An assembly-only product has no Bodies.
type SplineLensAssemblyPart struct {
	ID, Name string
	Bodies   []SplineLensBody
	// SeamCurves is authoritative only for WriteRefinedSplineLensAssembly.
	SeamCurves [][3]*geom.BSpline
	// RationalSeamCurves and Style are authoritative only for raw-weight assembly export.
	RationalSeamCurves [][3]RationalLensCurve
	Style              LensStyle
}
type SplineLensAssemblyModel struct {
	Products    []SplineLensAssemblyPart
	Occurrences []AssemblyInstance
	LengthUnit  float64
}
type SplineLensAssembly struct {
	Assembly      Assembly
	ProductBodies map[int64][]SplineLensBody
	// ProductPieces is populated by partitioned and refined assembly readers.
	ProductPieces map[int64][]LensSeamPiece
	// ProductCurves is populated only by ReadRefinedSplineLensAssembly.
	ProductCurves map[int64][][3]*geom.BSpline
	// Raw-weight assembly readers populate both maps below.
	ProductRationalCurves map[int64][][3]RationalLensCurve
	ProductStyles         map[int64]LensStyle
}

// WriteSplineLensAssembly retains products and proper rigid placements.
// Local payloads are authoritative and pass the lens proof before any
// output. This API neither merges touching bodies nor infers occurrences.
func WriteSplineLensAssembly(w io.Writer, m SplineLensAssemblyModel) error {
	return writeSplineLensAssembly(w, m, false)
}
func writeSplineLensAssembly(w io.Writer, m SplineLensAssemblyModel, partitioned bool, refined ...bool) error {
	writeBody := WriteSplineLensBody
	if partitioned {
		writeBody = WritePartitionedSplineLensBody
	}
	for _, occ := range m.Occurrences {
		if !lensRigidAxisPlacement(occ.Transform) {
			return fmt.Errorf("step: exact lens occurrence requires a proper signed-axis placement")
		}
	}
	graph := AssemblyModel{LengthUnit: m.LengthUnit, Occurrences: m.Occurrences}
	counts := make([]int, len(m.Products))
	for i, p := range m.Products {
		if len(refined) > 1 && refined[1] && len(p.SeamCurves) != 0 {
			return geom.ErrInvalid
		}
		if hasLensStyle(p.Style) && (!(len(refined) > 1 && refined[1]) || len(p.Bodies) != 1) {
			return geom.ErrInvalid
		}
		if len(p.Bodies) == 0 && (len(p.SeamCurves) != 0 || len(p.RationalSeamCurves) != 0) {
			return geom.ErrInvalid
		}
		if len(p.SeamCurves) > 0 && !(len(refined) > 0 && refined[0]) {
			return geom.ErrInvalid
		}
		if len(p.RationalSeamCurves) > 0 && !(len(refined) > 1 && refined[1]) {
			return geom.ErrInvalid
		}
		graph.Products = append(graph.Products, AssemblyPart{ID: p.ID, Name: p.Name})
		if len(p.Bodies) > 1 {
			return fmt.Errorf("step: one lens per product")
		}
		counts[i] = len(p.Bodies)
	}
	var staged bytes.Buffer
	e := writeAssemblyGeometry(&staged, graph, counts, func(i int) (*File, []int64, error) {
		var b bytes.Buffer
		body := m.Products[i].Bodies[0]
		body.LengthUnit = m.LengthUnit
		var e error
		if len(refined) > 1 && refined[1] {
			e = WriteRationalSplineLensBody(&b, RationalSplineLens{PartitionedSplineLens: PartitionedSplineLens{Body: body}, Curves: m.Products[i].RationalSeamCurves}, CurvedOvalOptions{})
		} else if len(refined) > 0 && refined[0] {
			e = WriteRefinedSplineLensBody(&b, RefinedSplineLens{PartitionedSplineLens: PartitionedSplineLens{Body: body}, Curves: m.Products[i].SeamCurves}, CurvedOvalOptions{})
		} else {
			e = writeBody(&b, body, CurvedOvalOptions{})
		}
		if e != nil {
			return nil, nil, e
		}
		f, e := Parse(&b, Limits{})
		if e != nil {
			return nil, nil, e
		}
		ids := []int64{}
		for _, id := range f.Order {
			en := f.Entities[id]
			if len(en.Records) == 1 && (en.Records[0].Type == "MANIFOLD_SOLID_BREP") {
				ids = append(ids, id)
			}
		}
		return f, ids, nil
	})
	if e != nil {
		return e
	}
	if _, e = readSplineLensAssembly(bytes.NewReader(staged.Bytes()), Limits{}, CurvedOvalOptions{}, partitioned, refined...); e != nil {
		return e
	}
	if w == nil {
		return fmt.Errorf("step: nil lens assembly writer")
	}
	n, e := w.Write(staged.Bytes())
	if e == nil && n != staged.Len() {
		e = io.ErrShortWrite
	}
	return e
}

// ReadSplineLensAssembly preserves explicit proper rigid occurrences and proves
// each local lens once. Bodies/topology are exclusive to their product; repeated
// occurrences share that product rather than duplicate or transform its charts.
func ReadSplineLensAssembly(r io.Reader, l Limits, o CurvedOvalOptions) (SplineLensAssembly, error) {
	return readSplineLensAssembly(r, l, o, false)
}
func readSplineLensAssembly(r io.Reader, l Limits, o CurvedOvalOptions, partitioned bool, refined ...bool) (SplineLensAssembly, error) {
	empty := SplineLensAssembly{}
	fail := func() (SplineLensAssembly, error) {
		return empty, fmt.Errorf("step: unsupported lens assembly ownership")
	}
	var e error
	l, e = l.defaults()
	if e != nil {
		return empty, e
	}
	o, e = o.defaults()
	if e != nil {
		return empty, e
	}
	a, e := ReadAssembly(r, l)
	if e != nil {
		return empty, e
	}
	for _, occ := range a.Occurrences {
		if !lensRigidAxisPlacement(occ.Transform) {
			return fail()
		}
	}
	for _, id := range a.Source.Order {
		tr, e := simple(a.Source, id, "ITEM_DEFINED_TRANSFORMATION", 4)
		if e != nil {
			continue
		}
		fromID, e := ref(tr.Args[2])
		if e != nil {
			return fail()
		}
		toID, e := ref(tr.Args[3])
		if e != nil {
			return fail()
		}
		from, e := placement(a.Source, fromID)
		if e != nil || frameTransform(from) != geom.Identity() {
			return fail()
		}
		to, e := placement(a.Source, toID)
		if e != nil || !lensRigidAxisPlacement(frameTransform(to)) {
			return fail()
		}
	}
	reps := map[int64]int64{}
	owners := map[int64]int64{}
	for _, p := range a.Products {
		ids, advanced, e := a.productItems(p.DefinitionID)
		if e != nil {
			return empty, e
		}
		if len(ids) > 1 {
			return fail()
		}
		if len(ids) == 0 {
			continue
		}
		if !advanced[0] || owners[ids[0]] != 0 {
			return fail()
		}
		if _, e = simple(a.Source, ids[0], "MANIFOLD_SOLID_BREP", 2); e != nil {
			return fail()
		}
		owners[ids[0]] = p.DefinitionID
		count := 0
		for _, rid := range p.Representations {
			if _, e = simple(a.Source, rid, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3); e == nil {
				reps[rid] = ids[0]
				count++
			}
		}
		if count != 1 {
			return fail()
		}
	}
	if len(owners) == 0 || len(owners) > l.MaxSolids || len(owners) > l.MaxFaces/2 || len(owners) > l.MaxEdgeUses/8 {
		return fail()
	}
	o.MaxOperations /= len(owners)
	if o.MaxOperations < 20 {
		return fail()
	}
	claimed := map[int64]int64{}
	out := SplineLensAssembly{Assembly: a, ProductBodies: map[int64][]SplineLensBody{}, ProductPieces: map[int64][]LensSeamPiece{}, ProductCurves: map[int64][][3]*geom.BSpline{}, ProductRationalCurves: map[int64][][3]RationalLensCurve{}}
	totalUses := 0
	for _, p := range a.Products {
		out.ProductBodies[p.DefinitionID] = nil
	}
	critical := func(t string) bool {
		switch t {
		case "MANIFOLD_SOLID_BREP", "CLOSED_SHELL", "ADVANCED_FACE", "EDGE_CURVE", "VERTEX_POINT", "FACE_BOUND", "FACE_OUTER_BOUND", "EDGE_LOOP", "ORIENTED_EDGE", "SURFACE_CURVE", "PCURVE", "B_SPLINE_CURVE", "B_SPLINE_CURVE_WITH_KNOTS", "B_SPLINE_SURFACE", "B_SPLINE_SURFACE_WITH_KNOTS":
			return true
		}
		return false
	}
	for _, rid := range a.Source.Order {
		body, ok := reps[rid]
		if !ok {
			continue
		}
		view := &File{Header: a.Source.Header, Entities: map[int64]Entity{}}
		active := map[int64]bool{}
		var visit func(int64, int) error
		visit = func(id int64, depth int) error {
			if active[id] || depth >= l.MaxDepth {
				return fmt.Errorf("step: cyclic/deep lens dependencies")
			}
			if _, ok := view.Entities[id]; ok {
				return nil
			}
			en, ok := a.Source.Entities[id]
			if !ok {
				return fmt.Errorf("step: missing lens dependency")
			}
			active[id] = true
			for _, rec := range en.Records {
				if critical(rec.Type) {
					if old := claimed[id]; old != 0 && old != body {
						return fmt.Errorf("step: lens topology shared across products")
					}
					claimed[id] = body
				}
				var value func(Value) error
				value = func(v Value) error {
					if v.Kind == Reference {
						return visit(v.Ref, depth+1)
					}
					for _, x := range v.Items {
						if e := value(x); e != nil {
							return e
						}
					}
					return nil
				}
				for _, v := range rec.Args {
					if e := value(v); e != nil {
						return e
					}
				}
			}
			delete(active, id)
			view.Entities[id] = en
			return nil
		}
		if e = visit(rid, 0); e != nil {
			return empty, e
		}
		for _, id := range a.Source.Order {
			if _, ok := view.Entities[id]; ok {
				view.Order = append(view.Order, id)
			}
		}
		m, e := readSplineLensBody(bytes.NewReader(serializeWrappedProofFile(view)), l, o, partitioned, refined...)
		if e != nil {
			return empty, e
		}
		if m.SourceBodyID != body || m.LengthUnit != a.LengthUnit {
			return fail()
		}
		out.ProductBodies[owners[body]] = []SplineLensBody{m}
		uses := 8
		if partitioned {
			out.ProductPieces[owners[body]] = m.sourcePieces
			if len(refined) > 0 && refined[0] {
				out.ProductCurves[owners[body]] = m.sourceCurves
				if len(refined) > 1 && refined[1] {
					out.ProductRationalCurves[owners[body]] = m.sourceRationalCurves
				}
			}
			uses = 2 * len(m.sourcePieces)
		}
		totalUses += uses
		if totalUses > l.MaxEdgeUses {
			return fail()
		}
	}
	points := 0
	for id, en := range a.Source.Entities {
		for _, rec := range en.Records {
			if rec.Type == "CARTESIAN_POINT" {
				points++
			}
			if critical(rec.Type) && claimed[id] == 0 {
				return fail()
			}
			switch rec.Type {
			case "BREP_WITH_VOIDS", "FACETED_BREP", "OPEN_SHELL", "SHELL_BASED_SURFACE_MODEL", "ORIENTED_CLOSED_SHELL":
				return fail()
			}
		}
	}
	if points > l.MaxVertices {
		return fail()
	}
	return out, nil
}

// Represented signed-axis proper rotations and finite translations are preserved
// exactly. The restricted source transform starts at the identity frame, avoiding
// rounded inversion or subtraction of independently placed source frames.
func lensRigidAxisPlacement(a geom.Affine) bool {
	if !a.IsFinite() {
		return false
	}
	for i := 0; i < 3; i++ {
		row, col := 0, 0
		for j := 0; j < 3; j++ {
			v := a[i][j]
			if v != 0 && v != 1 && v != -1 {
				return false
			}
			if v != 0 {
				row++
			}
			if a[j][i] != 0 {
				col++
			}
		}
		if row != 1 || col != 1 {
			return false
		}
	}
	return a.Determinant() == 1
}

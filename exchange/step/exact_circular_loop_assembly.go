package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
)

// ExactCircularLoopAssembly retains one original model-only rounded prism and
// 2..16 flat shared uses. Native is product-local through Body.Descriptor.Frame;
// occurrence transforms then map product coordinates to assembly coordinates.
// No regularized union or historical signed extrusion intent is inferred.
type ExactCircularLoopAssembly struct {
	Assembly    Assembly
	Body        OCCTExactCircularLoopExtrusion
	Occurrences []ExactNAUOOccurrence
}

// ReadExactCircularLoopAssembly authenticates original line/circle topology and
// independently bound NAUO relationships. Local geometry remains unflattened.
func ReadExactCircularLoopAssembly(ctx context.Context, r io.Reader, l Limits) (ExactCircularLoopAssembly, error) {
	return readExactCircularLoopAssembly(ctx, r, l, false)
}
func readExactCircularLoopAssembly(ctx context.Context, r io.Reader, l Limits, nested bool) (ExactCircularLoopAssembly, error) {
	var zero ExactCircularLoopAssembly
	l, e := occurrenceColorSourceLimits(l)
	if e != nil {
		return zero, e
	}
	base, e := readExactFlatSingleProductAssembly(ctx, r, l, nested)
	if e != nil {
		return zero, e
	}
	f := base.Assembly.Source
	schemas := 0
	for _, h := range f.Header {
		if h.Type == "FILE_SCHEMA" {
			schemas++
			if len(h.Args) != 1 || h.Args[0].Kind != Aggregate || len(h.Args[0].Items) != 1 || h.Args[0].Items[0].Kind != String || (h.Args[0].Items[0].Text != "AUTOMOTIVE_DESIGN" && h.Args[0].Items[0].Text != "AUTOMOTIVE_DESIGN { 1 0 10303 214 1 1 1 1 }") {
				return zero, geom.ErrDomain
			}
		}
	}
	if schemas != 1 {
		return zero, geom.ErrDomain
	}
	rep, e := simple(f, base.Product.Representations[0], "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if e != nil {
		return zero, e
	}
	items, e := listRefs(rep.Args[1], 2)
	if e != nil || len(items) != 2 || items[1] != base.SourceBodyID {
		return zero, geom.ErrDomain
	}
	frame, e := sideDiskReadFrame(f, items[0])
	if e != nil || frameTransform(frame) != geom.Identity() {
		return zero, geom.ErrDomain
	}
	for _, p := range base.Assembly.Products {
		en := f.Entities[p.Representations[0]]
		if len(en.Records) != 1 || len(en.Records[0].Args) != 3 {
			return zero, geom.ErrDomain
		}
		if e := occtLoopContextMode(f, en.Records[0].Args[2].Ref, true); e != nil {
			return zero, e
		}
	}
	closure, e := nauoClosure(f, []int64{base.SourceBodyID}, l.MaxDepth)
	if e != nil {
		return zero, e
	}
	allowed := map[string]bool{}
	for _, k := range []string{"MANIFOLD_SOLID_BREP", "CLOSED_SHELL", "ADVANCED_FACE", "FACE_BOUND", "FACE_OUTER_BOUND", "EDGE_LOOP", "ORIENTED_EDGE", "EDGE_CURVE", "VERTEX_POINT", "CARTESIAN_POINT", "DIRECTION", "AXIS2_PLACEMENT_3D", "CIRCLE", "LINE", "VECTOR", "PLANE", "CYLINDRICAL_SURFACE"} {
		allowed[k] = true
	}
	for id := range closure {
		en := f.Entities[id]
		if len(en.Records) != 1 || !allowed[en.Records[0].Type] {
			return zero, geom.ErrDomain
		}
	}
	body, e := authenticateOCCTLoopBody(ctx, f, l, base.Product, base.Assembly.LengthUnit, base.SourceBodyID)
	if e != nil {
		return zero, e
	}
	return ExactCircularLoopAssembly{base.Assembly, body, base.Occurrences}, nil
}

type ExactCircularLoopAssemblyPart struct {
	ID, Name string
	Source   *File
}
type ExactCircularLoopAssemblyModel struct {
	Products    []ExactCircularLoopAssemblyPart
	Occurrences []AssemblyInstance
	LengthUnit  float64
}

// WriteExactCircularLoopAssembly re-proves the one original standalone OCCT
// product, stages its unchanged local geometry and verifies the complete output.
// The root product has nil Source; names and occurrence poses are explicit.
func WriteExactCircularLoopAssembly(ctx context.Context, w io.Writer, m ExactCircularLoopAssemblyModel, l Limits) error {
	return writeExactCircularLoopAssembly(ctx, w, m, l, false)
}
func writeExactCircularLoopAssembly(ctx context.Context, w io.Writer, m ExactCircularLoopAssemblyModel, l Limits, nested bool) error {
	products, minUses, maxUses := 2, 2, 16
	if nested {
		products, minUses, maxUses = 3, 3, 17
	}
	if ctx == nil || w == nil || len(m.Products) != products || len(m.Occurrences) < minUses || len(m.Occurrences) > maxUses {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	graph := AssemblyModel{LengthUnit: m.LengthUnit, Occurrences: m.Occurrences}
	counts := make([]int, products)
	bodies := 0
	var bodyID int64
	for i, p := range m.Products {
		if len(p.ID) > 4096 || len(p.Name) > 4096 {
			return geom.ErrInvalid
		}
		graph.Products = append(graph.Products, AssemblyPart{ID: p.ID, Name: p.Name})
		if p.Source != nil {
			if !independentNativeSourceRecordBound(p.Source, 4096, 16) {
				return geom.ErrInvalid
			}
			raw, e := mappedPlanarSourceBytes(p.Source)
			if e != nil {
				return e
			}
			proof, e := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), l)
			if e != nil {
				return e
			}
			if proof.Descriptor.LengthUnit != m.LengthUnit {
				return geom.ErrDomain
			}
			bodyID = proof.Body
			counts[i] = 1
			bodies++
		}
	}
	if bodies != 1 {
		return geom.ErrInvalid
	}
	for _, u := range m.Occurrences {
		if len(u.ID) > 4096 || len(u.Name) > 4096 || len(u.Parent) > 4096 || len(u.Child) > 4096 || !planarSplineRigid(u.Transform) {
			return geom.ErrInvalid
		}
	}
	var staged bytes.Buffer
	e := writeAssemblyGeometry(&staged, graph, counts, func(i int) (*File, []int64, error) { return m.Products[i].Source, []int64{bodyID}, nil })
	if e != nil {
		return e
	}
	if _, e = readExactCircularLoopAssembly(ctx, bytes.NewReader(staged.Bytes()), l, nested); e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(staged.Bytes())
	if e == nil && n != staged.Len() {
		e = io.ErrShortWrite
	}
	return e
}

// WriteExactCircularLoopAssemblySource re-proves and exports original record IDs.
func WriteExactCircularLoopAssemblySource(ctx context.Context, w io.Writer, f *File, l Limits) error {
	if ctx == nil || w == nil || !independentNativeSourceRecordBound(f, 4096, 16) {
		return geom.ErrInvalid
	}
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	if _, e = ReadExactCircularLoopAssembly(ctx, bytes.NewReader(raw), l); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}

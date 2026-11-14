package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
	"plainsolid/internal/ellipticprism"
)

// IndependentCylinderMaterial preserves exact source cell roles separately from
// the native four-quarter rational cylinder topology.
type IndependentCylinderMaterial struct {
	Frame                       geom.Frame
	Radius, Height              float64
	Cells                       int
	SourceBodyID, SourceShellID int64
	SourceFaces, SourceEdges    []int64
	NativeFaceSourceIDs         [6]int64
	NativeRimEdges              [2][4]int
}

// ReadIndependentCylinderMaterials adds exactly the two-half/four-quarter model-
// only cylinder family to independent planar/ellipse products. Existing APIs
// remain strict; this file must contain at least one cylinder product.
func ReadIndependentCylinderMaterials(ctx context.Context, r io.Reader, l Limits, tol float64) (IndependentExactMaterialCollection, error) {
	m, e := readIndependentExactMaterialCollection(ctx, r, l, tol, 1, 16, true, true)
	if e != nil {
		return IndependentExactMaterialCollection{}, e
	}
	for _, p := range m.Members {
		if p.Cylinder != nil {
			return m, nil
		}
	}
	return IndependentExactMaterialCollection{}, geom.ErrInvalid
}
func WriteIndependentCylinderMaterials(ctx context.Context, w io.Writer, sources []*File, l Limits, tol float64) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var staged bytes.Buffer
	if e := writeIndependentExactMaterialCollection(ctx, &staged, sources, l, tol, 1, 16, true); e != nil {
		return e
	}
	if _, e := ReadIndependentCylinderMaterials(ctx, bytes.NewReader(staged.Bytes()), l, tol); e != nil {
		return e
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(staged.Bytes())
	if e == nil && n != staged.Len() {
		e = io.ErrShortWrite
	}
	return e
}
func readIndependentCylinderProduct(ctx context.Context, f *File, bodyID int64, l Limits, tol float64) (IndependentExactMaterial, error) {
	bad := func() (IndependentExactMaterial, error) { return IndependentExactMaterial{}, geom.ErrInvalid }
	body, e := simple(f, bodyID, "MANIFOLD_SOLID_BREP", 2)
	if e != nil || body.Args[1].Kind != Reference {
		return bad()
	}
	shell, e := simple(f, body.Args[1].Ref, "CLOSED_SHELL", 2)
	if e != nil {
		return bad()
	}
	faces, e := listRefs(shell.Args[1], 4)
	if e != nil || (len(faces) != 4 && len(faces) != 6) {
		return bad()
	}
	cells := len(faces) - 2
	planes, sides := 0, 0
	for _, id := range faces {
		face, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil || face.Args[2].Kind != Reference {
			return bad()
		}
		support := f.Entities[face.Args[2].Ref]
		if len(support.Records) != 1 {
			return bad()
		}
		switch support.Records[0].Type {
		case "PLANE":
			planes++
		case "CYLINDRICAL_SURFACE":
			sides++
		default:
			return bad()
		}
	}
	if planes != 2 || sides != cells {
		return bad()
	}
	// Remove only product metadata and the already-proved neutral representation
	// item. Remaining geometry is dependency-closed with original IDs and scalars.
	copyFile := &File{Header: f.Header, Entities: map[int64]Entity{}, Order: append([]int64(nil), f.Order...)}
	repID := int64(0)
	for id, en := range f.Entities {
		copyFile.Entities[id] = en
		if len(en.Records) == 1 && en.Records[0].Type == "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			if repID != 0 {
				return bad()
			}
			repID = id
		}
	}
	rep, e := simple(f, repID, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if e != nil {
		return bad()
	}
	rep.Args = append([]Value(nil), rep.Args...)
	rep.Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: bodyID}}}
	copyFile.Entities[repID] = Entity{ID: repID, Records: []Record{rep}}
	keep, e := nauoClosure(copyFile, []int64{repID}, l.MaxDepth)
	if e != nil {
		return bad()
	}
	view := &File{Header: f.Header, Entities: map[int64]Entity{}}
	for _, id := range f.Order {
		if keep[id] {
			view.Entities[id] = copyFile.Entities[id]
			view.Order = append(view.Order, id)
		}
	}
	raw, e := mappedPlanarSourceBytes(view)
	if e != nil {
		return bad()
	}
	proof, e := readCylinderCells(bytes.NewReader(raw), l, cells)
	if e != nil {
		return bad()
	}
	if e = ctx.Err(); e != nil {
		return IndependentExactMaterial{}, e
	}
	solid, rims, e := ellipticprism.CircularFrustumFrame(ctx, proof.Frame, proof.Radius, proof.Radius, proof.Height, tol)
	if e != nil {
		return bad()
	}
	p := IndependentCylinderMaterial{Frame: proof.Frame, Radius: proof.Radius, Height: proof.Height, Cells: cells, SourceBodyID: proof.Body, SourceShellID: proof.Shell, SourceFaces: append([]int64(nil), proof.Faces...), SourceEdges: append([]int64(nil), proof.Edges...), NativeRimEdges: rims}
	p.NativeFaceSourceIDs[0] = proof.Faces[cells]
	p.NativeFaceSourceIDs[1] = proof.Faces[cells+1]
	for i := 0; i < 4; i++ {
		p.NativeFaceSourceIDs[2+i] = proof.Faces[i*cells/4]
	}
	return IndependentExactMaterial{Name: proof.Name, LengthUnit: proof.LengthUnit, Source: f, Solid: solid, Cylinder: &p, SourceBodyID: proof.Body}, nil
}

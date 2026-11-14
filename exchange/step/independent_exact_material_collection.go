package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/csg/poly"
	"plainsolid/geom"
	"strconv"
)

// IndependentExactMaterial retains one separately owned original product.
// Planar and Elliptical are mutually exclusive authenticated payloads.
type IndependentExactMaterial struct {
	Name                                                                      string
	LengthUnit                                                                float64
	Source                                                                    *File
	Solid                                                                     trimmed.Solid
	Planar                                                                    *Body
	Elliptical                                                                *ExactEllipticalPrismFamily
	SourceBodyID                                                              int64
	SourceRootID, SourceProductID, SourceDefinitionID, SourceRepresentationID int64
	ProductName                                                               string
	Cylinder                                                                  *IndependentCylinderMaterial
	Sphere                                                                    *IndependentSphereMaterial `json:",omitempty"`
}

// IndependentExactMaterialCollection is a multi-root file, not an inferred assembly.
type IndependentExactMaterialCollection struct {
	tolerance  float64
	Members    []IndependentExactMaterial
	Source     *File
	LengthUnit float64
}

// ReadIndependentExactMaterials accepts one to sixteen exclusively owned exact
// cardinal planar or original exact ellipse-family products under cumulative limits.
func ReadIndependentExactMaterials(ctx context.Context, r io.Reader, l Limits, tol float64) (IndependentExactMaterialCollection, error) {
	return readIndependentExactMaterialCollection(ctx, r, l, tol, 1, 16, true)
}
func readIndependentExactMaterialCollection(ctx context.Context, r io.Reader, l Limits, tol float64, minRoots, maxRoots int, allowPlanar bool, cylinderMode ...bool) (IndependentExactMaterialCollection, error) {
	bad := func() (IndependentExactMaterialCollection, error) {
		return IndependentExactMaterialCollection{}, fmt.Errorf("step: unsupported independent exact material collection")
	}
	if ctx == nil || r == nil || !finiteNumber(tol) || tol <= 0 {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return IndependentExactMaterialCollection{}, e
	}
	l, e := l.defaults()
	if e != nil || l.MaxSolids < minRoots {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, int64(131072*maxRoots))
	l.MaxEntities = min(l.MaxEntities, 1024*maxRoots)
	l.MaxValues = min(l.MaxValues, 16384*maxRoots)
	l.MaxDepth = min(l.MaxDepth, 16)
	f, e := Parse(r, l)
	if e != nil {
		return IndependentExactMaterialCollection{}, e
	}
	roots, apds, categories := []int64{}, []int64{}, []int64{}
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) != 1 {
			continue
		}
		switch en.Records[0].Type {
		case "SHAPE_DEFINITION_REPRESENTATION":
			roots = append(roots, id)
		case "APPLICATION_PROTOCOL_DEFINITION":
			apds = append(apds, id)
		case "PRODUCT_RELATED_PRODUCT_CATEGORY":
			categories = append(categories, id)
		}
	}
	if len(roots) < minRoots || len(roots) > maxRoots || len(roots) > l.MaxSolids || len(apds) != len(roots) || len(categories) != len(roots) {
		return bad()
	}
	out := IndependentExactMaterialCollection{Source: f, Members: make([]IndependentExactMaterial, len(roots)), tolerance: tol}
	claimed := map[int64]bool{}
	sharedFrames := map[int64]bool{}
	faces, vertices, uses := 0, 0, 0
	for member, root := range roots {
		allowed := map[int64]bool{}
		sdr, e := simple(f, root, "SHAPE_DEFINITION_REPRESENTATION", 2)
		if e != nil || sdr.Args[1].Kind != Reference {
			return bad()
		}
		rep, e := simple(f, sdr.Args[1].Ref, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
		if e != nil {
			return bad()
		}
		items, e := listRefs(rep.Args[1], 2)
		if e != nil || len(items) != 2 {
			return bad()
		}
		neutral, e := simple(f, items[0], "AXIS2_PLACEMENT_3D", 4)
		if e != nil {
			return bad()
		}
		frame, e := mappedExactFrame(f, items[0])
		if e != nil || frameTransform(frame) != geom.Identity() {
			return bad()
		}
		allowed[items[0]] = true
		for _, v := range neutral.Args[1:] {
			if v.Kind != Reference {
				return bad()
			}
			allowed[v.Ref] = true
		}
		keep, active := map[int64]bool{}, map[int64]bool{}
		var visit func(int64, int) error
		visit = func(id int64, depth int) error {
			if e := ctx.Err(); e != nil {
				return e
			}
			if active[id] || depth >= l.MaxDepth {
				return geom.ErrInvalid
			}
			if keep[id] {
				return nil
			}
			en, ok := f.Entities[id]
			if !ok {
				return geom.ErrInvalid
			}
			active[id] = true
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
			for _, rec := range en.Records {
				for _, v := range rec.Args {
					if e := value(v); e != nil {
						return e
					}
				}
			}
			delete(active, id)
			keep[id] = true
			return nil
		}
		if e = visit(root, 0); e != nil {
			return bad()
		}
		attached := 0
		for _, id := range apds {
			rec, e := simple(f, id, "APPLICATION_PROTOCOL_DEFINITION", 4)
			if e != nil {
				return bad()
			}
			if _, e := textValue(rec.Args[0]); e != nil {
				return bad()
			}
			if _, e := textValue(rec.Args[1]); e != nil {
				return bad()
			}
			if rec.Args[2].Kind != Number || rec.Args[3].Kind != Reference {
				return bad()
			}
			if year, e := strconv.Atoi(rec.Args[2].Text); e != nil || year <= 0 {
				return bad()
			}

			if rec.Args[3].Kind == Reference && keep[rec.Args[3].Ref] {
				if e = visit(id, 0); e != nil {
					return bad()
				}
				attached++
			}
		}
		if attached != 1 {
			return bad()
		}
		attached = 0
		for _, id := range categories {
			rec, e := simple(f, id, "PRODUCT_RELATED_PRODUCT_CATEGORY", 3)
			if e != nil {
				return bad()
			}
			if _, e := textValue(rec.Args[0]); e != nil {
				return bad()
			}
			if rec.Args[1].Kind != Omitted {
				if _, e := textValue(rec.Args[1]); e != nil {
					return bad()
				}
			}

			ids, e := listRefs(rec.Args[2], 1)
			if e != nil || len(ids) != 1 {
				return bad()
			}
			if keep[ids[0]] {
				if e = visit(id, 0); e != nil {
					return bad()
				}
				attached++
			}
		}
		if attached != 1 {
			return bad()
		}
		view := &File{Header: f.Header, Entities: map[int64]Entity{}}
		for _, id := range f.Order {
			if !keep[id] {
				continue
			}
			if claimed[id] && (!allowed[id] || !sharedFrames[id]) {
				return bad()
			}
			if allowed[id] {
				sharedFrames[id] = true
			}
			claimed[id] = true
			view.Entities[id] = f.Entities[id]
			view.Order = append(view.Order, id)
		}
		leaf := l
		leaf.MaxFaces -= faces
		leaf.MaxVertices -= vertices
		leaf.MaxEdgeUses -= uses
		m, e := readIndependentExactMaterial(ctx, view, leaf, tol, allowPlanar, len(cylinderMode) >= 1 && cylinderMode[0], len(cylinderMode) == 2 && cylinderMode[1])
		if e != nil {
			return IndependentExactMaterialCollection{}, e
		}
		m.SourceRootID = root
		m.SourceRepresentationID = sdr.Args[1].Ref
		m.SourceProductID, m.SourceDefinitionID, m.ProductName, e = independentMaterialProduct(view, root)
		if e != nil {
			return bad()
		}

		faces += len(m.Solid.Faces())
		vertices += len(m.Solid.Vertices())
		for _, face := range m.Solid.Faces() {
			uses += len(face.Outer)
			for _, hole := range face.Holes {
				uses += len(hole)
			}
		}
		if faces > l.MaxFaces || vertices > l.MaxVertices || uses > l.MaxEdgeUses {
			return bad()
		}
		if member == 0 {
			out.LengthUnit = m.LengthUnit
		} else if m.LengthUnit != out.LengthUnit {
			return bad()
		}
		out.Members[member] = m
	}
	if len(claimed) != len(f.Entities) {
		return bad()
	}
	return out, nil
}

func readIndependentExactMaterial(ctx context.Context, f *File, l Limits, tol float64, allowPlanar bool, allowCylinder bool, allowSphere bool) (IndependentExactMaterial, error) {
	bad := func() (IndependentExactMaterial, error) { return IndependentExactMaterial{}, geom.ErrInvalid }
	ids := []int64{}
	elliptical := false
	cylindrical := false
	spherical := false
	for _, id := range f.Order {
		for _, r := range f.Entities[id].Records {
			if r.Type == "MANIFOLD_SOLID_BREP" {
				ids = append(ids, id)
			}
			if r.Type == "SPHERICAL_SURFACE" {
				spherical = true
			}
			if r.Type == "CYLINDRICAL_SURFACE" {
				cylindrical = true
			}
			if r.Type == "SURFACE_OF_LINEAR_EXTRUSION" {
				elliptical = true
			}
		}
	}
	if len(ids) != 1 {
		return bad()
	}
	if spherical {
		if !allowSphere || cylindrical || elliptical {
			return bad()
		}
		return readIndependentSphereProduct(ctx, f, ids[0], l, tol)
	}
	if cylindrical {
		if !allowCylinder || elliptical {
			return bad()
		}
		return readIndependentCylinderProduct(ctx, f, ids[0], l, tol)
	}
	if elliptical || !allowPlanar {
		m, e := readExactEllipticalPrismFamilyFile(ctx, f, l, tol)
		if e != nil {
			return IndependentExactMaterial{}, e
		}
		return IndependentExactMaterial{Name: m.Name, LengthUnit: m.LengthUnit, Source: f, Solid: m.Solid, Elliptical: &m, SourceBodyID: ids[0]}, nil
	}
	if e := mappedPlanarExactEdges(f, ids[0]); e != nil {
		return bad()
	}
	b, e := readBody(f, ids[0], l.MaxVertices, l.MaxFaces, l.MaxEdgeUses, true)
	if e != nil {
		return bad()
	}
	if e = poly.ValidateEmbeddedBoundary(ctx, b.Solid, poly.Options{}); e != nil {
		return bad()
	}
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return bad()
	}
	model, e := Read(bytes.NewReader(raw), l)
	if e != nil || len(model.Bodies) != 1 {
		return bad()
	}
	s, e := trimmed.FromPlanar(b.Solid, tol)
	if e != nil {
		return bad()
	}
	return IndependentExactMaterial{Name: b.Name, LengthUnit: model.LengthUnit, Source: f, Solid: s, Planar: &b, SourceBodyID: ids[0]}, nil
}

// WriteIndependentExactMaterials stages and re-proves complete standard product
// graphs. It preserves each local geometry and metadata without inferring placement.
func WriteIndependentExactMaterials(ctx context.Context, w io.Writer, sources []*File, l Limits, tol float64) error {
	return writeIndependentExactMaterialCollection(ctx, w, sources, l, tol, 1, 16)
}

func writeIndependentExactMaterialCollection(ctx context.Context, w io.Writer, sources []*File, l Limits, tol float64, minRoots, maxRoots int, cylinderMode ...bool) error {
	if ctx == nil || w == nil || len(sources) < minRoots || len(sources) > maxRoots {
		return geom.ErrInvalid
	}
	var out bytes.Buffer
	fmt.Fprintln(&out, "ISO-10303-21;\nHEADER;\nFILE_DESCRIPTION(('Independent exact planar and ellipse materials'),'2;1');\nFILE_NAME('ellipse-compound.step','1970-01-01T00:00:00',(''),(''),'PlainSolid','PlainSolid','');\nFILE_SCHEMA(('AUTOMOTIVE_DESIGN'));\nENDSEC;\nDATA;")
	next := int64(1)
	for _, source := range sources {
		if source == nil || (len(source.Entities) < 1 || len(source.Entities) > 1024) || !ellipseOwnedSourceBound(source, len(source.Entities)) {
			return geom.ErrInvalid
		}
		raw, e := mappedPlanarSourceBytes(source)
		if e != nil {
			return e
		}
		if _, e = readIndependentExactMaterialCollection(ctx, bytes.NewReader(raw), l, tol, 1, 16, true, cylinderMode...); e != nil {
			return e
		}
		ids := map[int64]int64{}
		for _, id := range source.Order {
			ids[id] = next
			next++
		}
		for _, id := range source.Order {
			fmt.Fprintf(&out, "#%d=%s;\n", ids[id], assemblyEntity(source.Entities[id], ids))
		}
	}
	fmt.Fprintln(&out, "ENDSEC;END-ISO-10303-21;")
	if _, e := readIndependentExactMaterialCollection(ctx, bytes.NewReader(out.Bytes()), l, tol, minRoots, maxRoots, true, cylinderMode...); e != nil {
		return e
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(out.Bytes())
	if e == nil && n != out.Len() {
		e = io.ErrShortWrite
	}
	return e
}

func independentMaterialProduct(f *File, root int64) (int64, int64, string, error) {
	bad := func() (int64, int64, string, error) { return 0, 0, "", geom.ErrInvalid }
	s, e := simple(f, root, "SHAPE_DEFINITION_REPRESENTATION", 2)
	if e != nil || s.Args[0].Kind != Reference {
		return bad()
	}
	pds, e := simple(f, s.Args[0].Ref, "PRODUCT_DEFINITION_SHAPE", 3)
	if e != nil || pds.Args[2].Kind != Reference {
		return bad()
	}
	for _, v := range pds.Args[:2] {
		if _, e := textValue(v); e != nil {
			return bad()
		}
	}
	pdID := pds.Args[2].Ref
	pd, e := simple(f, pdID, "PRODUCT_DEFINITION", 4)
	if e != nil || pd.Args[2].Kind != Reference || pd.Args[3].Kind != Reference {
		return bad()
	}
	for _, v := range pd.Args[:2] {
		if _, e := textValue(v); e != nil {
			return bad()
		}
	}
	formation, e := simple(f, pd.Args[2].Ref, "PRODUCT_DEFINITION_FORMATION", 3)
	if e != nil || formation.Args[2].Kind != Reference {
		return bad()
	}
	for _, v := range formation.Args[:2] {
		if _, e := textValue(v); e != nil {
			return bad()
		}
	}
	productID := formation.Args[2].Ref
	product, e := simple(f, productID, "PRODUCT", 4)
	if e != nil {
		return bad()
	}
	for _, v := range product.Args[:3] {
		if _, e := textValue(v); e != nil {
			return bad()
		}
	}
	name, _ := textValue(product.Args[1])
	contexts, e := listRefs(product.Args[3], 1)
	if e != nil || len(contexts) != 1 {
		return bad()
	}
	pc, e := simple(f, contexts[0], "PRODUCT_CONTEXT", 3)
	if e != nil || pc.Args[1].Kind != Reference {
		return bad()
	}
	dc, e := simple(f, pd.Args[3].Ref, "PRODUCT_DEFINITION_CONTEXT", 3)
	if e != nil || dc.Args[1].Kind != Reference || dc.Args[1].Ref != pc.Args[1].Ref {
		return bad()
	}
	for _, r := range []Record{pc, dc} {
		for _, i := range []int{0, 2} {
			if _, e := textValue(r.Args[i]); e != nil {
				return bad()
			}
		}
	}
	ac, e := simple(f, pc.Args[1].Ref, "APPLICATION_CONTEXT", 1)
	if e != nil {
		return bad()
	}
	if _, e := textValue(ac.Args[0]); e != nil {
		return bad()
	}
	return productID, pdID, name, nil
}

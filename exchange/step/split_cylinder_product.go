package step

import (
	"bytes"
	"fmt"
	"io"
)

// SplitCylinderProduct is the explicit single-product rooted dialect. Product
// name equals the body's name; all other product metadata is fixed by the
// documented dialect. No assembly or placement is inferred from the envelope.
type SplitCylinderProduct struct {
	Model                                SplitCylinderModel
	Product, Definition, ShapeDefinition int64
}

var splitProductKinds = []string{"APPLICATION_CONTEXT", "APPLICATION_PROTOCOL_DEFINITION", "MECHANICAL_CONTEXT", "DESIGN_CONTEXT", "PRODUCT", "PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE", "PRODUCT_DEFINITION", "PRODUCT_DEFINITION_SHAPE", "SHAPE_DEFINITION_REPRESENTATION"}

func splitProductRecords(ids []int64, rep int64, name string, cells int) []string {
	productID := "split-cylinder"
	if cells == 4 {
		productID = "quarter-cylinder"
	}
	return []string{
		"APPLICATION_CONTEXT('configuration controlled 3d designs of mechanical parts and assemblies')",
		fmt.Sprintf("APPLICATION_PROTOCOL_DEFINITION('international standard','config_control_design',1994,#%d)", ids[0]),
		fmt.Sprintf("MECHANICAL_CONTEXT('',#%d,'mechanical')", ids[0]),
		fmt.Sprintf("DESIGN_CONTEXT('',#%d,'design')", ids[0]),
		fmt.Sprintf("PRODUCT(%s,%s,'',(#%d))", quote(productID), quote(name), ids[2]),
		fmt.Sprintf("PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE('','',#%d,.NOT_KNOWN.)", ids[4]),
		fmt.Sprintf("PRODUCT_DEFINITION('design','',#%d,#%d)", ids[5], ids[3]),
		fmt.Sprintf("PRODUCT_DEFINITION_SHAPE('','',#%d)", ids[6]),
		fmt.Sprintf("SHAPE_DEFINITION_REPRESENTATION(#%d,#%d)", ids[7], rep),
	}
}
func splitProductFile(f *File, drop map[int64]bool, extra map[int64]string) []byte {
	var b bytes.Buffer
	b.WriteString("ISO-10303-21;\nHEADER;FILE_DESCRIPTION(('PlainSolid split cylinder product'),'2;1');FILE_NAME('','',(''),(''),'PlainSolid','','');FILE_SCHEMA(('CONFIG_CONTROL_DESIGN'));ENDSEC;\nDATA;\n")
	identity := map[int64]int64{}
	var max int64
	for id := range f.Entities {
		identity[id] = id
		if id > max {
			max = id
		}
	}
	for _, id := range f.Order {
		if !drop[id] {
			fmt.Fprintf(&b, "#%d=%s;\n", id, assemblyEntity(f.Entities[id], identity))
		}
	}
	for id := max + 1; id <= max+int64(len(extra)); id++ {
		fmt.Fprintf(&b, "#%d=%s;\n", id, extra[id])
	}
	b.WriteString("ENDSEC;END-ISO-10303-21;\n")
	return b.Bytes()
}

// WriteSplitCylinderProduct emits the proven model graph with one complete
// canonical AP203 product envelope. It validates the rooted output before I/O.
func WriteSplitCylinderProduct(w io.Writer, d SplitCylinderDescriptor) error {
	return writeCylinderProduct(w, d, 2)
}
func writeCylinderProduct(w io.Writer, d SplitCylinderDescriptor, cells int) error {
	if w == nil {
		return splitFailure()
	}
	var b bytes.Buffer
	if e := writeCylinderCells(&b, d, cells); e != nil {
		return e
	}
	f, e := Parse(&b, Limits{})
	if e != nil {
		return e
	}
	var max, rep int64
	for id, ent := range f.Entities {
		if id > max {
			max = id
		}
		if len(ent.Records) == 1 && ent.Records[0].Type == "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			rep = id
		}
	}
	ids := make([]int64, len(splitProductKinds))
	for i := range ids {
		ids[i] = max + 1 + int64(i)
	}
	extra := map[int64]string{}
	for i, r := range splitProductRecords(ids, rep, d.Name, cells) {
		extra[ids[i]] = r
	}
	raw := splitProductFile(f, nil, extra)
	if _, e = readCylinderProduct(bytes.NewReader(raw), Limits{}, cells); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadSplitCylinderProduct requires the complete fixed product dialect and
// exact product/body name binding. Metadata is validated, never ignored. The
// underlying model reader retains its original whole-graph strictness.
func ReadSplitCylinderProduct(r io.Reader, l Limits) (SplitCylinderProduct, error) {
	p, e := readCylinderProduct(r, l, 2)
	if e != nil {
		return SplitCylinderProduct{}, e
	}
	m := p.Model
	model := SplitCylinderModel{Frame: m.Frame, Name: m.Name, LengthUnit: m.LengthUnit, Radius: m.Radius, Height: m.Height, Solid: m.Solid, Body: m.Body, Shell: m.Shell}
	copy(model.Faces[:], m.Faces)
	copy(model.Edges[:], m.Edges)
	return SplitCylinderProduct{Model: model, Product: p.Product, Definition: p.Definition, ShapeDefinition: p.ShapeDefinition}, nil
}

type cylinderProduct struct {
	Model                                cylinderCellsModel
	Product, Definition, ShapeDefinition int64
}

func readCylinderProduct(r io.Reader, l Limits, cells int) (cylinderProduct, error) {
	fail := func() (cylinderProduct, error) { return cylinderProduct{}, splitFailure() }
	raw, e := io.ReadAll(io.LimitReader(r, (1<<20)+1))
	if e != nil {
		return cylinderProduct{}, e
	}
	if len(raw) > 1<<20 {
		return fail()
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return cylinderProduct{}, e
	}
	if len(f.Entities) > 265 {
		return fail()
	}
	ids := make([]int64, len(splitProductKinds))
	drop := map[int64]bool{}
	identity := map[int64]int64{}
	var rep, body int64
	for id, ent := range f.Entities {
		identity[id] = id
		for _, rec := range ent.Records {
			for i, k := range splitProductKinds {
				if rec.Type == k {
					if len(ent.Records) != 1 || ids[i] != 0 {
						return fail()
					}
					ids[i] = id
					drop[id] = true
				}
			}
			if rec.Type == "ADVANCED_BREP_SHAPE_REPRESENTATION" {
				if rep != 0 {
					return fail()
				}
				rep = id
			}
			if rec.Type == "MANIFOLD_SOLID_BREP" {
				if body != 0 {
					return fail()
				}
				body = id
			}
		}
	}
	for _, id := range ids {
		if id == 0 {
			return fail()
		}
	}
	br, e := simple(f, body, "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return fail()
	}
	name, e := textValue(br.Args[0])
	if e != nil {
		return fail()
	}
	for i, want := range splitProductRecords(ids, rep, name, cells) {
		if assemblyEntity(f.Entities[ids[i]], identity) != want {
			return fail()
		}
	}
	model, e := readCylinderCells(bytes.NewReader(splitProductFile(f, drop, nil)), l, cells)
	if e != nil {
		return fail()
	}
	return cylinderProduct{Model: model, Product: ids[4], Definition: ids[6], ShapeDefinition: ids[7]}, nil
}

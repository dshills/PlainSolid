package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/brep/trimmed"
)

// DiskHoleAssemblyPart retains original local circular stock and attached cuts.
// Empty Bodies denotes an assembly-only product, never an inferred solid.
type DiskHoleAssemblyPart struct {
	ID, Name string
	Bodies   []DiskHoleExtrusionBody
}
type DiskHoleAssemblyModel struct {
	Products    []DiskHoleAssemblyPart
	Occurrences []AssemblyInstance
	LengthUnit  float64
}
type DiskHoleAssemblyProduct struct {
	Bodies        []DiskHoleExtrusionBody
	Native        []trimmed.Solid
	SourceBodyIDs []int64
}
type DiskHoleAssembly struct {
	Assembly Assembly
	Products map[int64]DiskHoleAssemblyProduct
}

func diskAssemblyFailure() error { return fmt.Errorf("step: unsupported canonical disk-hole assembly") }

// WriteDiskHoleAssembly shares product-local analytic bores/pockets across
// explicit proper signed-coordinate occurrences. Complete source graph proof
// precedes output; occurrences are not baked into or rounded onto local supports.
func WriteDiskHoleAssembly(w io.Writer, m DiskHoleAssemblyModel) error {
	if w == nil {
		return diskAssemblyFailure()
	}
	graph := AssemblyModel{LengthUnit: m.LengthUnit, Occurrences: m.Occurrences}
	counts := make([]int, len(m.Products))
	for _, o := range m.Occurrences {
		if !planarSplineRigid(o.Transform) {
			return diskAssemblyFailure()
		}
	}
	for i, p := range m.Products {
		graph.Products = append(graph.Products, AssemblyPart{ID: p.ID, Name: p.Name})
		counts[i] = len(p.Bodies)
	}
	var out bytes.Buffer
	e := writeAssemblyGeometry(&out, graph, counts, func(i int) (*File, []int64, error) {
		var b bytes.Buffer
		if e := WriteDiskHoleExtrusions(&b, DiskHoleExtrusionModel{Bodies: m.Products[i].Bodies, LengthUnit: m.LengthUnit}); e != nil {
			return nil, nil, e
		}
		return stagedAssemblyBodies(&b)
	})
	if e != nil {
		return e
	}
	if _, e = ReadDiskHoleAssembly(bytes.NewReader(out.Bytes()), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(out.Bytes())
	if e == nil && n != out.Len() {
		return io.ErrShortWrite
	}
	return e
}

// ReadDiskHoleAssembly accepts the existing canonical disk-extrusion body graph
// inside an explicit assembly. Each local body is reconstructed and its complete
// continuous model/pcurve topology compared bijectively with the original source.
// Existing standalone and heterogeneous assembly readers remain unchanged.
func ReadDiskHoleAssembly(r io.Reader, l Limits) (DiskHoleAssembly, error) {
	fail := func() (DiskHoleAssembly, error) { return DiskHoleAssembly{}, diskAssemblyFailure() }
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	a, e := ReadAssembly(r, l)
	if e != nil {
		return DiskHoleAssembly{}, e
	}
	for _, o := range a.Occurrences {
		if !planarSplineRigid(o.Transform) {
			return fail()
		}
	}
	counts := map[string]int{}
	for _, ent := range a.Source.Entities {
		for _, rec := range ent.Records {
			counts[rec.Type]++
		}
	}
	if counts["MANIFOLD_SOLID_BREP"] < 1 || counts["MANIFOLD_SOLID_BREP"] > min(l.MaxSolids, 32) || counts["ADVANCED_FACE"] > l.MaxFaces || counts["VERTEX_POINT"] > l.MaxVertices || counts["ORIENTED_EDGE"] > l.MaxEdgeUses {
		return fail()
	}
	out := DiskHoleAssembly{Assembly: a, Products: map[int64]DiskHoleAssemblyProduct{}}
	owners := map[int64]int64{}
	seen := map[int64]bool{}
	for _, p := range a.Products {
		ids, advanced, e := a.productItems(p.DefinitionID)
		if e != nil {
			return fail()
		}
		product := DiskHoleAssemblyProduct{SourceBodyIDs: append([]int64(nil), ids...)}
		for i, id := range ids {
			if seen[id] || !advanced[i] {
				return fail()
			}
			seen[id] = true
			var outer trimmed.ExtrusionCircle
			b, e := recoverCircularExtrusion(a.Source, id, &outer, nil)
			if e != nil {
				return fail()
			}
			d := DiskHoleExtrusionBody{Name: b.Name, Frame: b.Frame, Outer: outer, Holes: b.Holes, Blind: b.Blind, Height: b.Height}
			var raw bytes.Buffer
			if e = WriteDiskHoleExtrusions(&raw, DiskHoleExtrusionModel{Bodies: []DiskHoleExtrusionBody{d}, LengthUnit: a.LengthUnit}); e != nil {
				return fail()
			}
			f, _, _, bodyIDs, e := parseCanonicalCircularExtrusions(&raw, l)
			if e != nil || len(bodyIDs) != 1 {
				return fail()
			}
			used, e := equalCircularExtrusionGraph(a.Source, id, f, bodyIDs[0])
			if e != nil {
				return fail()
			}
			for sourceID := range used {
				for _, rec := range a.Source.Entities[sourceID].Records {
					if planarSplineOwnedType(rec.Type) {
						if prior := owners[sourceID]; prior != 0 && prior != p.DefinitionID {
							return fail()
						}
						owners[sourceID] = p.DefinitionID
					}
				}
			}
			native, e := trimmed.ExtrudeDiskWithHoles(context.Background(), d.Frame, d.Outer, d.Holes, d.Blind, d.Height)
			if e != nil {
				return fail()
			}
			product.Bodies = append(product.Bodies, d)
			product.Native = append(product.Native, native)
		}
		out.Products[p.DefinitionID] = product
	}
	for id, ent := range a.Source.Entities {
		for _, rec := range ent.Records {
			if planarSplineOwnedType(rec.Type) && owners[id] == 0 {
				return fail()
			}
		}
	}
	if len(seen) != counts["MANIFOLD_SOLID_BREP"] {
		return fail()
	}
	return out, nil
}

package step

import (
	"bytes"
	"fmt"
	"io"
	"plainsolid/geom"
)

// PlacedLongEdgeChainModel preserves one canonical local chain and one explicit
// occurrence. Transform is a proper signed-coordinate rotation and finite
// translation. Geometry is never flattened into rounded world coordinates.
type PlacedLongEdgeChainModel struct {
	Model     LongEdgeChainModel
	Transform geom.Affine
}

// WritePlacedLongEdgeChains writes a two-product, single-occurrence assembly.
func WritePlacedLongEdgeChains(w io.Writer, m PlacedLongEdgeChainModel) error {
	if w == nil || !lensRigidAxisPlacement(m.Transform) || len(m.Model.Bodies) != 1 {
		return geom.ErrDomain
	}
	graph := AssemblyModel{LengthUnit: m.Model.LengthUnit, Products: []AssemblyPart{{ID: "placed-chain", Name: "Placed chain"}, {ID: "local-chain", Name: m.Model.Bodies[0].Name}}, Occurrences: []AssemblyInstance{{ID: "chain-occurrence", Name: "Placed chain", Parent: "placed-chain", Child: "local-chain", Transform: m.Transform}}}
	var out bytes.Buffer
	err := writeAssemblyGeometry(&out, graph, []int{0, 1}, func(int) (*File, []int64, error) {
		var b bytes.Buffer
		if e := WriteLongEdgeChains(&b, m.Model); e != nil {
			return nil, nil, e
		}
		return stagedAssemblyBodies(&b)
	})
	if err != nil {
		return err
	}
	n, err := w.Write(out.Bytes())
	if err == nil && n != out.Len() {
		err = io.ErrShortWrite
	}
	return err
}

// ReadPlacedLongEdgeChains accepts the canonical entity order and graph emitted
// by WritePlacedLongEdgeChains, allowing entity ID renumbering. It validates the
// local recipe and reauthors the entire assembly, including source placements.
// Other assembly layouts and equivalent independently authored geometry fail.
func ReadPlacedLongEdgeChains(r io.Reader, l Limits) (PlacedLongEdgeChainModel, error) {
	fail := func() (PlacedLongEdgeChainModel, error) {
		return PlacedLongEdgeChainModel{}, fmt.Errorf("step: unsupported placed long-edge recipe")
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	a, e := ReadAssembly(r, l)
	if e != nil {
		return PlacedLongEdgeChainModel{}, e
	}
	if len(a.Products) != 2 || len(a.Roots) != 1 || len(a.Occurrences) != 1 {
		return fail()
	}
	occ := a.Occurrences[0]
	if occ.Parent != a.Roots[0] || !lensRigidAxisPlacement(occ.Transform) {
		return fail()
	}
	var rep, body int64
	for _, p := range a.Products {
		ids, advanced, err := a.productItems(p.DefinitionID)
		if err != nil {
			return fail()
		}
		if p.DefinitionID == occ.Parent {
			if len(ids) != 0 {
				return fail()
			}
			continue
		}
		if p.DefinitionID != occ.Child || len(ids) != 1 || !advanced[0] {
			return fail()
		}
		body = ids[0]
		for _, rid := range p.Representations {
			if _, err := simple(a.Source, rid, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3); err == nil {
				if rep != 0 {
					return fail()
				}
				rep = rid
			}
		}
	}
	if rep == 0 {
		return fail()
	}
	counts := map[string]int{}
	for _, en := range a.Source.Entities {
		for _, rec := range en.Records {
			counts[rec.Type]++
		}
	}
	if counts["ADVANCED_FACE"] > l.MaxFaces || counts["VERTEX_POINT"] > l.MaxVertices || counts["ORIENTED_EDGE"] > l.MaxEdgeUses {
		return fail()
	}
	base, features, e := recoverLongEdgeRecipe(a.Source, body, true)
	if e != nil {
		return fail()
	}
	features.LongEdgeTreatmentBody = base
	local := LongEdgeChainModel{Bodies: []LongEdgeChainBody{features}, LengthUnit: a.LengthUnit}
	out := PlacedLongEdgeChainModel{Model: local, Transform: occ.Transform}
	var b bytes.Buffer
	if e = WritePlacedLongEdgeChains(&b, out); e != nil {
		return fail()
	}
	expected, e := Parse(&b, l)
	if e != nil || len(expected.Order) != len(a.Source.Order) {
		return fail()
	}
	left, right := map[int64]int64{}, map[int64]int64{}
	for i, id := range a.Source.Order {
		left[id] = int64(i + 1)
		right[expected.Order[i]] = int64(i + 1)
	}
	for i, id := range a.Source.Order {
		if assemblyEntity(a.Source.Entities[id], left) != assemblyEntity(expected.Entities[expected.Order[i]], right) {
			return fail()
		}
	}
	return out, nil
}

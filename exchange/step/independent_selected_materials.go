package step

import (
	"context"
	"io"
	"plainsolid/geom"
)

// SelectedIndependentMaterials retains the complete input separately from the
// qualified subset. OmittedRootIDs identifies products whose geometry was NOT
// accepted; their product envelope and dependency ownership were checked.
// It is not a whole-file material certificate or an assembly interpretation.
type SelectedIndependentMaterials struct {
	Members        []IndependentExactMaterial
	Source         *File
	OmittedRootIDs []int64
	LengthUnit     float64
}

// ReadSelectedIndependentMaterials selects original SHAPE_DEFINITION_REPRESENTATION
// IDs. All roots undergo the same bounded ownership, acyclicity and product proof;
// only selected roots undergo the supported planar/ellipse/cylinder/sphere proof.
// MaxSolids bounds every input root. Face/vertex/use limits bound selected native
// output. Selected units must agree; omitted contexts are not interpreted as units.
func ReadSelectedIndependentMaterials(ctx context.Context, r io.Reader, rootIDs []int64, l Limits, tol float64) (SelectedIndependentMaterials, error) {
	bad := func() (SelectedIndependentMaterials, error) { return SelectedIndependentMaterials{}, geom.ErrInvalid }
	if len(rootIDs) < 1 || len(rootIDs) > 16 {
		return bad()
	}
	selected := map[int64]bool{}
	for _, id := range rootIDs {
		if id <= 0 || selected[id] {
			return bad()
		}
		selected[id] = true
	}
	m, e := readIndependentMaterialSelection(ctx, r, l, tol, 1, 16, true, selected, true, true)
	if e != nil {
		return SelectedIndependentMaterials{}, e
	}
	out := SelectedIndependentMaterials{Members: m.Members, Source: m.Source, LengthUnit: m.LengthUnit}
	for _, id := range m.Source.Order {
		en := m.Source.Entities[id]
		if len(en.Records) == 1 && en.Records[0].Type == "SHAPE_DEFINITION_REPRESENTATION" && !selected[id] {
			out.OmittedRootIDs = append(out.OmittedRootIDs, id)
		}
	}
	return out, nil
}

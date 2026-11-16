package step

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// MixedConicalAP242Part holds one original leaf product and its proved
// local boundary. Repeated occurrences share this single product proof.
type MixedConicalAP242Part struct {
	Product   AssemblyProduct
	Rounded   *ConicalRimFilletBoreSTEP
	Chamfered *EccentricAnnularChamferSTEP
}

// MixedConicalAP242Assembly retains one flat occurrence graph containing
// a rounded conical bore and a chamfered eccentric tapered annulus.
// Occurrences preserve the exact rational rigid placement records.
type MixedConicalAP242Assembly struct {
	Assembly    Assembly
	Parts       map[int64]MixedConicalAP242Part
	Occurrences []ExactNAUOOccurrence
}

var errMixedConicalAP242 = errors.New("step: unsupported exact mixed conical AP242 assembly")

// ReadMixedConicalAP242Assembly authenticates a flat AP242 assembly with
// two original exact local materials and independently proved rigid uses.
// Local topology stays in original coordinates and file units.
func ReadMixedConicalAP242Assembly(ctx context.Context, r io.Reader, l Limits, o geom.SurfacePatchOptions) (MixedConicalAP242Assembly, error) {
	m, err := readMixedConicalAP242Assembly(ctx, r, l, o, false)
	return m.MixedConicalAP242Assembly, err
}

type mixedConicalAP242Source struct {
	MixedConicalAP242Assembly
	group AssemblyProduct
}

func readMixedConicalAP242Assembly(ctx context.Context, r io.Reader, l Limits, o geom.SurfacePatchOptions, nested bool) (mixedConicalAP242Source, error) {
	var zero mixedConicalAP242Source
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	l, err := occurrenceColorSourceLimits(l)
	if err != nil {
		return zero, err
	}
	a, err := ReadAssembly(r, l)
	if err != nil {
		return zero, err
	}
	products, minUses := 3, 2
	if nested {
		products, minUses = 4, 4
	}
	if len(a.Products) != products || len(a.Roots) != 1 || len(a.Occurrences) < minUses || len(a.Occurrences) > 16 {
		return zero, errMixedConicalAP242
	}
	f := a.Source
	if !mixedConicalAP242Schema(f) {
		return zero, errMixedConicalAP242
	}
	layout, err := mixedConicalAP242Parts(a, l, nested)
	if err != nil {
		return zero, err
	}
	records, err := proveMixedConicalAP242Records(a, layout, l)
	if err != nil {
		return zero, err
	}
	out := mixedConicalAP242Source{
		MixedConicalAP242Assembly: MixedConicalAP242Assembly{Assembly: a, Occurrences: records.occurrences, Parts: map[int64]MixedConicalAP242Part{}},
		group:                     layout.group,
	}
	var rounded, chamfered bool
	var faces, vertices, edgeUses int
	for _, product := range a.Products {
		bodyID, ok := layout.parts[product.DefinitionID]
		if !ok {
			continue
		}
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		ids := []int64{records.definitionShapes[product.DefinitionID], records.protocol, records.categories[product.ProductID]}
		raw, err := mixedConicalAP242View(f, ids, l)
		if err != nil {
			return zero, err
		}
		local := MixedConicalAP242Part{Product: product}
		var solid trimmed.Solid
		if hasMixedConicalToroid(f, bodyID) {
			native, err := ReadConicalRimFilletBore(ctx, bytes.NewReader(raw), l, o)
			if err != nil {
				return zero, fmt.Errorf("step: rolling-rim product %d: %w", product.DefinitionID, err)
			}
			if rounded || native.SourceBodyID != bodyID || native.LengthUnit != a.LengthUnit {
				return zero, errMixedConicalAP242
			}
			local.Rounded, rounded = &native, true
			solid = native.Solid
		} else {
			native, err := ReadEccentricAnnularChamfer(ctx, bytes.NewReader(raw), l, o)
			if err != nil {
				return zero, fmt.Errorf("step: chamfered product %d: %w", product.DefinitionID, err)
			}
			if chamfered || native.SourceBodyID != bodyID || native.LengthUnit != a.LengthUnit {
				return zero, errMixedConicalAP242
			}
			local.Chamfered, chamfered = &native, true
			solid = native.Solid
		}
		vertices += len(solid.Vertices())
		for _, face := range solid.Faces() {
			faces++
			edgeUses += len(face.Outer)
			for _, wire := range face.Holes {
				edgeUses += len(wire)
			}
		}
		if faces > l.MaxFaces || vertices > l.MaxVertices || edgeUses > l.MaxEdgeUses {
			return zero, errMixedConicalAP242
		}
		out.Parts[product.DefinitionID] = local
	}
	if !rounded || !chamfered {
		return zero, errMixedConicalAP242
	}
	return out, nil
}

func mixedConicalAP242Schema(f *File) bool {
	schema := "AP242_MANAGED_MODEL_BASED_3D_ENGINEERING_MIM_LF {1 0 10303 442 1 1 4 }"
	count := 0
	for _, r := range f.Header {
		if r.Type != "FILE_SCHEMA" {
			continue
		}
		count++
		if len(r.Args) != 1 || r.Args[0].Kind != Aggregate || len(r.Args[0].Items) != 1 {
			return false
		}
		name, err := textValue(r.Args[0].Items[0])
		if err != nil || name != schema {
			return false
		}
	}
	return count == 1
}

type mixedConicalAP242Layout struct {
	parts    map[int64]int64
	geometry map[int64]bool
	group    AssemblyProduct
}

func mixedConicalAP242Parts(a Assembly, l Limits, nested bool) (mixedConicalAP242Layout, error) {
	var zero mixedConicalAP242Layout
	f := a.Source
	out := mixedConicalAP242Layout{parts: map[int64]int64{}, geometry: map[int64]bool{}}
	owned := map[int64]bool{}
	var datums []int64
	for _, p := range a.Products {
		if len(p.Representations) != 1 {
			return zero, errMixedConicalAP242
		}
		assemblyOnly := p.DefinitionID == a.Roots[0]
		if nested && !assemblyOnly {
			if _, err := simple(f, p.Representations[0], "SHAPE_REPRESENTATION", 3); err == nil {
				if out.group.DefinitionID != 0 {
					return zero, errMixedConicalAP242
				}
				out.group = p
				assemblyOnly = true
			}
		}
		kind := "ADVANCED_BREP_SHAPE_REPRESENTATION"
		if assemblyOnly {
			kind = "SHAPE_REPRESENTATION"
		}
		rep, err := simple(f, p.Representations[0], kind, 3)
		if err != nil {
			return zero, err
		}
		if err := occtLoopContextMode(f, rep.Args[2].Ref, true); err != nil {
			return zero, err
		}
		modelIDs, advanced, err := a.productItems(p.DefinitionID)
		if err != nil {
			return zero, err
		}
		if assemblyOnly {
			if len(modelIDs) != 0 {
				return zero, errMixedConicalAP242
			}
		} else {
			if len(modelIDs) != 1 || !advanced[0] {
				return zero, errMixedConicalAP242
			}
			out.parts[p.DefinitionID] = modelIDs[0]
			closure, err := nauoClosure(f, modelIDs, l.MaxDepth)
			if err != nil {
				return zero, err
			}
			for id := range closure {
				if owned[id] {
					return zero, errMixedConicalAP242
				}
				owned[id], out.geometry[id] = true, true
			}
		}
		items, err := listRefs(rep.Args[1], 1)
		if err != nil {
			return zero, err
		}
		for _, id := range items {
			if _, err := simple(f, id, "AXIS2_PLACEMENT_3D", 4); err == nil {
				if _, err := mappedRationalFrame(f, id); err != nil {
					return zero, err
				}
				datums = append(datums, id)
			}
		}
	}
	datumRecords, err := nauoClosure(f, datums, l.MaxDepth)
	if err != nil {
		return zero, err
	}
	for id := range datumRecords {
		if owned[id] {
			return zero, errMixedConicalAP242
		}
		out.geometry[id] = true
	}
	if len(out.parts) != 2 || nested != (out.group.DefinitionID != 0) {
		return zero, errMixedConicalAP242
	}
	return out, nil
}

func hasMixedConicalToroid(f *File, id int64) bool {
	owned, err := nauoClosure(f, []int64{id}, 16)
	if err != nil {
		return false
	}
	for key := range owned {
		if len(f.Entities[key].Records) == 1 && f.Entities[key].Records[0].Type == "TOROIDAL_SURFACE" {
			return true
		}
	}
	return false
}

func mixedConicalAP242View(f *File, roots []int64, l Limits) ([]byte, error) {
	owned, err := nauoClosure(f, roots, l.MaxDepth)
	if err != nil {
		return nil, err
	}
	view := &File{Header: f.Header, Entities: make(map[int64]Entity, len(owned))}
	for _, id := range f.Order {
		if owned[id] {
			view.Order = append(view.Order, id)
			view.Entities[id] = f.Entities[id]
		}
	}
	return mappedPlanarSourceBytes(view)
}

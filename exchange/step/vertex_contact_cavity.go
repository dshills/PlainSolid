package step

import (
	"context"
	"fmt"
	"io"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

func readExactPlanarCavityBody(f *File, id int64, maxVertices, maxFaces, maxUses int) (Body, error) {
	r, err := simple(f, id, "BREP_WITH_VOIDS", 3)
	if err != nil {
		return Body{}, err
	}
	name, err := textValue(r.Args[0])
	if err != nil {
		return Body{}, err
	}
	outer, err := ref(r.Args[1])
	if err != nil {
		return Body{}, err
	}
	wrappers, err := listRefs(r.Args[2], 1)
	if err != nil || len(wrappers) > 64 || len(wrappers) >= maxFaces {
		return Body{}, fmt.Errorf("step: invalid exact planar void shell count")
	}
	shellIDs, seen := []int64{outer}, map[int64]bool{outer: true}
	for _, wrapper := range wrappers {
		r, err := simple(f, wrapper, "ORIENTED_CLOSED_SHELL", 4)
		if err != nil || r.Args[1].Kind != Derived {
			return Body{}, fmt.Errorf("step: invalid exact oriented closed shell")
		}
		if _, err := textValue(r.Args[0]); err != nil {
			return Body{}, err
		}
		child, err := ref(r.Args[2])
		if err != nil || seen[child] {
			return Body{}, fmt.Errorf("step: repeated or invalid exact closed shell")
		}
		same, err := boolean(r.Args[3])
		if err != nil || same {
			return Body{}, fmt.Errorf("step: exact planar void must reverse its outward shell")
		}
		seen[child] = true
		shellIDs = append(shellIDs, child)
	}
	var vertices []geom.Vec3
	var edges []brep.Edge
	var faces []brep.Face
	expandedUses := 0
	for i, shell := range shellIDs {
		b, err := readPlanarClosedShell(f, shell, "", maxVertices-len(vertices), maxFaces-len(faces), maxUses-expandedUses, true, true)
		if err != nil {
			return Body{}, fmt.Errorf("step: original shell #%d: %w", shell, err)
		}
		s := b.Solid
		if i != 0 {
			s = s.Reversed()
		}
		vo, eo := len(vertices), len(edges)
		vertices = append(vertices, s.Vertices()...)
		for _, edge := range s.Edges() {
			edges = append(edges, brep.Edge{Start: edge.Start + vo, End: edge.End + vo})
		}
		for _, face := range s.Faces() {
			for _, loop := range append([][]brep.EdgeUse{face.Boundary}, face.Holes...) {
				expandedUses += len(loop)
				for j := range loop {
					loop[j].Edge += eo
				}
			}
			faces = append(faces, face)
		}
	}
	solid, err := brep.NewSolid(vertices, edges, faces)
	if err != nil {
		return Body{}, err
	}
	if solid.Topology().Shells != len(shellIDs) {
		return Body{}, fmt.Errorf("step: exact planar shell declarations do not match native incidence")
	}
	return Body{Name: name, Solid: solid}, nil
}

func writeVertexContactModel(ctx context.Context, w io.Writer, m Model, o VertexContactSTEPOptions) error {
	hasVoids := false
	for _, b := range m.Bodies {
		hasVoids = hasVoids || b.Solid.Topology().Shells > 1
	}
	if !hasVoids {
		return writePlanarModel(w, m, !o.FacetedWriter, true)
	}
	if o.FacetedWriter || !o.Proof.AllowCavities {
		return fmt.Errorf("step: exact planar cavities require advanced output and explicit cavity proof")
	}
	flat := Model{LengthUnit: m.LengthUnit}
	recipes := make([]planarCavitySTEPRecipe, len(m.Bodies))
	for i, b := range m.Bodies {
		recipes[i].name = b.Name
		if b.Solid.Topology().Shells == 1 {
			flat.Bodies = append(flat.Bodies, b)
			continue
		}
		shells, err := b.Solid.Shells()
		if err != nil {
			return err
		}
		proof, err := collision.InspectShellNesting(ctx, b.Solid, o.Proof.Validation)
		if err != nil {
			return err
		}
		if len(proof) != len(shells) || len(shells) > 65 {
			return fmt.Errorf("step: exact planar cavity source ordering failed")
		}
		outer := -1
		for j, p := range proof {
			if p.Depth == 0 {
				if outer >= 0 {
					return fmt.Errorf("step: more than one exact planar exterior")
				}
				outer = j
			}
		}
		if outer < 0 {
			return fmt.Errorf("step: no exact planar exterior")
		}
		flat.Bodies = append(flat.Bodies, Body{Name: b.Name, Solid: shells[outer]})
		for j, s := range shells {
			if j != outer {
				if proof[j].Parent != outer || proof[j].Depth != 1 {
					return fmt.Errorf("step: exact planar void is not directly enclosed")
				}
				flat.Bodies = append(flat.Bodies, Body{Solid: s.Reversed()})
				recipes[i].reversed = append(recipes[i].reversed, true)
			}
		}
	}
	return writePlanarCavitySTEPGraph(w, flat, recipes, o.Limits, true)
}

package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/brep/parametric"
	"plainsolid/csg/poly"
)

// RepairedNAUOFamily explicitly distinguishes preserved and reconstructed assets.
type RepairedNAUOFamily string

const (
	RepairedNAUOExactPlanar RepairedNAUOFamily = "exact-planar"
	RepairedNAUOCircular    RepairedNAUOFamily = "repaired-circular"
	RepairedNAUOTube        RepairedNAUOFamily = "repaired-circular-tube"
)

type RepairedNAUOProduct struct {
	Product      AssemblyProduct
	SourceBodyID int64
	Family       RepairedNAUOFamily
	Planar       Body
	Cylinder     CylinderBody
	Tube         TubeBody
	Circular     parametric.Solid
}

// RepairedCircularNAUOAssembly distinguishes exact planar assets from explicitly
// reconstructed circular assets, while retaining exact original occurrence paths.
type RepairedCircularNAUOAssembly struct {
	Source        *File
	Root          AssemblyProduct
	Groups        []AssemblyProduct
	Parts         []RepairedNAUOProduct
	LengthUnit    float64
	Relationships []ExactNAUOOccurrence
	Occurrences   []ExactDAGNAUOOccurrence
	Repairs       []RepairedCircularNAUO
	TubeRepairs   []RepairedCircularTubeNAUO
}

// ReadRepairedCircularNAUOAssembly is an opt-in mixed import. It requires at least
// one exact cardinal planar and one explicitly repaired canonical cylinder or
// through-tube product. Flat and bounded DAG ownership are preserved; unsupported families
// fail atomically. Repairs are returned once per local circular product, never
// once per expanded occurrence. No exact source-equivalence is claimed.
func ReadRepairedCircularNAUOAssembly(r io.Reader, l Limits, o CircularNAUORepairOptions) (RepairedCircularNAUOAssembly, error) {
	fail := func() (RepairedCircularNAUOAssembly, error) {
		return RepairedCircularNAUOAssembly{}, mappedPlanarFailure()
	}
	lim, e := l.defaults()
	if e != nil {
		return fail()
	}
	if r == nil {
		return fail()
	}
	raw, e := io.ReadAll(io.LimitReader(r, lim.MaxBytes+1))
	if e != nil {
		return RepairedCircularNAUOAssembly{}, e
	}
	if int64(len(raw)) > lim.MaxBytes {
		return fail()
	}
	a, e := ReadAssembly(bytes.NewReader(raw), l)
	if e != nil {
		return RepairedCircularNAUOAssembly{}, e
	}
	if len(a.Products) < 3 || len(a.Products) > 8 || len(a.Roots) != 1 || len(a.Occurrences) < 2 || len(a.Occurrences) > 32 {
		return fail()
	}
	f := a.Source
	if len(f.Entities) > 8192 {
		return fail()
	}
	out := RepairedCircularNAUOAssembly{Source: f, LengthUnit: a.LengthUnit}
	parts := map[int64]bool{}
	var rootRep Record
	groupReps := []Record{}
	groups := map[int64]bool{}
	for _, p := range a.Products {
		if len(p.Representations) != 1 {
			return fail()
		}
		if p.DefinitionID == a.Roots[0] {
			out.Root = p
			rootRep, e = simple(f, p.Representations[0], "SHAPE_REPRESENTATION", 3)
			if e != nil {
				return fail()
			}
		} else {
			en := f.Entities[p.Representations[0]]
			if len(en.Records) == 1 && en.Records[0].Type == "SHAPE_REPRESENTATION" {
				groups[p.DefinitionID] = true
				out.Groups = append(out.Groups, p)
				rep, e := simple(f, p.Representations[0], "SHAPE_REPRESENTATION", 3)
				if e != nil {
					return fail()
				}
				groupReps = append(groupReps, rep)
			} else {
				parts[p.DefinitionID] = true
			}
		}
	}
	if len(parts) < 2 {
		return fail()
	}
	prefix, e := nauoSIPrefix(f, rootRep.Args[2].Ref)
	if e != nil {
		return fail()
	}
	for _, rep := range groupReps {
		gp, e := nauoSIPrefix(f, rep.Args[2].Ref)
		if e != nil || gp != prefix {
			return fail()
		}
	}
	allowedRoots := []int64{}
	reps := append([]Record{rootRep}, groupReps...)
	owned := map[int64]int64{}
	nv, nf, nu := 0, 0, 0
	planarCount, circularCount := 0, 0
	for _, p := range a.Products {
		if !parts[p.DefinitionID] {
			continue
		}
		rep, e := simple(f, p.Representations[0], "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
		if e != nil {
			return fail()
		}
		u, e := nauoSIPrefix(f, rep.Args[2].Ref)
		if e != nil || u != prefix {
			return fail()
		}
		ids, advanced, e := a.productItems(p.DefinitionID)
		if e != nil || len(ids) != 1 || !advanced[0] {
			return fail()
		}
		closure, e := nauoClosure(f, ids, lim.MaxDepth)
		if e != nil {
			return fail()
		}
		for id := range closure {
			for _, r := range f.Entities[id].Records {
				if planarSplineOwnedType(r.Type) || r.Type == "SEAM_CURVE" || r.Type == "CARTESIAN_POINT" || r.Type == "DIRECTION" || r.Type == "VECTOR" || r.Type == "AXIS2_PLACEMENT_3D" {
					if owner := owned[id]; owner != 0 && owner != p.DefinitionID {
						return fail()
					}
					owned[id] = p.DefinitionID
				}
			}
		}
		family, e := planarSplineBodyFamily(f, ids[0], true)
		if e != nil {
			if !repairedNAUOTubeFamily(f, ids[0]) {
				return fail()
			}
			family = PlanarSplineFamily("repaired-tube")
		}
		product := RepairedNAUOProduct{Product: p, SourceBodyID: ids[0]}
		switch family {
		case PlanarSplinePlanar:
			if e = mappedPlanarExactEdges(f, ids[0]); e != nil {
				return fail()
			}
			body, e := readBody(f, ids[0], lim.MaxVertices, lim.MaxFaces, lim.MaxEdgeUses, true)
			if e != nil {
				return fail()
			}
			nv += len(body.Solid.Vertices())
			nf += len(body.Solid.Faces())
			for _, face := range body.Solid.Faces() {
				nu += len(face.Boundary)
				for _, hole := range face.Holes {
					nu += len(hole)
				}
			}
			if e = poly.ValidateEmbeddedBoundary(context.Background(), body.Solid, poly.Options{}); e != nil {
				return fail()
			}
			product.Family = RepairedNAUOExactPlanar
			product.Planar = body
			planarCount++
		case planarSplineCylinder:
			repaired, e := RepairCircularNAUOProduct(bytes.NewReader(raw), l, p.DefinitionID, o)
			if e != nil {
				return fail()
			}
			if repaired.Source.SourceBodyID != ids[0] {
				return fail()
			}
			nv += len(repaired.Solid.Vertices())
			nf += len(repaired.Solid.Faces())
			nu += 4 * len(repaired.Solid.Faces())
			product.Family = RepairedNAUOCircular
			product.Cylinder = repaired.Body
			product.Circular = repaired.Solid
			out.Repairs = append(out.Repairs, repaired)
			circularCount++
		case PlanarSplineFamily("repaired-tube"):
			repaired, e := RepairCircularTubeNAUOProduct(bytes.NewReader(raw), l, p.DefinitionID, o)
			if e != nil || repaired.SourceBodyID != ids[0] {
				return fail()
			}
			nv += len(repaired.Solid.Vertices())
			nf += len(repaired.Solid.Faces())
			nu += 4 * len(repaired.Solid.Faces())
			product.Family = RepairedNAUOTube
			product.Tube = repaired.Body
			product.Circular = repaired.Solid
			out.TubeRepairs = append(out.TubeRepairs, repaired)
			circularCount++
		default:
			return fail()
		}
		if nv > lim.MaxVertices || nf > lim.MaxFaces || nu > lim.MaxEdgeUses {
			return fail()
		}
		out.Parts = append(out.Parts, product)
		allowedRoots = append(allowedRoots, ids[0])
		reps = append(reps, rep)
	}
	if planarCount == 0 || circularCount == 0 {
		return fail()
	}
	for _, rep := range reps {
		items, e := listRefs(rep.Args[1], 1)
		if e != nil {
			return fail()
		}
		for _, id := range items {
			en := f.Entities[id]
			if len(en.Records) == 1 && en.Records[0].Type == "AXIS2_PLACEMENT_3D" {
				if _, e = mappedRationalFrame(f, id); e != nil {
					return fail()
				}
				allowedRoots = append(allowedRoots, id)
			}
		}
	}
	geometry, e := nauoClosure(f, allowedRoots, lim.MaxDepth)
	if e != nil {
		return fail()
	}
	occurrences := map[int64]AssemblyOccurrence{}
	incoming, outgoing := map[int64]int{}, map[int64]int{}
	for _, u := range a.Occurrences {
		if u.Parent != out.Root.DefinitionID && !groups[u.Parent] {
			return fail()
		}
		if !groups[u.Child] && !parts[u.Child] {
			return fail()
		}
		occurrences[u.ID] = u
		incoming[u.Child]++
		outgoing[u.Parent]++
	}
	for _, p := range a.Products {
		if p.DefinitionID != out.Root.DefinitionID && incoming[p.DefinitionID] == 0 {
			return fail()
		}
		if (p.DefinitionID == out.Root.DefinitionID || groups[p.DefinitionID]) && outgoing[p.DefinitionID] == 0 {
			return fail()
		}
	}
	roots := []int64{}
	linked := map[int64]bool{}
	relSeen, trSeen := map[int64]bool{}, map[int64]bool{}
	for _, id := range f.Order {
		en := f.Entities[id]
		for _, rec := range en.Records {
			if (planarSplineOwnedType(rec.Type) || rec.Type == "SEAM_CURVE" || rec.Type == "CARTESIAN_POINT" || rec.Type == "DIRECTION" || rec.Type == "VECTOR" || rec.Type == "AXIS2_PLACEMENT_3D") && !geometry[id] {
				return fail()
			}
		}
		if len(en.Records) != 1 {
			continue
		}
		rec := en.Records[0]
		switch rec.Type {
		case "SHAPE_DEFINITION_REPRESENTATION", "APPLICATION_PROTOCOL_DEFINITION", "PRODUCT_RELATED_PRODUCT_CATEGORY":
			roots = append(roots, id)
		case "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION":
			roots = append(roots, id)
			if len(rec.Args) != 2 {
				return fail()
			}
			pds, e := simple(f, rec.Args[1].Ref, "PRODUCT_DEFINITION_SHAPE", 3)
			if e != nil {
				return fail()
			}
			oid := pds.Args[2].Ref
			u, ok := occurrences[oid]
			if !ok || linked[oid] {
				return fail()
			}
			linked[oid] = true
			relID := rec.Args[0].Ref
			var tid int64
			for _, v := range f.Entities[relID].Records {
				if v.Type == "REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION" {
					tid = v.Args[0].Ref
				}
			}
			if relSeen[relID] || trSeen[tid] {
				return fail()
			}
			relSeen[relID] = true
			trSeen[tid] = true
			tr, e := simple(f, tid, "ITEM_DEFINED_TRANSFORMATION", 4)
			if e != nil {
				return fail()
			}
			fromID, toID := tr.Args[2].Ref, tr.Args[3].Ref
			from, e := mappedRationalFrame(f, fromID)
			if e != nil {
				return fail()
			}
			to, e := mappedRationalFrame(f, toID)
			if e != nil {
				return fail()
			}
			out.Relationships = append(out.Relationships, ExactNAUOOccurrence{Name: u.Name, NAUOID: oid, CDSRID: id, RelationshipID: relID, TransformationID: tid, FromPlacementID: fromID, ToPlacementID: toID, ParentID: u.Parent, ChildID: u.Child, pose: composeMappedPose(to, inverseMappedPose(from))})
		}
	}
	if len(linked) != len(a.Occurrences) {
		return fail()
	}
	all, e := nauoClosure(f, roots, lim.MaxDepth)
	if e != nil || len(all) != len(f.Entities) {
		return fail()
	}
	children := map[int64][]ExactNAUOOccurrence{}
	for _, u := range out.Relationships {
		children[u.ParentID] = append(children[u.ParentID], u)
	}
	active := map[int64]bool{}
	reached := map[int64]bool{}
	var expand func(int64, []ExactNAUOOccurrence, mappedRationalBasis) error
	expand = func(id int64, path []ExactNAUOOccurrence, pose mappedRationalBasis) error {
		if len(path) > 4 || len(path) > lim.MaxDepth || active[id] {
			return mappedPlanarFailure()
		}
		reached[id] = true
		if parts[id] {
			if len(out.Occurrences) >= 64 {
				return mappedPlanarFailure()
			}
			out.Occurrences = append(out.Occurrences, ExactDAGNAUOOccurrence{Path: append([]ExactNAUOOccurrence(nil), path...), pose: pose})
			return nil
		}
		active[id] = true
		for _, u := range children[id] {
			next := u.pose
			if len(path) > 0 {
				next = composeMappedPose(pose, u.pose)
			}
			if e := expand(u.ChildID, append(path, u), next); e != nil {
				return e
			}
		}
		delete(active, id)
		return nil
	}
	if e = expand(out.Root.DefinitionID, nil, mappedRationalBasis{}); e != nil || len(reached) != len(a.Products) {
		return fail()
	}
	return out, nil
}

// Structural discriminator only; the separate tube repair re-proves every
// support, owner, material sense and displacement before accepting this family.
func repairedNAUOTubeFamily(f *File, id int64) bool {
	b, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return false
	}
	sh, e := simple(f, b.Args[1].Ref, "CLOSED_SHELL", 2)
	if e != nil {
		return false
	}
	fs, e := listRefs(sh.Args[1], 4)
	if e != nil || len(fs) != 4 {
		return false
	}
	p, c := 0, 0
	for _, id := range fs {
		face, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil {
			return false
		}
		ent := f.Entities[face.Args[2].Ref]
		if len(ent.Records) != 1 {
			return false
		}
		switch ent.Records[0].Type {
		case "PLANE":
			p++
		case "CYLINDRICAL_SURFACE":
			c++
		default:
			return false
		}
	}
	return p == 2 && c == 2
}

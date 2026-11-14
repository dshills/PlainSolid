package step

import (
	"bytes"
	"io"
	"plainsolid/geom"
)

// DAGExactMappedOccurrence records one expanded source path. Local leaf records
// may be shared by different groups; Path, not leaf ItemID, identifies a use.
type DAGExactMappedOccurrence struct {
	Occurrence ExactMappedOccurrence
	Path       []MappedSourceStep
}
type DAGRationalMappedPlanarMaterials struct {
	Source      *File
	Name        string
	LengthUnit  float64
	Bodies      map[int64]Body
	MapOrder    []int64
	Occurrences []DAGExactMappedOccurrence
}

// ReadDAGRationalMappedPlanarMaterials accepts a bounded acyclic mapped group
// graph: at most8 nonroot group definitions,4 mapping steps per path,16 children
// per representation,16 unique leaf uses and128 expanded uses. Identity mapping
// origins and a single source unit/context are required. Local proof is unchanged.
func ReadDAGRationalMappedPlanarMaterials(r io.Reader, l Limits) (DAGRationalMappedPlanarMaterials, error) {
	fail := func() (DAGRationalMappedPlanarMaterials, error) {
		return DAGRationalMappedPlanarMaterials{}, mappedPlanarFailure()
	}
	lim, e := l.defaults()
	if e != nil {
		return fail()
	}
	f, e := Parse(r, l)
	if e != nil {
		return DAGRationalMappedPlanarMaterials{}, e
	}
	var sdrID int64
	count := 0
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) == 1 && en.Records[0].Type == "SHAPE_DEFINITION_REPRESENTATION" {
			sdrID = id
			count++
		}
	}
	if count != 1 {
		return fail()
	}
	sdr, e := simple(f, sdrID, "SHAPE_DEFINITION_REPRESENTATION", 2)
	if e != nil {
		return fail()
	}
	rootID := sdr.Args[1].Ref
	root, e := simple(f, rootID, "SHAPE_REPRESENTATION", 3)
	if e != nil {
		return fail()
	}
	name, e := textValue(root.Args[0])
	if e != nil {
		return fail()
	}
	rootItems, e := listRefs(root.Args[1], 1)
	if e != nil {
		return fail()
	}
	out := DAGRationalMappedPlanarMaterials{Source: f, Name: name}
	leafIDs := []int64{}
	leafSeen := map[int64]bool{}
	groups := map[int64]bool{}
	active := map[int64]bool{}
	identity := mappedRationalBasis{Origin: mappedRatVec(geom.Vec3{}), X: mappedRatVec(geom.Vec3{X: 1}), Y: mappedRatVec(geom.Vec3{Y: 1}), Z: mappedRatVec(geom.Vec3{Z: 1})}
	var walk func(int64, mappedRationalBasis, []MappedSourceStep, int) error
	walk = func(repID int64, pose mappedRationalBasis, path []MappedSourceStep, depth int) error {
		if depth >= 4 || depth >= lim.MaxDepth || active[repID] {
			return mappedPlanarFailure()
		}
		active[repID] = true
		defer delete(active, repID)
		rep, e := simple(f, repID, "SHAPE_REPRESENTATION", 3)
		if e != nil || rep.Args[2].Ref != root.Args[2].Ref {
			return mappedPlanarFailure()
		}
		if _, e = textValue(rep.Args[0]); e != nil {
			return e
		}
		items, e := listRefs(rep.Args[1], 1)
		if e != nil || len(items) > 16 {
			return mappedPlanarFailure()
		}
		seen := map[int64]bool{}
		for _, id := range items {
			if seen[id] {
				return mappedPlanarFailure()
			}
			seen[id] = true
			item, e := simple(f, id, "MAPPED_ITEM", 3)
			if e != nil {
				return e
			}
			n, e := textValue(item.Args[0])
			if e != nil {
				return e
			}
			mid := item.Args[1].Ref
			mapping, e := simple(f, mid, "REPRESENTATION_MAP", 2)
			if e != nil {
				return e
			}
			origin, e := mappedExactFrame(f, mapping.Args[0].Ref)
			if e != nil || frameTransform(origin) != geom.Identity() {
				return mappedPlanarFailure()
			}
			target := mapping.Args[1].Ref
			en, ok := f.Entities[target]
			if !ok || len(en.Records) != 1 {
				return mappedPlanarFailure()
			}
			local, e := mappedRationalOperator(f, item.Args[2].Ref)
			if e != nil {
				return e
			}
			composed := composeMappedPose(pose, local)
			next := append(append([]MappedSourceStep(nil), path...), MappedSourceStep{Name: n, ItemID: id, TargetID: item.Args[2].Ref, MapID: mid, RepresentationID: target})
			switch en.Records[0].Type {
			case "SHAPE_REPRESENTATION":
				groups[target] = true
				if len(groups) > 8 {
					return mappedPlanarFailure()
				}
				if e = walk(target, composed, next, depth+1); e != nil {
					return e
				}
			case "ADVANCED_BREP_SHAPE_REPRESENTATION":
				if len(out.Occurrences) >= 128 {
					return mappedPlanarFailure()
				}
				if !leafSeen[id] {
					leafSeen[id] = true
					leafIDs = append(leafIDs, id)
					if len(leafIDs) > 16 {
						return mappedPlanarFailure()
					}
				}
				out.Occurrences = append(out.Occurrences, DAGExactMappedOccurrence{Occurrence: ExactMappedOccurrence{Name: n, ItemID: id, TargetID: item.Args[2].Ref, MapID: mid, pose: composed}, Path: next})
			default:
				return mappedPlanarFailure()
			}
		}
		return nil
	}
	if e = walk(rootID, identity, nil, 0); e != nil || len(groups) == 0 {
		return fail()
	}
	whole, e := mappedPlanarPartition(f, rootID, rootItems, lim)
	if e != nil || len(whole.Entities) != len(f.Entities) {
		return fail()
	}
	flat, e := mappedPlanarPartition(f, rootID, leafIDs, lim)
	if e != nil {
		return fail()
	}
	raw, e := mappedPlanarSourceBytes(flat)
	if e != nil {
		return fail()
	}
	local, e := ReadRationalMappedPlanarMaterials(bytes.NewReader(raw), l)
	if e != nil {
		return fail()
	}
	out.LengthUnit, out.Bodies, out.MapOrder = local.LengthUnit, local.Bodies, local.MapOrder
	for _, u := range out.Occurrences {
		if _, ok := out.Bodies[u.Occurrence.MapID]; !ok {
			return fail()
		}
	}
	return out, nil
}
func WriteDAGRationalMappedPlanarSource(w io.Writer, source *File, l Limits) error {
	if w == nil || source == nil {
		return mappedPlanarFailure()
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	if _, e = ReadDAGRationalMappedPlanarMaterials(bytes.NewReader(raw), l); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}

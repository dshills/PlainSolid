package step

import (
	"bytes"
	"io"
)

// MappedPlanarMaterials preserves distinct local source representations and
// their explicit uses. Material keys and every nested source ID refer to actual
// original entity IDs. Nested Source files are independently validated proof
// partitions; Source retains the complete authored graph.
type MappedPlanarMaterials struct {
	Source     *File
	Name       string
	LengthUnit float64
	Materials  map[int64]MappedPlanarMaterial
	MapOrder   []int64
}

func mappedPlanarPartition(f *File, rootID int64, items []int64, l Limits) (*File, error) {
	view := &File{Entities: map[int64]Entity{}, Header: f.Header}
	root := f.Entities[rootID]
	root.Records = append([]Record(nil), root.Records...)
	root.Records[0].Args = append([]Value(nil), root.Records[0].Args...)
	values := make([]Value, len(items))
	for i, id := range items {
		values[i] = Value{Kind: Reference, Ref: id}
	}
	root.Records[0].Args[1] = Value{Kind: Aggregate, Items: values}
	active := map[int64]bool{}
	var walk func(int64, int) error
	walk = func(id int64, depth int) error {
		if depth > l.MaxDepth || active[id] {
			return mappedPlanarFailure()
		}
		if _, ok := view.Entities[id]; ok {
			return nil
		}
		en, ok := f.Entities[id]
		if !ok {
			return mappedPlanarFailure()
		}
		if id == rootID {
			en = root
		}
		active[id] = true
		var value func(Value) error
		value = func(v Value) error {
			if v.Kind == Reference {
				return walk(v.Ref, depth+1)
			}
			for _, x := range v.Items {
				if e := value(x); e != nil {
					return e
				}
			}
			return nil
		}
		for _, r := range en.Records {
			for _, v := range r.Args {
				if e := value(v); e != nil {
					return e
				}
			}
		}
		delete(active, id)
		view.Entities[id] = en
		return nil
	}
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) != 1 {
			continue
		}
		switch en.Records[0].Type {
		case "SHAPE_DEFINITION_REPRESENTATION", "APPLICATION_PROTOCOL_DEFINITION", "PRODUCT_RELATED_PRODUCT_CATEGORY":
			if e := walk(id, 0); e != nil {
				return nil, e
			}
		}
	}
	for _, id := range f.Order {
		if _, ok := view.Entities[id]; ok {
			view.Order = append(view.Order, id)
		}
	}
	return view, nil
}

// ReadMappedPlanarMaterials accepts2–8 separately owned local material maps.
// Each uses the complete exact cardinal-plane/model-line/pcurve proof of
// ReadMappedPlanarMaterial. Nested maps, geometry shared between local materials,
// extra geometry, mixed contexts and unsupported target operators fail atomically.
func ReadMappedPlanarMaterials(r io.Reader, l Limits) (MappedPlanarMaterials, error) {
	fail := func() (MappedPlanarMaterials, error) { return MappedPlanarMaterials{}, mappedPlanarFailure() }
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	f, e := Parse(r, l)
	if e != nil {
		return MappedPlanarMaterials{}, e
	}
	types := map[string][]int64{}
	for _, id := range f.Order {
		for _, rec := range f.Entities[id].Records {
			types[rec.Type] = append(types[rec.Type], id)
		}
	}
	maps := types["REPRESENTATION_MAP"]
	if len(maps) < 2 || len(maps) > min(8, l.MaxSolids) || len(types["MANIFOLD_SOLID_BREP"]) != len(maps) || len(types["ADVANCED_BREP_SHAPE_REPRESENTATION"]) != len(maps) || len(types["SHAPE_REPRESENTATION"]) != 1 || len(types["ADVANCED_FACE"]) > l.MaxFaces || len(types["VERTEX_POINT"]) > l.MaxVertices || len(types["ORIENTED_EDGE"]) > l.MaxEdgeUses || len(types["MAPPED_ITEM"]) > 16 {
		return fail()
	}
	rootID := types["SHAPE_REPRESENTATION"][0]
	root, e := simple(f, rootID, "SHAPE_REPRESENTATION", 3)
	if e != nil {
		return fail()
	}
	items, e := listRefs(root.Args[1], 1)
	if e != nil || len(items) != len(types["MAPPED_ITEM"]) {
		return fail()
	}
	known := map[int64]bool{}
	for _, id := range maps {
		known[id] = true
	}
	byMap := map[int64][]int64{}
	seen := map[int64]bool{}
	for _, id := range items {
		if seen[id] {
			return fail()
		}
		seen[id] = true
		m, e := simple(f, id, "MAPPED_ITEM", 3)
		if e != nil || !known[m.Args[1].Ref] {
			return fail()
		}
		byMap[m.Args[1].Ref] = append(byMap[m.Args[1].Ref], id)
	}
	out := MappedPlanarMaterials{Source: f, Materials: map[int64]MappedPlanarMaterial{}, MapOrder: append([]int64(nil), maps...)}
	covered := map[int64]bool{}
	owned := map[int64]int64{}
	bodySeen := map[int64]bool{}
	for _, mid := range maps {
		if len(byMap[mid]) == 0 {
			return fail()
		}
		view, e := mappedPlanarPartition(f, rootID, byMap[mid], l)
		if e != nil {
			return fail()
		}
		raw, e := mappedPlanarSourceBytes(view)
		if e != nil {
			return fail()
		}
		m, e := ReadMappedPlanarMaterial(bytes.NewReader(raw), l)
		if e != nil || m.MapID != mid || bodySeen[m.SourceBodyID] {
			return fail()
		}
		bodySeen[m.SourceBodyID] = true
		if out.LengthUnit != 0 && out.LengthUnit != m.LengthUnit {
			return fail()
		}
		out.LengthUnit = m.LengthUnit
		out.Name = m.Name
		out.Materials[mid] = m
		for id := range view.Entities {
			covered[id] = true
		}
		// The source local-body closure may share no geometric record with another
		// local material. Common contexts and map-target datums are not body geometry.
		todo := []int64{m.SourceBodyID}
		visited := map[int64]bool{}
		for len(todo) > 0 {
			id := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			if visited[id] {
				continue
			}
			visited[id] = true
			en, ok := f.Entities[id]
			if !ok {
				return fail()
			}
			var value func(Value)
			value = func(v Value) {
				if v.Kind == Reference {
					todo = append(todo, v.Ref)
				}
				for _, x := range v.Items {
					value(x)
				}
			}
			for _, rec := range en.Records {
				geometry := planarSplineOwnedType(rec.Type) || rec.Type == "CARTESIAN_POINT" || rec.Type == "DIRECTION" || rec.Type == "VECTOR" || rec.Type == "AXIS2_PLACEMENT_3D"
				if geometry {
					if prior := owned[id]; prior != 0 && prior != mid {
						return fail()
					}
					owned[id] = mid
				}
				for _, v := range rec.Args {
					value(v)
				}
			}
		}
	}
	if len(covered) != len(f.Entities) {
		return fail()
	}
	return out, nil
}

// WriteMappedPlanarMaterialsSource validates the complete source graph before
// source write-back; edited cached Body or Transform fields are not writer inputs.
func WriteMappedPlanarMaterialsSource(w io.Writer, source *File) error {
	if w == nil || source == nil {
		return mappedPlanarFailure()
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	if _, e := ReadMappedPlanarMaterials(bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}

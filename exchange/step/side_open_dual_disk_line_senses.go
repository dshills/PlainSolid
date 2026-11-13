package step

import (
	"fmt"
	"plainsolid/geom"
)

// SideOpenDualDiskReadOptions permits selected exact equivalent STEP encodings.
// Zero options preserve the canonical ReadSideOpenDualDiskProduct contract.
type SideOpenDualDiskReadOptions struct {
	// AllowReversedModelLines permits a direct LINE with a negated cardinal unit
	// direction paired with EDGE_CURVE.same_sense=false. The line origin must be
	// the original edge-start vertex, vector magnitude exactly one, and the line,
	// vector and direction must each be singly owned. Coedge and face encodings
	// remain canonical; circle changes require the separate circle option.
	// This does not admit pcurves or repair geometry.
	AllowReversedModelLines bool
	// AllowRephasedModelCircles permits a different signed cardinal reference
	// direction on the same oriented supporting circle. Exact endpoints and the
	// directed minor arc are proved; normals and edge senses are not flipped.
	AllowRephasedModelCircles bool
}

// f is the reader's private parsed graph. Only successfully proved source line
// encodings are normalized; the full graph comparison still runs afterward.
func sideDiskNormalizeReversedLines(f *File) ([]int64, error) {
	fail := func() ([]int64, error) { return nil, fmt.Errorf("step: unsupported reversed side-disk model line") }
	counts := map[int64]int{}
	for _, en := range f.Entities {
		var values []Value
		for _, r := range en.Records {
			values = append(values, r.Args...)
		}
		for len(values) > 0 {
			v := values[len(values)-1]
			values = values[:len(values)-1]
			if v.Kind == Reference {
				counts[v.Ref]++
			}
			values = append(values, v.Items...)
		}
	}
	var normalized []int64
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) != 1 || en.Records[0].Type != "EDGE_CURVE" {
			continue
		}
		edge, e := simple(f, id, "EDGE_CURVE", 5)
		if e != nil {
			return fail()
		}
		model, e := ref(edge.Args[3])
		if e != nil {
			return fail()
		}
		me := f.Entities[model]
		if len(me.Records) != 1 || me.Records[0].Type != "LINE" {
			continue
		}
		same, e := boolean(edge.Args[4])
		if e != nil {
			return fail()
		}
		if same {
			continue
		}
		if len(normalized) >= 128 {
			return fail()
		}
		line, e := simple(f, model, "LINE", 3)
		if e != nil {
			return fail()
		}
		pointID, e := ref(line.Args[1])
		if e != nil {
			return fail()
		}
		origin, e := splitTriple(f, pointID, "CARTESIAN_POINT")
		if e != nil {
			return fail()
		}
		var ends [2]geom.Vec3
		for i := range ends {
			vertexID, e := ref(edge.Args[1+i])
			if e != nil {
				return fail()
			}
			vertex, e := simple(f, vertexID, "VERTEX_POINT", 2)
			if e != nil {
				return fail()
			}
			pid, e := ref(vertex.Args[1])
			if e != nil {
				return fail()
			}
			ends[i], e = splitTriple(f, pid, "CARTESIAN_POINT")
			if e != nil {
				return fail()
			}
		}
		axis, e := sideDiskCardinal(ends[0], ends[1])
		if e != nil || origin != ends[0] {
			return fail()
		}
		vectorID, e := ref(line.Args[2])
		if e != nil {
			return fail()
		}
		vector, e := simple(f, vectorID, "VECTOR", 3)
		if e != nil {
			return fail()
		}
		magnitude, e := cylinderSupportNumber(vector.Args[2])
		if e != nil || magnitude != 1 {
			return fail()
		}
		directionID, e := ref(vector.Args[1])
		if e != nil {
			return fail()
		}
		direction, e := splitTriple(f, directionID, "DIRECTION")
		if e != nil || direction != axis.Scale(-1) {
			return fail()
		}
		if counts[model] != 1 || counts[vectorID] != 1 || counts[directionID] != 1 {
			return fail()
		}
		// Exact endpoint coordinates differ only along this cardinal axis. Therefore
		// P(s)=start+s*(-axis), traversed with same_sense=false, is precisely the
		// canonical segment from start to end. No angular or sampled proof is used.
		de := f.Entities[directionID]
		dr := de.Records[0]
		dr.Args[1].Items = []Value{{Kind: Number, Text: real(axis.X)}, {Kind: Number, Text: real(axis.Y)}, {Kind: Number, Text: real(axis.Z)}}
		edge.Args[4] = Value{Kind: Enumeration, Text: "T"}
		normalized = append(normalized, id)
	}
	return normalized, nil
}

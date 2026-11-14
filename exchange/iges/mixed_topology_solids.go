package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/geom"
	"sort"
	"strings"
)

// MixedTopologyFamily selects one independently proved closed material family.
type MixedTopologyFamily string

const (
	MixedTopologyTetrahedron MixedTopologyFamily = "tetrahedron"
	MixedTopologyRoof        MixedTopologyFamily = "parabolic-roof"
	MixedTopologyHexahedron  MixedTopologyFamily = "hexahedron"
)

type MixedTopologyBody struct {
	Family        MixedTopologyFamily
	SourceSolidID int
	// LocalToSource maps the reindexed proof graph's IDs back to actual source IDs.
	// All nested decoded source IDs use the local proof graph explicitly.
	LocalToSource map[int]int
	Tetrahedron   *TetrahedronSolid
	Hexahedron    *HexahedronSolid
	Roof          *MappedParabolicRoofSolid
}
type MixedTopologySolids struct {
	Bodies     []MixedTopologyBody
	LengthUnit float64
}
type MixedTopologyInput struct {
	Family      MixedTopologyFamily
	Vertices    [4]geom.Vec3
	HexVertices [8]geom.Vec3
	Roof        ParabolicRoofDescriptor
}

func mixedTopologyFailure() error {
	return fmt.Errorf("iges: unsupported independently owned mixed topology solids")
}

// topologyReferenceSlots describes only the explicit bounded 186/514/510/508/
// 504/502 topology grammar. No arbitrary numeric field is treated as a pointer.
func topologyReferenceSlots(r topologyRecord) ([]int, error) {
	fail := func() ([]int, error) { return nil, mixedTopologyFailure() }
	switch r.kind {
	case 126, 128, 502:
		return nil, nil
	case 186:
		v, e := topologyInts(r, 4)
		if e != nil || v[3] != 0 {
			return fail()
		}
		return []int{1}, nil
	case 504, 508, 510, 514:
		if len(r.values) < 2 {
			return fail()
		}
		idx := 1
		if r.kind == 510 {
			idx = 2
		}
		if len(r.values) <= idx {
			return fail()
		}
		n, e := intValue(r.values[idx])
		if e != nil || n < 1 || n > 64 {
			return fail()
		}
		size := 2
		switch r.kind {
		case 504:
			size += 5 * n
		case 508:
			size += 7 * n
		case 510:
			size = 4 + n
		case 514:
			size += 2 * n
		}
		if _, e = topologyInts(r, size); e != nil {
			return fail()
		}
		var slots []int
		switch r.kind {
		case 504:
			for j := 0; j < n; j++ {
				slots = append(slots, 2+5*j, 3+5*j, 5+5*j)
			}
		case 508:
			for j := 0; j < n; j++ {
				slots = append(slots, 3+7*j, 8+7*j)
			}
		case 510:
			slots = append(slots, 1)
			for j := 0; j < n; j++ {
				slots = append(slots, 4+j)
			}
		case 514:
			for j := 0; j < n; j++ {
				slots = append(slots, 2+2*j)
			}
		}
		return slots, nil
	}
	return fail()
}
func remapTopologyRecords(records []topologyRecord, to map[int]int, unit float64) ([]byte, error) {
	a := topologyAuthor{}
	a.geometry.Entities = []Entity{{Point: &geom.Vec3{}}}
	for _, r := range records {
		slots, e := topologyReferenceSlots(r)
		if e != nil {
			return nil, e
		}
		v := make([]string, len(r.values))
		for i, p := range r.values {
			if p.stringValue {
				return nil, mixedTopologyFailure()
			}
			v[i] = p.text
		}
		for _, i := range slots {
			id, e := intValue(r.values[i])
			if e != nil || to[id] == 0 {
				return nil, mixedTopologyFailure()
			}
			v[i] = fmt.Sprint(to[id])
		}
		form := 0
		if r.kind >= 500 {
			form = 1
		}
		a.records = append(a.records, nativeTopologyRecord{kind: r.kind, form: form, label: r.label, subscript: r.subscript, data: strings.Join(v, ",") + ";"})
	}
	return a.file(unit)
}
func mixedTopologyDirectory(raw []byte) error {
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if len(line) != 80 || line[72] != 'D' {
			continue
		}
		seq, e := field(string(line[73:]))
		if e != nil {
			return e
		}
		indexes := []int{1, 2}
		if seq%2 == 1 {
			indexes = []int{3, 4}
			k, e := field(string(line[:8]))
			if e != nil {
				return e
			}
			v, e := field(string(line[64:72]))
			want := 10000
			if k == 186 {
				want = 0
			}
			if k == 504 {
				want = 10001
			}
			if e != nil || v != want {
				return mixedTopologyFailure()
			}
		}
		for _, i := range indexes {
			v, e := field(string(line[8*i : 8*i+8]))
			if e != nil || v != 0 {
				return mixedTopologyFailure()
			}
		}
	}
	return nil
}

// ReadMixedTopologySolids accepts two to eight independently rooted materials
// containing at least two of the tetrahedral, affine hexahedral and original
// parabolic spline families. Every
// source entity belongs to exactly one root; no sharing or orphan data is lost.
// This is a collection of solids, not an assembly or a Boolean union.
func ReadMixedTopologySolids(ctx context.Context, r io.Reader, l Limits) (MixedTopologySolids, error) {
	fail := func() (MixedTopologySolids, error) { return MixedTopologySolids{}, mixedTopologyFailure() }
	if ctx == nil {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return MixedTopologySolids{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 524288)
	l.MaxRecords = min(l.MaxRecords, 8192)
	l.MaxEntities = min(l.MaxEntities, 464)
	l.MaxValues = min(l.MaxValues, 32768)
	l.MaxControlPoints = min(l.MaxControlPoints, 832)
	doc, e := readTopologySource(r, l, true)
	if e != nil {
		return MixedTopologySolids{}, e
	}
	if e = mixedTopologyDirectory(doc.bytes); e != nil {
		return fail()
	}
	records := map[int]topologyRecord{}
	refs := map[int][]int{}
	var roots []int
	controls := 0
	for _, rec := range doc.entities {
		records[rec.id] = rec
		if rec.kind == 186 {
			roots = append(roots, rec.id)
		}
		slots, e := topologyReferenceSlots(rec)
		if e != nil {
			return fail()
		}
		for _, i := range slots {
			id, e := intValue(rec.values[i])
			if e != nil {
				return fail()
			}
			refs[rec.id] = append(refs[rec.id], id)
		}
		if rec.kind == 126 || rec.kind == 128 {
			en, e := decode(rec.kind, rec.values, l.MaxControlPoints-controls)
			if e != nil || en.sourceWeightsRounded {
				return fail()
			}
			if en.Spline != nil {
				controls += len(en.Spline.Curve.Poles())
			} else if en.Surface != nil {
				controls += len(en.Surface.Geometry.Data().Poles)
			} else {
				return fail()
			}
			if controls > l.MaxControlPoints {
				return fail()
			}
		}
	}
	if len(roots) < 2 || len(roots) > 8 {
		return fail()
	}
	owners := map[int]int{}
	out := MixedTopologySolids{LengthUnit: doc.unit}
	nt, nr, nh := 0, 0, 0
	for _, root := range roots {
		if e = ctx.Err(); e != nil {
			return MixedTopologySolids{}, e
		}
		todo := []int{root}
		visited := map[int]bool{}
		for len(todo) > 0 {
			id := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			if visited[id] {
				continue
			}
			visited[id] = true
			if _, ok := records[id]; !ok {
				return fail()
			}
			if prior := owners[id]; prior != 0 && prior != root {
				return fail()
			}
			owners[id] = root
			todo = append(todo, refs[id]...)
		}
		if len(visited) != 34 && len(visited) != 58 {
			return fail()
		}
		ids := make([]int, 0, len(visited))
		for id := range visited {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		localToSource := map[int]int{}
		sourceToLocal := map[int]int{}
		var selected []topologyRecord
		for i, id := range ids {
			sourceToLocal[id] = 2*i + 1
			localToSource[2*i+1] = id
			selected = append(selected, records[id])
		}
		raw, e := remapTopologyRecords(selected, sourceToLocal, doc.unit)
		if e != nil {
			return fail()
		}
		body := MixedTopologyBody{SourceSolidID: root, LocalToSource: localToSource}
		if len(visited) == 34 {
			v, e := ReadTetrahedronSolid(bytes.NewReader(raw), Limits{})
			if e != nil {
				return fail()
			}
			body.Family = MixedTopologyTetrahedron
			body.Tetrahedron = &v
			nt++
		} else if mixedTopologyBilinear(selected) {
			v, e := ReadHexahedronSolid(bytes.NewReader(raw), Limits{})
			if e != nil {
				return fail()
			}
			body.Family = MixedTopologyHexahedron
			body.Hexahedron = &v
			nh++
		} else {
			v, e := ReadMappedParabolicRoofSolid(ctx, bytes.NewReader(raw), Limits{})
			if e != nil {
				return fail()
			}
			body.Family = MixedTopologyRoof
			body.Roof = &v
			nr++
		}
		out.Bodies = append(out.Bodies, body)
	}
	if len(owners) != len(records) || mixedTopologyFamilyCount(nt, nr, nh) < 2 {
		return fail()
	}
	return out, nil
}

// WriteMixedTopologySolids stages each independently proved original material,
// remaps only typed topology references and validates the combined ownership graph.
func WriteMixedTopologySolids(ctx context.Context, w io.Writer, bodies []MixedTopologyInput, unit float64) error {
	if ctx == nil || w == nil || len(bodies) < 2 || len(bodies) > 8 {
		return mixedTopologyFailure()
	}
	var all []topologyRecord
	to := map[int]int{}
	nt, nr, nh := 0, 0, 0
	for _, b := range bodies {
		if e := ctx.Err(); e != nil {
			return e
		}
		var raw bytes.Buffer
		var e error
		switch b.Family {
		case MixedTopologyTetrahedron:
			if b.Roof != (ParabolicRoofDescriptor{}) || b.HexVertices != ([8]geom.Vec3{}) {
				return mixedTopologyFailure()
			}
			e = WriteTetrahedronSolid(&raw, b.Vertices, unit)
			nt++
		case MixedTopologyHexahedron:
			if b.Vertices != ([4]geom.Vec3{}) || b.Roof != (ParabolicRoofDescriptor{}) {
				return mixedTopologyFailure()
			}
			e = WriteHexahedronSolid(&raw, b.HexVertices, unit)
			nh++
		case MixedTopologyRoof:
			if b.Vertices != ([4]geom.Vec3{}) || b.HexVertices != ([8]geom.Vec3{}) {
				return mixedTopologyFailure()
			}
			e = WriteParabolicRoofSolid(ctx, &raw, b.Roof, unit)
			nr++
		default:
			return mixedTopologyFailure()
		}
		if e != nil {
			return e
		}
		d, e := readTopologySource(bytes.NewReader(raw.Bytes()), Limits{}, true)
		if e != nil {
			return e
		}
		offset := 2 * len(all)
		for _, rec := range d.entities {
			slots, e := topologyReferenceSlots(rec)
			if e != nil {
				return e
			}
			rec.values = append([]parameter(nil), rec.values...)
			for _, i := range slots {
				id, e := intValue(rec.values[i])
				if e != nil {
					return e
				}
				rec.values[i].text = fmt.Sprint(id + offset)
			}
			rec.id += offset
			to[rec.id] = rec.id
			all = append(all, rec)
		}
	}
	if mixedTopologyFamilyCount(nt, nr, nh) < 2 {
		return mixedTopologyFailure()
	}
	raw, e := remapTopologyRecords(all, to, unit)
	if e != nil {
		return e
	}
	if _, e = ReadMixedTopologySolids(ctx, bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}

func mixedTopologyFamilyCount(counts ...int) int {
	n := 0
	for _, v := range counts {
		if v > 0 {
			n++
		}
	}
	return n
}

// A syntactic family discriminator, never an acceptance certificate. A warped
// bilinear net still has to pass every exact affine/convex hexahedron proof.
func mixedTopologyBilinear(records []topologyRecord) bool {
	n := 0
	for _, r := range records {
		if r.kind != 128 {
			continue
		}
		n++
		if len(r.values) < 5 {
			return false
		}
		for i := 1; i <= 4; i++ {
			v, e := intValue(r.values[i])
			if e != nil || v != 1 {
				return false
			}
		}
	}
	return n == 6
}

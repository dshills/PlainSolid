package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/geom"
	"strings"
)

type reindexedSphericalGraph struct {
	Canonical, Source []byte
	Roles, Vertices   map[int]int
	Edges             map[int]int
	Uses              map[int][]int
	Placed            bool
}

func readReindexedSphericalGraph(ctx context.Context, r io.Reader, l Limits, tol float64) (reindexedSphericalGraph, error) {
	bad := func() (reindexedSphericalGraph, error) {
		return reindexedSphericalGraph{}, fmt.Errorf("iges: unsupported reindexed spherical material")
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return reindexedSphericalGraph{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 17)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true, true, true, true, true, true)
	if e != nil {
		return reindexedSphericalGraph{}, e
	}
	if mixedTopologyDirectory(doc.bytes) != nil {
		return bad()
	}
	n := len(doc.entities)
	placed := false
	switch n {
	case 16:
	case 17:
		placed = true
	default:
		return bad()
	}
	a, e := sphericalMaterialAuthor(SphericalMaterialDescriptor{Radius: 2})
	if e != nil {
		return bad()
	}
	tid := 0
	if placed {
		tid = a.add(124, 0, "124,1,0,0,0,0,1,0,0,0,0,1,0;")
	}
	want := map[int]topologyRecord{}
	root := 0
	for i, x := range a.records {
		budget := 2048
		v, er := parseParameters(x.data, ',', ';', &budget)
		if er != nil {
			return bad()
		}
		id := 2*i + 1
		tr := 0
		if placed && id == 7 {
			tr = tid
		}
		if id == 23 {
			tr = 25
		}
		want[id] = topologyRecord{id: id, kind: x.kind, values: v, transform: tr}
		if x.kind == 186 {
			root = id
		}
	}
	actual := map[int]topologyRecord{}
	sourceRoot := 0
	for _, x := range doc.entities {
		actual[x.id] = x
		if x.kind == 186 {
			if sourceRoot != 0 {
				return bad()
			}
			sourceRoot = x.id
		}
	}
	if sourceRoot == 0 {
		return bad()
	}
	// Identify pole roles from exact world coordinates after the source support
	// placement. Retain vertex ordinals; no role is inferred from entity numbers.
	var support, vertices, edges, loop topologyRecord
	for _, rec := range actual {
		switch rec.kind {
		case 196:
			if support.id != 0 {
				return bad()
			}
			support = rec
		case 502:
			if vertices.id != 0 {
				return bad()
			}
			vertices = rec
		case 504:
			if edges.id != 0 {
				return bad()
			}
			edges = rec
		case 508:
			if loop.id != 0 {
				return bad()
			}
			loop = rec
		}
	}
	if len(support.values) != 5 || len(vertices.values) != 8 || len(edges.values) != 7 || len(loop.values) != 30 {
		return bad()
	}
	radius, e := realValue(support.values[2])
	if e != nil || radius <= 0 {
		return bad()
	}
	pose := geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}
	if placed {
		tr, ok := actual[support.transform]
		if !ok || tr.kind != 124 || compositeSourceNumbers(124, tr.values) != nil {
			return bad()
		}
		pose, e = decodeMatrix(tr.values)
		if e != nil || !roofRigid(pose) {
			return bad()
		}
	} else if support.transform != 0 {
		return bad()
	}
	if compositeSourceNumbers(110, vertices.values) != nil {
		return bad()
	}
	vertexRoles := map[int]int{}
	for old := 1; old <= 2; old++ {
		var p [3]float64
		for k := range p {
			p[k], e = realValue(vertices.values[2+3*(old-1)+k])
			if e != nil {
				return bad()
			}
		}
		q, e := roofMapPoint(pose, geom.Vec3{X: p[0], Y: p[1], Z: p[2]}, true)
		if e != nil || q.X != 0 || q.Y != 0 {
			return bad()
		}
		role := 0
		if q.Z == radius {
			role = 1
		}
		if q.Z == -radius {
			role = 2
		}
		if role == 0 || vertexRoles[role] != 0 {
			return bad()
		}
		vertexRoles[role] = old
	}
	inverse := map[int]int{vertexRoles[1]: 1, vertexRoles[2]: 2}
	vv := append([]parameter(nil), vertices.values...)
	for role := 1; role <= 2; role++ {
		old := vertexRoles[role]
		copy(vv[2+3*(role-1):2+3*role], vertices.values[2+3*(old-1):2+3*old])
	}
	vertices.values = vv
	actual[vertices.id] = vertices
	ev := append([]parameter(nil), edges.values...)
	for _, j := range []int{4, 6} {
		old, err := intValue(ev[j])
		if err != nil || inverse[old] == 0 {
			return bad()
		}
		ev[j].text = fmt.Sprint(inverse[old])
	}
	edges.values = ev
	actual[edges.id] = edges
	lv := append([]parameter(nil), loop.values...)
	start := -1
	for i := 0; i < 4; i++ {
		j := 2 + 7*i
		kind, err := intValue(lv[j])
		if err != nil {
			return bad()
		}
		owner, err := intValue(lv[j+1])
		if err != nil {
			return bad()
		}
		ordinal, err := intValue(lv[j+2])
		if err != nil {
			return bad()
		}
		if kind == 1 {
			if owner != vertices.id || inverse[ordinal] == 0 {
				return bad()
			}
			lv[j+2].text = fmt.Sprint(inverse[ordinal])
			if inverse[ordinal] == 1 {
				if start != -1 {
					return bad()
				}
				start = i
			}
		} else if kind != 0 || owner != edges.id || ordinal != 1 {
			return bad()
		}
	}
	if start < 0 {
		return bad()
	}
	out := append([]parameter(nil), lv...)
	ord := make([]int, 4)
	for i := 0; i < 4; i++ {
		old := (start + i) % 4
		copy(out[2+7*i:2+7*(i+1)], lv[2+7*old:2+7*(old+1)])
		ord[i] = old + 1
	}
	loop.values = out
	actual[loop.id] = loop
	roles := map[int]int{1: 1}
	useRoles := map[int][]int{loop.id: ord}
	forward, reverse := map[int]int{}, map[int]int{}
	queue := [][2]int{{root, sourceRoot}}
	steps := 0
	for len(queue) > 0 {
		if e := ctx.Err(); e != nil {
			return reindexedSphericalGraph{}, e
		}
		steps++
		if steps > 512 {
			return bad()
		}
		pair := queue[0]
		queue = queue[1:]
		c, s := pair[0], pair[1]
		if old, ok := forward[c]; ok {
			if old != s {
				return bad()
			}
			continue
		}
		if _, ok := reverse[s]; ok {
			return bad()
		}
		w, ok := want[c]
		x, present := actual[s]
		if !ok || !present || w.kind != x.kind || len(w.values) != len(x.values) {
			return bad()
		}
		forward[c], reverse[s] = s, c
		wr, er := sphericalReferenceSlots(w)
		xr, ex := sphericalReferenceSlots(x)
		if er != nil || ex != nil || len(wr) != len(xr) {
			return bad()
		}
		for i, index := range wr {
			if index != xr[i] {
				return bad()
			}
			cw, er := intValue(w.values[index])
			sx, ex := intValue(x.values[index])
			if er != nil || ex != nil {
				return bad()
			}
			queue = append(queue, [2]int{cw, sx})
		}
		if w.transform == 0 {
			if x.transform != 0 {
				return bad()
			}
		} else {
			if x.transform == 0 {
				return bad()
			}
			queue = append(queue, [2]int{w.transform, x.transform})
		}
	}
	if len(forward) != n || len(reverse) != n {
		return bad()
	}
	for i := range a.records {
		id := 2*i + 1
		x := actual[forward[id]]
		slots, er := sphericalReferenceSlots(x)
		if er != nil {
			return bad()
		}
		v := append([]parameter(nil), x.values...)
		for _, j := range slots {
			target, er := intValue(v[j])
			if er != nil {
				return bad()
			}
			canonical, ok := reverse[target]
			if !ok {
				return bad()
			}
			v[j].text = fmt.Sprint(canonical)
		}
		str := []string{}
		for _, p := range v {
			if p.stringValue {
				return bad()
			}
			str = append(str, p.text)
		}
		a.records[i].data = strings.Join(str, ",") + ";"
		a.records[i].label = x.label
		a.records[i].subscript = x.subscript
	}
	raw, e := a.file(doc.unit)
	if e != nil {
		return bad()
	}
	{
		for _, line := range bytes.Split(raw, []byte("\n")) {
			if len(line) != 80 || line[72] != 'D' {
				continue
			}
			id, _ := field(string(line[73:]))
			if id%2 != 1 {
				continue
			}
			copy(line[48:56], fmt.Sprintf("%8d", want[id].transform))
		}
	}
	return reindexedSphericalGraph{raw, append([]byte(nil), doc.bytes...), forward, vertexRoles, roles, useRoles, placed}, nil
}

func sphericalReferenceSlots(r topologyRecord) ([]int, error) {
	if r.kind == 196 {
		if len(r.values) != 5 {
			return nil, fmt.Errorf("iges: invalid sphere support")
		}
		return []int{1, 3, 4}, nil
	}
	return topologyReferenceSlots(r)
}

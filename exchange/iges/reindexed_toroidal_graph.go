package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
)

type reindexedToroidalGraph struct {
	Canonical, Source []byte
	Roles, Vertices   map[int]int
	Edges             map[int]int
	Uses              map[int][]int
	Placed            bool
}

func readReindexedToroidalGraph(ctx context.Context, r io.Reader, l Limits, tol float64) (reindexedToroidalGraph, error) {
	bad := func() (reindexedToroidalGraph, error) {
		return reindexedToroidalGraph{}, fmt.Errorf("iges: unsupported reindexed toroidal material")
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return reindexedToroidalGraph{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 14)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true, true, true, true, true)
	if e != nil {
		return reindexedToroidalGraph{}, e
	}
	if mixedTopologyDirectory(doc.bytes) != nil {
		return bad()
	}
	n := len(doc.entities)
	placed := false
	switch n {
	case 13:
	case 14:
		placed = true
	default:
		return bad()
	}
	a, e := toroidalMaterialAuthor(ToroidalMaterialDescriptor{MajorRadius: 4, MinorRadius: 1})
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
		if placed && (id == 7 || id == 11) {
			tr = tid
		}
		if id == 13 {
			tr = 15
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
	// The two circles have distinct support-frame ancestry. Identify major
	// seam by its shared transform with198, never by its numeric entity ID.
	var support, edges, loop topologyRecord
	for _, rec := range actual {
		switch rec.kind {
		case 198:
			if support.id != 0 {
				return bad()
			}
			support = rec
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
	if support.id == 0 || len(edges.values) != 12 || len(loop.values) != 22 {
		return bad()
	}
	roles := map[int]int{}
	for old := 1; old <= 2; old++ {
		id, err := intValue(edges.values[2+5*(old-1)])
		if err != nil {
			return bad()
		}
		c, ok := actual[id]
		if !ok || c.kind != 100 {
			return bad()
		}
		role := 2
		if c.transform == support.transform {
			role = 1
		}
		if roles[role] != 0 {
			return bad()
		}
		roles[role] = old
	}
	if len(roles) != 2 {
		return bad()
	}
	v := append([]parameter(nil), edges.values...)
	for role := 1; role <= 2; role++ {
		copy(v[2+5*(role-1):2+5*role], edges.values[2+5*(roles[role]-1):2+5*roles[role]])
	}
	edges.values = v
	actual[edges.id] = edges
	inverse := map[int]int{roles[1]: 1, roles[2]: 2}
	lv := append([]parameter(nil), loop.values...)
	start := -1
	for i := 0; i < 4; i++ {
		j := 2 + 5*i
		list, err := intValue(lv[j+1])
		if err != nil || list != edges.id {
			return bad()
		}
		old, err := intValue(lv[j+2])
		if err != nil || inverse[old] == 0 {
			return bad()
		}
		lv[j+2].text = fmt.Sprint(inverse[old])
		sense, err := intValue(lv[j+3])
		if err != nil {
			return bad()
		}
		if inverse[old] == 1 && sense == 1 {
			if start != -1 {
				return bad()
			}
			start = i
		}
	}
	if start < 0 {
		return bad()
	}
	out := append([]parameter(nil), lv...)
	ord := make([]int, 4)
	for i := 0; i < 4; i++ {
		old := (start + i) % 4
		copy(out[2+5*i:2+5*(i+1)], lv[2+5*old:2+5*(old+1)])
		ord[i] = old + 1
	}
	loop.values = out
	actual[loop.id] = loop
	vertexRoles := map[int]int{1: 1}
	useRoles := map[int][]int{loop.id: ord}
	forward, reverse := map[int]int{}, map[int]int{}
	queue := [][2]int{{root, sourceRoot}}
	steps := 0
	for len(queue) > 0 {
		if e := ctx.Err(); e != nil {
			return reindexedToroidalGraph{}, e
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
		wr, er := toroidalReferenceSlots(w)
		xr, ex := toroidalReferenceSlots(x)
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
		slots, er := toroidalReferenceSlots(x)
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
	return reindexedToroidalGraph{raw, append([]byte(nil), doc.bytes...), forward, vertexRoles, roles, useRoles, placed}, nil
}

func toroidalReferenceSlots(r topologyRecord) ([]int, error) {
	if r.kind == 198 {
		if len(r.values) != 8 {
			return nil, fmt.Errorf("iges: invalid torus support")
		}
		return []int{1, 2, 5}, nil
	}
	return topologyReferenceSlots(r)
}

package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
)

type reindexedConical45Graph struct {
	Canonical, Source []byte
	Roles, Vertices   map[int]int
	Uses              map[int][]int
	Placed            bool
}

func readReindexedConical45Graph(ctx context.Context, r io.Reader, l Limits, tol float64) (reindexedConical45Graph, error) {
	bad := func() (reindexedConical45Graph, error) {
		return reindexedConical45Graph{}, fmt.Errorf("iges: unsupported reindexed conical material")
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return reindexedConical45Graph{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 21)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true, true, true, true)
	if e != nil {
		return reindexedConical45Graph{}, e
	}
	if mixedTopologyDirectory(doc.bytes) != nil {
		return bad()
	}
	n := len(doc.entities)
	placed := false
	switch n {
	case 20:
	case 21:
		placed = true
	default:
		return bad()
	}
	a, e := conical45FrustumAuthor(Conical45FrustumDescriptor{Radius: 1, Height: 2})
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
		if placed && conical45PlacedLeaf(x.kind) {
			tr = tid
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
	// Classify shell face roles from support type and material orientation,
	// never from record IDs. Geometry and incidence are independently proved
	// by the unchanged typed reader after this structural normalization.
	for id, rec := range actual {
		if rec.kind == 514 {
			normalized, err := conicShellFaceOrder(rec, actual, [][2]int{{194, 1}, {190, 1}, {190, 0}})
			if err != nil {
				return bad()
			}
			actual[id] = normalized
		}
	}
	vertexRoles, err := conical45VertexRoles(actual, placed)
	if err != nil {
		return bad()
	}
	if err := conicEdgeListOrder(actual, false, 100); err != nil {
		return bad()
	}
	useRoles, err := ellipseLoopStarts(actual, false)
	if err != nil {
		return bad()
	}
	forward, reverse := map[int]int{}, map[int]int{}
	queue := [][2]int{{root, sourceRoot}}
	steps := 0
	for len(queue) > 0 {
		if e := ctx.Err(); e != nil {
			return reindexedConical45Graph{}, e
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
		wr, er := conical45ReferenceSlots(w)
		xr, ex := conical45ReferenceSlots(x)
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
		slots, er := conical45ReferenceSlots(x)
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
	if placed {
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
	return reindexedConical45Graph{raw, append([]byte(nil), doc.bytes...), forward, vertexRoles, useRoles, placed}, nil
}

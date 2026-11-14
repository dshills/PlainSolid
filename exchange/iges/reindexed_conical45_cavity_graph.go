package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/geom"
	"strings"
)

type reindexedConical45CavityGraph struct {
	Canonical, Source      []byte
	Roles                  map[int]int
	Vertices, Edges, Faces map[int]map[int]int
	Uses                   map[int][]int
	Placed                 bool
}

func readReindexedConical45CavityGraph(ctx context.Context, r io.Reader, l Limits, tol float64) (reindexedConical45CavityGraph, error) {
	bad := func() (reindexedConical45CavityGraph, error) {
		return reindexedConical45CavityGraph{}, fmt.Errorf("iges: unsupported reindexed conical material")
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return reindexedConical45CavityGraph{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 40)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true, true, true, true)
	if e != nil {
		return reindexedConical45CavityGraph{}, e
	}
	if mixedTopologyDirectory(doc.bytes) != nil {
		return bad()
	}
	n := len(doc.entities)
	placed := false
	switch n {
	case 39:
	case 40:
		placed = true
	default:
		return bad()
	}
	a, e := conical45CavityMaterialAuthor(Conical45CavityMaterialDescriptor{Outer: Conical45FrustumDescriptor{Radius: 3, Height: 4}, Inner: Conical45FrustumDescriptor{Origin: geom.Vec3{Z: 1}, Radius: 1, Height: 2}})
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
	vertexRoles := map[int]map[int]int{}
	edgeRoles := map[int]map[int]int{}
	faceRoles := map[int]map[int]int{}
	useRoles := map[int][]int{}
	owned := map[int]bool{}
	count := 0
	for id, shell := range actual {
		if shell.kind != 514 {
			continue
		}
		count++
		part := map[int]topologyRecord{}
		todo := []int{id}
		steps := 0
		for len(todo) > 0 {
			steps++
			if steps > 256 {
				return bad()
			}
			node := todo[0]
			todo = todo[1:]
			if _, ok := part[node]; ok {
				continue
			}
			rec, ok := actual[node]
			if !ok || rec.kind == 186 {
				return bad()
			}
			part[node] = rec
			slots, e := conical45ReferenceSlots(rec)
			if e != nil {
				return bad()
			}
			for _, j := range slots {
				target, e := intValue(rec.values[j])
				if e != nil {
					return bad()
				}
				todo = append(todo, target)
			}
			if rec.transform != 0 {
				todo = append(todo, rec.transform)
			}
		}
		for key, rec := range part {
			if owned[key] && rec.kind != 124 {
				return bad()
			}
			owned[key] = true
		}
		normalized, e := conicShellFaceOrder(shell, part, [][2]int{{194, 1}, {190, 1}, {190, 0}})
		if e != nil {
			return bad()
		}
		fr := map[int]int{}
		for i := 0; i < 3; i++ {
			f, e := intValue(normalized.values[2+2*i])
			if e != nil {
				return bad()
			}
			for j := 0; j < 3; j++ {
				old, e := intValue(shell.values[2+2*j])
				if e != nil {
					return bad()
				}
				if old == f {
					fr[i+1] = j + 1
				}
			}
		}
		faceRoles[id] = fr
		part[id] = normalized
		vid, eid := 0, 0
		oldCurves := map[int]int{}
		for key, rec := range part {
			if rec.kind == 502 {
				if vid != 0 {
					return bad()
				}
				vid = key
			}
			if rec.kind == 504 {
				if eid != 0 || len(rec.values) != 17 {
					return bad()
				}
				eid = key
				for i := 0; i < 3; i++ {
					curve, e := intValue(rec.values[2+5*i])
					if e != nil || oldCurves[curve] != 0 {
						return bad()
					}
					oldCurves[curve] = i + 1
				}
			}
		}
		vr, e := conical45VertexRoles(part, placed)
		if e != nil {
			return bad()
		}
		vertexRoles[vid] = vr
		if e = conicEdgeListOrder(part, false, 100); e != nil {
			return bad()
		}
		er := map[int]int{}
		for i := 0; i < 3; i++ {
			curve, e := intValue(part[eid].values[2+5*i])
			if e != nil || oldCurves[curve] == 0 {
				return bad()
			}
			er[i+1] = oldCurves[curve]
		}
		edgeRoles[eid] = er
		uses, e := ellipseLoopStarts(part, false)
		if e != nil {
			return bad()
		}
		for key, value := range uses {
			useRoles[key] = value
		}
		for key, value := range part {
			actual[key] = value
		}
	}
	if count != 2 {
		return bad()
	}
	forward, reverse := map[int]int{}, map[int]int{}
	queue := [][2]int{{root, sourceRoot}}
	steps := 0
	for len(queue) > 0 {
		if e := ctx.Err(); e != nil {
			return reindexedConical45CavityGraph{}, e
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
		wr, er := conical45CavityReferenceSlots(w)
		xr, ex := conical45CavityReferenceSlots(x)
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
		slots, er := conical45CavityReferenceSlots(x)
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
	return reindexedConical45CavityGraph{raw, append([]byte(nil), doc.bytes...), forward, vertexRoles, edgeRoles, faceRoles, useRoles, placed}, nil
}

func conical45CavityReferenceSlots(r topologyRecord) ([]int, error) {
	if r.kind == 186 {
		v, e := topologyInts(r, 6)
		if e != nil || v[2] != 1 || v[3] != 1 || v[5] != 0 {
			return nil, geom.ErrInvalid
		}
		return []int{1, 4}, nil
	}
	return conical45ReferenceSlots(r)
}

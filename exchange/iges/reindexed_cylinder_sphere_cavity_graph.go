package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/geom"
	"strings"
)

type reindexedCylinderSphereCavityGraph struct {
	Canonical, Source      []byte
	Roles                  map[int]int
	Vertices, Edges, Faces map[int]map[int]int
	Uses                   map[int][]int
	Placed                 bool
}

func readReindexedCylinderSphereCavityGraph(ctx context.Context, r io.Reader, l Limits, tol float64) (reindexedCylinderSphereCavityGraph, error) {
	bad := func() (reindexedCylinderSphereCavityGraph, error) {
		return reindexedCylinderSphereCavityGraph{}, fmt.Errorf("iges: unsupported reindexed conical material")
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return reindexedCylinderSphereCavityGraph{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 40)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true, true, true, true, true, true)
	if e != nil {
		return reindexedCylinderSphereCavityGraph{}, e
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
	a, e := cylinderSphereCavityMaterialAuthor(CylinderSphereCavityMaterialDescriptor{Stock: CircularCylinderDescriptor{Origin: geom.Vec3{Z: -3}, Radius: 3, Height: 7}, SphereRadius: 1})
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
		if placed && (id == 9 || id == 11 || id == 13 || id == 17 || id == 19 || id == 21 || id == 53) {
			tr = tid
		}
		if id == 69 {
			tr = 71
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
			slots, e := cylinderSphereCavityReferenceSlots(rec)
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
		sphere := false
		for _, rec := range part {
			if rec.kind == 196 {
				sphere = true
			}
		}
		if sphere {
			vr, uses, e := mixedCavitySphereIncidence(part, placed)
			if e != nil {
				return bad()
			}
			for key, rec := range part {
				switch rec.kind {
				case 502:
					vertexRoles[key] = vr
				case 504:
					edgeRoles[key] = map[int]int{1: 1}
				}
			}
			faceRoles[id] = map[int]int{1: 1}
			for key, value := range uses {
				useRoles[key] = value
			}
			for key, value := range part {
				actual[key] = value
			}
			continue
		}
		// Preserve authoritative degree pcurves while applying model-only incidence
		// normalization to the first five fields of each original cylinder use.
		originalLoops := map[int]topologyRecord{}
		for key, rec := range part {
			if rec.kind != 508 {
				continue
			}
			if len(rec.values) < 2 {
				return bad()
			}
			n, e := intValue(rec.values[1])
			if e != nil || n != 1 && n != 4 {
				return bad()
			}
			width := 5
			if n == 4 {
				width = 7
			}
			if len(rec.values) != 2+width*n {
				return bad()
			}
			originalLoops[key] = rec
			v := append([]parameter(nil), rec.values[:2]...)
			for j := 0; j < n; j++ {
				pos := 2 + width*j
				pc, e := intValue(rec.values[pos+4])
				if e != nil || pc != (width-5)/2 {
					return bad()
				}
				use := append([]parameter(nil), rec.values[pos:pos+5]...)
				use[4].text = "0"
				v = append(v, use...)
			}
			rec.values = v
			part[key] = rec
		}
		normalized, e := conicShellFaceOrder(shell, part, [][2]int{{192, 1}, {190, 1}, {190, 0}})
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
		for key, original := range originalLoops {
			n, _ := intValue(original.values[1])
			if n == 1 {
				continue
			}
			normalized := part[key]
			v := append([]parameter(nil), normalized.values[:2]...)
			for j, old := range uses[key] {
				use := append([]parameter(nil), normalized.values[2+5*j:2+5*(j+1)]...)
				use[4] = original.values[2+7*(old-1)+4]
				use = append(use, original.values[2+7*(old-1)+5:2+7*old]...)
				v = append(v, use...)
			}
			normalized.values = v
			part[key] = normalized
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
			return reindexedCylinderSphereCavityGraph{}, e
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
		wr, er := cylinderSphereCavityReferenceSlots(w)
		xr, ex := cylinderSphereCavityReferenceSlots(x)
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
		slots, er := cylinderSphereCavityReferenceSlots(x)
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
	return reindexedCylinderSphereCavityGraph{raw, append([]byte(nil), doc.bytes...), forward, vertexRoles, edgeRoles, faceRoles, useRoles, placed}, nil
}

func cylinderSphereCavityReferenceSlots(r topologyRecord) ([]int, error) {
	if r.kind == 186 {
		v, e := topologyInts(r, 6)
		if e != nil || v[2] != 1 || v[3] != 1 || v[5] != 0 {
			return nil, geom.ErrInvalid
		}
		return []int{1, 4}, nil
	}
	return sphericalReferenceSlots(r)
}

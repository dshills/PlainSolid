package iges

import (
	"fmt"
	"plainsolid/geom"
)

// Match source list ordinals from independently recovered analytic support dimensions.
// The final canonical graph reproof authenticates every scalar and ownership link.
func conicalThroughIncidence(actual map[int]topologyRecord, placed bool) (map[int]map[int]int, map[int]map[int]int, map[int]map[int]int, map[int][]int, error) {
	bad := func() (map[int]map[int]int, map[int]map[int]int, map[int]map[int]int, map[int][]int, error) {
		return nil, nil, nil, nil, geom.ErrInvalid
	}
	unique := map[int]topologyRecord{}
	for _, r := range actual {
		switch r.kind {
		case 194, 192, 502, 504, 514, 124:
			if unique[r.kind].id != 0 {
				return bad()
			}
			unique[r.kind] = r
		}
	}
	cone, cylinder, vertices, edges, shell := unique[194], unique[192], unique[502], unique[504], unique[514]
	if len(cone.values) != 8 || len(cylinder.values) != 5 || len(vertices.values) != 14 || len(edges.values) != 32 || len(shell.values) != 10 {
		return bad()
	}
	read := func(r topologyRecord, i int) (float64, error) { return realValue(r.values[i]) }
	oid, e := intValue(cone.values[1])
	if e != nil {
		return bad()
	}
	o := actual[oid]
	if o.kind != 116 || len(o.values) != 5 {
		return bad()
	}
	x, e := read(o, 1)
	if e != nil {
		return bad()
	}
	y, e := read(o, 2)
	if e != nil {
		return bad()
	}
	z, e := read(o, 3)
	if e != nil {
		return bad()
	}
	lower, e := read(cone, 3)
	if e != nil {
		return bad()
	}
	inner, e := read(cylinder, 3)
	if e != nil {
		return bad()
	}
	upperZ := z
	planes := 0
	for _, r := range actual {
		if r.kind != 190 {
			continue
		}
		planes++
		if len(r.values) != 4 {
			return bad()
		}
		id, e := intValue(r.values[1])
		if e != nil {
			return bad()
		}
		p := actual[id]
		if p.kind != 116 || len(p.values) != 5 {
			return bad()
		}
		h, e := read(p, 3)
		if e != nil {
			return bad()
		}
		if h > upperZ {
			upperZ = h
		}
	}
	if planes != 2 {
		return bad()
	}
	height, e := cylinderExactSum(upperZ, -z)
	if e != nil {
		return bad()
	}
	author, e := conical45CylinderThroughAuthor(Conical45CylinderThroughDescriptor{Origin: geom.Vec3{X: x, Y: y, Z: z}, Radius: lower, Height: height, BoreRadius: inner})
	if e != nil {
		return bad()
	}
	budget := 2048
	want, e := parseParameters(author.records[8].data, ',', ';', &budget)
	if e != nil {
		return bad()
	}
	var pose geom.Affine
	if placed {
		tr := unique[124]
		if len(tr.values) != 13 || tr.transform != 0 || compositeSourceNumbers(124, tr.values) != nil {
			return bad()
		}
		pose, e = decodeMatrix(tr.values)
		if e != nil || !roofRigid(pose) {
			return bad()
		}
	} else if unique[124].id != 0 {
		return bad()
	}
	if compositeSourceNumbers(110, vertices.values) != nil {
		return bad()
	}
	vr, reverse := map[int]int{}, map[int]int{}
	normalized := append([]parameter(nil), vertices.values...)
	for old := 1; old <= 4; old++ {
		v := [3]float64{}
		for k := 0; k < 3; k++ {
			v[k], e = read(vertices, 2+3*(old-1)+k)
			if e != nil {
				return bad()
			}
		}
		p := geom.Vec3{X: v[0], Y: v[1], Z: v[2]}
		if placed {
			p, e = roofMapPoint(pose, p, true)
			if e != nil {
				return bad()
			}
		}
		for role := 1; role <= 4; role++ {
			q := [3]float64{}
			for k := 0; k < 3; k++ {
				q[k], e = realValue(want[2+3*(role-1)+k])
				if e != nil {
					return bad()
				}
			}
			if p != (geom.Vec3{X: q[0], Y: q[1], Z: q[2]}) {
				continue
			}
			if vr[role] != 0 || reverse[old] != 0 {
				return bad()
			}
			vr[role] = old
			reverse[old] = role
			copy(normalized[2+3*(role-1):2+3*role], vertices.values[2+3*(old-1):2+3*old])
		}
	}
	if len(vr) != 4 {
		return bad()
	}
	vertices.values = normalized
	actual[vertices.id] = vertices
	er, edgeReverse := map[int]int{}, map[int]int{}
	ev := append([]parameter(nil), edges.values...)
	pairs := [][2]int{{1, 1}, {2, 1}, {2, 2}, {3, 3}, {4, 3}, {4, 4}}
	for old := 1; old <= 6; old++ {
		pos := 2 + 5*(old-1)
		a, e := intValue(edges.values[pos+1])
		if e != nil || a != vertices.id {
			return bad()
		}
		b, e := intValue(edges.values[pos+3])
		if e != nil || b != vertices.id {
			return bad()
		}
		s, e := intValue(edges.values[pos+2])
		if e != nil {
			return bad()
		}
		t, e := intValue(edges.values[pos+4])
		if e != nil {
			return bad()
		}
		curve, e := intValue(edges.values[pos])
		if e != nil {
			return bad()
		}
		for i, p := range pairs {
			role := i + 1
			if reverse[s] != p[0] || reverse[t] != p[1] {
				continue
			}
			kind := 100
			if role == 2 || role == 5 {
				kind = 110
			}
			if actual[curve].kind != kind || er[role] != 0 {
				return bad()
			}
			er[role] = old
			edgeReverse[old] = role
			copy(ev[2+5*i:2+5*(i+1)], edges.values[pos:pos+5])
			ev[2+5*i+2].text = fmt.Sprint(p[0])
			ev[2+5*i+4].text = fmt.Sprint(p[1])
		}
	}
	if len(er) != 6 {
		return bad()
	}
	edges.values = ev
	actual[edges.id] = edges
	uses := map[int][]int{}
	for id, r := range actual {
		if r.kind != 508 {
			continue
		}
		if len(r.values) < 2 {
			return bad()
		}
		n, e := intValue(r.values[1])
		if e != nil || (n != 1 && n != 4) || len(r.values) != 2+5*n {
			return bad()
		}
		v := append([]parameter(nil), r.values...)
		for j := 0; j < n; j++ {
			pos := 2 + 5*j
			owner, e := intValue(v[pos+1])
			if e != nil || owner != edges.id {
				return bad()
			}
			old, e := intValue(v[pos+2])
			if e != nil || edgeReverse[old] == 0 {
				return bad()
			}
			v[pos+2].text = fmt.Sprint(edgeReverse[old])
		}
		if n == 1 {
			uses[id] = []int{1}
			r.values = v
			actual[id] = r
			continue
		}
		found := false
		for offset := 0; offset <= 3; offset += 3 {
			for start := 0; start < 4; start++ {
				ok := true
				for j, edge := range []int{1, 2, 3, 2} {
					pos := 2 + 5*((start+j)%4)
					got, e := intValue(v[pos+2])
					sense, se := intValue(v[pos+3])
					wantSense := 0
					if j >= 2 {
						wantSense = 1
					}
					if e != nil || se != nil || got != edge+offset || sense != wantSense {
						ok = false
						break
					}
				}
				if !ok {
					continue
				}
				if found {
					return bad()
				}
				found = true
				out := append([]parameter(nil), v[:2]...)
				ord := []int{}
				for j := 0; j < 4; j++ {
					old := (start + j) % 4
					out = append(out, v[2+5*old:2+5*(old+1)]...)
					ord = append(ord, old+1)
				}
				r.values = out
				uses[id] = ord
			}
		}
		if !found {
			return bad()
		}
		actual[id] = r
	}
	ordered, e := conicShellFaceOrder(shell, actual, [][2]int{{194, 1}, {192, 0}, {190, 1}, {190, 0}})
	if e != nil {
		return bad()
	}
	fr := map[int]int{}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			if ordered.values[2+2*i].text == shell.values[2+2*j].text {
				fr[i+1] = j + 1
			}
		}
	}
	if len(fr) != 4 {
		return bad()
	}
	actual[shell.id] = ordered
	return map[int]map[int]int{vertices.id: vr}, map[int]map[int]int{edges.id: er}, map[int]map[int]int{shell.id: fr}, uses, nil
}

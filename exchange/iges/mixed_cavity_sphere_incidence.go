package iges

import (
	"fmt"
	"plainsolid/geom"
)

func mixedCavitySphereIncidence(actual map[int]topologyRecord, placed bool) (map[int]int, map[int][]int, error) {
	// Identify pole roles from exact world coordinates after the source support
	// placement. Retain vertex ordinals; no role is inferred from entity numbers.
	var support, vertices, edges, loop topologyRecord
	for _, rec := range actual {
		switch rec.kind {
		case 196:
			if support.id != 0 {
				return nil, nil, geom.ErrInvalid
			}
			support = rec
		case 502:
			if vertices.id != 0 {
				return nil, nil, geom.ErrInvalid
			}
			vertices = rec
		case 504:
			if edges.id != 0 {
				return nil, nil, geom.ErrInvalid
			}
			edges = rec
		case 508:
			if loop.id != 0 {
				return nil, nil, geom.ErrInvalid
			}
			loop = rec
		}
	}
	if len(support.values) != 5 || len(vertices.values) != 8 || len(edges.values) != 7 || len(loop.values) != 30 {
		return nil, nil, geom.ErrInvalid
	}
	radius, e := realValue(support.values[2])
	if e != nil || radius <= 0 {
		return nil, nil, geom.ErrInvalid
	}
	pose := geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}
	if placed {
		tr, ok := actual[support.transform]
		if !ok || tr.kind != 124 || compositeSourceNumbers(124, tr.values) != nil {
			return nil, nil, geom.ErrInvalid
		}
		pose, e = decodeMatrix(tr.values)
		if e != nil || !roofRigid(pose) {
			return nil, nil, geom.ErrInvalid
		}
	} else if support.transform != 0 {
		return nil, nil, geom.ErrInvalid
	}
	if compositeSourceNumbers(110, vertices.values) != nil {
		return nil, nil, geom.ErrInvalid
	}
	vertexRoles := map[int]int{}
	for old := 1; old <= 2; old++ {
		var p [3]float64
		for k := range p {
			p[k], e = realValue(vertices.values[2+3*(old-1)+k])
			if e != nil {
				return nil, nil, geom.ErrInvalid
			}
		}
		q, e := roofMapPoint(pose, geom.Vec3{X: p[0], Y: p[1], Z: p[2]}, true)
		if e != nil || q.X != 0 || q.Y != 0 {
			return nil, nil, geom.ErrInvalid
		}
		role := 0
		if q.Z == radius {
			role = 1
		}
		if q.Z == -radius {
			role = 2
		}
		if role == 0 || vertexRoles[role] != 0 {
			return nil, nil, geom.ErrInvalid
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
			return nil, nil, geom.ErrInvalid
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
			return nil, nil, geom.ErrInvalid
		}
		owner, err := intValue(lv[j+1])
		if err != nil {
			return nil, nil, geom.ErrInvalid
		}
		ordinal, err := intValue(lv[j+2])
		if err != nil {
			return nil, nil, geom.ErrInvalid
		}
		if kind == 1 {
			if owner != vertices.id || inverse[ordinal] == 0 {
				return nil, nil, geom.ErrInvalid
			}
			lv[j+2].text = fmt.Sprint(inverse[ordinal])
			if inverse[ordinal] == 1 {
				if start != -1 {
					return nil, nil, geom.ErrInvalid
				}
				start = i
			}
		} else if kind != 0 || owner != edges.id || ordinal != 1 {
			return nil, nil, geom.ErrInvalid
		}
	}
	if start < 0 {
		return nil, nil, geom.ErrInvalid
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
	useRoles := map[int][]int{loop.id: ord}
	return vertexRoles, useRoles, nil
}

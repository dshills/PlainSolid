package iges

import (
	"fmt"
	"plainsolid/geom"
)

// Vertex ordinals remain strict. Their actual coordinates and every model
// curve are checked by canonical reproof after this incidence normalization.
func ellipseEdgeListOrder(records map[int]topologyRecord, annular bool) error {
	count := 3
	if annular {
		count = 6
	}
	listID := 0
	oldToNew := map[int]int{}
	for id, r := range records {
		if r.kind != 504 {
			continue
		}
		if listID != 0 {
			return geom.ErrInvalid
		}
		listID = id
		if len(r.values) < 2 {
			return geom.ErrInvalid
		}
		n, e := intValue(r.values[1])
		if e != nil || n != count || len(r.values) != 2+5*count {
			return geom.ErrInvalid
		}
		out := append([]parameter(nil), r.values...)
		seen := map[int]bool{}
		vertexList := 0
		for i := 0; i < count; i++ {
			pos := 2 + 5*i
			curve, e := intValue(r.values[pos])
			if e != nil {
				return geom.ErrInvalid
			}
			c, ok := records[curve]
			if !ok {
				return geom.ErrInvalid
			}
			vl, e := intValue(r.values[pos+1])
			if e != nil {
				return geom.ErrInvalid
			}
			endList, e := intValue(r.values[pos+3])
			if e != nil || vl != endList {
				return geom.ErrInvalid
			}
			vertices, ok := records[vl]
			if !ok || vertices.kind != 502 {
				return geom.ErrInvalid
			}
			if vertexList != 0 && vertexList != vl {
				return geom.ErrInvalid
			}
			vertexList = vl
			start, e := intValue(r.values[pos+2])
			if e != nil {
				return geom.ErrInvalid
			}
			end, e := intValue(r.values[pos+4])
			if e != nil {
				return geom.ErrInvalid
			}
			role := 0
			for wall := 0; wall < count/3; wall++ {
				top, bottom := 1+2*wall, 2+2*wall
				switch {
				case c.kind == 104 && start == top && end == top:
					role = 1 + 3*wall
				case c.kind == 110 && start == bottom && end == top:
					role = 2 + 3*wall
				case c.kind == 104 && start == bottom && end == bottom:
					role = 3 + 3*wall
				}
			}
			if role == 0 || seen[role] {
				return geom.ErrInvalid
			}
			seen[role] = true
			oldToNew[i+1] = role
			copy(out[2+5*(role-1):2+5*role], r.values[pos:pos+5])
		}
		r.values = out
		records[id] = r
	}
	if listID == 0 {
		return geom.ErrInvalid
	}
	for id, r := range records {
		if r.kind != 508 {
			continue
		}
		if len(r.values) < 2 {
			return geom.ErrInvalid
		}
		n, e := intValue(r.values[1])
		if e != nil || n < 1 || n > 4 || len(r.values) != 2+5*n {
			return geom.ErrInvalid
		}
		v := append([]parameter(nil), r.values...)
		for i := 0; i < n; i++ {
			pos := 2 + 5*i
			kind, e := intValue(v[pos])
			if e != nil || kind != 0 {
				return geom.ErrInvalid
			}
			owner, e := intValue(v[pos+1])
			if e != nil || owner != listID {
				return geom.ErrInvalid
			}
			old, e := intValue(v[pos+2])
			role, ok := oldToNew[old]
			if e != nil || !ok {
				return geom.ErrInvalid
			}
			pcurves, e := intValue(v[pos+4])
			if e != nil || pcurves != 0 {
				return geom.ErrInvalid
			}
			v[pos+2].text = fmt.Sprint(role)
		}
		r.values = v
		records[id] = r
	}
	return nil
}

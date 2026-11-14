package iges

import (
	"fmt"
	"math/big"
	"plainsolid/geom"
)

func ellipseSourcePoint(r topologyRecord, indices [3]int) ([3]*big.Rat, error) {
	var p [3]*big.Rat
	if compositeSourceNumbers(110, r.values) != nil {
		return p, geom.ErrInvalid
	}
	for i, j := range indices {
		if j < 0 || j >= len(r.values) {
			return p, geom.ErrInvalid
		}
		x, e := realValue(r.values[j])
		if e != nil {
			return p, e
		}
		p[i] = compositeRat(x)
	}
	return p, nil
}
func ellipseDirectrixEndpoint(c topologyRecord, records map[int]topologyRecord) ([3]*big.Rat, error) {
	p, e := ellipseSourcePoint(c, [3]int{8, 9, 7})
	if e != nil || c.kind != 104 || len(c.values) != 12 {
		return p, geom.ErrInvalid
	}
	if c.transform == 0 {
		return p, nil
	}
	tr, ok := records[c.transform]
	if !ok || tr.kind != 124 || tr.transform != 0 || len(tr.values) != 13 || compositeSourceNumbers(124, tr.values) != nil {
		return p, geom.ErrInvalid
	}
	var out [3]*big.Rat
	for i := 0; i < 3; i++ {
		x, e := realValue(tr.values[4+4*i])
		if e != nil {
			return out, e
		}
		out[i] = compositeRat(x)
		for j := 0; j < 3; j++ {
			v, e := realValue(tr.values[1+4*i+j])
			if e != nil {
				return out, e
			}
			out[i].Add(out[i], new(big.Rat).Mul(compositeRat(v), p[j]))
		}
	}
	return out, nil
}
func ellipseVertexListOrder(records map[int]topologyRecord, annular bool) (map[int]int, error) {
	count := 2
	if annular {
		count = 4
	}
	var vertices, shell topologyRecord
	for _, r := range records {
		if r.kind == 502 {
			if vertices.id != 0 {
				return nil, geom.ErrInvalid
			}
			vertices = r
		}
		if r.kind == 514 {
			if shell.id != 0 {
				return nil, geom.ErrInvalid
			}
			shell = r
		}
	}
	if vertices.id == 0 || shell.id == 0 || len(vertices.values) != 2+3*count {
		return nil, geom.ErrInvalid
	}
	n, e := intValue(vertices.values[1])
	if e != nil || n != count {
		return nil, geom.ErrInvalid
	}
	expected := make([][3]*big.Rat, count)
	for wall := 0; wall < count/2; wall++ {
		// Shell face order has already been proved from support kind and sense.
		faceID, e := intValue(shell.values[2+2*wall])
		if e != nil {
			return nil, e
		}
		face := records[faceID]
		if len(face.values) < 2 {
			return nil, geom.ErrInvalid
		}
		sid, e := intValue(face.values[1])
		if e != nil {
			return nil, e
		}
		side := records[sid]
		if side.kind != 122 || len(side.values) != 5 {
			return nil, geom.ErrInvalid
		}
		curveID, e := intValue(side.values[1])
		if e != nil {
			return nil, e
		}
		expected[2*wall], e = ellipseSourcePoint(side, [3]int{2, 3, 4})
		if e != nil {
			return nil, e
		}
		expected[2*wall+1], e = ellipseDirectrixEndpoint(records[curveID], records)
		if e != nil {
			return nil, e
		}
	}
	return conicMatchVertices(records, vertices, expected)
}
func conicMatchVertices(records map[int]topologyRecord, vertices topologyRecord, expected [][3]*big.Rat) (map[int]int, error) {
	count := len(expected)
	canonicalToSource, sourceToCanonical := map[int]int{}, map[int]int{}
	out := append([]parameter(nil), vertices.values...)
	for old := 1; old <= count; old++ {
		p, e := ellipseSourcePoint(vertices, [3]int{2 + 3*(old-1), 3 + 3*(old-1), 4 + 3*(old-1)})
		if e != nil {
			return nil, e
		}
		role := 0
		for i, q := range expected {
			equal := true
			for axis := 0; axis < 3; axis++ {
				equal = equal && p[axis].Cmp(q[axis]) == 0
			}
			if equal {
				if role != 0 {
					return nil, geom.ErrInvalid
				}
				role = i + 1
			}
		}
		if role == 0 || canonicalToSource[role] != 0 {
			return nil, geom.ErrInvalid
		}
		canonicalToSource[role], sourceToCanonical[old] = old, role
		copy(out[2+3*(role-1):2+3*role], vertices.values[2+3*(old-1):2+3*old])
	}
	vertices.values = out
	records[vertices.id] = vertices
	for id, r := range records {
		if r.kind != 504 {
			continue
		}
		if len(r.values) < 2 {
			return nil, geom.ErrInvalid
		}
		n, e := intValue(r.values[1])
		if e != nil || n < 1 || n > 6 || len(r.values) != 2+5*n {
			return nil, geom.ErrInvalid
		}
		v := append([]parameter(nil), r.values...)
		for i := 0; i < n; i++ {
			for _, offset := range []int{1, 3} {
				pos := 2 + 5*i + offset
				owner, e := intValue(v[pos])
				if e != nil || owner != vertices.id {
					return nil, geom.ErrInvalid
				}
				old, e := intValue(v[pos+1])
				role := sourceToCanonical[old]
				if e != nil || role == 0 {
					return nil, geom.ErrInvalid
				}
				v[pos+1].text = fmt.Sprint(role)
			}
		}
		r.values = v
		records[id] = r
	}
	return canonicalToSource, nil
}

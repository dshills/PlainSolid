package iges

import "plainsolid/geom"

// Normalize only cyclic starts of the established directed lateral loop.
// Endpoint incidence has already been normalized; the complete geometry proof
// still follows. No reversal, coedge sense change or pcurve omission is allowed.
func ellipseLoopStarts(records map[int]topologyRecord, annular bool) (map[int][]int, error) {
	roles := map[int][]int{}
	for id, r := range records {
		if r.kind != 508 {
			continue
		}
		if len(r.values) < 2 {
			return nil, geom.ErrInvalid
		}
		n, e := intValue(r.values[1])
		if e != nil || (n != 1 && n != 4) || len(r.values) != 2+5*n {
			return nil, geom.ErrInvalid
		}
		if n == 1 {
			roles[id] = []int{1}
			continue
		}
		matched := -1
		for offset := 0; offset <= 3; offset += 3 {
			if offset == 3 && !annular {
				break
			}
			for shift := 0; shift < 4; shift++ {
				valid := true
				for j, edge := range []int{1, 2, 3, 2} {
					pos := 2 + 5*((j+shift)%4)
					kind, ek := intValue(r.values[pos])
					idx, ei := intValue(r.values[pos+2])
					sense, es := intValue(r.values[pos+3])
					pc, ep := intValue(r.values[pos+4])
					wantSense := 0
					if j >= 2 {
						wantSense = 1
					}
					valid = valid && ek == nil && ei == nil && es == nil && ep == nil && kind == 0 && idx == edge+offset && sense == wantSense && pc == 0
				}
				if valid {
					if matched != -1 {
						return nil, geom.ErrInvalid
					}
					matched = shift
				}
			}
		}
		if matched < 0 {
			return nil, geom.ErrInvalid
		}
		v := append([]parameter(nil), r.values...)
		source := make([]int, 4)
		for j := 0; j < 4; j++ {
			old := (j + matched) % 4
			copy(v[2+5*j:2+5*(j+1)], r.values[2+5*old:2+5*(old+1)])
			source[j] = old + 1
		}
		r.values = v
		records[id] = r
		roles[id] = source
	}
	return roles, nil
}

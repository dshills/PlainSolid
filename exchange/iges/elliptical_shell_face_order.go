package iges

import "plainsolid/geom"

// Each supported shell has exactly one face per (support kind, shell sense).
// This classifies roles, not geometry: the caller must still prove all source
// supports, bounds, coedges and shared edge incidences against the exact model.
func ellipseShellFaceOrder(shell topologyRecord, records map[int]topologyRecord, annular bool) (topologyRecord, error) {
	roles := [][2]int{{122, 1}, {190, 1}, {190, 0}}
	if annular {
		roles = [][2]int{{122, 1}, {122, 0}, {190, 1}, {190, 0}}
	}
	return conicShellFaceOrder(shell, records, roles)
}
func conicShellFaceOrder(shell topologyRecord, records map[int]topologyRecord, roles [][2]int) (topologyRecord, error) {
	if len(shell.values) != 2+2*len(roles) {
		return topologyRecord{}, geom.ErrInvalid
	}
	count, err := intValue(shell.values[1])
	if err != nil || count != len(roles) {
		return topologyRecord{}, geom.ErrInvalid
	}
	pairs := map[[2]int][2]parameter{}
	faces := map[int]bool{}
	for i := 0; i < count; i++ {
		id, e := intValue(shell.values[2+2*i])
		if e != nil || faces[id] {
			return topologyRecord{}, geom.ErrInvalid
		}
		faces[id] = true
		sense, e := intValue(shell.values[3+2*i])
		if e != nil || (sense != 0 && sense != 1) {
			return topologyRecord{}, geom.ErrInvalid
		}
		face, ok := records[id]
		if !ok || face.kind != 510 || len(face.values) < 5 {
			return topologyRecord{}, geom.ErrInvalid
		}
		support, e := intValue(face.values[1])
		if e != nil {
			return topologyRecord{}, geom.ErrInvalid
		}
		s, ok := records[support]
		if !ok {
			return topologyRecord{}, geom.ErrInvalid
		}
		key := [2]int{s.kind, sense}
		if _, exists := pairs[key]; exists {
			return topologyRecord{}, geom.ErrInvalid
		}
		pairs[key] = [2]parameter{shell.values[2+2*i], shell.values[3+2*i]}
	}
	shell.values = append([]parameter(nil), shell.values...)
	for i, key := range roles {
		p, ok := pairs[key]
		if !ok {
			return topologyRecord{}, geom.ErrInvalid
		}
		shell.values[2+2*i], shell.values[3+2*i] = p[0], p[1]
	}
	return shell, nil
}

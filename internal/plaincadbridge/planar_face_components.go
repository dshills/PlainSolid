package plaincadbridge

import (
	"plainsolid/brep"
)

// connectedPlanarFaces distinguishes representation seams from genuinely
// disconnected supports. Only shared native edges connect selected faces.
func connectedPlanarFaces(s brep.Solid, selected map[int]bool) bool {
	if len(selected) == 0 {
		return false
	}
	owners := map[int][]int{}
	faces := s.Faces()
	first := -1
	for id := range selected {
		if first < 0 {
			first = id
		}
		f := faces[id]
		for _, loop := range append([][]brep.EdgeUse{f.Boundary}, f.Holes...) {
			for _, u := range loop {
				owners[u.Edge] = append(owners[u.Edge], id)
			}
		}
	}
	adj := map[int][]int{}
	for _, ids := range owners {
		if len(ids) == 2 {
			adj[ids[0]] = append(adj[ids[0]], ids[1])
			adj[ids[1]] = append(adj[ids[1]], ids[0])
		}
	}
	seen := map[int]bool{first: true}
	queue := []int{first}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, next := range adj[id] {
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return len(seen) == len(selected)
}

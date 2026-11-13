package brep

import "sort"

// Shells extracts the edge-connected components of the closed boundary.
// Orientation is preserved, so a cavity's component can be inward.
// Result ordering follows the first original face in each component.
func (s Solid) Shells() ([]Solid, error) {
	incident := make([][]int, len(s.edges))
	for i, f := range s.faces {
		for _, l := range faceLoops(f) {
			for _, u := range l {
				incident[u.Edge] = append(incident[u.Edge], i)
			}
		}
	}
	seen := make([]bool, len(s.faces))
	var out []Solid
	for i := range s.faces {
		if seen[i] {
			continue
		}
		seen[i] = true
		queue := []int{i}
		for k := 0; k < len(queue); k++ {
			for _, l := range faceLoops(s.faces[queue[k]]) {
				for _, u := range l {
					for _, j := range incident[u.Edge] {
						if !seen[j] {
							seen[j] = true
							queue = append(queue, j)
						}
					}
				}
			}
		}
		sort.Ints(queue)
		var r Solid
		vm, em := map[int]int{}, map[int]int{}
		vertex := func(i int) int {
			if j, ok := vm[i]; ok {
				return j
			}
			j := len(r.vertices)
			vm[i] = j
			r.vertices = append(r.vertices, s.vertices[i])
			return j
		}
		edge := func(i int) int {
			if j, ok := em[i]; ok {
				return j
			}
			j := len(r.edges)
			em[i] = j
			e := s.edges[i]
			r.edges = append(r.edges, Edge{vertex(e.Start), vertex(e.End)})
			return j
		}
		for _, i := range queue {
			f := cloneFaces(s.faces[i : i+1])[0]
			for _, l := range faceLoops(f) {
				for j, u := range l {
					l[j].Edge = edge(u.Edge)
				}
			}
			r.faces = append(r.faces, f)
		}
		x, e := NewSolid(r.vertices, r.edges, r.faces)
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, nil
}

package mesh

import "fmt"

func referenceValidateSolid(m Mesh) error {
	if err := m.Validate(); err != nil {
		return err
	}
	edges := make(map[edge][]use)
	incident := make(map[int][]int)
	for f, t := range m.Triangles {
		for i, a := range t {
			b := t[(i+1)%3]
			k := edge{a, b}
			s := 1
			if a > b {
				k = edge{b, a}
				s = -1
			}
			edges[k] = append(edges[k], use{f, s})
			incident[a] = append(incident[a], f)
		}
	}
	adjacent := make(map[int][]int)
	for e, u := range edges {
		if len(u) != 2 || u[0].sign == u[1].sign {
			return fmt.Errorf("mesh: edge (%d,%d) is not closed and consistently oriented", e.a, e.b)
		}
		adjacent[u[0].face] = append(adjacent[u[0].face], u[1].face)
		adjacent[u[1].face] = append(adjacent[u[1].face], u[0].face)
	}
	for v, faces := range incident {
		allowed := make(map[int]bool, len(faces))
		for _, f := range faces {
			allowed[f] = true
		}
		visited := make(map[int]bool)
		stack := []int{faces[0]}
		for len(stack) > 0 {
			f := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if visited[f] {
				continue
			}
			visited[f] = true
			for _, g := range adjacent[f] {
				if allowed[g] && !visited[g] {
					stack = append(stack, g)
				}
			}
		}
		if len(visited) != len(allowed) {
			return fmt.Errorf("mesh: disconnected fan at vertex %d", v)
		}
	}
	return nil
}

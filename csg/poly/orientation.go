package poly

import (
	"fmt"
	"math/big"
	"plainsolid/mesh"
)

// Independently check material orientation through shell containment parity.
// Embeddedness is established by the exact collision pass before this routine.
func orientations(m mesh.Mesh, ps []polygon, b *budget) error {
	adj := make([][]int, len(ps))
	edges := map[[2]int]int{}
	for i, t := range m.Triangles {
		for j, v := range t {
			w := t[(j+1)%3]
			if v > w {
				v, w = w, v
			}
			k := [2]int{v, w}
			if x, ok := edges[k]; ok {
				adj[i] = append(adj[i], x)
				adj[x] = append(adj[x], i)
			} else {
				edges[k] = i
			}
		}
	}
	seen := make([]bool, len(ps))
	var shells [][]int
	for i := range ps {
		if seen[i] {
			continue
		}
		seen[i] = true
		todo := []int{i}
		var shell []int
		for len(todo) > 0 {
			x := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			shell = append(shell, x)
			for _, j := range adj[x] {
				if !seen[j] {
					seen[j] = true
					todo = append(todo, j)
				}
			}
		}
		shells = append(shells, shell)
	}
	for i, s := range shells {
		origin := ps[s[0]].v[0]
		volume := new(big.Rat)
		for _, j := range s {
			p := ps[j]
			volume = add(volume, dot(diff(p.v[0], origin), cross(diff(p.v[1], origin), diff(p.v[2], origin))))
			b.check(volume)
			if !b.tick(1) {
				return b.err
			}
		}
		depth := 0
		for j, other := range shells {
			if i == j {
				continue
			}
			inside, err := contains(origin, other, ps, b)
			if err != nil {
				return err
			}
			if inside {
				depth++
			}
		}
		expected := 1
		if depth%2 == 1 {
			expected = -1
		}
		if volume.Sign() != expected {
			return fmt.Errorf("%w: shell orientation disagrees with nesting", ErrInvalid)
		}
	}
	return b.err
}
func contains(origin point, shell []int, ps []polygon, b *budget) (bool, error) {
	for k := int64(1); k <= 64; k++ {
		dir := point{big.NewRat(1, 1), big.NewRat(k, 1), big.NewRat(k*k, 1)}
		count := 0
		ambiguous := false
		for _, i := range shell {
			if !b.tick(1) {
				return false, b.err
			}
			p := ps[i]
			den := dot(p.p.n, dir)
			num := sub(p.p.d, dot(p.p.n, origin))
			if den.Sign() == 0 {
				if num.Sign() == 0 {
					ambiguous = true
					break
				}
				continue
			}
			t := div(num, den)
			if t.Sign() <= 0 {
				continue
			}
			var x point
			for j := range x {
				x[j] = add(origin[j], mul(t, dir[j]))
			}
			b.point(x)
			if b.err != nil {
				return false, b.err
			}
			inside, on := true, false
			for j, a := range p.v {
				c := p.v[(j+1)%len(p.v)]
				sign := dot(cross(diff(c, a), diff(x, a)), p.p.n).Sign()
				if sign < 0 {
					inside = false
					break
				}
				on = on || sign == 0
			}
			if inside {
				if on {
					ambiguous = true
					break
				}
				count++
			}
		}
		if !ambiguous {
			return count%2 == 1, nil
		}
	}
	return false, fmt.Errorf("%w: shell containment ray degeneracy", ErrLimit)
}

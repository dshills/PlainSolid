package boolean

import "sort"

// sourcesOf is the face source list of a face made of the given pieces
// (P5): each piece contributes its own input face, and a shared piece
// also the face it is shared with. The list has no duplicates and is
// ordered A before B, then by face index.
func sourcesOf(pieces []piece) [][2]int {
	var out [][2]int
	for _, p := range pieces {
		out = append(out, [2]int{p.op, p.face})
		if p.mate >= 0 {
			out = append(out, [2]int{1 - p.op, p.mate})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i][0] < out[j][0] || out[i][0] == out[j][0] && out[i][1] < out[j][1]
	})
	unique := out[:0]
	for i, s := range out {
		if i == 0 || s != out[i-1] {
			unique = append(unique, s)
		}
	}
	return unique
}

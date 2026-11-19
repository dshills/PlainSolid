package boolean

// class is where a piece lies relative to the other operand (section 7.7).
type class uint8

const (
	outside class = iota
	inside
)

// verdict is one cell of the selection table: whether a piece is kept and
// whether it is kept with its orientation reversed.
type verdict struct{ keep, flip bool }

// selection is the table of section 7.8. It is indexed by the operation,
// by the role of the piece's operand (0 for the first operand, 1 for the
// second) and by the class of the piece.
var selection = [3][2][2]verdict{
	Union:  {{outside: {keep: true}}, {outside: {keep: true}}},
	Common: {{inside: {keep: true}}, {inside: {keep: true}}},
	Cut:    {{outside: {keep: true}}, {inside: {keep: true, flip: true}}},
}

// kept is a piece that the selection keeps.
type kept struct {
	piece int
	flip  bool
}

// choose applies the selection table to the pieces. With swap set the
// operands change roles, which gives B − A for Cut.
func choose(pieces []piece, op Operation, swap bool) []kept {
	var out []kept
	for i, p := range pieces {
		role := p.op
		if swap {
			role = 1 - role
		}
		if v := selection[op][role][p.class]; v.keep {
			out = append(out, kept{i, v.flip})
		}
	}
	return out
}

// volume is the volume that the kept pieces enclose, from the pieces
// alone (V5).
func (f *fabric) volume(pieces []piece, keep []kept) float64 {
	sum := 0.
	for _, k := range keep {
		if v := f.flux(pieces[k.piece]); k.flip {
			sum -= v
		} else {
			sum += v
		}
	}
	return sum
}

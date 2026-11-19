package boolean

import "plainsolid/geom"

// point is a node of the arrangement as the building stage reads it: a
// position and a radius (AR1, T2).
type point struct {
	pos    geom.Vec3
	radius float64
}

// element is one edge of the building stage: a span that is in no bundle,
// a bundle, or a trace. It runs from point a to point b along its carrier,
// with t0 < t1.
type element struct {
	c      curve
	t0, t1 float64
	a, b   int
}

// hop is one step of a loop: an element walked along its direction or,
// when reversed, against it.
type hop struct {
	elem     int
	reversed bool
}

// fabric is the part of the arrangement that the stages from splitting to
// assembly walk: the points, the elements between them, and the boundary
// of every input face as loops of hops.
type fabric struct {
	ops    [2]*operand
	points []point
	elems  []element
	rims   [2][][][]hop
}

// piece is a connected part of one input face (section 3). Its loops keep
// the piece on their left in the chart of the face: the outer loop first,
// then the holes. It records its input face (F5).
type piece struct {
	op, face int
	loops    [][]hop
	class    class
	// mate is the face of the other operand that a shared piece lies on
	// (K1), and -1 for a piece that is not shared.
	mate int
}

// whole gives every input face as one piece, classified outside the other
// operand (I2).
func (f *fabric) whole() []piece {
	var out []piece
	for i := range f.rims {
		for fi, loops := range f.rims[i] {
			out = append(out, piece{op: i, face: fi, loops: loops, class: outside, mate: -1})
		}
	}
	return out
}

// legsOf gives loops of hops over a list of elements as legs.
func legsOf(elems []element, loops [][]hop) [][]leg {
	out := make([][]leg, len(loops))
	for i, loop := range loops {
		for _, h := range loop {
			e := elems[h.elem]
			l := leg{e.c, e.t0, e.t1}
			if h.reversed {
				l = l.back()
			}
			out[i] = append(out[i], l)
		}
	}
	return out
}

// flux is the share of an enclosed volume that a piece contributes when it
// is kept with its own orientation.
func (f *fabric) flux(p piece) float64 {
	face := f.ops[p.op].faces[p.face]
	v := face.s.chart().flux(legsOf(f.elems, p.loops))
	if face.reversed {
		return -v
	}
	return v
}

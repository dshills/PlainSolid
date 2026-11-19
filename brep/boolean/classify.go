package boolean

import "plainsolid/brep/parametric"

// classify gives every piece its class (section 7.7).
func (a *arrangement) classify(pieces []piece) error {
	for i := range pieces {
		p := &pieces[i]
		if err := a.ctx.Err(); err != nil {
			return err
		}
		probes := a.probes(*p)
		// K1: a piece of a face in a same-surface pair is shared when
		// its interior point lies inside the other face of the pair. An
		// edge that the other face uses twice, a seam or the radial edge
		// of a polar chart, runs through its region and is not its
		// boundary, so a point that falls on one is passed over for the
		// next; only when every point is on a boundary is the
		// arrangement inconsistent.
		settled := false
		for _, q := range probes {
			rim := false
			for _, pair := range a.pairs {
				if pair.faces[p.op] != p.face || p.mate >= 0 {
					continue
				}
				switch a.inFace(1-p.op, pair.faces[1-p.op], q, a.t.eps) {
				case onRim:
					rim = true
				case within:
					p.mate, p.class = pair.faces[1-p.op], sharedSame
					if pair.opposite {
						p.class = sharedOpposite
					}
				}
			}
			if settled = p.mate >= 0 || !rim; settled {
				break
			}
		}
		if !settled {
			return fail(ErrInconsistent, "K1", "every interior point of a piece of face %d of %s is on the boundary of a face of %s on the same surface", p.face, operandName(p.op), operandName(1-p.op))
		}
		if p.mate >= 0 {
			continue
		}
		// K2: any other piece is inside or outside the other operand,
		// at an interior point at least ε from its boundary.
		settled = false
		for _, q := range probes {
			at, err := a.ops[1-p.op].solid.Classify(q, a.t.eps)
			if err != nil {
				return failWith(ErrInconsistent, "K2", err)
			}
			if at != parametric.Boundary {
				settled = true
				if p.class = outside; at == parametric.Inside {
					p.class = inside
				}
				break
			}
		}
		if !settled {
			return fail(ErrInconsistent, "K2", "every interior point of a piece of face %d of %s is on the boundary of %s", p.face, operandName(p.op), operandName(1-p.op))
		}
	}
	return a.sameAcross(pieces)
}

// sameAcross is K3: two pieces of one operand that meet along a span
// which the other operand does not touch have the same class. Such a
// span is in no bundle, is not a trace, and does not lie in a face of
// the other operand.
func (a *arrangement) sameAcross(pieces []piece) error {
	touched := filled(a.elems, false)
	for _, b := range a.bundles {
		touched[b.elem] = true
	}
	for i := range a.inner {
		for _, list := range a.inner[i] {
			for _, e := range list {
				touched[e] = true
			}
		}
	}
	first := filled(a.elems, -1)
	for i, p := range pieces {
		for _, loop := range p.loops {
			for _, h := range loop {
				if touched[h.elem] {
					continue
				}
				if first[h.elem] < 0 {
					first[h.elem] = i
				} else if q := pieces[first[h.elem]]; q.class != p.class {
					return fail(ErrInconsistent, "K3", "pieces of faces %d and %d of %s differ in class across an edge the other operand does not touch", q.face, p.face, operandName(p.op))
				}
			}
		}
	}
	return nil
}

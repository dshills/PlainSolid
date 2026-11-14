package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicGraphCapUse is one directed boundary piece. Cell holds its exact
// world curve and algebraic domain, always in increasing cell parameter. If
// Reversed is true, the wire uses the cell from its upper endpoint to its lower
// endpoint. EndpointContacts refer to the section contacts in wire order, with
// -1 for an untouched input vertex.
type AlgebraicGraphCapUse struct {
	Cell             *geom.ExactComposedCurveCell
	Reversed         bool
	EndpointContacts [2]int
	// InputWire and InputPiece locate the actual input occurrence. Both are
	// -1 on a new cut seam; Seam is -1 on an existing boundary piece.
	InputWire, InputPiece, Seam int
	Source                      RationalGraphCapEdgeSource
	// SourceParameters are the normalized directed parameters on the
	// original section piece. They are nil on new cut seams.
	SourceParameters [2]*geom.ExactAlgebraicRoot
}

// AlgebraicGraphCapClip stores constructive exact boundary cycles for the
// retained material on the input cap. Cycles follow the original orientation
// (outer rings and voids have opposite winding). It does not construct a sewn
// solid or infer a second owner for the new seams.
type AlgebraicGraphCapClip struct {
	Section                       AlgebraicGraphCapSection
	Wires                         [][]AlgebraicGraphCapUse
	ExactOperations, Subdivisions int
	wires                         [][]AlgebraicGraphCapUse
	regions                       *RationalGraphShellRegions
	cut                           polynomialPlaneEquation
	keepSign                      int
	recipe                        *algebraicGraphCapRecipe
}

// ClipAlgebraicHalfspace clips the previously certified cap against the
// oriented plane. Existing rational curves stay on their original supports;
// cut endpoints and seam terminal parameters can be irrational. An exact
// contact partition selects the maximal kept runs, then the signed material
// intervals pair those runs into closed wires. The operation is atomic and
// has the same regular crossing restrictions as SectionAlgebraicHalfspace.
func (r *RationalGraphShellRegions) ClipAlgebraicHalfspace(ctx context.Context, points [3]geom.Vec3, keepPositive bool, o RationalGraphShellRegionOptions) (AlgebraicGraphCapClip, error) {
	var zero AlgebraicGraphCapClip
	var proof algebraicGraphCapProof
	s, e := r.sectionAlgebraicHalfspace(ctx, points, keepPositive, o, &proof)
	if e != nil {
		return zero, e
	}
	options, _ := o.Topology.WithDefaults() // already checked by the section proof
	b := capClipBudget{ctx, options.MaxWork - s.ExactOperations, options.MaxSubdivisions - s.Subdivisions, options.MaxBits}
	out := AlgebraicGraphCapClip{Section: s, regions: r, cut: proof.cut, keepSign: proof.keepSign}
	type run struct {
		uses []AlgebraicGraphCapUse
		end  int
	}
	runs := make(map[int]run)
	total := 0
	for wi, wire := range r.wires {
		var row []AlgebraicGraphCapUse
		for pi, p := range wire {
			record := proof.pieces[wi][pi]
			initial, e := b.side(proof.form, p.control[0])
			if e != nil {
				return zero, e
			}
			kept := initial * proof.keepSign
			if kept == 0 {
				return zero, geom.ErrConvergence
			}
			var model *geom.ExactComposedCurveCell
			n := len(record.local)
			for j := 0; j <= n; j++ {
				if j < n && s.Contacts[proof.contactOrder[record.contacts[j]]].KeepBefore != (kept > 0) {
					return zero, geom.ErrConvergence
				}
				if kept > 0 {
					if model == nil {
						model, e = r.capClipCell(p.control, &b)
						if e != nil {
							return zero, e
						}
					}
					lo, _ := model.EndpointRoot(0)
					hi, _ := model.EndpointRoot(1)
					use := AlgebraicGraphCapUse{Cell: model, InputWire: wi, InputPiece: pi, Seam: -1, Source: p.source, EndpointContacts: [2]int{-1, -1}}
					if j > 0 {
						lo, use.EndpointContacts[0] = record.local[j-1], proof.contactOrder[record.contacts[j-1]]
					}
					if j < n {
						hi, use.EndpointContacts[1] = record.local[j], proof.contactOrder[record.contacts[j]]
					}
					if n > 0 {
						use.Cell, e = b.restrict(model, lo, hi)
						if e != nil {
							return zero, e
						}
					}
					use.SourceParameters, e = capClipSourceParameters(p, use.EndpointContacts, [2]*geom.ExactAlgebraicRoot{lo, hi}, s, &b)
					if e != nil {
						return zero, e
					}
					row = append(row, use)
					total++
					if total+len(s.Material) > options.MaxEdges {
						return zero, geom.ErrConvergence
					}
				}
				kept = -kept
			}
			last, e := b.side(proof.form, p.control[len(p.control)-1])
			if e != nil {
				return zero, e
			}
			if last*proof.keepSign != -kept {
				return zero, geom.ErrConvergence
			}
		}
		if len(row) == 0 {
			continue
		}
		start := -1
		for i, use := range row {
			if use.EndpointContacts[0] >= 0 {
				start = i
				break
			}
		}
		if start < 0 {
			for _, use := range row {
				if use.EndpointContacts != ([2]int{-1, -1}) {
					return zero, geom.ErrConvergence
				}
			}
			out.Wires = append(out.Wires, row)
			continue
		}
		// Rotate to a known entering contact and partition into maximal runs.
		// Adjacent kept pieces without contacts share a certified old vertex.
		rotated := append(append(make([]AlgebraicGraphCapUse, 0, len(row)), row[start:]...), row[:start]...)
		first := 0
		for i, use := range rotated {
			if (i == first) != (use.EndpointContacts[0] >= 0) {
				return zero, geom.ErrConvergence
			}
			if end := use.EndpointContacts[1]; end >= 0 {
				begin := rotated[first].EndpointContacts[0]
				if _, exists := runs[begin]; exists {
					return zero, geom.ErrConvergence
				}
				runs[begin] = run{rotated[first : i+1], end}
				first = i + 1
			}
		}
		if first != len(rotated) {
			return zero, geom.ErrConvergence
		}
	}
	seamAt := make(map[int]int, len(s.Material))
	for i, seam := range s.Material {
		seamAt[seam.From] = i
	}
	seen := make(map[int]bool, len(s.Material))
	for _, seam := range s.Material {
		start := seam.To
		if _, exists := runs[start]; !exists {
			return zero, geom.ErrConvergence
		}
		if seen[start] {
			continue
		}
		var wire []AlgebraicGraphCapUse
		for next := start; ; {
			run, exists := runs[next]
			id, found := seamAt[run.end]
			if !exists || !found || seen[next] {
				return zero, geom.ErrConvergence
			}
			seen[next] = true
			line, e := r.capClipSeam(s, &proof, id, &b)
			if e != nil {
				return zero, e
			}
			wire = append(wire, run.uses...)
			wire = append(wire, line)
			next = line.EndpointContacts[1]
			if next == start {
				break
			}
		}
		out.Wires = append(out.Wires, wire)
	}
	if len(seen) != len(s.Material) || len(seen) != len(runs) {
		return zero, geom.ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	out.wires = make([][]AlgebraicGraphCapUse, len(out.Wires))
	for i, wire := range out.Wires {
		out.wires[i] = append([]AlgebraicGraphCapUse(nil), wire...)
	}
	out.ExactOperations, out.Subdivisions = options.MaxWork-b.work, options.MaxSubdivisions-b.subdiv
	return out, nil
}

type capClipBudget struct {
	ctx                context.Context
	work, subdiv, bits int
}

func (b *capClipBudget) charge(ops, subdiv int) error {
	b.work, b.subdiv = b.work-ops, b.subdiv-subdiv
	if b.work < 1 || b.subdiv < 1 {
		return geom.ErrConvergence
	}
	return b.ctx.Err()
}

func (b *capClipBudget) limits() geom.ExactCurvePlaneEventOptions {
	return geom.ExactCurvePlaneEventOptions{MaxOperations: b.work, MaxCoefficientBits: b.bits, MaxSubdivisions: b.subdiv}
}

func (b *capClipBudget) restrict(c *geom.ExactComposedCurveCell, lo, hi *geom.ExactAlgebraicRoot) (*geom.ExactComposedCurveCell, error) {
	p, e := c.RestrictAlgebraic(b.ctx, lo, hi, b.limits())
	if e != nil {
		return nil, e
	}
	if e := b.charge(p.Operations, p.Subdivisions); e != nil {
		return nil, e
	}
	if !p.Proven {
		return nil, geom.ErrConvergence
	}
	return p.Cell, nil
}

func (b *capClipBudget) mapRoot(root *geom.ExactAlgebraicRoot, offset, scale *big.Rat) (*geom.ExactAlgebraicRoot, error) {
	q, ops, e := root.AffineMap(b.ctx, offset, scale, b.limits())
	if e != nil {
		return nil, e
	}
	return q, b.charge(ops, 0)
}

func (b *capClipBudget) side(form [3]*big.Rat, p [3]*big.Rat) (int, error) {
	sum := new(big.Rat)
	for i, q := range form {
		product := new(big.Rat).Mul(p[i], q)
		sum.Add(sum, product)
		for _, x := range []*big.Rat{product, sum} {
			if x.Num().BitLen() > b.bits || x.Denom().BitLen() > b.bits {
				return 0, geom.ErrConvergence
			}
		}
	}
	return sum.Sign(), b.charge(6, 0)
}

func capClipSourceParameters(p projectedGraphSectionPiece, contacts [2]int, local [2]*geom.ExactAlgebraicRoot, section AlgebraicGraphCapSection, b *capClipBudget) ([2]*geom.ExactAlgebraicRoot, error) {
	for side, id := range contacts {
		if id >= 0 {
			local[side] = section.Contacts[id].Parameter
		} else if p.span[0] != nil || p.span[1] != nil {
			lo, hi := graphCapOriginalParameter(p.span, new(big.Rat)), graphCapOriginalParameter(p.span, big.NewRat(1, 1))
			var e error
			local[side], e = b.mapRoot(local[side], lo, new(big.Rat).Sub(hi, lo))
			if e != nil {
				return [2]*geom.ExactAlgebraicRoot{}, e
			}
		}
	}
	return local, nil
}

// Lift rational homogeneous controls without changing their parameter. By
// construction every model point lies exactly in the certified cap plane.
func (r *RationalGraphShellRegions) capClipCell(control [][3]*big.Rat, b *capClipBudget) (*geom.ExactComposedCurveCell, error) {
	h := make([][4]*big.Rat, len(control))
	axes, missing := orientedRationalGraphProjection(r.plane), r.plane.dropped
	for i, q := range control {
		h[i][axes[0]], h[i][axes[1]], h[i][3] = q[0], q[1], q[2]
		z := new(big.Rat).Mul(r.plane.offset, q[2])
		for _, axis := range axes {
			z.Sub(z, new(big.Rat).Mul(h[i][axis], r.plane.normal[axis]))
		}
		h[i][missing] = z
	}
	if e := b.charge(5*len(h), 0); e != nil {
		return nil, e
	}
	lim := geom.SurfacePatchOptions{MaxOperations: b.work, MaxCoefficientBits: b.bits}
	curve, e := geom.NewExactHomogeneousBezier(b.ctx, h, lim)
	if e != nil {
		return nil, e
	}
	if e := b.charge(curve.ExactOperations(), 0); e != nil {
		return nil, e
	}
	lim.MaxOperations = b.work
	cell, used, e := geom.NewExactComposedBezierCellWithWork(b.ctx, curve, lim)
	if e != nil {
		return nil, e
	}
	return cell, b.charge(used, 0)
}

// Build the line in a rational enclosing coordinate interval. The actual
// endpoints are the same exact contact coordinates mapped to this interval's
// parameter. Both cap equations give unique model points at those coordinates.
func (r *RationalGraphShellRegions) capClipSeam(s AlgebraicGraphCapSection, p *algebraicGraphCapProof, id int, b *capClipBudget) (AlgebraicGraphCapUse, error) {
	var zero AlgebraicGraphCapUse
	seam := s.Material[id]
	axis, dep := p.axis, 1-p.axis
	a, z := s.Contacts[seam.From].CoordinateRoots[axis], s.Contacts[seam.To].CoordinateRoots[axis]
	reverse := p.direction < 0
	if reverse {
		a, z = z, a
	}
	x, y := a.Interval().Lower, z.Interval().Upper
	if x.Cmp(y) >= 0 {
		return zero, geom.ErrConvergence
	}
	point := func(free *big.Rat) (q [3]*big.Rat) {
		q[axis], q[2] = free, big.NewRat(1, 1)
		q[dep] = new(big.Rat).Mul(p.form[axis], free)
		q[dep].Add(q[dep], p.form[2]).Neg(q[dep]).Quo(q[dep], p.form[dep])
		return q
	}
	cell, e := r.capClipCell([][3]*big.Rat{point(x), point(y)}, b)
	if e != nil {
		return zero, e
	}
	scale := new(big.Rat).Inv(new(big.Rat).Sub(y, x))
	offset := new(big.Rat).Neg(new(big.Rat).Mul(x, scale))
	lo, e := b.mapRoot(a, offset, scale)
	if e != nil {
		return zero, e
	}
	hi, e := b.mapRoot(z, offset, scale)
	if e != nil {
		return zero, e
	}
	cell, e = b.restrict(cell, lo, hi)
	if e != nil {
		return zero, e
	}
	return AlgebraicGraphCapUse{Cell: cell, Reversed: reverse, EndpointContacts: [2]int{seam.From, seam.To}, InputWire: -1, InputPiece: -1, Seam: id,
		Source: RationalGraphCapEdgeSource{Wire: -1, Branch: -1, Piece: -1, Face: -1}}, nil
}

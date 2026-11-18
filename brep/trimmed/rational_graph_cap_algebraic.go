//go:build research

package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/geom"
)

// AlgebraicGraphCapContact binds one regular contact to its original cap
// piece. Parameter refers to that piece's normalized directed interval;
// CoordinateRoots use the section's original oriented projection axes.
type AlgebraicGraphCapContact struct {
	Source          RationalGraphCapEdgeSource
	Parameter       *geom.ExactAlgebraicRoot
	CoordinateRoots [2]*geom.ExactAlgebraicRoot
	KeepBefore      bool
}

// AlgebraicGraphCapSection is the complete list of line contacts and
// intervals of original cap material, ordered along the oriented cut line.
// It retains algebraic endpoint identities without constructing trim wires.
type AlgebraicGraphCapSection struct {
	ProjectionAxes                [2]int
	Contacts                      []AlgebraicGraphCapContact
	Material                      []RationalGraphCapSeam
	ExactOperations, Subdivisions int
}

// SectionAlgebraicHalfspace proves an exhaustive regular intersection with
// the section boundary. The input is one immutable previously verified cap
// arrangement. A selected coordinate is isolated and compared exactly,
// including irrational and arbitrarily close contacts. Original vertices,
// tangencies and intervals on the cutting plane remain unresolved.
func (r *RationalGraphShellRegions) SectionAlgebraicHalfspace(ctx context.Context, points [3]geom.Vec3, keepPositive bool, o RationalGraphShellRegionOptions) (AlgebraicGraphCapSection, error) {
	return r.sectionAlgebraicHalfspace(ctx, points, keepPositive, o, nil)
}

type algebraicGraphCapPieceProof struct {
	local    []*geom.ExactAlgebraicRoot
	contacts []int // temporary encounter IDs (before the cut line sort)
}

type algebraicGraphCapProof struct {
	cut                       polynomialPlaneEquation
	form                      [3]*big.Rat
	axis, direction, keepSign int
	pieces                    [][]algebraicGraphCapPieceProof
	contactOrder              []int
}

func (r *RationalGraphShellRegions) sectionAlgebraicHalfspace(ctx context.Context, points [3]geom.Vec3, keepPositive bool, o RationalGraphShellRegionOptions, proof *algebraicGraphCapProof) (AlgebraicGraphCapSection, error) {
	var zero AlgebraicGraphCapSection
	if ctx == nil || r == nil || r.certificate == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	for _, p := range points {
		if !p.IsFinite() {
			return zero, geom.ErrInvalid
		}
	}
	other, e := polynomialPlaneFromPoints(points)
	if e != nil {
		return zero, e
	}
	opts, e := o.Topology.WithDefaults()
	if e != nil {
		return zero, e
	}
	if opts.MaxEdges > 4096 {
		return zero, geom.ErrInvalid
	}
	keep := -1
	if keepPositive {
		keep = 1
	}
	work, subdiv := opts.MaxWork, opts.MaxSubdivisions
	limits := func() geom.ExactCurvePlaneEventOptions {
		return geom.ExactCurvePlaneEventOptions{MaxOperations: work, MaxCoefficientBits: opts.MaxBits, MaxSubdivisions: subdiv}
	}
	charge := func(ops, splits int) error {
		work, subdiv = work-ops, subdiv-splits
		if work < 1 || subdiv < 1 {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	form := r.capHalfspaceForm(other)
	for _, f := range form {
		if f.Num().BitLen() > opts.MaxBits || f.Denom().BitLen() > opts.MaxBits {
			return zero, geom.ErrConvergence
		}
	}
	axis, direction := 0, form[1].Sign()
	if direction == 0 {
		axis, direction = 1, -form[0].Sign()
	}
	if direction == 0 {
		return zero, geom.ErrConvergence
	}
	direction *= other.orientation * keep
	if proof != nil {
		proof.form, proof.axis, proof.direction, proof.keepSign = form, axis, direction, other.orientation*keep
		proof.cut = other
		proof.pieces = make([][]algebraicGraphCapPieceProof, len(r.wires))
	}
	raw := [4]*big.Rat{form[0], form[1], nil, form[2]}
	out := AlgebraicGraphCapSection{ProjectionAxes: orientedRationalGraphProjection(r.plane)}
	type orderedContact struct {
		AlgebraicGraphCapContact
		id int
	}
	var roots []orderedContact
	count := 0
	for wi, wire := range r.wires {
		if proof != nil {
			proof.pieces[wi] = make([]algebraicGraphCapPieceProof, len(wire))
		}
		for pi, piece := range wire {
			count++
			if count > opts.MaxEdges || len(piece.control) < 2 || len(piece.control) > 5 {
				return zero, geom.ErrConvergence
			}
			h := make([][4]*big.Rat, len(piece.control))
			for i, p := range piece.control {
				h[i] = [4]*big.Rat{p[0], p[1], new(big.Rat), p[2]}
			}
			lim := geom.SurfacePatchOptions{MaxOperations: work, MaxCoefficientBits: opts.MaxBits}
			curve, e := geom.NewExactHomogeneousBezier(ctx, h, lim)
			if e != nil {
				return zero, e
			}
			if e = charge(curve.ExactOperations(), 0); e != nil {
				return zero, e
			}
			lim.MaxOperations = work
			cell, used, e := geom.NewExactComposedBezierCellWithWork(ctx, curve, lim)
			if e != nil {
				return zero, e
			}
			if e = charge(used, 0); e != nil {
				return zero, e
			}
			hits, e := cell.IntersectRawPlane(ctx, raw, limits())
			if e != nil {
				return zero, e
			}
			if e = charge(hits.Operations, hits.Subdivisions); e != nil {
				return zero, e
			}
			if !hits.Complete || hits.Coincident {
				return zero, geom.ErrConvergence
			}
			var record algebraicGraphCapPieceProof
			for _, event := range hits.Events {
				p, e := event.ClassifyPlaneContact(ctx, raw, limits())
				if e != nil {
					return zero, e
				}
				if e = charge(p.Operations, p.Subdivisions); e != nil {
					return zero, e
				}
				if !p.Proven || !p.Crossing || !p.Transverse || p.EndpointMask != 0 || len(roots) >= opts.MaxEdges {
					return zero, geom.ErrConvergence
				}
				entry := AlgebraicGraphCapContact{Source: piece.source, KeepBefore: p.SideSigns[0]*other.orientation*keep > 0}
				entry.Parameter = event.Root()
				if proof != nil {
					record.local = append(record.local, entry.Parameter)
					record.contacts = append(record.contacts, len(roots))
				}
				if piece.span[0] != nil || piece.span[1] != nil {
					a, b := graphCapOriginalParameter(piece.span, new(big.Rat)), graphCapOriginalParameter(piece.span, big.NewRat(1, 1))
					scale := new(big.Rat).Sub(b, a)
					var ops int
					entry.Parameter, ops, e = entry.Parameter.AffineMap(ctx, a, scale, limits())
					if e != nil {
						return zero, e
					}
					if e = charge(ops, 0); e != nil {
						return zero, e
					}
				}
				for i := range entry.CoordinateRoots {
					root, ops, e := event.CoordinateRoot(ctx, i, limits())
					if e != nil {
						return zero, e
					}
					if e = charge(ops, 0); e != nil {
						return zero, e
					}
					entry.CoordinateRoots[i] = root
				}
				roots = append(roots, orderedContact{entry, len(roots)})
			}
			if proof != nil {
				proof.pieces[wi][pi] = record
			}
		}
	}
	if len(roots)%2 != 0 {
		return zero, geom.ErrConvergence
	}
	// A conservative endpoint sort is only a proposal. Reprove every
	// adjacent order, inserting by exact order if two brackets overlap.
	sort.SliceStable(roots, func(i, j int) bool {
		a, b := roots[i].CoordinateRoots[axis].Interval(), roots[j].CoordinateRoots[axis].Interval()
		return a.Lower.Cmp(b.Lower)*direction < 0
	})
	for i := 1; i < len(roots); i++ {
		for j := i; j > 0; j-- {
			order, e := roots[j-1].CoordinateRoots[axis].Compare(ctx, roots[j].CoordinateRoots[axis], limits())
			if e != nil {
				return zero, e
			}
			if e = charge(order.Operations, order.Subdivisions); e != nil {
				return zero, e
			}
			if !order.Proven || order.Order == 0 {
				return zero, geom.ErrConvergence
			}
			if order.Order*direction < 0 {
				break
			}
			roots[j-1], roots[j] = roots[j], roots[j-1]
		}
	}
	// Outside each bounded cap there is no material. Each regular boundary
	// crossing reverses the material side because the source ring forest
	// has alternating winding and pairwise disjoint boundaries.
	for i := 0; i < len(roots); i += 2 {
		if !roots[i].KeepBefore || roots[i+1].KeepBefore {
			return zero, geom.ErrConvergence
		}
		out.Material = append(out.Material, RationalGraphCapSeam{i, i + 1})
	}
	if proof != nil {
		proof.contactOrder = make([]int, len(roots))
	}
	for i, root := range roots {
		out.Contacts = append(out.Contacts, root.AlgebraicGraphCapContact)
		if proof != nil {
			proof.contactOrder[root.id] = i
		}
	}
	out.ExactOperations = opts.MaxWork - work
	out.Subdivisions = opts.MaxSubdivisions - subdiv
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}

package step

import (
	"fmt"
	"math/big"
	"slices"
)

func repairConicalCircularFrames(original, fixed *File, g conicalRepairGraph, budget *big.Rat, e *ConicalRimFilletBoreRepairEvidence) error {
	for _, fc := range g.faces {
		if fc.kind != "TOROIDAL_SURFACE" {
			continue
		}
		r, err := simple(original, fc.support, fc.kind, 4)
		if err != nil {
			return err
		}
		major, err := liftSourceRat(r.Args[2])
		if err != nil || major.Sign() <= 0 {
			return errConicalRimFilletBoreRepair
		}
		minor, err := liftSourceRat(r.Args[3])
		if err != nil || minor.Sign() <= 0 || minor.Cmp(major) >= 0 {
			return errConicalRimFilletBoreRepair
		}
		q, err := repairConicalCircularFrame(original, fixed, g, fc.support, r.Args[1], 0, new(big.Rat).Add(major, minor), budget)
		if err != nil {
			return err
		}
		if q != (ConicalRimFilletBoreFrameRepair{}) {
			e.Frames = append(e.Frames, q)
		}
	}
	for _, id := range conicalRepairEdgeOrder(g) {
		edge := g.edges[id]
		r, err := simple(original, edge.curve, "CIRCLE", 3)
		if err != nil {
			continue // Exact lines and the repaired cone seam are proved separately.
		}
		radius, err := liftSourceRat(r.Args[2])
		if err != nil || radius.Sign() <= 0 {
			return errConicalRimFilletBoreRepair
		}
		var meridian int64
		if edge.start != edge.end {
			u := g.uses[id]
			if u[0].face != u[1].face || g.faces[u[0].face].kind != "TOROIDAL_SURFACE" {
				return errConicalRimFilletBoreRepair
			}
			meridian = id
		}
		q, err := repairConicalCircularFrame(original, fixed, g, edge.curve, r.Args[1], meridian, radius, budget)
		if err != nil {
			return err
		}
		if q != (ConicalRimFilletBoreFrameRepair{}) {
			e.Frames = append(e.Frames, q)
		}
	}
	return nil
}

func repairConicalCircularFrame(original, fixed *File, g conicalRepairGraph, support int64, placement Value, edge int64, radius, budget *big.Rat) (ConicalRimFilletBoreFrameRepair, error) {
	var zero ConicalRimFilletBoreFrameRepair
	fail := errConicalRimFilletBoreRepair
	p, err := conicalRepairFrame(original, placement.Ref)
	if err != nil || placement.Kind != Reference || g.incoming[placement.Ref] != 1 {
		return zero, fail
	}
	normal, err := mappedRationalCoordinate(original, p[1], "DIRECTION", 3)
	if err != nil {
		return zero, err
	}
	reference, err := mappedRationalCoordinate(original, p[2], "DIRECTION", 3)
	if err != nil || reference[0].Sign() <= 0 || mappedRationalDot(normal, reference).Sign() != 0 {
		return zero, fail
	}
	axis := 2
	if edge != 0 {
		axis = 1
	}
	sign := normal[axis].Sign()
	if sign == 0 || (edge == 0 && !conicalExactDirection(normal, axis, sign)) {
		return zero, fail
	}
	dn := conicalDirectionBound(normal, axis)
	dx := conicalDirectionBound(reference, 0)
	// Y is the cross product of the normalized axes, so its error
	// is bounded by dn+dx. The resulting frame displacement bounds
	// the models at corresponding analytic angles.
	bound := new(big.Rat).Mul(big.NewRat(2, 1), dx)
	bound.Add(bound, dn).Mul(bound, radius)
	if edge != 0 {
		center, err := mappedRationalCoordinate(original, p[0], "CARTESIAN_POINT", 3)
		if err != nil {
			return zero, err
		}
		if err := verifyConicalMeridian(original, g.edges[edge], normal, axis, sign, center, radius, bound); err != nil {
			return zero, err
		}
		bound.Mul(bound, big.NewRat(3, 1))
	}
	if bound.Cmp(budget) > 0 {
		return zero, fmt.Errorf("%w: circular frame #%d exceeds the source-unit limit", fail, support)
	}
	claim := ConicalRimFilletBoreFrameRepair{SupportID: support, PlacementID: placement.Ref, EdgeID: edge, DisplacementBound: bound.RatString()}
	for i, id := range p[1:] {
		directionAxis, directionSign := axis, sign
		if i == 1 {
			directionAxis, directionSign = 0, 1
		}
		v := [2][3]*big.Rat{normal, reference}[i]
		if conicalExactDirection(v, directionAxis, directionSign) {
			continue
		}
		if g.incoming[id] != 1 {
			return zero, fail
		}
		claim.CorrectedDirectionIDs[i] = id
		r, err := ovalRecord(original, id, "DIRECTION", 2)
		if err != nil {
			return zero, err
		}
		r.Args[1] = conicalCardinalDirection(directionAxis, directionSign)
		ovalSet(fixed, id, r)
	}
	if claim.CorrectedDirectionIDs == ([2]int64{}) {
		return zero, nil
	}
	return claim, nil
}

// Normalizing v before comparing it to its signed coordinate direction
// costs at most twice the norm of the ratio of off-axis to axial components.
func conicalDirectionBound(v [3]*big.Rat, axis int) *big.Rat {
	out := new(big.Rat)
	for i, x := range v {
		if i != axis {
			out.Add(out, new(big.Rat).Abs(x))
		}
	}
	return out.Mul(out, big.NewRat(2, 1)).Quo(out, new(big.Rat).Abs(v[axis]))
}

func verifyConicalMeridian(f *File, edge conicalRepairEdge, normal [3]*big.Rat, axis, sign int, center [3]*big.Rat, radius, frameBound *big.Rat) error {
	fail := errConicalRimFilletBoreRepair
	if !edge.sense || frameBound.Cmp(new(big.Rat).Quo(radius, big.NewRat(16, 1))) >= 0 {
		return fail
	}
	rr := new(big.Rat).Mul(radius, radius)
	var ends [2][3]*big.Rat
	for i, id := range [2]int64{edge.start, edge.end} {
		_, p, err := conicalRepairVertex(f, id)
		if err != nil {
			return err
		}
		for j := range p {
			ends[i][j] = new(big.Rat).Sub(p[j], center[j])
		}
		if ends[i][axis].Sign() != 0 || mappedRationalDot(ends[i], ends[i]).Cmp(rr) != 0 {
			return fail
		}
	}
	cross := mappedRationalCross(ends[0], ends[1])
	// Orthogonal projection to the raw circle's plane preserves the
	// directed cross sign: projecting either vector changes its cross
	// only by terms with zero dot product against the plane normal.
	if cross[axis].Sign() != sign || mappedRationalDot(cross, normal).Sign() <= 0 {
		return fail
	}
	// Each original endpoint is within frameBound of the raw circle at
	// the corresponding angle. Its closest point on the raw circle
	// exists uniquely (that distance is less than the radius), and
	// differs from the corresponding-angle point by at most twice the
	// bound. The two positive short arcs have sweeps in (0, pi). Both
	// endpoint phase shifts are less than pi/2 by the chord bound, so
	// their difference is the shift of those short sweeps without any
	// turn. Interpolating phase shifts along the arc keeps their chord
	// bound. Adding the original frame bound yields three times it.
	return nil
}

func conicalCardinalDirection(axis, sign int) Value {
	xyz := []Value{{Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}}
	xyz[axis].Text = "1."
	if sign < 0 {
		xyz[axis].Text = "-1."
	}
	return Value{Kind: Aggregate, Items: xyz}
}

func repairLowerConicalToroidBound(original, fixed *File, g conicalRepairGraph, e *ConicalRimFilletBoreRepairEvidence) error {
	fail := errConicalRimFilletBoreRepair
	var conflicts []int64
	owner := -1
	for _, id := range conicalRepairEdgeOrder(g) {
		uses := g.uses[id]
		if uses[0].sense != uses[1].sense {
			continue
		}
		edge := g.edges[id]
		if edge.start != edge.end || uses[0].face == uses[1].face {
			return fail
		}
		selected := -1
		for _, use := range uses {
			fc := g.faces[use.face]
			if fc.kind != "TOROIDAL_SURFACE" {
				continue
			}
			r, err := simple(original, fc.support, fc.kind, 4)
			if err != nil || r.Args[1].Kind != Reference {
				return fail
			}
			_, axis, err := boreRimSTEPFrame(fixed, r.Args[1].Ref)
			if err != nil || axis.Z != -1 || selected != -1 {
				return fail
			}
			selected = use.face
		}
		if selected == -1 || (owner != -1 && owner != selected) {
			return fail
		}
		owner = selected
		conflicts = append(conflicts, id)
	}
	if len(conflicts) != 2 || !slices.Contains(conflicts, e.ConeRimEdgeIDs[0]) {
		return fail
	}
	fc := g.faces[owner]
	if !fc.sense || len(fc.bounds) != 1 || len(fc.loops) != 1 || len(fc.loops[0]) != 4 {
		return fail
	}
	// The two conflicts must be the lower toroid's complete cone rim and
	// cap rim. Its repeated meridian must already have opposite uses.
	for i, id := range conflicts {
		kind := "PLANE"
		if id == e.ConeRimEdgeIDs[0] {
			kind = "CONICAL_SURFACE"
		}
		uses := g.uses[id]
		for j, u := range uses {
			if u.face != owner && g.faces[u.face].kind != kind {
				return fail
			}
			e.OriginalRimOwnerFaceIDs[i][j] = g.faces[u.face].id
		}
		e.OriginalRimUseSenses[i] = uses[0].sense
	}
	bound := fc.bounds[0]
	r, err := ovalRecord(original, bound, "FACE_BOUND", 3)
	if err != nil || r.Args[2].Kind != Enumeration || r.Args[2].Text != "F" {
		return fail
	}
	r.Args[2] = Value{Kind: Enumeration, Text: "T"}
	ovalSet(fixed, bound, r)
	e.ReversedFaceID, e.ReversedBoundID = fc.id, bound
	copy(e.ReversedRimEdgeIDs[:], conflicts)
	return nil
}

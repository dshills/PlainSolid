package iges

import (
	"fmt"
	"math/big"
	"slices"
	"strconv"
)

func repairIGESConicalOccurrences(p *igesBoreRimSource, g igesConicalRepairGraph, edits map[int]topologyRecord, budget *big.Rat, evidence *ConicalRimFilletBoreRepairEvidence) error {
	for i, edge := range g.edges {
		rec := p.records[edge.curve]
		if rec.kind != 100 {
			continue // The strict graph independently checks unmodified lines.
		}
		claim, edit, err := repairIGESConicalArc(p, edge, i+1, budget)
		if err != nil {
			return err
		}
		if edit.id != 0 {
			if g.transformRefs[edit.id] != 1 {
				return errIGESConicalRimFilletBoreRepair
			}
			edits[edit.id] = edit
			evidence.ArcOccurrences = append(evidence.ArcOccurrences, claim)
		}
	}
	for _, fc := range g.faces {
		if fc.kind != 198 {
			continue
		}
		claim, edit, err := repairIGESConicalTorus(p, fc, budget)
		if err != nil {
			return err
		}
		if edit.id != 0 {
			if g.supportRefs[edit.id] != 1 {
				return errIGESConicalRimFilletBoreRepair
			}
			edits[edit.id] = edit
			evidence.TorusOccurrences = append(evidence.TorusOccurrences, claim)
		}
	}
	return nil
}

func repairIGESConicalArc(p *igesBoreRimSource, edge igesConicalRepairEdge, index int, budget *big.Rat) (ConicalRimFilletBoreArcRepair, topologyRecord, error) {
	var claim ConicalRimFilletBoreArcRepair
	var edit topologyRecord
	fail := errIGESConicalRimFilletBoreRepair
	rec, err := p.entity(edge.curve, 100, true)
	if err != nil || len(rec.values) != 8 {
		return claim, edit, fail
	}
	c, err := igesPlanarCoordinates(rec.values[1:4])
	if err != nil || !c.equal(igesConicalIntVector(0, 0, 0)) {
		return claim, edit, fail
	}
	var ends [2]igesPlanarVec
	for i := range ends {
		v, err := igesPlanarCoordinates([]parameter{rec.values[4+2*i], rec.values[5+2*i], rec.values[1]})
		if err != nil || v.dot(v).Sign() <= 0 {
			return claim, edit, fail
		}
		ends[i] = v
	}
	if ends[0].dot(ends[0]).Cmp(ends[1].dot(ends[1])) != 0 {
		return claim, edit, fail
	}
	meridian := edge.start != edge.end
	if meridian {
		if edge.uses[0].face != edge.uses[1].face || edge.uses[0].supportKind != 198 ||
			ends[0].cross(ends[1])[2].Sign() <= 0 {
			return claim, edit, fail
		}
	} else if !ends[0].equal(ends[1]) {
		return claim, edit, fail
	}
	raw, err := p.entity(rec.transform, 124, false)
	if err != nil || len(raw.values) != 13 {
		return claim, edit, fail
	}
	cardinal, phase, err := igesConicalArcMatrix(raw, meridian)
	if err != nil {
		return claim, edit, err
	}
	if phase.Sign() == 0 {
		return claim, edit, nil
	}
	// The source and repaired charts use the same finite directed local
	// arc. Its local coordinates never exceed the radius, which in turn
	// is bounded by the L1 norm of either exact endpoint.
	radiusBound := igesConicalArcRadiusBound(ends)
	bound := new(big.Rat).Mul(radiusBound, new(big.Rat).Abs(phase))
	if !meridian {
		bound.Mul(bound, big.NewRat(2, 1))
	}
	if bound.Cmp(budget) > 0 {
		return claim, edit, fmt.Errorf("%w: type 124 arc occurrence #%d exceeds source-unit limit", fail, raw.id)
	}
	edit = raw
	edit.values = slices.Clone(raw.values)
	for i, row := range cardinal {
		for j, v := range row {
			edit.values[1+4*i+j] = parameter{text: strconv.FormatInt(v, 10)}
		}
	}
	return ConicalRimFilletBoreArcRepair{CurveID: edge.curve, TransformID: raw.id, EdgeIndex: index,
		FaceIDs: [2]int{edge.uses[0].face, edge.uses[1].face}, Meridian: meridian, DisplacementBound: bound.RatString()}, edit, nil
}

// The two source matrices are a signed cardinal frame followed by the
// original azimuth block [1 e; -e 1]. Translation and the exact coordinate
// entries must already agree. The plane of a vertical type 100 chart is
// local Z=0, so only its X column contributes to the phase displacement.
func igesConicalArcMatrix(frame topologyRecord, meridian bool) ([3][3]int64, *big.Rat, error) {
	fail := errIGESConicalRimFilletBoreRepair
	identity := [3][3]int64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
	cardinal := identity
	if meridian {
		y, err := igesConicalRepairNumber(frame.values[10])
		if err != nil || y.Cmp(big.NewRat(-1, 1)) != 0 && y.Cmp(big.NewRat(1, 1)) != 0 {
			return cardinal, nil, fail
		}
		s := int64(y.Sign())
		cardinal = [3][3]int64{{1, 0, 0}, {0, 0, -s}, {0, s, 0}}
	}
	phase, err := igesConicalRepairNumber(frame.values[5])
	if err != nil {
		return cardinal, nil, err
	}
	phase.Neg(phase)
	for i, row := range cardinal {
		for j, q := range row {
			got, err := igesConicalRepairNumber(frame.values[1+4*i+j])
			if err != nil {
				return cardinal, nil, err
			}
			want := big.NewRat(q, 1)
			switch {
			case i == 1 && j == 0:
				want.Neg(phase)
			case i == 0 && j > 0:
				want.Mul(phase, big.NewRat(cardinal[1][j], 1))
			}
			if got.Cmp(want) != 0 {
				return cardinal, nil, fail
			}
		}
	}
	return cardinal, phase, nil
}

func igesConicalArcRadiusBound(ends [2]igesPlanarVec) *big.Rat {
	l1 := func(v igesPlanarVec) *big.Rat {
		return new(big.Rat).Add(new(big.Rat).Abs(v[0]), new(big.Rat).Abs(v[1]))
	}
	a, b := l1(ends[0]), l1(ends[1])
	if a.Cmp(b) > 0 {
		return b
	}
	return a
}

func repairIGESConicalTorus(p *igesBoreRimSource, fc igesConicalRepairFace, budget *big.Rat) (ConicalRimFilletBoreTorusRepair, topologyRecord, error) {
	var claim ConicalRimFilletBoreTorusRepair
	var edit topologyRecord
	fail := errIGESConicalRimFilletBoreRepair
	torus := p.records[fc.support]
	ref, err := intValue(torus.values[5])
	if err != nil {
		return claim, edit, err
	}
	v, err := readIGESConicalCoordinate(p, ref, 123)
	if err != nil || v[0].Cmp(big.NewRat(1, 1)) != 0 || v[2].Sign() != 0 {
		return claim, edit, fail
	}
	if v[1].Sign() == 0 {
		return claim, edit, nil
	}
	ra, err := igesConicalRepairNumber(torus.values[3])
	if err != nil || ra.Sign() <= 0 {
		return claim, edit, fail
	}
	rb, err := igesConicalRepairNumber(torus.values[4])
	if err != nil || rb.Sign() <= 0 || rb.Cmp(ra) >= 0 {
		return claim, edit, fail
	}
	// Normalizing the phase vector displaces +X by at most twice its
	// off-axis magnitude. The orthogonal second chart axis has the same
	// bound. At every angle the radial coefficient is at most R+r.
	bound := new(big.Rat).Mul(new(big.Rat).Add(ra, rb), new(big.Rat).Abs(v[1]))
	bound.Mul(bound, big.NewRat(4, 1))
	if bound.Cmp(budget) > 0 {
		return claim, edit, fmt.Errorf("%w: type 198 phase occurrence #%d exceeds source-unit limit", fail, torus.id)
	}
	edit = p.records[ref]
	edit.values = slices.Clone(edit.values)
	edit.values[2] = parameter{text: "0."}
	return ConicalRimFilletBoreTorusRepair{fc.id, fc.support, ref, bound.RatString()}, edit, nil
}

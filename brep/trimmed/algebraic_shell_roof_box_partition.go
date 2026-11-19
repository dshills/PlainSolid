//go:build research

package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// RoofBoxVerticalStation classifies an exact line station. SourceHeights
// records original floor and roof faces at this parameter. These remain
// original incidences even when regularization removes a coincident face.
// A clip endpoint is included regardless of whether it lies on a height.
type RoofBoxVerticalStation struct {
	Parameter     *big.Rat
	Point         MixedRoofMaterialPoint
	SourceHeights []MixedRoofFaceOwner
	ClipEndpoint  bool
}

// RoofBoxVerticalSpan classifies the open interval between adjacent stations
// in the partition. Endpoint classification is reported independently.
type RoofBoxVerticalSpan struct {
	Start, End int
	Location   parametric.Location
	Operands   [2]parametric.Location
}

// RoofBoxVerticalPartition covers the entire requested finite parameter range.
// It includes exterior, interior and boundary spans and classifies isolated
// contacts in the exact station ledger.
type RoofBoxVerticalPartition struct {
	Stations        []RoofBoxVerticalStation
	Spans           []RoofBoxVerticalSpan
	ExactOperations int
}

// PartitionVerticalRational classifies (xy, originZ+directionZ*t) on the
// closed clip range using the original roof and floor heights. Restricting to
// a vertical line makes the height order constant on every open gap: no
// other station can change the local footprint-sector occupancy. The query
// applies the same regularization and fail-closed tangent policy as Locate.
func (r *RoofBoxPointQuery) PartitionVerticalRational(ctx context.Context, xy [2]*big.Rat, originZ, directionZ, minT, maxT *big.Rat, o geom.SurfacePatchEvaluationOptions) (RoofBoxVerticalPartition, error) {
	var zero RoofBoxVerticalPartition
	if r == nil || r.material == nil {
		return zero, geom.ErrInvalid
	}
	q := r.material
	o, err := q.evaluationOptions(ctx, o, xy[0], xy[1], originZ, directionZ, minT, maxT)
	if err != nil {
		return zero, err
	}
	if directionZ.Sign() == 0 || minT.Cmp(maxT) > 0 {
		return zero, geom.ErrInvalid
	}
	w := mixedRoofLineWork{o: MixedRoofLineOptions{SurfacePatchEvaluationOptions: o}}
	cuts := []RoofBoxVerticalStation{{Parameter: new(big.Rat).Set(minT), ClipEndpoint: true}}
	if maxT.Cmp(minT) != 0 {
		cuts = append(cuts, RoofBoxVerticalStation{Parameter: new(big.Rat).Set(maxT), ClipEndpoint: true})
	}
	var footprint [2]parametric.Location
	var tops [2]*big.Rat
	for i, b := range q.box {
		footprint[i] = b.footprint(xy)
		if footprint[i] == parametric.Outside {
			continue
		}
		local := o
		local.MaxOperations -= w.operations
		if local.MaxOperations < 1 {
			return zero, geom.ErrConvergence
		}
		roof, err := b.roof.EvaluateRational(ctx, xy[0], xy[1], local)
		if err != nil {
			return zero, err
		}
		w.operations += roof.ExactOperations
		tops[i] = roof.Point[2]
		for j, h := range [2]*big.Rat{b.floor, tops[i]} {
			t, err := w.rat(new(big.Rat).Sub(h, originZ))
			if err != nil {
				return zero, err
			}
			if _, err := w.rat(t.Quo(t, directionZ)); err != nil {
				return zero, err
			}
			if t.Cmp(minT) < 0 || t.Cmp(maxT) > 0 {
				continue
			}
			face := MixedRoofFaceOwner{i, [2]int{b.floorFace, b.roofFace}[j]}
			k := 0
			for ; k < len(cuts); k++ {
				if cuts[k].Parameter.Cmp(t) == 0 {
					break
				}
			}
			if k == len(cuts) {
				cuts = append(cuts, RoofBoxVerticalStation{Parameter: t})
			}
			cuts[k].SourceHeights = append(cuts[k].SourceHeights, face)
		}
	}
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].Parameter.Cmp(cuts[j].Parameter) < 0 })
	locate := func(t *big.Rat) (MixedRoofMaterialPoint, error) {
		z, err := w.rat(new(big.Rat).Mul(directionZ, t))
		if err != nil {
			return MixedRoofMaterialPoint{}, err
		}
		if _, err := w.rat(z.Add(z, originZ)); err != nil {
			return MixedRoofMaterialPoint{}, err
		}
		var out MixedRoofMaterialPoint
		var levels [2]int
		for i, b := range q.box {
			if tops[i] == nil {
				continue
			}
			floor, roof := z.Cmp(b.floor), z.Cmp(tops[i])
			if floor < 0 || roof > 0 {
				continue
			}
			if floor == 0 {
				levels[i] = 1
			} else if roof == 0 {
				levels[i] = -1
			}
			out.Operands[i] = footprint[i]
			if levels[i] != 0 {
				out.Operands[i] = parametric.Boundary
			}
		}
		return q.regularizePoint(ctx, [3]*big.Rat{xy[0], xy[1], z}, out, levels, &w)
	}
	out := RoofBoxVerticalPartition{Stations: cuts}
	for i := range cuts {
		point, err := locate(cuts[i].Parameter)
		if err != nil {
			return zero, err
		}
		cuts[i].Point = point
		if i == 0 {
			continue
		}
		mid, err := w.rat(new(big.Rat).Add(cuts[i-1].Parameter, cuts[i].Parameter))
		if err != nil {
			return zero, err
		}
		if _, err := w.rat(mid.Quo(mid, big.NewRat(2, 1))); err != nil {
			return zero, err
		}
		gap, err := locate(mid)
		if err != nil {
			return zero, err
		}
		out.Spans = append(out.Spans, RoofBoxVerticalSpan{i - 1, i, gap.Location, gap.Operands})
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	out.ExactOperations = w.operations
	return out, nil
}

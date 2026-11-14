package step

import (
	"bytes"
	"io"
	"math/big"
)

// ExactNAUOPlacement specifies a child-to-parent proper rigid pose for an
// original NAUO use. The rational origin and raw perpendicular X/Z ratios are
// copied. No caller rational is retained. The normalized axes must be rational.
type ExactNAUOPlacement struct {
	NAUOID               int64
	Origin, XAxis, ZAxis [3]*big.Rat
}

// WriteExactDAGNAUORootPlacements edits a nonempty subset of original root uses.
// It retains every source ID and all records except the target datum's three
// point/direction records. Those records and the target datum must be exclusively
// owned by that root use and representation; shared targets fail. The original
// from frame is retained, so the new target is desiredPose composed with from.
// No rounding, entity cloning, geometry changes or implicit shared-use edits occur.
func WriteExactDAGNAUORootPlacements(w io.Writer, source *File, placements []ExactNAUOPlacement, l Limits) error {
	if w == nil || source == nil || len(placements) == 0 || len(placements) > 32 {
		return mappedPlanarFailure()
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	a, e := ReadExactDAGPlanarNAUO(bytes.NewReader(raw), l)
	if e != nil {
		return e
	}
	f := a.Source
	known := map[int64]ExactNAUOOccurrence{}
	for _, u := range a.Relationships {
		if u.ParentID == a.Root.DefinitionID {
			known[u.NAUOID] = u
		}
	}
	// Count each original reference, not just each distinct referring entity.
	incoming := map[int64]map[int64]int{}
	for id, en := range f.Entities {
		var visit func(Value)
		visit = func(v Value) {
			if v.Kind == Reference {
				if incoming[v.Ref] == nil {
					incoming[v.Ref] = map[int64]int{}
				}
				incoming[v.Ref][id]++
			}
			for _, x := range v.Items {
				visit(x)
			}
		}
		for _, r := range en.Records {
			for _, v := range r.Args {
				visit(v)
			}
		}
	}
	wanted := map[int64]mappedRationalBasis{}
	for _, p := range placements {
		u, ok := known[p.NAUOID]
		if !ok {
			return mappedPlanarFailure()
		}
		delete(known, p.NAUOID)
		pose, e := mappedRationalPose(RationalMappedPlacement{Origin: p.Origin, XAxis: p.XAxis, ZAxis: p.ZAxis})
		if e != nil {
			return e
		}
		wanted[p.NAUOID] = pose
		from, e := mappedRationalFrame(f, u.FromPlacementID)
		if e != nil {
			return e
		}
		target := composeMappedPose(pose, from)
		users := incoming[u.ToPlacementID]
		if len(users) != 2 || users[u.TransformationID] != 1 || users[a.Root.Representations[0]] != 1 {
			return mappedPlanarFailure()
		}
		frame, e := simple(f, u.ToPlacementID, "AXIS2_PLACEMENT_3D", 4)
		if e != nil {
			return e
		}
		for _, slot := range []struct {
			index int
			kind  string
			value [3]*big.Rat
		}{{1, "CARTESIAN_POINT", target.Origin}, {2, "DIRECTION", target.Z}, {3, "DIRECTION", target.X}} {
			id := frame.Args[slot.index].Ref
			if len(incoming[id]) != 1 || incoming[id][u.ToPlacementID] != 1 {
				return mappedPlanarFailure()
			}
			rec, e := simple(f, id, slot.kind, 2)
			if e != nil {
				return e
			}
			values := slot.value
			if slot.kind == "DIRECTION" {
				// STEP directions are ratios. A common denominator gives exact finite
				// integer tokens even when normalized unit components contain thirds.
				denominator := big.NewInt(1)
				for _, q := range values {
					g := new(big.Int).GCD(nil, nil, denominator, q.Denom())
					denominator.Mul(new(big.Int).Quo(denominator, g), q.Denom())
				}
				for i, q := range values {
					values[i] = new(big.Rat).Mul(q, new(big.Rat).SetInt(denominator))
				}
			}
			items := make([]Value, 3)
			for i, q := range values {
				token, e := mappedTerminatingDecimal(q)
				if e != nil {
					return e
				}
				items[i] = Value{Kind: Number, Text: token}
			}
			rec.Args[1] = Value{Kind: Aggregate, Items: items}
			en := f.Entities[id]
			en.Records = []Record{rec}
			f.Entities[id] = en
		}
	}
	raw, e = mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	fresh, e := ReadExactDAGPlanarNAUO(bytes.NewReader(raw), l)
	if e != nil {
		return e
	}
	for _, u := range fresh.Relationships {
		p, ok := wanted[u.NAUOID]
		if !ok {
			continue
		}
		for i := 0; i < 3; i++ {
			if u.pose.Origin[i].Cmp(p.Origin[i]) != 0 || u.pose.X[i].Cmp(p.X[i]) != 0 || u.pose.Y[i].Cmp(p.Y[i]) != 0 || u.pose.Z[i].Cmp(p.Z[i]) != 0 {
				return mappedPlanarFailure()
			}
		}
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}

package step

import (
	"errors"
	"math/big"
)

// Normalize only polynomial unit isoparametric maps. Exact Greville coefficient
// identities prove the original B-spline is (t,c) or (c,t), not sampled collinearity.
func normalizeWrappedIsoSplines(f *File, surface int64, maxPoles int) error {
	fail := func() error { return errors.New("step: unsupported exact affine spline pcurve") }
	support, e := DecodeBSplineSurface(f, surface, maxPoles)
	if e != nil {
		return e
	}
	u0, u1, v0, v1 := support.Geometry.Domain()
	next := int64(-1)
	for id := range f.Entities {
		if id <= next {
			next = id - 1
		}
	}
	emit := func(kind string, args ...Value) int64 {
		id := next
		next--
		f.Entities[id] = Entity{ID: id, Records: []Record{{Type: kind, Args: args}}}
		return id
	}
	str := Value{Kind: String}
	num := func(x float64) Value { return Value{Kind: Number, Text: real(x)} }
	refv := func(id int64) Value { return Value{Kind: Reference, Ref: id} }
	for _, id := range f.Order {
		ent := f.Entities[id]
		if len(ent.Records) != 1 || ent.Records[0].Type != "PCURVE" {
			continue
		}
		pc, e := simple(f, id, "PCURVE", 3)
		if e != nil {
			return fail()
		}
		owner, _ := ref(pc.Args[1])
		if owner != surface {
			continue
		}
		rid, _ := ref(pc.Args[2])
		rep, e := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
		if e != nil {
			return fail()
		}
		ids, e := listRefs(rep.Args[1], 1)
		if e != nil || len(ids) != 1 {
			return fail()
		}
		cid := ids[0]
		if _, e = simple(f, cid, "LINE", 3); e == nil {
			continue
		}
		curve, e := simple(f, cid, "B_SPLINE_CURVE_WITH_KNOTS", 9)
		if e != nil {
			return fail()
		}
		if curve.Args[4].Kind != Enumeration || curve.Args[4].Text != "F" || curve.Args[5].Kind != Enumeration || curve.Args[5].Text != "F" {
			return fail()
		}
		degree, e := integer(curve.Args[1])
		if e != nil || degree < 1 || degree > 8 {
			return fail()
		}
		pids, e := listRefs(curve.Args[2], 2)
		if e != nil || len(pids) > maxPoles {
			return fail()
		}
		knots, e := expandSurfaceKnots(curve.Args[6], curve.Args[7], int(degree), len(pids))
		if e != nil {
			return fail()
		}
		points := make([][2]float64, len(pids))
		for i, pid := range pids {
			p, e := simple(f, pid, "CARTESIAN_POINT", 2)
			if e != nil || p.Args[1].Kind != Aggregate || len(p.Args[1].Items) != 2 {
				return fail()
			}
			for k := 0; k < 2; k++ {
				points[i][k], e = numeric(p.Args[1].Items[k])
				if e != nil {
					return fail()
				}
			}
		}
		axis := -1
		fixed := 0.
		for candidate := 0; candidate < 2; candidate++ {
			ok := true
			constant := points[0][1-candidate]
			for i, p := range points {
				g := new(big.Rat)
				for k := 1; k <= int(degree); k++ {
					g.Add(g, new(big.Rat).SetFloat64(knots[i+k]))
				}
				g.Quo(g, big.NewRat(int64(degree), 1))
				if g.Cmp(new(big.Rat).SetFloat64(p[candidate])) != 0 || p[1-candidate] != constant {
					ok = false
					break
				}
			}
			if ok {
				if axis != -1 {
					return fail()
				}
				axis = candidate
				fixed = constant
			}
		}
		if axis < 0 {
			return fail()
		}
		lo, hi := u0, u1
		if axis == 1 {
			lo, hi = v0, v1
		}
		if knots[degree] != lo || knots[len(pids)] != hi {
			return fail()
		}
		p := [2]float64{}
		d := [2]float64{}
		p[1-axis] = fixed
		d[axis] = 1
		pid := emit("CARTESIAN_POINT", str, Value{Kind: Aggregate, Items: []Value{num(p[0]), num(p[1])}})
		did := emit("DIRECTION", str, Value{Kind: Aggregate, Items: []Value{num(d[0]), num(d[1])}})
		vid := emit("VECTOR", str, refv(did), num(1))
		f.Entities[cid] = Entity{ID: cid, Records: []Record{{Type: "LINE", Args: []Value{str, refv(pid), refv(vid)}}}}
	}
	return nil
}

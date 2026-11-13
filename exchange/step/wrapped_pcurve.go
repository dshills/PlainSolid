package step

import "errors"

// The analytic codec allows geometric tolerances; certified wrapped spline
// identity instead requires an exact unit affine UV parameter mapping.
func wrappedIsoPCurve(f *File, id, surface int64, axis int) (float64, error) {
	fixed, e := torusPCurve(f, id, surface, axis)
	if e != nil {
		return 0, e
	}
	fail := func() (float64, error) { return 0, errors.New("step: inexact wrapped UV parameter mapping") }
	pc, _ := simple(f, id, "PCURVE", 3)
	rid, _ := ref(pc.Args[2])
	rep, _ := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
	ids, _ := listRefs(rep.Args[1], 1)
	line, _ := simple(f, ids[0], "LINE", 3)
	pid, _ := ref(line.Args[1])
	vid, _ := ref(line.Args[2])
	vector, _ := simple(f, vid, "VECTOR", 3)
	m, e := numeric(vector.Args[2])
	if e != nil || m != 1 {
		return fail()
	}
	did, _ := ref(vector.Args[1])
	point, e := simple(f, pid, "CARTESIAN_POINT", 2)
	if e != nil {
		return fail()
	}
	direction, e := simple(f, did, "DIRECTION", 2)
	if e != nil {
		return fail()
	}
	if point.Args[1].Kind != Aggregate || direction.Args[1].Kind != Aggregate || len(point.Args[1].Items) != 2 || len(direction.Args[1].Items) != 2 {
		return fail()
	}
	for k := 0; k < 2; k++ {
		p, e := numeric(point.Args[1].Items[k])
		if e != nil {
			return fail()
		}
		d, e := numeric(direction.Args[1].Items[k])
		if e != nil {
			return fail()
		}
		if k == axis {
			if p != 0 || d != 1 {
				return fail()
			}
		} else if p != fixed || d != 0 {
			return fail()
		}
	}
	return fixed, nil
}

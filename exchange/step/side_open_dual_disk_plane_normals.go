package step

import "fmt"

// Model-only loops are unchanged in 3D. Reversing the exact plane normal and
// face same_sense together preserves both the supporting set and material normal.
// There are no owner pcurves whose UV coordinates could be silently reinterpreted.
func sideDiskNormalizeOppositePlanes(f, expected *File, faces []int64) ([]int64, error) {
	fail := func() ([]int64, error) { return nil, fmt.Errorf("step: unsupported side-disk plane normal reversal") }
	var bodyID int64
	for id, en := range expected.Entities {
		if len(en.Records) == 1 && en.Records[0].Type == "MANIFOLD_SOLID_BREP" {
			if bodyID != 0 {
				return fail()
			}
			bodyID = id
		}
	}
	body, e := simple(expected, bodyID, "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return fail()
	}
	shellID, e := ref(body.Args[1])
	if e != nil {
		return fail()
	}
	shell, e := simple(expected, shellID, "CLOSED_SHELL", 2)
	if e != nil {
		return fail()
	}
	efaces, e := listRefs(shell.Args[1], 1)
	if e != nil || len(efaces) != len(faces) || len(faces) > 32 {
		return fail()
	}
	counts := map[int64]int{}
	for _, en := range f.Entities {
		var values []Value
		for _, r := range en.Records {
			values = append(values, r.Args...)
		}
		for len(values) > 0 {
			v := values[len(values)-1]
			values = values[:len(values)-1]
			if v.Kind == Reference {
				counts[v.Ref]++
			}
			values = append(values, v.Items...)
		}
	}
	var normalized []int64
	for i, id := range faces {
		face, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		sid, e := ref(face.Args[2])
		if e != nil {
			return fail()
		}
		se := f.Entities[sid]
		if len(se.Records) != 1 || se.Records[0].Type != "PLANE" {
			continue
		}
		plane, e := simple(f, sid, "PLANE", 2)
		if e != nil {
			return fail()
		}
		pid, e := ref(plane.Args[1])
		if e != nil {
			return fail()
		}
		frame, e := sideDiskReadFrame(f, pid)
		if e != nil {
			return fail()
		}
		ef, e := simple(expected, efaces[i], "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		esid, e := ref(ef.Args[2])
		if e != nil {
			return fail()
		}
		ep, e := simple(expected, esid, "PLANE", 2)
		if e != nil {
			return fail()
		}
		epid, e := ref(ep.Args[1])
		if e != nil {
			return fail()
		}
		canonical, e := sideDiskReadFrame(expected, epid)
		if e != nil {
			return fail()
		}
		if frame.ZAxis() == canonical.ZAxis() {
			continue
		}
		// With the same origin and X axis, opposite Z implies opposite evaluated Y:
		// P_source(u,v)=P_canonical(u,-v). This exact affine identity preserves the
		// entire plane, not merely the boundary samples.
		if frame.ZAxis() != canonical.ZAxis().Scale(-1) || frame.Origin() != canonical.Origin() || frame.XAxis() != canonical.XAxis() || frame.YAxis() != canonical.YAxis().Scale(-1) {
			return fail()
		}
		same, e := boolean(face.Args[3])
		if e != nil {
			return fail()
		}
		want, e := boolean(ef.Args[3])
		if e != nil || same == want {
			return fail()
		}
		placement, e := simple(f, pid, "AXIS2_PLACEMENT_3D", 4)
		if e != nil {
			return fail()
		}
		normalID, e := ref(placement.Args[2])
		if e != nil {
			return fail()
		}
		if counts[sid] != 1 || counts[pid] != 1 || counts[normalID] != 1 || counts[id] != 1 {
			return fail()
		}
		normal, e := simple(f, normalID, "DIRECTION", 2)
		if e != nil {
			return fail()
		}
		z := canonical.ZAxis()
		normal.Args[1].Items = []Value{{Kind: Number, Text: real(z.X)}, {Kind: Number, Text: real(z.Y)}, {Kind: Number, Text: real(z.Z)}}
		face.Args[3] = Value{Kind: Enumeration, Text: "F"}
		if want {
			face.Args[3].Text = "T"
		}
		normalized = append(normalized, id)
	}
	return normalized, nil
}

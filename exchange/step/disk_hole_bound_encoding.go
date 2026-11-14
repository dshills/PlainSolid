package step

import (
	"bytes"
	"io"
)

// DiskHoleBoundEncoding retains the actual input graph and the exact encoding
// changes needed to enter the existing canonical disk-hole proof. No coordinate,
// support, model curve or pcurve is modified.
type DiskHoleBoundEncoding struct {
	Model                DiskHoleExtrusionModel
	Source               *File
	ReversedBoundIDs     []int64
	GenericOuterBoundIDs []int64
}

// ReadDiskHoleBoundEncoding is a separate opt-in representation adapter. A false
// bound orientation is normalized by reversing its complete ordered coedge list
// and every coedge orientation. A generic first FACE_BOUND may occupy the outer
// slot only after the whole reconstructed material graph proves that assignment.
// Bounds, loops and coedge records must be exclusively owned. Wrong partial
// reversals, rounded vendor supports and every other graph variation fail.
func ReadDiskHoleBoundEncoding(r io.Reader, l Limits) (DiskHoleBoundEncoding, error) {
	fail := func() (DiskHoleBoundEncoding, error) { return DiskHoleBoundEncoding{}, diskAssemblyFailure() }
	f, _, _, _, e := parseCanonicalCircularExtrusions(r, l)
	if e != nil {
		return DiskHoleBoundEncoding{}, e
	}
	source := f
	f, e = Parse(bytes.NewReader(serializeWrappedProofFile(f)), l)
	if e != nil {
		return fail()
	}
	counts := map[int64]int{}
	var visit func(Value)
	visit = func(v Value) {
		if v.Kind == Reference {
			counts[v.Ref]++
		}
		for _, x := range v.Items {
			visit(x)
		}
	}
	for _, ent := range f.Entities {
		for _, rec := range ent.Records {
			for _, v := range rec.Args {
				visit(v)
			}
		}
	}
	out := DiskHoleBoundEncoding{Source: source}
	for _, id := range f.Order {
		ent := f.Entities[id]
		if len(ent.Records) != 1 || ent.Records[0].Type != "ADVANCED_FACE" {
			continue
		}
		face, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		bounds, e := listRefs(face.Args[1], 1)
		if e != nil {
			return fail()
		}
		for index, bid := range bounds {
			bound := f.Entities[bid]
			if len(bound.Records) != 1 || counts[bid] != 1 {
				return fail()
			}
			b := bound.Records[0]
			if (b.Type != "FACE_BOUND" && b.Type != "FACE_OUTER_BOUND") || len(b.Args) != 3 {
				return fail()
			}
			if index == 0 && b.Type == "FACE_BOUND" {
				b.Type = "FACE_OUTER_BOUND"
				out.GenericOuterBoundIDs = append(out.GenericOuterBoundIDs, bid)
			}
			orientation, e := boolean(b.Args[2])
			if e != nil {
				return fail()
			}
			lid, e := ref(b.Args[1])
			if e != nil || counts[lid] != 1 {
				return fail()
			}
			loop, e := simple(f, lid, "EDGE_LOOP", 2)
			if e != nil {
				return fail()
			}
			uses, e := listRefs(loop.Args[1], 1)
			if e != nil {
				return fail()
			}
			for _, uid := range uses {
				if counts[uid] != 1 {
					return fail()
				}
				if _, e := simple(f, uid, "ORIENTED_EDGE", 5); e != nil {
					return fail()
				}
			}
			if !orientation {
				for _, uid := range uses {
					use := f.Entities[uid]
					rec := use.Records[0]
					sense, e := boolean(rec.Args[4])
					if e != nil {
						return fail()
					}
					value := "T"
					if sense {
						value = "F"
					}
					rec.Args[4] = Value{Kind: Enumeration, Text: value}
					use.Records[0] = rec
					f.Entities[uid] = use
				}
				for a, z := 0, len(uses)-1; a < z; a, z = a+1, z-1 {
					uses[a], uses[z] = uses[z], uses[a]
				}
				items := make([]Value, len(uses))
				for i, u := range uses {
					items[i] = Value{Kind: Reference, Ref: u}
				}
				loop.Args[1] = Value{Kind: Aggregate, Items: items}
				le := f.Entities[lid]
				le.Records[0] = loop
				f.Entities[lid] = le
				b.Args[2] = Value{Kind: Enumeration, Text: "T"}
				out.ReversedBoundIDs = append(out.ReversedBoundIDs, bid)
			}
			bound.Records[0] = b
			f.Entities[bid] = bound
		}
	}
	m, e := ReadDiskHoleExtrusions(bytes.NewReader(serializeWrappedProofFile(f)), l)
	if e != nil {
		return fail()
	}
	out.Model = m
	return out, nil
}

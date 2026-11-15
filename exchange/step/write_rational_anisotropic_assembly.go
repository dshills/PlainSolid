package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
)

// RationalAnisotropicAssemblyInstance supplies a proper placement by rational
// origin and perpendicular X/Z direction ratios. Every input and normalized
// component must have an exact terminating decimal and at most 256 bits.
// Inputs are copied; Y is the exact right-handed cross product, never rounded.
type RationalAnisotropicAssemblyInstance struct {
	ID, Name, Parent, Child string
	Origin, XAxis, ZAxis    [3]*big.Rat
}
type RationalWrappedAnisotropicStationAssemblyModel struct {
	Products    []WrappedAnisotropicStationAssemblyPart
	Occurrences []RationalAnisotropicAssemblyInstance
	LengthUnit  float64
}
type RationalWrappedAnisotropicStationBoreAssemblyModel struct {
	Products    []WrappedAnisotropicStationBoreAssemblyPart
	Occurrences []RationalAnisotropicAssemblyInstance
	LengthUnit  float64
}

func stageRationalAnisotropicUses(ctx context.Context, uses []RationalAnisotropicAssemblyInstance, nestedMode ...bool) ([]AssemblyInstance, map[string]mappedRationalBasis, error) {
	minimum, maximum := 2, 16
	if len(nestedMode) == 1 && nestedMode[0] {
		minimum, maximum = 3, 17
	}
	if len(uses) < minimum || len(uses) > maximum {
		return nil, nil, geom.ErrInvalid
	}
	neutral := make([]AssemblyInstance, 0, len(uses))
	poses := map[string]mappedRationalBasis{}
	for _, u := range uses {
		if e := ctx.Err(); e != nil {
			return nil, nil, e
		}
		if u.ID == "" || len(u.ID) > 4096 || len(u.Name) > 4096 || len(u.Parent) > 4096 || len(u.Child) > 4096 {
			return nil, nil, geom.ErrInvalid
		}
		if _, ok := poses[u.ID]; ok {
			return nil, nil, geom.ErrInvalid
		}
		for _, a := range [][3]*big.Rat{u.Origin, u.XAxis, u.ZAxis} {
			for _, q := range a {
				if q == nil || q.Num().BitLen() > 256 || q.Denom().BitLen() > 256 {
					return nil, nil, geom.ErrInvalid
				}
				if _, e := extrusionDirectionDecimal(q); e != nil {
					return nil, nil, e
				}
			}
		}
		p, e := mappedRationalPose(RationalMappedPlacement{Origin: u.Origin, XAxis: u.XAxis, ZAxis: u.ZAxis})
		if e != nil {
			return nil, nil, e
		}
		if e = rationalHierarchyFrameBound(p); e != nil {
			return nil, nil, e
		}
		poses[u.ID] = p
		neutral = append(neutral, AssemblyInstance{ID: u.ID, Name: u.Name, Parent: u.Parent, Child: u.Child, Transform: geom.Identity()})
	}
	return neutral, poses, nil
}

// WriteRationalWrappedAnisotropicStationAssembly authors AP242 with one shared
// original local filled material and 2..16 caller-selected exact rational uses.
// It stages and re-proves the whole generated file before writing any bytes.
func WriteRationalWrappedAnisotropicStationAssembly(ctx context.Context, w io.Writer, m RationalWrappedAnisotropicStationAssemblyModel, l Limits, o WrappedSurfaceOptions) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	uses, poses, e := stageRationalAnisotropicUses(ctx, m.Occurrences)
	if e != nil {
		return e
	}
	var b bytes.Buffer
	if e = WriteWrappedAnisotropicStationAssembly(ctx, &b, WrappedAnisotropicStationAssemblyModel{Products: m.Products, Occurrences: uses, LengthUnit: m.LengthUnit}, l, o); e != nil {
		return e
	}
	return writeRationalAnisotropicPlaced(ctx, w, b.Bytes(), poses, l, o, false)
}

// WriteRationalWrappedAnisotropicStationBoreAssembly separately proves the
// annular material and preserves the original inner/outer weighted supports.
func WriteRationalWrappedAnisotropicStationBoreAssembly(ctx context.Context, w io.Writer, m RationalWrappedAnisotropicStationBoreAssemblyModel, l Limits, o WrappedSurfaceOptions) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	uses, poses, e := stageRationalAnisotropicUses(ctx, m.Occurrences)
	if e != nil {
		return e
	}
	var b bytes.Buffer
	if e = WriteWrappedAnisotropicStationBoreAssembly(ctx, &b, WrappedAnisotropicStationBoreAssemblyModel{Products: m.Products, Occurrences: uses, LengthUnit: m.LengthUnit}, l, o); e != nil {
		return e
	}
	return writeRationalAnisotropicPlaced(ctx, w, b.Bytes(), poses, l, o, true)
}
func writeRationalAnisotropicPlaced(ctx context.Context, w io.Writer, raw []byte, poses map[string]mappedRationalBasis, l Limits, o WrappedSurfaceOptions, bore bool, nestedMode ...bool) error {
	nested := len(nestedMode) == 1 && nestedMode[0]
	base, e := readExactFlatSingleProductAssembly(ctx, bytes.NewReader(raw), l, nested)
	if e != nil {
		return e
	}
	f := base.Assembly.Source
	byID := map[int64]string{}
	for _, u := range base.Assembly.Occurrences {
		byID[u.ID] = u.Identifier
	}
	edited := map[int64]bool{}
	set := func(id int64, kind string, a [3]*big.Rat) error {
		if edited[id] {
			return geom.ErrInvalid
		}
		r, e := simple(f, id, kind, 2)
		if e != nil {
			return e
		}
		vals := make([]Value, 3)
		for i, q := range a {
			s, e := extrusionDirectionDecimal(q)
			if e != nil {
				return e
			}
			vals[i] = Value{Kind: Number, Text: s}
		}
		r.Args[1] = Value{Kind: Aggregate, Items: vals}
		f.Entities[id] = Entity{ID: id, Records: []Record{r}}
		edited[id] = true
		return nil
	}
	for _, u := range base.Occurrences {
		p, ok := poses[byID[u.NAUOID]]
		if !ok {
			return geom.ErrInvalid
		}
		r, e := simple(f, u.ToPlacementID, "AXIS2_PLACEMENT_3D", 4)
		if e != nil {
			return e
		}
		if e = set(r.Args[1].Ref, "CARTESIAN_POINT", p.Origin); e != nil {
			return e
		}
		if e = set(r.Args[2].Ref, "DIRECTION", p.Z); e != nil {
			return e
		}
		if e = set(r.Args[3].Ref, "DIRECTION", p.X); e != nil {
			return e
		}
	}
	// Same standard BRep/NAUO/CDSR entities, with an explicit AP242 declaration
	// matching the independently qualified XCAF protocol/application context.
	for i, h := range f.Header {
		if h.Type == "FILE_SCHEMA" {
			h.Args = []Value{{Kind: Aggregate, Items: []Value{{Kind: String, Text: "AP242_MANAGED_MODEL_BASED_3D_ENGINEERING_MIM_LF {1 0 10303 442 1 1 4 }"}}}}
			f.Header[i] = h
		}
	}
	for id, en := range f.Entities {
		if len(en.Records) != 1 {
			continue
		}
		r := en.Records[0]
		switch r.Type {
		case "APPLICATION_CONTEXT":
			r.Args[0] = Value{Kind: String, Text: "Managed model based 3d engineering"}
		case "APPLICATION_PROTOCOL_DEFINITION":
			r.Args[1] = Value{Kind: String, Text: "ap242_managed_model_based_3d_engineering"}
			r.Args[2] = Value{Kind: Number, Text: "2013"}
		default:
			continue
		}
		en.Records[0] = r
		f.Entities[id] = en
	}
	out, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	var verified []ExactNAUOOccurrence
	if nested {
		if bore {
			m, e := ReadNestedRationalWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(out), l, o)
			if e != nil {
				return e
			}
			verified = m.Occurrences
		} else {
			m, e := ReadNestedRationalWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(out), l, o)
			if e != nil {
				return e
			}
			verified = m.Occurrences
		}
	} else {
		if bore {
			m, e := ReadRationalWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(out), l, o)
			if e != nil {
				return e
			}
			verified = m.Occurrences
		} else {
			m, e := ReadRationalWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(out), l, o)
			if e != nil {
				return e
			}
			verified = m.Occurrences
		}
	}
	if len(verified) != len(poses) {
		return geom.ErrInvalid
	}
	for _, u := range verified {
		want, ok := poses[byID[u.NAUOID]]
		if !ok {
			return geom.ErrInvalid
		}
		a, b := u.Matrix(), (ExactMappedOccurrence{pose: want}).Matrix()
		for i := range a {
			for j := range a[i] {
				if a[i][j].Cmp(b[i][j]) != 0 {
					return geom.ErrInvalid
				}
			}
		}
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(out)
	if e == nil && n != len(out) {
		e = io.ErrShortWrite
	}
	return e
}

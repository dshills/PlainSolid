package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
)

// NestedRationalWrappedAnisotropicStationAssembly preserves one shared group
// and original filled local product under exactly composed rational occurrences.
type NestedRationalWrappedAnisotropicStationAssembly struct {
	RationalWrappedAnisotropicStationAssembly
	GroupID   int64
	Instances []RationalHierarchyInstance
}

// NestedRationalWrappedAnisotropicStationBoreAssembly retains the separate
// original annular material proof and both inward/outward support families.
type NestedRationalWrappedAnisotropicStationBoreAssembly struct {
	RationalWrappedAnisotropicStationBoreAssembly
	GroupID   int64
	Instances []RationalHierarchyInstance
}

func nestedRationalAnisotropicUses(leaf int64, uses []ExactNAUOOccurrence) (int64, []RationalHierarchyInstance, error) {
	var inner ExactNAUOOccurrence
	var group int64
	for _, u := range uses {
		if u.ChildID == leaf {
			if group != 0 {
				return 0, nil, geom.ErrInvalid
			}
			inner = u
			group = u.ParentID
		}
	}
	if group == 0 {
		return 0, nil, geom.ErrInvalid
	}
	var out []RationalHierarchyInstance
	for _, u := range uses {
		if u.ChildID == group {
			pose := composeMappedPose(u.pose, inner.pose)
			if e := rationalHierarchyFrameBound(pose); e != nil {
				return 0, nil, e
			}
			out = append(out, RationalHierarchyInstance{ProductID: leaf, Path: []int64{u.NAUOID, inner.NAUOID}, pose: pose})
		}
	}
	if len(out) < 2 || len(out) > 16 || len(out)+1 != len(uses) {
		return 0, nil, geom.ErrInvalid
	}
	return group, out, nil
}
func ReadNestedRationalWrappedAnisotropicStationAssembly(ctx context.Context, r io.Reader, l Limits, o WrappedSurfaceOptions) (NestedRationalWrappedAnisotropicStationAssembly, error) {
	var zero NestedRationalWrappedAnisotropicStationAssembly
	m, e := readRationalWrappedAnisotropicStationAssembly(ctx, r, l, o, true)
	if e != nil {
		return zero, e
	}
	group, instances, e := nestedRationalAnisotropicUses(m.Product.DefinitionID, m.Occurrences)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return NestedRationalWrappedAnisotropicStationAssembly{m, group, instances}, nil
}
func ReadNestedRationalWrappedAnisotropicStationBoreAssembly(ctx context.Context, r io.Reader, l Limits, o WrappedSurfaceOptions) (NestedRationalWrappedAnisotropicStationBoreAssembly, error) {
	var zero NestedRationalWrappedAnisotropicStationBoreAssembly
	m, e := readRationalWrappedAnisotropicStationBoreAssembly(ctx, r, l, o, true)
	if e != nil {
		return zero, e
	}
	group, instances, e := nestedRationalAnisotropicUses(m.Product.DefinitionID, m.Occurrences)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return NestedRationalWrappedAnisotropicStationBoreAssembly{m, group, instances}, nil
}
func WriteNestedRationalWrappedAnisotropicStationAssemblySource(ctx context.Context, w io.Writer, source *File, l Limits, o WrappedSurfaceOptions) error {
	return writeNestedRationalAnisotropicSource(ctx, w, source, l, o, false)
}
func WriteNestedRationalWrappedAnisotropicStationBoreAssemblySource(ctx context.Context, w io.Writer, source *File, l Limits, o WrappedSurfaceOptions) error {
	return writeNestedRationalAnisotropicSource(ctx, w, source, l, o, true)
}
func writeNestedRationalAnisotropicSource(ctx context.Context, w io.Writer, source *File, l Limits, o WrappedSurfaceOptions, bore bool) error {
	if ctx == nil || w == nil || !independentNativeSourceRecordBound(source, 4096, 16) {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	if bore {
		_, e = ReadNestedRationalWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(raw), l, o)
	} else {
		_, e = ReadNestedRationalWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), l, o)
	}
	if e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}

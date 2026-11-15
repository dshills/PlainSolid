package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
)

// WrappedAnisotropicAssemblyInstance retains a root-use/shared-group-use source
// path and its exactly represented product-to-root rigid transform.
type WrappedAnisotropicAssemblyInstance struct {
	Path      [2]int64
	ProductID int64
	Transform geom.Affine
}
type NestedWrappedAnisotropicStationAssembly struct {
	WrappedAnisotropicStationAssembly
	GroupID   int64
	Instances []WrappedAnisotropicAssemblyInstance
}
type NestedWrappedAnisotropicStationBoreAssembly struct {
	WrappedAnisotropicStationBoreAssembly
	GroupID   int64
	Instances []WrappedAnisotropicAssemblyInstance
}

func anisotropicNestedUses(source WrappedAnisotropicAssemblySource) (int64, []WrappedAnisotropicAssemblyInstance, error) {
	var inner ExactNAUOOccurrence
	var group int64
	for _, u := range source.Occurrences {
		if u.ChildID == source.Product.DefinitionID {
			inner = u
			group = u.ParentID
		}
	}
	if group == 0 {
		return 0, nil, geom.ErrDomain
	}
	var out []WrappedAnisotropicAssemblyInstance
	for _, u := range source.Occurrences {
		if u.ChildID == group {
			pose, e := independentTorusAssemblyPose(composeMappedPose(u.pose, inner.pose))
			if e != nil || !planarSplineRigid(pose) {
				return 0, nil, geom.ErrDomain
			}
			out = append(out, WrappedAnisotropicAssemblyInstance{[2]int64{u.NAUOID, inner.NAUOID}, source.Product.DefinitionID, pose})
		}
	}
	return group, out, nil
}

// ReadNestedWrappedAnisotropicStationAssembly supports exactly one reusable
// intermediate group and one filled anisotropic leaf, with 2..16 root uses.
func ReadNestedWrappedAnisotropicStationAssembly(ctx context.Context, r io.Reader, l Limits, o WrappedSurfaceOptions) (NestedWrappedAnisotropicStationAssembly, error) {
	var zero NestedWrappedAnisotropicStationAssembly
	m, e := readWrappedAnisotropicStationAssembly(ctx, r, l, o, true)
	if e != nil {
		return zero, e
	}
	group, instances, e := anisotropicNestedUses(m.WrappedAnisotropicAssemblySource)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return NestedWrappedAnisotropicStationAssembly{m, group, instances}, nil
}

// ReadNestedWrappedAnisotropicStationBoreAssembly separately proves the local
// annular material and original inner/outer weighted support roles.
func ReadNestedWrappedAnisotropicStationBoreAssembly(ctx context.Context, r io.Reader, l Limits, o WrappedSurfaceOptions) (NestedWrappedAnisotropicStationBoreAssembly, error) {
	var zero NestedWrappedAnisotropicStationBoreAssembly
	m, e := readWrappedAnisotropicStationBoreAssembly(ctx, r, l, o, true)
	if e != nil {
		return zero, e
	}
	group, instances, e := anisotropicNestedUses(m.WrappedAnisotropicAssemblySource)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return NestedWrappedAnisotropicStationBoreAssembly{m, group, instances}, nil
}
func WriteNestedWrappedAnisotropicStationAssembly(ctx context.Context, w io.Writer, m WrappedAnisotropicStationAssemblyModel, l Limits, o WrappedSurfaceOptions) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	var staged bytes.Buffer
	if e := writeWrappedAnisotropicStationAssembly(ctx, &staged, m, l, o, true); e != nil {
		return e
	}
	if _, e := ReadNestedWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(staged.Bytes()), l, o); e != nil {
		return e
	}
	n, e := w.Write(staged.Bytes())
	if e == nil && n != staged.Len() {
		e = io.ErrShortWrite
	}
	return e
}
func WriteNestedWrappedAnisotropicStationBoreAssembly(ctx context.Context, w io.Writer, m WrappedAnisotropicStationBoreAssemblyModel, l Limits, o WrappedSurfaceOptions) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	var staged bytes.Buffer
	if e := writeWrappedAnisotropicStationBoreAssembly(ctx, &staged, m, l, o, true); e != nil {
		return e
	}
	if _, e := ReadNestedWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(staged.Bytes()), l, o); e != nil {
		return e
	}
	n, e := w.Write(staged.Bytes())
	if e == nil && n != staged.Len() {
		e = io.ErrShortWrite
	}
	return e
}
func WriteNestedWrappedAnisotropicStationAssemblySource(ctx context.Context, w io.Writer, f *File, l Limits, o WrappedSurfaceOptions) error {
	return writeNestedAnisotropicSource(ctx, w, f, l, o, false)
}
func WriteNestedWrappedAnisotropicStationBoreAssemblySource(ctx context.Context, w io.Writer, f *File, l Limits, o WrappedSurfaceOptions) error {
	return writeNestedAnisotropicSource(ctx, w, f, l, o, true)
}
func writeNestedAnisotropicSource(ctx context.Context, w io.Writer, f *File, l Limits, o WrappedSurfaceOptions, bore bool) error {
	if ctx == nil || w == nil || !independentNativeSourceRecordBound(f, 4096, 16) {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	if bore {
		_, e = ReadNestedWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(raw), l, o)
	} else {
		_, e = ReadNestedWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), l, o)
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

package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
)

// NestedExactCircularLoopInstance retains the original root-use/group-use path.
// Transform is proved exactly representable and maps original product coordinates
// into the root; Body.Descriptor.Frame first maps local native coordinates.
type NestedExactCircularLoopInstance struct {
	Path      [2]int64
	ProductID int64
	Transform geom.Affine
}

// NestedExactCircularLoopAssembly is exactly one reusable intermediate group,
// one original local product and 2..16 expanded material occurrences.
type NestedExactCircularLoopAssembly struct {
	ExactCircularLoopAssembly
	GroupID   int64
	Instances []NestedExactCircularLoopInstance
}

func ReadNestedExactCircularLoopAssembly(ctx context.Context, r io.Reader, l Limits) (NestedExactCircularLoopAssembly, error) {
	var zero NestedExactCircularLoopAssembly
	base, e := readExactCircularLoopAssembly(ctx, r, l, true)
	if e != nil {
		return zero, e
	}
	out := NestedExactCircularLoopAssembly{ExactCircularLoopAssembly: base}
	var inner ExactNAUOOccurrence
	for _, u := range base.Occurrences {
		if u.ChildID == base.Body.Product.DefinitionID {
			inner = u
			out.GroupID = u.ParentID
		}
	}
	if out.GroupID == 0 {
		return zero, geom.ErrDomain
	}
	for _, u := range base.Occurrences {
		if u.ChildID == out.GroupID {
			composed := composeMappedPose(u.pose, inner.pose)
			pose, e := independentTorusAssemblyPose(composed)
			if e != nil || !planarSplineRigid(pose) {
				return zero, geom.ErrDomain
			}
			out.Instances = append(out.Instances, NestedExactCircularLoopInstance{[2]int64{u.NAUOID, inner.NAUOID}, base.Body.Product.DefinitionID, pose})
		}
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}

// WriteNestedExactCircularLoopAssembly uses the established explicit product model
// with a geometry-free root and group; exactly one part has standalone Source.
func WriteNestedExactCircularLoopAssembly(ctx context.Context, w io.Writer, m ExactCircularLoopAssemblyModel, l Limits) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	var staged bytes.Buffer
	if e := writeExactCircularLoopAssembly(ctx, &staged, m, l, true); e != nil {
		return e
	}
	if _, e := ReadNestedExactCircularLoopAssembly(ctx, bytes.NewReader(staged.Bytes()), l); e != nil {
		return e
	}
	n, e := w.Write(staged.Bytes())
	if e == nil && n != staged.Len() {
		e = io.ErrShortWrite
	}
	return e
}
func WriteNestedExactCircularLoopAssemblySource(ctx context.Context, w io.Writer, f *File, l Limits) error {
	if ctx == nil || w == nil || !independentNativeSourceRecordBound(f, 4096, 16) {
		return geom.ErrInvalid
	}
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	if _, e = ReadNestedExactCircularLoopAssembly(ctx, bytes.NewReader(raw), l); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}

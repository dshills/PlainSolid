package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
)

// NestedRationalWrappedAnisotropicStationAssemblyModel has exactly three
// products: root, shared subgroup and one original filled local material.
// There are 2..16 root-to-group uses and exactly one group-to-leaf use.
type NestedRationalWrappedAnisotropicStationAssemblyModel RationalWrappedAnisotropicStationAssemblyModel

// NestedRationalWrappedAnisotropicStationBoreAssemblyModel uses the same graph
// with a separately authenticated annular local material.
type NestedRationalWrappedAnisotropicStationBoreAssemblyModel RationalWrappedAnisotropicStationBoreAssemblyModel

func WriteNestedRationalWrappedAnisotropicStationAssembly(ctx context.Context, w io.Writer, m NestedRationalWrappedAnisotropicStationAssemblyModel, l Limits, o WrappedSurfaceOptions) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	uses, poses, e := stageRationalAnisotropicUses(ctx, m.Occurrences, true)
	if e != nil {
		return e
	}
	var staged bytes.Buffer
	if e = WriteNestedWrappedAnisotropicStationAssembly(ctx, &staged, WrappedAnisotropicStationAssemblyModel{Products: m.Products, Occurrences: uses, LengthUnit: m.LengthUnit}, l, o); e != nil {
		return e
	}
	return writeRationalAnisotropicPlaced(ctx, w, staged.Bytes(), poses, l, o, false, true)
}
func WriteNestedRationalWrappedAnisotropicStationBoreAssembly(ctx context.Context, w io.Writer, m NestedRationalWrappedAnisotropicStationBoreAssemblyModel, l Limits, o WrappedSurfaceOptions) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	uses, poses, e := stageRationalAnisotropicUses(ctx, m.Occurrences, true)
	if e != nil {
		return e
	}
	var staged bytes.Buffer
	if e = WriteNestedWrappedAnisotropicStationBoreAssembly(ctx, &staged, WrappedAnisotropicStationBoreAssemblyModel{Products: m.Products, Occurrences: uses, LengthUnit: m.LengthUnit}, l, o); e != nil {
		return e
	}
	return writeRationalAnisotropicPlaced(ctx, w, staged.Bytes(), poses, l, o, true, true)
}

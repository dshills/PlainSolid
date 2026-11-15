package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
)

// RationalWrappedAnisotropicStationBoreAssembly preserves one original annular local
// material and immutable exact rational occurrence frames. No rounded world
// transform or baked world solid is supplied. Source child frames are neutral;
// all products share metre or millimetre units and the flat 2..16-use grammar.
type RationalWrappedAnisotropicStationBoreAssembly struct {
	Source        *File
	Definitions   []AssemblyProduct
	Roots         []int64
	Product       AssemblyProduct
	SourceBodyID  int64
	SourceFaceIDs []int64
	Body          WrappedAnisotropicStationBoreBody
	Occurrences   []ExactNAUOOccurrence
}

func ReadRationalWrappedAnisotropicStationBoreAssembly(ctx context.Context, r io.Reader, l Limits, o WrappedSurfaceOptions) (RationalWrappedAnisotropicStationBoreAssembly, error) {
	var zero RationalWrappedAnisotropicStationBoreAssembly
	source, view, frames, e := rationalAnisotropicProofView(ctx, r, l)
	if e != nil {
		return zero, e
	}
	proof, e := ReadWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(view), l, o)
	if e != nil {
		return zero, e
	}
	out := RationalWrappedAnisotropicStationBoreAssembly{Source: source, Definitions: proof.Assembly.Products, Roots: proof.Assembly.Roots, Product: proof.Product, SourceBodyID: proof.SourceBodyID, SourceFaceIDs: proof.SourceFaceIDs, Body: proof.Body}
	for _, u := range proof.Occurrences {
		from, ok := frames[u.FromPlacementID]
		if !ok {
			return zero, geom.ErrInvalid
		}
		neutral, e := independentTorusAssemblyPose(from)
		if e != nil || neutral != geom.Identity() {
			return zero, geom.ErrInvalid
		}
		to, ok := frames[u.ToPlacementID]
		if !ok {
			return zero, geom.ErrInvalid
		}
		u.pose = to
		out.Occurrences = append(out.Occurrences, u)
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}

// WriteRationalWrappedAnisotropicStationBoreAssemblySource re-proves source and
// exports its actual standard STEP placements, never the private neutral view.
func WriteRationalWrappedAnisotropicStationBoreAssemblySource(ctx context.Context, w io.Writer, source *File, l Limits, o WrappedSurfaceOptions) error {
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
	if _, e = ReadRationalWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(raw), l, o); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

type nativeTrimmedSectionFace struct {
	Outer    []nativeUse
	Holes    [][]nativeUse `json:",omitempty"`
	Reversed bool          `json:",omitempty"`
}

type nativeTrimmedPairSection struct {
	Version    int                         `json:"version"`
	Kind       string                      `json:"kind"`
	Charts     [2]nativeTubePatch          `json:"charts"`
	Faces      [2]nativeTrimmedSectionFace `json:"faces"`
	Tubes      []nativeTubeRecipe          `json:"tubes,omitempty"`
	Projection []int                       `json:"projection,omitempty"`
}

func encodeTrimmedPairSection(patches [2]intersect.Patch, faces [2]Face, branches [][]intersect.TubeSegment, projection *[2]int, coincident bool) *nativeTrimmedPairSection {
	n := &nativeTrimmedPairSection{Version: 1, Kind: "trimmed-algebraic-pair-section"}
	if projection != nil {
		n.Projection = append([]int(nil), projection[:]...)
	}
	var encoder trimEncoder
	for i, p := range patches {
		s, e := parametric.EncodeSurface(p.Surface)
		if e != nil {
			return nil
		}
		n.Charts[i] = nativeTubePatch{Surface: s, Min: p.Min, Max: p.Max}
		f := faces[i]
		q := &n.Faces[i]
		q.Reversed = f.Reversed
		q.Outer, e = encodeOriginalSplineSectionUses(&encoder, f.Outer)
		if e != nil {
			return nil
		}
		for _, h := range f.Holes {
			l, e := encodeOriginalSplineSectionUses(&encoder, h)
			if e != nil {
				return nil
			}
			q.Holes = append(q.Holes, l)
		}
	}
	for _, b := range branches {
		if len(b) == 0 {
			return nil
		}
		p, _, _ := b[0].TrimA().Recipe()
		q, e := encodeTubeRecipe(p.Data())
		if e != nil {
			return nil
		}
		n.Tubes = append(n.Tubes, q)
	}
	if encoder.exactBezier {
		n.Version = 2
	}
	if coincident {
		n.Version = 3
	}
	if validateTubeRecipes(n.Tubes, 4096) != nil {
		return nil
	}
	return n
}

func encodeOriginalSplineSectionUses(encoder *trimEncoder, loop []Use) ([]nativeUse, error) {
	uses := append([]Use(nil), loop...)
	for i := range uses {
		switch c := uses[i].Curve.(type) {
		case nil, QuadraticUV, ExactQuadraticUV, ExactBezierUV, SplineUV:
		case *SplineUV:
			if c == nil {
				return nil, geom.ErrInvalid
			}
			uses[i].Curve = *c
		case *QuadraticUV:
			if c == nil {
				return nil, geom.ErrInvalid
			}
			uses[i].Curve = *c
		default:
			return nil, geom.ErrConvergence
		}
	}
	return encoder.encodeUses(uses)
}

// WriteTrimmedSplinePairSectionJSON writes original geometry and independently
// verifiable tube proposals. It never writes mutable results or proof flags.
func WriteTrimmedSplinePairSectionJSON(w io.Writer, r AlgebraicTrimmedPairSection) error {
	if w == nil || r.native == nil {
		return geom.ErrInvalid
	}
	b, e := json.Marshal(r.native)
	if e != nil {
		return e
	}
	b = append(b, '\n')
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return e
}

// ReadTrimmedSplinePairSectionJSON rebuilds the exact section from original
// supports, loops and bounded tube proposals. No stored contact, display
// coordinate, location or incidence claim is trusted.
func ReadTrimmedSplinePairSectionJSON(ctx context.Context, reader io.Reader, l JSONLimits,
	o AlgebraicTrimmedPairSectionOptions) (r AlgebraicTrimmedPairSection, err error) {
	if ctx == nil || reader == nil || l.MaxBytes < 0 || l.MaxBytes > 1<<30 || l.MaxControlPoints < 0 || l.MaxEntities < 0 {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 32 << 20
	}
	if l.MaxControlPoints == 0 {
		l.MaxControlPoints = 4096
	}
	if l.MaxEntities == 0 {
		l.MaxEntities = 20000
	}
	b, e := io.ReadAll(io.LimitReader(reader, l.MaxBytes+1))
	if e != nil {
		return r, e
	}
	if int64(len(b)) > l.MaxBytes {
		return r, geom.ErrConvergence
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var n nativeTrimmedPairSection
	if e := d.Decode(&n); e != nil {
		return r, e
	}
	if e := d.Decode(new(any)); e != io.EOF || n.Version < 1 || n.Version > 3 || n.Kind != "trimmed-algebraic-pair-section" {
		return r, geom.ErrInvalid
	}
	if len(n.Projection) > 0 {
		if len(n.Projection) != 2 || n.Projection[0] < 0 || n.Projection[0] > 2 ||
			n.Projection[1] < 0 || n.Projection[1] > 2 || n.Projection[0] == n.Projection[1] {
			return r, geom.ErrInvalid
		}
		q := [2]int{n.Projection[0], n.Projection[1]}
		if o.Clip.Projection != nil && *o.Clip.Projection != q {
			return r, geom.ErrInvalid
		}
		o.Clip.Projection = &q
	}
	if e := validateTubeRecipes(n.Tubes, l.MaxControlPoints); e != nil {
		return r, e
	}
	version := 5
	if n.Version >= 2 {
		version = 6
	}
	if n.Version == 3 {
		o.Clip.CoincidentSpans = true
	}
	td := trimDecoder{maxControls: l.MaxControlPoints, version: version}
	var faces [2]Face
	for i, f := range n.Faces {
		if faces[i], e = decodeOriginalSplineSectionFace(ctx, &td, f, &l.MaxEntities); e != nil {
			return r, e
		}
	}
	charts, seeds, e := rebuildAlgebraicTubeSeeds(ctx, n.Charts, n.Tubes, l, o.Trace)
	if e != nil {
		return r, e
	}
	for i, p := range charts {
		faces[i].Surface = p.Surface
	}
	return SectionTrimmedSplinePair(ctx, charts, faces, &seeds, o)
}

func decodeOriginalSplineSectionFace(ctx context.Context, decoder *trimDecoder, source nativeTrimmedSectionFace, remaining *int) (Face, error) {
	var face Face
	face.Reversed = source.Reversed
	for i, loop := range append([][]nativeUse{source.Outer}, source.Holes...) {
		if len(loop) < 2 || len(loop) > *remaining {
			return Face{}, geom.ErrConvergence
		}
		*remaining -= len(loop)
		for _, use := range loop {
			if e := ctx.Err(); e != nil {
				return Face{}, e
			}
			if use.Wall != nil || use.Tube != nil || use.Implicit != nil || use.Ellipse != nil || use.Subcurve != nil ||
				len(use.Spline) == 0 && (use.SplineRange != (UVInterval{}) || use.SplineReversed) {
				return Face{}, geom.ErrInvalid
			}
		}
		uses, e := decoder.decodeUses(loop, 0)
		if e != nil {
			return Face{}, e
		}
		if i == 0 {
			face.Outer = uses
		} else {
			face.Holes = append(face.Holes, uses)
		}
	}
	return face, nil
}

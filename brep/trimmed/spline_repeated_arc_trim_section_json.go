package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type nativeRepeatedSplineArcSection struct {
	Version       int           `json:"version"`
	Kind          string        `json:"kind"`
	DependentAxis int           `json:"dependentAxis"`
	Faces         [2]nativeFace `json:"faces"`
}

func encodeRepeatedSplineArcSection(faces [2]Face, dependent int) *nativeRepeatedSplineArcSection {
	n := &nativeRepeatedSplineArcSection{Version: 1, Kind: "repeated-spline-arc-section", DependentAxis: dependent}
	var encoder trimEncoder
	for i, f := range faces {
		surface, e := parametric.EncodeSurface(f.Surface)
		if e != nil {
			return nil
		}
		q := &n.Faces[i]
		q.Surface, q.Reversed = surface, f.Reversed
		for j, loop := range append([][]Use{f.Outer}, f.Holes...) {
			encoded, e := encodeOriginalSplineSectionUses(&encoder, loop)
			if e != nil {
				return nil
			}
			if j == 0 {
				q.Outer = encoded
			} else {
				q.Holes = append(q.Holes, encoded)
			}
		}
	}
	return n
}

// WriteRepeatedSplineGraphArcJSON writes a source recipe. Points, curve
// fragments, classifications and contact proof flags are recomputed on read.
func WriteRepeatedSplineGraphArcJSON(w io.Writer, section RepeatedSplineArcTrimSection) error {
	if w == nil || section.native == nil {
		return geom.ErrInvalid
	}
	raw, e := json.Marshal(section.native)
	if e != nil {
		return e
	}
	raw = append(raw, '\n')
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}

// ReadRepeatedSplineGraphArcJSON re-establishes the double-contact relation
// and both original trimmed-face interval ledgers from bounded source data.
func ReadRepeatedSplineGraphArcJSON(ctx context.Context, r io.Reader, l JSONLimits,
	o RepeatedSplineArcTrimOptions) (RepeatedSplineArcTrimSection, error) {
	var empty RepeatedSplineArcTrimSection
	if ctx == nil || r == nil || l.MaxBytes < 0 || l.MaxBytes > 1<<30 || l.MaxControlPoints < 0 || l.MaxEntities < 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
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
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return empty, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return empty, geom.ErrConvergence
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var n nativeRepeatedSplineArcSection
	if e := d.Decode(&n); e != nil {
		return empty, e
	}
	if e := d.Decode(new(any)); e != io.EOF || n.Version != 1 || n.Kind != "repeated-spline-arc-section" || n.DependentAxis < 0 || n.DependentAxis > 1 {
		return empty, geom.ErrInvalid
	}
	var faces [2]Face
	decoder := trimDecoder{maxControls: l.MaxControlPoints, version: 6}
	for i, face := range n.Faces {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		surface, e := parametric.DecodeSurface(face.Surface, l.MaxControlPoints)
		if e != nil {
			return empty, e
		}
		f, e := decodeOriginalSplineSectionFace(ctx, &decoder, nativeTrimmedSectionFace{Outer: face.Outer, Holes: face.Holes, Reversed: face.Reversed}, &l.MaxEntities)
		if e != nil {
			return empty, e
		}
		f.Surface = surface
		faces[i] = f
	}
	return SectionRepeatedSplineGraphArc(ctx, faces, n.DependentAxis, o)
}

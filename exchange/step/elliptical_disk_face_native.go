package step

import (
	"bytes"
	"encoding/json"
	"io"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"reflect"
)

type ellipticalDiskNative struct {
	Format                               string
	Version                              int
	Source                               string
	Name                                 string
	LengthUnit, MajorRadius, MinorRadius float64
	Surface, ModelCurve                  json.RawMessage
	Vertex                               geom.Vec3
	Outer                                trimmed.EllipseUV
	Ancestry                             [6]int64
}

func ellipticalDiskNativeData(f EllipticalDiskFace) (ellipticalDiskNative, error) {
	if f.Face.Reversed || len(f.Face.Outer) != 1 || len(f.Face.Holes) != 0 {
		return ellipticalDiskNative{}, ellipticalDiskFailure()
	}
	u := f.Face.Outer[0]
	uv, ok := u.Curve.(trimmed.EllipseUV)
	if !ok || u.Edge != 0 || u.Reversed || u.UVStart != (geom.Vec2{X: f.MajorRadius}) || u.UVEnd != u.UVStart {
		return ellipticalDiskNative{}, ellipticalDiskFailure()
	}
	s, e := parametric.EncodeSurface(f.Face.Surface)
	if e != nil {
		return ellipticalDiskNative{}, e
	}
	c, e := parametric.EncodeCurve(f.Boundary)
	if e != nil {
		return ellipticalDiskNative{}, e
	}
	return ellipticalDiskNative{Format: "plainsolid-step-elliptical-disk-face", Version: 1, Name: f.Name, LengthUnit: f.LengthUnit, MajorRadius: f.MajorRadius, MinorRadius: f.MinorRadius, Surface: s, ModelCurve: c, Vertex: f.Vertex, Outer: uv, Ancestry: [6]int64{f.SourceFaceID, f.SourceEdgeID, f.SourceVertexID, f.SourcePCurveID, f.SourceModelCurveID, f.SourcePlaneID}}, nil
}

// WriteEllipticalDiskFaceJSON stores native plane/model-ellipse descriptors and
// the full-loop recipe, bound to the independently revalidated original STEP.
// It rejects stale geometry, UV orientation and source ancestry before output.
// This is a face-only envelope; it never serializes a fictitious closed solid.
func WriteEllipticalDiskFaceJSON(w io.Writer, f EllipticalDiskFace) error {
	if w == nil {
		return ellipticalDiskFailure()
	}
	var source bytes.Buffer
	if e := WriteEllipticalDiskFaceSource(&source, f.Source, Limits{}); e != nil {
		return e
	}
	proved, e := ReadEllipticalDiskFace(bytes.NewReader(source.Bytes()), Limits{})
	if e != nil {
		return e
	}
	got, e := ellipticalDiskNativeData(f)
	if e != nil {
		return e
	}
	want, e := ellipticalDiskNativeData(proved)
	if e != nil || !reflect.DeepEqual(got, want) {
		return ellipticalDiskFailure()
	}
	want.Source = source.String()
	raw, e := json.Marshal(want)
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

// ReadEllipticalDiskFaceJSON reconstructs and re-proves both the original graph
// and every stored native geometry/trim/ancestry field. MaxBytes bounds the JSON
// envelope (capped at256KiB); source parsing additionally caps STEP at64KiB.
// Invalid input yields no partial face.
func ReadEllipticalDiskFaceJSON(r io.Reader, l Limits) (EllipticalDiskFace, error) {
	fail := func() (EllipticalDiskFace, error) { return EllipticalDiskFace{}, ellipticalDiskFailure() }
	if r == nil {
		return fail()
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	maxBytes := min(l.MaxBytes, int64(262144))
	raw, e := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if e != nil || int64(len(raw)) > maxBytes {
		return fail()
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var n ellipticalDiskNative
	if e = d.Decode(&n); e != nil {
		return fail()
	}
	var extra any
	if e = d.Decode(&extra); e != io.EOF {
		return fail()
	}
	if n.Format != "plainsolid-step-elliptical-disk-face" || n.Version != 1 {
		return fail()
	}
	f, e := ReadEllipticalDiskFace(bytes.NewBufferString(n.Source), l)
	if e != nil {
		return fail()
	}
	want, e := ellipticalDiskNativeData(f)
	if e != nil {
		return fail()
	}
	want.Source = n.Source
	if !reflect.DeepEqual(n, want) {
		return fail()
	}
	return f, nil
}

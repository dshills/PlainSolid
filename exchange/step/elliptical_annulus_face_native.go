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

type ellipticalAnnulusNative struct {
	Format                             string
	Version                            int
	Source                             string
	Outer                              ellipticalDiskNative
	InnerCurve                         json.RawMessage
	InnerVertex                        geom.Vec3
	InnerMajorRadius, InnerMinorRadius float64
	Hole                               trimmed.EllipseUV
	InnerAncestry                      [5]int64
}

func ellipticalAnnulusNativeData(f EllipticalAnnulusFace) (ellipticalAnnulusNative, error) {
	if len(f.Face.Holes) != 1 || len(f.Face.Holes[0]) != 1 {
		return ellipticalAnnulusNative{}, ellipticalAnnulusFailure()
	}
	u := f.Face.Holes[0][0]
	uv, ok := u.Curve.(trimmed.EllipseUV)
	if !ok || u.Edge != 1 || !u.Reversed || u.UVStart != (geom.Vec2{X: f.InnerMajorRadius}) || u.UVEnd != u.UVStart {
		return ellipticalAnnulusNative{}, ellipticalAnnulusFailure()
	}
	outer := f.EllipticalDiskFace
	outer.Face.Holes = nil
	data, e := ellipticalDiskNativeData(outer)
	if e != nil {
		return ellipticalAnnulusNative{}, e
	}
	data.Format = ""
	data.Version = 0
	c, e := parametric.EncodeCurve(f.InnerBoundary)
	if e != nil {
		return ellipticalAnnulusNative{}, e
	}
	return ellipticalAnnulusNative{Format: "plainsolid-step-elliptical-annulus-face", Version: 1, Outer: data, InnerCurve: c, InnerVertex: f.InnerVertex, InnerMajorRadius: f.InnerMajorRadius, InnerMinorRadius: f.InnerMinorRadius, Hole: uv, InnerAncestry: [5]int64{f.SourceInnerBoundID, f.SourceInnerEdgeID, f.SourceInnerVertexID, f.SourceInnerPCurveID, f.SourceInnerModelCurveID}}, nil
}

// WriteEllipticalAnnulusFaceJSON persists the native analytic face and both
// original source owner bindings after re-proving geometry and hole orientation.
func WriteEllipticalAnnulusFaceJSON(w io.Writer, f EllipticalAnnulusFace) error {
	if w == nil {
		return ellipticalAnnulusFailure()
	}
	var source bytes.Buffer
	if e := WriteEllipticalAnnulusFaceSource(&source, f.Source, Limits{}); e != nil {
		return e
	}
	proved, e := ReadEllipticalAnnulusFace(bytes.NewReader(source.Bytes()), Limits{})
	if e != nil {
		return e
	}
	got, e := ellipticalAnnulusNativeData(f)
	if e != nil {
		return e
	}
	want, e := ellipticalAnnulusNativeData(proved)
	if e != nil || !reflect.DeepEqual(got, want) {
		return ellipticalAnnulusFailure()
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

// ReadEllipticalAnnulusFaceJSON re-proves the original STEP and checks every
// native chart, model/UV curve and ancestry field. JSON is capped at256KiB;
// source STEP remains capped at64KiB. No partial face is returned on error.
func ReadEllipticalAnnulusFaceJSON(r io.Reader, l Limits) (EllipticalAnnulusFace, error) {
	fail := func() (EllipticalAnnulusFace, error) { return EllipticalAnnulusFace{}, ellipticalAnnulusFailure() }
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
	var n ellipticalAnnulusNative
	if e = d.Decode(&n); e != nil {
		return fail()
	}
	var extra any
	if e = d.Decode(&extra); e != io.EOF {
		return fail()
	}
	if n.Format != "plainsolid-step-elliptical-annulus-face" || n.Version != 1 {
		return fail()
	}
	f, e := ReadEllipticalAnnulusFace(bytes.NewBufferString(n.Source), l)
	if e != nil {
		return fail()
	}
	want, e := ellipticalAnnulusNativeData(f)
	if e != nil {
		return fail()
	}
	want.Source = n.Source
	if !reflect.DeepEqual(n, want) {
		return fail()
	}
	return f, nil
}

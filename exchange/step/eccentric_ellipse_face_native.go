package step

import (
	"bytes"
	"encoding/json"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"reflect"
)

type eccentricEllipseNative struct {
	Format                              string
	Version                             int
	Source                              string
	Face                                ellipticalAnnulusNative
	Center, Reference                   geom.Vec2
	BoxNormSquaredUpper, ClearanceLower string
}

func eccentricEllipseNativeData(f EccentricEllipticalFace) (eccentricEllipseNative, error) {
	fail := func() (eccentricEllipseNative, error) { return eccentricEllipseNative{}, eccentricEllipseFailure() }
	p := f.Clearance
	if p.BoxNormSquaredUpper == nil || p.ClearanceLower == nil || p.BoxNormSquaredUpper.Num().BitLen() > 4096 || p.BoxNormSquaredUpper.Denom().BitLen() > 4096 || p.ClearanceLower.Num().BitLen() > 4096 || p.ClearanceLower.Denom().BitLen() > 4096 {
		return fail()
	}
	if len(f.Face.Holes) != 1 || len(f.Face.Holes[0]) != 1 {
		return fail()
	}
	uv, ok := f.Face.Holes[0][0].Curve.(trimmed.EllipseUV)
	if !ok {
		return fail()
	}
	start, e := eccentricUVStart(uv.Center, uv.Cos)
	if e != nil {
		return fail()
	}
	data, e := ellipticalAnnulusNativeDataAt(f.EllipticalAnnulusFace, start)
	if e != nil {
		return fail()
	}
	data.Format = ""
	data.Version = 0
	return eccentricEllipseNative{Format: "plainsolid-step-eccentric-ellipse-face", Version: 1, Face: data, Center: p.Center, Reference: p.Reference, BoxNormSquaredUpper: p.BoxNormSquaredUpper.RatString(), ClearanceLower: p.ClearanceLower.RatString()}, nil
}

// WriteEccentricEllipticalFaceJSON binds both original owner paths, native
// curves/UV loops and exact clearance evidence to a freshly re-proved source.
func WriteEccentricEllipticalFaceJSON(w io.Writer, f EccentricEllipticalFace) error {
	if w == nil {
		return eccentricEllipseFailure()
	}
	var b bytes.Buffer
	if e := WriteEccentricEllipticalFaceSource(&b, f.Source, Limits{}); e != nil {
		return e
	}
	proved, e := ReadEccentricEllipticalFace(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		return e
	}
	got, e := eccentricEllipseNativeData(f)
	if e != nil {
		return e
	}
	want, e := eccentricEllipseNativeData(proved)
	if e != nil || !reflect.DeepEqual(got, want) {
		return eccentricEllipseFailure()
	}
	want.Source = b.String()
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

// ReadEccentricEllipticalFaceJSON re-proves all source and native data, including
// the exact containment bound. JSON is capped at256KiB, source STEP at64KiB.
func ReadEccentricEllipticalFaceJSON(r io.Reader, l Limits) (EccentricEllipticalFace, error) {
	fail := func() (EccentricEllipticalFace, error) { return EccentricEllipticalFace{}, eccentricEllipseFailure() }
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
	var n eccentricEllipseNative
	if e = d.Decode(&n); e != nil {
		return fail()
	}
	var extra any
	if e = d.Decode(&extra); e != io.EOF || n.Format != "plainsolid-step-eccentric-ellipse-face" || n.Version != 1 {
		return fail()
	}
	f, e := ReadEccentricEllipticalFace(bytes.NewBufferString(n.Source), l)
	if e != nil {
		return fail()
	}
	want, e := eccentricEllipseNativeData(f)
	if e != nil {
		return fail()
	}
	want.Source = n.Source
	if !reflect.DeepEqual(n, want) {
		return fail()
	}
	return f, nil
}

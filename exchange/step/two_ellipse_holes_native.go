package step

import (
	"bytes"
	"encoding/json"
	"io"
	"plainsolid/brep/trimmed"
	"reflect"
)

type twoEllipseNative struct {
	Format          string
	Version         int
	Source          string
	Parts           [2]eccentricEllipseNative
	SeparationAxis  int
	SeparationLower string
}

func twoEllipseNativeData(f TwoEllipticalHoleFace) (twoEllipseNative, error) {
	fail := func() (twoEllipseNative, error) { return twoEllipseNative{}, twoEllipseHoleFailure() }
	if len(f.Face.Holes) != 2 || f.SeparationLower == nil || f.SeparationLower.Num().BitLen() > 4096 || f.SeparationLower.Denom().BitLen() > 4096 {
		return fail()
	}
	n := twoEllipseNative{Format: "plainsolid-step-two-elliptical-hole-face", Version: 1, SeparationAxis: f.SeparationAxis, SeparationLower: f.SeparationLower.RatString()}
	for i, h := range f.Holes {
		if len(f.Face.Holes[i]) != 1 || f.Face.Holes[i][0].Edge != i+1 {
			return fail()
		}
		base := f.EllipticalDiskFace
		use := f.Face.Holes[i][0]
		use.Edge = 1
		base.Face.Holes = [][]trimmed.Use{{use}}
		part := EccentricEllipticalFace{EllipticalAnnulusFace: EllipticalAnnulusFace{EllipticalDiskFace: base, InnerBoundary: h.Boundary, InnerVertex: h.Vertex, InnerMajorRadius: h.MajorRadius, InnerMinorRadius: h.MinorRadius, SourceInnerBoundID: h.SourceBoundID, SourceInnerEdgeID: h.SourceEdgeID, SourceInnerVertexID: h.SourceVertexID, SourceInnerPCurveID: h.SourcePCurveID, SourceInnerModelCurveID: h.SourceModelCurveID}, Clearance: h.Clearance}
		v, e := eccentricEllipseNativeData(part)
		if e != nil {
			return fail()
		}
		v.Format = ""
		v.Version = 0
		n.Parts[i] = v
	}
	return n, nil
}

// WriteTwoEllipticalHoleFaceJSON stores original source plus native charts,
// all three model/UV owner bindings and exact containment/separation evidence.
// It independently re-proves the complete source before any output.
func WriteTwoEllipticalHoleFaceJSON(w io.Writer, f TwoEllipticalHoleFace) error {
	if w == nil {
		return twoEllipseHoleFailure()
	}
	var b bytes.Buffer
	if e := WriteTwoEllipticalHoleFaceSource(&b, f.Source, Limits{}); e != nil {
		return e
	}
	p, e := ReadTwoEllipticalHoleFace(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		return e
	}
	got, e := twoEllipseNativeData(f)
	if e != nil {
		return e
	}
	want, e := twoEllipseNativeData(p)
	if e != nil || !reflect.DeepEqual(got, want) {
		return twoEllipseHoleFailure()
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

// ReadTwoEllipticalHoleFaceJSON re-proves all stored source/native ownership and
// clearance fields. JSON is capped at256KiB; source STEP remains capped at64KiB.
func ReadTwoEllipticalHoleFaceJSON(r io.Reader, l Limits) (TwoEllipticalHoleFace, error) {
	fail := func() (TwoEllipticalHoleFace, error) { return TwoEllipticalHoleFace{}, twoEllipseHoleFailure() }
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
	var n twoEllipseNative
	if e = d.Decode(&n); e != nil {
		return fail()
	}
	var extra any
	if e = d.Decode(&extra); e != io.EOF || n.Format != "plainsolid-step-two-elliptical-hole-face" || n.Version != 1 {
		return fail()
	}
	f, e := ReadTwoEllipticalHoleFace(bytes.NewBufferString(n.Source), l)
	if e != nil {
		return fail()
	}
	want, e := twoEllipseNativeData(f)
	if e != nil {
		return fail()
	}
	want.Source = n.Source
	if !reflect.DeepEqual(n, want) {
		return fail()
	}
	return f, nil
}

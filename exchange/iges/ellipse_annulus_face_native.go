package iges

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
)

type ellipseAnnulusEnvelope struct {
	Format  string
	Version int
	Source  string
	Face    EllipseAnnulusFace
}

// WriteEllipseAnnulusFaceJSON persists a validated face descriptor and its source
// proof. Stale geometry, ancestry or metadata is rejected before output.
func WriteEllipseAnnulusFaceJSON(w io.Writer, m EllipseAnnulusFace) error {
	if w == nil {
		return fmt.Errorf("iges: nil face output")
	}
	p, e := ReadEllipseAnnulusFace(bytes.NewReader(m.Outer.source), Limits{})
	if e != nil {
		return e
	}
	source := p.Outer.source
	p.Outer.source = nil
	m.Outer.source = nil
	if !reflect.DeepEqual(p, m) {
		return fmt.Errorf("iges: stale ellipse annulus provenance")
	}
	b, e := json.Marshal(ellipseAnnulusEnvelope{"plainsolid-iges-ellipse-annulus", 1, string(source), p})
	if e != nil {
		return e
	}
	b = append(b, '\n')
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadEllipseAnnulusFaceJSON reconstructs native analytic geometry only after the
// bounded source and every public descriptor/provenance field are re-proved.
func ReadEllipseAnnulusFaceJSON(r io.Reader, l Limits) (EllipseAnnulusFace, error) {
	fail := func() (EllipseAnnulusFace, error) {
		return EllipseAnnulusFace{}, fmt.Errorf("iges: invalid ellipse annulus envelope")
	}
	if r == nil {
		return fail()
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	cap := min(l.MaxBytes, int64(131072))
	b, e := io.ReadAll(io.LimitReader(r, cap+1))
	if e != nil || int64(len(b)) > cap {
		return fail()
	}
	var env ellipseAnnulusEnvelope
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if dec.Decode(&env) != nil || env.Format != "plainsolid-iges-ellipse-annulus" || env.Version != 1 {
		return fail()
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return fail()
	}
	m, e := ReadEllipseAnnulusFace(bytes.NewReader([]byte(env.Source)), l)
	if e != nil {
		return fail()
	}
	source := m.Outer.source
	m.Outer.source = nil
	if !reflect.DeepEqual(m, env.Face) {
		return fail()
	}
	m.Outer.source = source
	return m, nil
}

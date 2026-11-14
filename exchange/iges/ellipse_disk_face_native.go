package iges

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
)

type ellipseDiskEnvelope struct {
	Format  string
	Version int
	Source  string
	Face    EllipseDiskFace
}

// WriteEllipseDiskFaceJSON persists a validated face descriptor and its source
// proof. Stale geometry, ancestry or metadata is rejected before output.
func WriteEllipseDiskFaceJSON(w io.Writer, m EllipseDiskFace) error {
	if w == nil {
		return fmt.Errorf("iges: nil face output")
	}
	p, e := ReadEllipseDiskFace(bytes.NewReader(m.source), Limits{})
	if e != nil {
		return e
	}
	source := p.source
	p.source = nil
	m.source = nil
	if !reflect.DeepEqual(p, m) {
		return fmt.Errorf("iges: stale ellipse disk provenance")
	}
	b, e := json.Marshal(ellipseDiskEnvelope{"plainsolid-iges-ellipse-disk", 1, string(source), p})
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

// ReadEllipseDiskFaceJSON reconstructs native analytic geometry only after the
// bounded source and every public descriptor/provenance field are re-proved.
func ReadEllipseDiskFaceJSON(r io.Reader, l Limits) (EllipseDiskFace, error) {
	fail := func() (EllipseDiskFace, error) {
		return EllipseDiskFace{}, fmt.Errorf("iges: invalid ellipse disk envelope")
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
	var env ellipseDiskEnvelope
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if dec.Decode(&env) != nil || env.Format != "plainsolid-iges-ellipse-disk" || env.Version != 1 {
		return fail()
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return fail()
	}
	m, e := ReadEllipseDiskFace(bytes.NewReader([]byte(env.Source)), l)
	if e != nil {
		return fail()
	}
	source := m.source
	m.source = nil
	if !reflect.DeepEqual(m, env.Face) {
		return fail()
	}
	m.source = source
	return m, nil
}

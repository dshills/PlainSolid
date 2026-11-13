// Package exchange reads and writes triangle meshes using bounded, dependency-free
// OBJ, STL, and versioned JSON codecs. These formats do not preserve CAD topology.
package exchange

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"plainsolid/mesh"
)

// Limits bounds input consumption and decoded geometry. Zero fields use defaults.
// Readers buffer at most MaxBytes+1 bytes before parsing. Limits do not bound CPU
// independently of input size. Callers should supply deadlines on network readers.
type Limits struct {
	MaxBytes                  int64
	MaxVertices, MaxTriangles int
}

func (l Limits) normalized() (Limits, error) {
	if l.MaxBytes == 0 {
		l.MaxBytes = 64 << 20
	}
	if l.MaxVertices == 0 {
		l.MaxVertices = 3_000_000
	}
	if l.MaxTriangles == 0 {
		l.MaxTriangles = 1_000_000
	}
	if l.MaxBytes < 1 || l.MaxBytes > 1<<30 || l.MaxVertices < 1 || l.MaxTriangles < 1 {
		return l, errors.New("exchange: invalid limits (MaxBytes must be at most 1 GiB)")
	}
	return l, nil
}
func input(r io.Reader, l Limits) ([]byte, Limits, error) {
	l, e := l.normalized()
	if e != nil {
		return nil, l, e
	}
	if r == nil {
		return nil, l, errors.New("exchange: nil reader")
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return nil, l, e
	}
	if int64(len(b)) > l.MaxBytes {
		return nil, l, errors.New("exchange: byte limit exceeded")
	}
	return b, l, nil
}
func validate(m mesh.Mesh, l Limits) error {
	if len(m.Vertices) > l.MaxVertices || len(m.Triangles) > l.MaxTriangles {
		return errors.New("exchange: geometry limit exceeded")
	}
	return m.Validate()
}

type document struct {
	Version int       `json:"version"`
	Mesh    mesh.Mesh `json:"mesh"`
}

// ReadJSON reads the native version 1 mesh document. Unknown fields and trailing
// values are rejected; the file does not represent analytic geometry or solids.
func ReadJSON(r io.Reader, l Limits) (mesh.Mesh, error) {
	b, l, e := input(r, l)
	if e != nil {
		return mesh.Mesh{}, e
	}
	var d document
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&d); e != nil {
		return mesh.Mesh{}, e
	}
	var extra any
	if e = dec.Decode(&extra); e != io.EOF {
		return mesh.Mesh{}, errors.New("exchange: trailing JSON")
	}
	if d.Version != 1 {
		return mesh.Mesh{}, fmt.Errorf("exchange: unsupported version %d", d.Version)
	}
	if e = validate(d.Mesh, l); e != nil {
		return mesh.Mesh{}, e
	}
	return d.Mesh, nil
}

// WriteJSON writes a native version 1 mesh document.
func WriteJSON(w io.Writer, m mesh.Mesh) error {
	if w == nil {
		return errors.New("exchange: nil writer")
	}
	if e := m.Validate(); e != nil {
		return e
	}
	b := bufio.NewWriter(w)
	if e := json.NewEncoder(b).Encode(document{1, m}); e != nil {
		return e
	}
	return b.Flush()
}

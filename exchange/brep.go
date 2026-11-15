package exchange

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"plainsolid/brep"
	"plainsolid/geom"
)

// BRepLimits bounds native planar topology input. Zero fields select defaults
// of 64 MiB, one million vertices/edges, 100,000 faces, and one million edge
// uses. Counts constrain accepted topology; byte limits bound parsing memory.
// Polygon validation is superlinear; use tighter limits for untrusted inputs.
type BRepLimits struct {
	MaxBytes                                     int64
	MaxVertices, MaxEdges, MaxFaces, MaxEdgeUses int
}

func (l BRepLimits) normalized() (BRepLimits, error) {
	if l.MaxBytes == 0 {
		l.MaxBytes = 64 << 20
	}
	if l.MaxVertices == 0 {
		l.MaxVertices = 1_000_000
	}
	if l.MaxEdges == 0 {
		l.MaxEdges = 1_000_000
	}
	if l.MaxFaces == 0 {
		l.MaxFaces = 100_000
	}
	if l.MaxEdgeUses == 0 {
		l.MaxEdgeUses = 1_000_000
	}
	if l.MaxBytes < 1 || l.MaxBytes > 1<<30 || l.MaxVertices < 1 || l.MaxEdges < 1 || l.MaxFaces < 1 || l.MaxEdgeUses < 1 {
		return l, errors.New("exchange: invalid BRep limits")
	}
	return l, nil
}

type brepDocument struct {
	Version  int         `json:"version"`
	Kind     string      `json:"kind"`
	Vertices []geom.Vec3 `json:"vertices"`
	Edges    []brep.Edge `json:"edges"`
	Faces    []brep.Face `json:"faces"`
}

// WriteBRepJSON preserves the indexed vertex/edge/oriented-face graph of a
// planar BRep in native version 1 JSON. It does not support curved surfaces,
// assemblies, metadata or compatibility with OCCT persistence.
func WriteBRepJSON(w io.Writer, s brep.Solid) error {
	if w == nil {
		return errors.New("exchange: nil writer")
	}
	if e := s.Validate(); e != nil {
		return e
	}
	d := brepDocument{1, "planar-brep", s.Vertices(), s.Edges(), s.Faces()}
	b := bufio.NewWriter(w)
	if e := json.NewEncoder(b).Encode(d); e != nil {
		return e
	}
	return b.Flush()
}

// ReadBRepJSON reads native version 1 planar topology, validates all incidence
// and per-face geometric constraints through brep.NewSolid, and preserves input entity
// indices. Unknown fields, versions, kinds and trailing documents are rejected.
// Use ReadBRepJSONValidated for exact shell embedding and material nesting checks.
func ReadBRepJSON(r io.Reader, l BRepLimits) (brep.Solid, error) {
	l, e := l.normalized()
	if e != nil {
		return brep.Solid{}, e
	}
	b, _, e := input(r, Limits{MaxBytes: l.MaxBytes})
	if e != nil {
		return brep.Solid{}, e
	}
	var d brepDocument
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&d); e != nil {
		return brep.Solid{}, e
	}
	var extra any
	if e = dec.Decode(&extra); e != io.EOF {
		return brep.Solid{}, errors.New("exchange: trailing BRep JSON")
	}
	if d.Version != 1 || d.Kind != "planar-brep" {
		return brep.Solid{}, errors.New("exchange: unsupported BRep schema")
	}
	if e := checkBRepDocument(d, l); e != nil {
		return brep.Solid{}, e
	}
	return brep.NewSolid(d.Vertices, d.Edges, d.Faces)
}

func checkBRepDocument(d brepDocument, l BRepLimits) error {
	if len(d.Vertices) > l.MaxVertices || len(d.Edges) > l.MaxEdges || len(d.Faces) > l.MaxFaces {
		return errors.New("exchange: BRep entity limit exceeded")
	}
	uses := 0
	for _, f := range d.Faces {
		if len(f.Boundary) > l.MaxEdgeUses-uses {
			return errors.New("exchange: BRep edge-use limit exceeded")
		}
		uses += len(f.Boundary)
		for _, hole := range f.Holes {
			if len(hole) > l.MaxEdgeUses-uses {
				return errors.New("exchange: BRep edge-use limit exceeded")
			}
			uses += len(hole)
		}
	}
	return nil
}

package exchange

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"plainsolid/geom"
	"plainsolid/mesh"
)

// ReadSTL detects binary STL by its exact declared length, including files whose
// binary header starts with "solid". Otherwise it parses strict ASCII STL. Exact
// coordinate duplicates are welded; supplied normals are validated then discarded.
func ReadSTL(r io.Reader, l Limits) (mesh.Mesh, error) {
	b, l, e := input(r, l)
	if e != nil {
		return mesh.Mesh{}, e
	}
	if len(b) >= 84 {
		n := uint64(binary.LittleEndian.Uint32(b[80:84]))
		if 84+50*n == uint64(len(b)) {
			return readBinarySTL(b, l)
		}
	}
	return readASCIISTL(b, l)
}

type builder struct {
	m   mesh.Mesh
	ids map[geom.Vec3]int
	l   Limits
}

func (s *builder) add(v [3]geom.Vec3) error {
	if len(s.m.Triangles) >= s.l.MaxTriangles {
		return errors.New("exchange: triangle limit exceeded")
	}
	var t [3]int
	for i, p := range v {
		if !p.IsFinite() {
			return errors.New("exchange: non-finite STL vertex")
		}
		id, ok := s.ids[p]
		if !ok {
			if len(s.m.Vertices) >= s.l.MaxVertices {
				return errors.New("exchange: vertex limit exceeded")
			}
			id = len(s.m.Vertices)
			s.ids[p] = id
			s.m.Vertices = append(s.m.Vertices, p)
		}
		t[i] = id
	}
	s.m.Triangles = append(s.m.Triangles, t)
	return nil
}
func newBuilder(l Limits) *builder { return &builder{ids: make(map[geom.Vec3]int), l: l} }
func (s *builder) finish() (mesh.Mesh, error) {
	if e := s.m.Validate(); e != nil {
		return mesh.Mesh{}, e
	}
	return s.m, nil
}
func readBinarySTL(b []byte, l Limits) (mesh.Mesh, error) {
	n := int(binary.LittleEndian.Uint32(b[80:84]))
	if n > l.MaxTriangles {
		return mesh.Mesh{}, errors.New("exchange: triangle limit exceeded")
	}
	s := newBuilder(l)
	for i := 0; i < n; i++ {
		rec := b[84+i*50 : 84+(i+1)*50]
		var points [4]geom.Vec3
		for j := 0; j < 4; j++ {
			p := &points[j]
			c := []*float64{&p.X, &p.Y, &p.Z}
			for k, x := range c {
				*x = float64(math.Float32frombits(binary.LittleEndian.Uint32(rec[j*12+k*4:])))
			}
			if !p.IsFinite() {
				return mesh.Mesh{}, errors.New("exchange: non-finite STL coordinates")
			}
		}
		if e := s.add([3]geom.Vec3{points[1], points[2], points[3]}); e != nil {
			return mesh.Mesh{}, e
		}
	}
	return s.finish()
}
func readASCIISTL(b []byte, l Limits) (mesh.Mesh, error) {
	scan := bufio.NewScanner(bytes.NewReader(b))
	scan.Buffer(make([]byte, 4096), len(b)+1)
	next := func() []string {
		for scan.Scan() {
			f := strings.Fields(scan.Text())
			if len(f) > 0 {
				return f
			}
		}
		return nil
	}
	f := next()
	if len(f) == 0 || f[0] != "solid" {
		return mesh.Mesh{}, errors.New("exchange: invalid STL header or binary length")
	}
	s := newBuilder(l)
	vector := func(f []string, offset int) (geom.Vec3, error) {
		var p geom.Vec3
		v := []*float64{&p.X, &p.Y, &p.Z}
		if len(f) != offset+3 {
			return p, errors.New("exchange: invalid ASCII STL vector")
		}
		for i, x := range v {
			a, e := number(f[offset+i])
			if e != nil {
				return p, e
			}
			*x = a
		}
		return p, nil
	}
	for {
		f = next()
		if len(f) > 0 && f[0] == "endsolid" {
			if next() != nil {
				return mesh.Mesh{}, errors.New("exchange: trailing STL data")
			}
			if e := scan.Err(); e != nil {
				return mesh.Mesh{}, e
			}
			return s.finish()
		}
		if len(f) != 5 || f[0] != "facet" || f[1] != "normal" {
			return mesh.Mesh{}, errors.New("exchange: expected STL facet or endsolid")
		}
		if _, e := vector(f, 2); e != nil {
			return mesh.Mesh{}, e
		}
		f = next()
		if len(f) != 2 || f[0] != "outer" || f[1] != "loop" {
			return mesh.Mesh{}, errors.New("exchange: expected outer loop")
		}
		var p [3]geom.Vec3
		for i := range p {
			f = next()
			if len(f) != 4 || f[0] != "vertex" {
				return mesh.Mesh{}, errors.New("exchange: expected vertex")
			}
			v, e := vector(f, 1)
			if e != nil {
				return mesh.Mesh{}, e
			}
			p[i] = v
		}
		f = next()
		if len(f) != 1 || f[0] != "endloop" {
			return mesh.Mesh{}, errors.New("exchange: expected endloop")
		}
		f = next()
		if len(f) != 1 || f[0] != "endfacet" {
			return mesh.Mesh{}, errors.New("exchange: expected endfacet")
		}
		if e := s.add(p); e != nil {
			return mesh.Mesh{}, e
		}
	}
}

// WriteSTL writes binary STL. It rejects float32 overflow and triangles that
// collapse at STL precision. Attribute bytes are zero; normals are recomputed.
func WriteSTL(w io.Writer, m mesh.Mesh) error {
	if w == nil {
		return errors.New("exchange: nil writer")
	}
	if e := m.Validate(); e != nil {
		return e
	}
	if uint64(len(m.Triangles)) > math.MaxUint32 {
		return errors.New("exchange: too many STL triangles")
	}
	// Preflight all coordinates before writing so precision loss is explicit.
	verts := make([]geom.Vec3, len(m.Vertices))
	for i, v := range m.Vertices {
		p := geom.Vec3{X: float64(float32(v.X)), Y: float64(float32(v.Y)), Z: float64(float32(v.Z))}
		if !p.IsFinite() {
			return errors.New("exchange: STL float32 overflow")
		}
		verts[i] = p
	}
	rounded := mesh.Mesh{Vertices: verts, Triangles: m.Triangles}
	if e := rounded.Validate(); e != nil {
		return fmt.Errorf("exchange: STL precision: %w", e)
	}
	b := bufio.NewWriter(w)
	var h [84]byte
	copy(h[:], "PlainSolid binary STL")
	binary.LittleEndian.PutUint32(h[80:], uint32(len(m.Triangles)))
	if _, e := b.Write(h[:]); e != nil {
		return e
	}
	for _, t := range m.Triangles {
		a, c, d := verts[t[0]], verts[t[1]], verts[t[2]]
		n, e := c.Sub(a).Cross(d.Sub(a)).Unit()
		if e != nil {
			return e
		}
		var rec [50]byte
		points := []geom.Vec3{n, a, c, d}
		for j, p := range points {
			for k, v := range []float64{p.X, p.Y, p.Z} {
				binary.LittleEndian.PutUint32(rec[j*12+k*4:], math.Float32bits(float32(v)))
			}
		}
		if _, e = b.Write(rec[:]); e != nil {
			return e
		}
	}
	return b.Flush()
}

// WriteASCIISTL writes ASCII STL using float64 round-trip precision.
func WriteASCIISTL(w io.Writer, m mesh.Mesh) error {
	if w == nil {
		return errors.New("exchange: nil writer")
	}
	if e := m.Validate(); e != nil {
		return e
	}
	b := bufio.NewWriter(w)
	if _, e := fmt.Fprintln(b, "solid PlainSolid"); e != nil {
		return e
	}
	for _, t := range m.Triangles {
		a, c, d := m.Vertices[t[0]], m.Vertices[t[1]], m.Vertices[t[2]]
		n, e := c.Sub(a).Cross(d.Sub(a)).Unit()
		if e != nil {
			return e
		}
		if _, e = fmt.Fprintf(b, "facet normal %.17g %.17g %.17g\nouter loop\n", n.X, n.Y, n.Z); e != nil {
			return e
		}
		for _, p := range []geom.Vec3{a, c, d} {
			if _, e = fmt.Fprintf(b, "vertex %.17g %.17g %.17g\n", p.X, p.Y, p.Z); e != nil {
				return e
			}
		}
		if _, e = fmt.Fprintln(b, "endloop\nendfacet"); e != nil {
			return e
		}
	}
	if _, e := fmt.Fprintln(b, "endsolid PlainSolid"); e != nil {
		return e
	}
	return b.Flush()
}

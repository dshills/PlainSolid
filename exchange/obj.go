package exchange

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"plainsolid/geom"
	"plainsolid/mesh"
)

func number(s string) (float64, error) {
	v, e := strconv.ParseFloat(s, 64)
	if e != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("exchange: invalid finite number %q", s)
	}
	return v, nil
}

// ReadOBJ accepts vertices and triangular faces, including negative indices and
// v/vt/vn references. Material, texture and normal metadata is discarded. Polygon
// faces and freeform geometry are rejected instead of silently approximated.
func ReadOBJ(r io.Reader, l Limits) (mesh.Mesh, error) {
	b, l, e := input(r, l)
	if e != nil {
		return mesh.Mesh{}, e
	}
	m := mesh.Mesh{}
	scan := bufio.NewScanner(bytes.NewReader(b))
	scan.Buffer(make([]byte, 4096), len(b)+1)
	line := 0
	nt, nn := 0, 0
	for scan.Scan() {
		line++
		s := strings.SplitN(scan.Text(), "#", 2)[0]
		f := strings.Fields(s)
		if len(f) == 0 {
			continue
		}
		fail := func() (mesh.Mesh, error) { return mesh.Mesh{}, fmt.Errorf("exchange: invalid OBJ line %d", line) }
		switch f[0] {
		case "v":
			if len(f) != 4 || len(m.Vertices) >= l.MaxVertices {
				return fail()
			}
			var v geom.Vec3
			vals := []*float64{&v.X, &v.Y, &v.Z}
			for i, p := range vals {
				*p, e = number(f[i+1])
				if e != nil {
					return fail()
				}
			}
			m.Vertices = append(m.Vertices, v)
		case "vt", "vn":
			if f[0] == "vn" && len(f) != 4 || f[0] == "vt" && (len(f) < 2 || len(f) > 4) {
				return fail()
			}
			for _, v := range f[1:] {
				if _, e = number(v); e != nil {
					return fail()
				}
			}
			if f[0] == "vt" {
				nt++
			} else {
				nn++
			}
		case "f":
			if len(f) != 4 || len(m.Triangles) >= l.MaxTriangles {
				return fail()
			}
			var t [3]int
			for i, s := range f[1:] {
				parts := strings.Split(s, "/")
				if len(parts) > 3 || parts[0] == "" || (len(parts) == 2 && parts[1] == "") || (len(parts) == 3 && parts[2] == "") {
					return fail()
				}
				t[i], e = objIndex(parts[0], len(m.Vertices))
				if e != nil {
					return fail()
				}
				if len(parts) > 1 && parts[1] != "" {
					if _, e = objIndex(parts[1], nt); e != nil {
						return fail()
					}
				}
				if len(parts) > 2 && parts[2] != "" {
					if _, e = objIndex(parts[2], nn); e != nil {
						return fail()
					}
				}
			}
			m.Triangles = append(m.Triangles, t)
		case "o", "g", "s", "usemtl", "mtllib": // Non-geometric metadata.
		default:
			return mesh.Mesh{}, fmt.Errorf("exchange: unsupported OBJ directive %q at line %d", f[0], line)
		}
	}
	if e = scan.Err(); e != nil {
		return mesh.Mesh{}, e
	}
	if e = validate(m, l); e != nil {
		return mesh.Mesh{}, e
	}
	return m, nil
}
func objIndex(s string, n int) (int, error) {
	v, e := strconv.Atoi(s)
	if e != nil || v == 0 {
		return 0, errors.New("invalid index")
	}
	if v < 0 {
		v = n + v
	} else {
		v--
	}
	if v < 0 || v >= n {
		return 0, errors.New("index out of bounds")
	}
	return v, nil
}

// WriteOBJ writes triangle geometry at float64 round-trip precision.
func WriteOBJ(w io.Writer, m mesh.Mesh) error {
	if w == nil {
		return errors.New("exchange: nil writer")
	}
	if e := m.Validate(); e != nil {
		return e
	}
	b := bufio.NewWriter(w)
	for _, v := range m.Vertices {
		if _, e := fmt.Fprintf(b, "v %.17g %.17g %.17g\n", v.X, v.Y, v.Z); e != nil {
			return e
		}
	}
	for _, t := range m.Triangles {
		if _, e := fmt.Fprintf(b, "f %d %d %d\n", t[0]+1, t[1]+1, t[2]+1); e != nil {
			return e
		}
	}
	return b.Flush()
}

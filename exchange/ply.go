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

// ReadPLY reads ASCII or binary little/big-endian PLY 1.0 triangle data with exactly
// x, y, z float/double properties and face elements with one vertex_indices list.
// Additional properties, elements and polygon faces are
// explicitly rejected. Comments and obj_info metadata are discarded.
func ReadPLY(r io.Reader, l Limits) (mesh.Mesh, error) {
	b, l, e := input(r, l)
	if e != nil {
		return mesh.Mesh{}, e
	}
	headerProbe := bufio.NewScanner(bytes.NewReader(b))
	headerProbe.Buffer(make([]byte, 4096), len(b)+1)
	nonempty := 0
	for headerProbe.Scan() {
		fields := strings.Fields(headerProbe.Text())
		if len(fields) == 0 {
			continue
		}
		nonempty++
		if nonempty == 2 {
			if len(fields) == 3 && fields[0] == "format" && (fields[1] == "binary_little_endian" || fields[1] == "binary_big_endian") {
				return readBinaryPLY(b, l)
			}
			break
		}
	}
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
	equal := func(f []string, want string) bool { return strings.Join(f, " ") == want }
	if !equal(next(), "ply") || !equal(next(), "format ascii 1.0") {
		return mesh.Mesh{}, errors.New("exchange: expected ASCII PLY 1.0")
	}
	nv, nf := -1, -1
	stage := 0
	prop := 0
	coordinateBits := [3]int{}
	signedIndices := true
	for {
		f := next()
		if len(f) == 0 {
			return mesh.Mesh{}, errors.New("exchange: missing PLY end_header")
		}
		if f[0] == "comment" || f[0] == "obj_info" {
			continue
		}
		if equal(f, "end_header") {
			if stage != 2 || prop != 1 {
				return mesh.Mesh{}, errors.New("exchange: incomplete PLY header")
			}
			break
		}
		switch f[0] {
		case "element":
			if len(f) != 3 {
				return mesh.Mesh{}, errors.New("exchange: invalid PLY element")
			}
			n, e := strconv.Atoi(f[2])
			if e != nil || n < 0 {
				return mesh.Mesh{}, errors.New("exchange: invalid PLY element count")
			}
			if f[1] == "vertex" && stage == 0 {
				if n > l.MaxVertices {
					return mesh.Mesh{}, errors.New("exchange: vertex limit exceeded")
				}
				nv = n
				stage = 1
				prop = 0
			} else if f[1] == "face" && stage == 1 && prop == 3 {
				if n > l.MaxTriangles {
					return mesh.Mesh{}, errors.New("exchange: triangle limit exceeded")
				}
				nf = n
				stage = 2
				prop = 0
			} else {
				return mesh.Mesh{}, errors.New("exchange: unsupported PLY element layout")
			}
		case "property":
			if stage == 1 {
				names := []string{"x", "y", "z"}
				if prop >= 3 || len(f) != 3 || (f[1] != "float" && f[1] != "double" && f[1] != "float32" && f[1] != "float64") || f[2] != names[prop] {
					return mesh.Mesh{}, errors.New("exchange: unsupported PLY vertex property")
				}
				if f[1] == "float" || f[1] == "float32" {
					coordinateBits[prop] = 32
				} else {
					coordinateBits[prop] = 64
				}
				prop++
			} else if stage == 2 && prop == 0 && len(f) == 5 && f[1] == "list" && (f[2] == "uchar" || f[2] == "uint8") && (f[3] == "int" || f[3] == "int32" || f[3] == "uint" || f[3] == "uint32") && f[4] == "vertex_indices" {
				signedIndices = f[3] == "int" || f[3] == "int32"
				prop++
			} else {
				return mesh.Mesh{}, errors.New("exchange: unsupported PLY face property")
			}
		default:
			return mesh.Mesh{}, fmt.Errorf("exchange: unsupported PLY header directive %q", f[0])
		}
	}
	// No allocation from declared counts: truncated files remain cheap to reject.
	m := mesh.Mesh{}
	for i := 0; i < nv; i++ {
		f := next()
		if len(f) != 3 {
			return mesh.Mesh{}, errors.New("exchange: invalid PLY vertex")
		}
		var p geom.Vec3
		for j, x := range []*float64{&p.X, &p.Y, &p.Z} {
			*x, e = strconv.ParseFloat(f[j], coordinateBits[j])
			if e != nil || math.IsNaN(*x) || math.IsInf(*x, 0) {
				return mesh.Mesh{}, errors.New("exchange: invalid PLY coordinate")
			}
		}
		m.Vertices = append(m.Vertices, p)
	}
	for i := 0; i < nf; i++ {
		f := next()
		if len(f) != 4 || f[0] != "3" {
			return mesh.Mesh{}, errors.New("exchange: expected PLY triangle")
		}
		var t [3]int
		for j := range t {
			t[j], e = strconv.Atoi(f[j+1])
			if e != nil || t[j] < 0 || t[j] >= nv || uint64(t[j]) > math.MaxUint32 || (signedIndices && int64(t[j]) > math.MaxInt32) {
				return mesh.Mesh{}, errors.New("exchange: invalid PLY index")
			}
		}
		m.Triangles = append(m.Triangles, t)
	}
	if next() != nil {
		return mesh.Mesh{}, errors.New("exchange: trailing PLY records")
	}
	if e = scan.Err(); e != nil {
		return mesh.Mesh{}, e
	}
	if e = validate(m, l); e != nil {
		return mesh.Mesh{}, e
	}
	return m, nil
}

// WritePLY writes the ASCII PLY 1.0 triangle subset with double coordinates.
func WritePLY(w io.Writer, m mesh.Mesh) error {
	if w == nil {
		return errors.New("exchange: nil writer")
	}
	if e := m.Validate(); e != nil {
		return e
	}
	for _, t := range m.Triangles {
		for _, index := range t {
			if int64(index) > math.MaxInt32 {
				return errors.New("exchange: ASCII PLY index exceeds int32")
			}
		}
	}
	b := bufio.NewWriter(w)
	if _, e := fmt.Fprintf(b, "ply\nformat ascii 1.0\nelement vertex %d\nproperty double x\nproperty double y\nproperty double z\nelement face %d\nproperty list uchar int vertex_indices\nend_header\n", len(m.Vertices), len(m.Triangles)); e != nil {
		return e
	}
	for _, v := range m.Vertices {
		if _, e := fmt.Fprintf(b, "%.17g %.17g %.17g\n", v.X, v.Y, v.Z); e != nil {
			return e
		}
	}
	for _, t := range m.Triangles {
		if _, e := fmt.Fprintf(b, "3 %d %d %d\n", t[0], t[1], t[2]); e != nil {
			return e
		}
	}
	return b.Flush()
}

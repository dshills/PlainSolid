package exchange

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"plainsolid/geom"
	"plainsolid/mesh"
	"strconv"
	"strings"
)

func readBinaryPLY(data []byte, l Limits) (mesh.Mesh, error) {
	fail := func() (mesh.Mesh, error) {
		return mesh.Mesh{}, errors.New("exchange: invalid or unsupported binary PLY")
	}
	pos := 0
	line := func() []string {
		for pos < len(data) {
			i := bytes.IndexByte(data[pos:], '\n')
			if i < 0 {
				return nil
			}
			s := string(data[pos : pos+i])
			pos += i + 1
			f := strings.Fields(s)
			if len(f) > 0 {
				return f
			}
		}
		return nil
	}
	if strings.Join(line(), " ") != "ply" {
		return fail()
	}
	format := strings.Join(line(), " ")
	var order binary.ByteOrder
	switch format {
	case "format binary_little_endian 1.0":
		order = binary.LittleEndian
	case "format binary_big_endian 1.0":
		order = binary.BigEndian
	default:
		return fail()
	}
	nv, nf, stage, prop := 0, 0, 0, 0
	bits := [3]int{}
	signed := true
	for {
		f := line()
		if len(f) == 0 {
			return fail()
		}
		if f[0] == "comment" || f[0] == "obj_info" {
			continue
		}
		if len(f) == 1 && f[0] == "end_header" {
			if stage != 2 || prop != 1 {
				return fail()
			}
			break
		}
		switch f[0] {
		case "element":
			if len(f) != 3 {
				return fail()
			}
			n, e := strconv.Atoi(f[2])
			if e != nil || n < 0 {
				return fail()
			}
			if f[1] == "vertex" && stage == 0 && n <= l.MaxVertices {
				nv = n
				stage = 1
				prop = 0
			} else if f[1] == "face" && stage == 1 && prop == 3 && n <= l.MaxTriangles {
				nf = n
				stage = 2
				prop = 0
			} else {
				return fail()
			}
		case "property":
			if stage == 1 {
				if len(f) != 3 || prop >= 3 || f[2] != []string{"x", "y", "z"}[prop] {
					return fail()
				}
				switch f[1] {
				case "float", "float32":
					bits[prop] = 32
				case "double", "float64":
					bits[prop] = 64
				default:
					return fail()
				}
				prop++
			} else if stage == 2 && prop == 0 && len(f) == 5 && f[1] == "list" && (f[2] == "uchar" || f[2] == "uint8") && (f[3] == "int" || f[3] == "int32" || f[3] == "uint" || f[3] == "uint32") && f[4] == "vertex_indices" {
				signed = f[3] == "int" || f[3] == "int32"
				prop++
			} else {
				return fail()
			}
		default:
			return fail()
		}
	}
	stride := (bits[0] + bits[1] + bits[2]) / 8
	remain := len(data) - pos
	if nv > remain/stride {
		return fail()
	}
	vertexBytes := nv * stride
	remain -= vertexBytes
	if nf > remain/13 || nf*13 != remain {
		return fail()
	}
	m := mesh.Mesh{Vertices: make([]geom.Vec3, nv), Triangles: make([][3]int, nf)}
	for i := range m.Vertices {
		p := &m.Vertices[i]
		for j, x := range []*float64{&p.X, &p.Y, &p.Z} {
			if bits[j] == 32 {
				*x = float64(math.Float32frombits(order.Uint32(data[pos:])))
				pos += 4
			} else {
				*x = math.Float64frombits(order.Uint64(data[pos:]))
				pos += 8
			}
			if math.IsNaN(*x) || math.IsInf(*x, 0) {
				return fail()
			}
		}
	}
	for i := range m.Triangles {
		if data[pos] != 3 {
			return fail()
		}
		pos++
		for j := range m.Triangles[i] {
			n := order.Uint32(data[pos:])
			pos += 4
			if (signed && n > math.MaxInt32) || uint64(n) >= uint64(nv) {
				return fail()
			}
			m.Triangles[i][j] = int(n)
		}
	}
	if e := validate(m, l); e != nil {
		return mesh.Mesh{}, e
	}
	return m, nil
}

// WriteBinaryPLY emits double coordinates and unsigned 32-bit triangle indices.
// bigEndian selects the byte order. Double coordinates avoid float32 rounding.
func WriteBinaryPLY(w io.Writer, m mesh.Mesh, bigEndian bool) error {
	if w == nil {
		return errors.New("exchange: nil writer")
	}
	if e := m.Validate(); e != nil {
		return e
	}
	if uint64(len(m.Vertices)) > uint64(math.MaxUint32)+1 {
		return errors.New("exchange: PLY index range exceeded")
	}
	var order binary.ByteOrder = binary.LittleEndian
	name := "binary_little_endian"
	if bigEndian {
		order = binary.BigEndian
		name = "binary_big_endian"
	}
	b := bufio.NewWriter(w)
	if _, e := fmt.Fprintf(b, "ply\nformat %s 1.0\nelement vertex %d\nproperty double x\nproperty double y\nproperty double z\nelement face %d\nproperty list uchar uint vertex_indices\nend_header\n", name, len(m.Vertices), len(m.Triangles)); e != nil {
		return e
	}
	var record [24]byte
	for _, p := range m.Vertices {
		for j, x := range []float64{p.X, p.Y, p.Z} {
			order.PutUint64(record[j*8:], math.Float64bits(x))
		}
		if _, e := b.Write(record[:]); e != nil {
			return e
		}
	}
	for _, t := range m.Triangles {
		record[0] = 3
		for j, n := range t {
			order.PutUint32(record[1+4*j:], uint32(n))
		}
		if _, e := b.Write(record[:13]); e != nil {
			return e
		}
	}
	return b.Flush()
}

package exchange

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math"
	"plainsolid/mesh"
)

// ReadGLB reads GLB2 with one JSON chunk followed by at most one embedded BIN
// chunk. It applies ReadGLTF's strict scene/accessor subset; unknown chunks,
// external buffers, extensions and multiple buffers are rejected.
func ReadGLB(r io.Reader, l Limits) (mesh.Mesh, error) {
	b, l, e := input(r, l)
	if e != nil {
		return mesh.Mesh{}, e
	}
	fail := func() (mesh.Mesh, error) {
		return mesh.Mesh{}, errors.New("exchange: invalid or unsupported GLB container")
	}
	if len(b) < 20 || !bytes.Equal(b[:4], []byte("glTF")) || binary.LittleEndian.Uint32(b[4:]) != 2 || uint64(binary.LittleEndian.Uint32(b[8:])) != uint64(len(b)) {
		return fail()
	}
	pos := 12
	var js, bin []byte
	for pos < len(b) {
		if len(b)-pos < 8 {
			return fail()
		}
		n := uint64(binary.LittleEndian.Uint32(b[pos:]))
		kind := binary.LittleEndian.Uint32(b[pos+4:])
		pos += 8
		if n%4 != 0 || n > uint64(len(b)-pos) {
			return fail()
		}
		chunk := b[pos : pos+int(n)]
		pos += int(n)
		if js == nil {
			if kind != 0x4e4f534a || len(chunk) == 0 {
				return fail()
			}
			js = chunk
		} else if bin == nil && kind == 0x004e4942 {
			bin = chunk
			if len(bin) == 0 {
				return fail()
			}
		} else {
			return fail()
		}
	}
	return readGLTFJSON(js, bin, true, l)
}

// WriteGLB writes the same float32 triangle scene as WriteGLTF, in a GLB2
// container with embedded binary data. Float32 overflow/collapse is rejected.
func WriteGLB(w io.Writer, m mesh.Mesh) error { return writeGLTF(w, m, true) }
func writeGLBChunks(w io.Writer, d gltfDocument, bin []byte) error {
	js, e := json.Marshal(d)
	if e != nil {
		return e
	}
	for len(js)%4 != 0 {
		js = append(js, ' ')
	}
	size := uint64(20) + uint64(len(js))
	if len(bin) > 0 {
		size += 8 + uint64(len(bin)) + (4-uint64(len(bin))%4)%4
	}
	if size > math.MaxUint32 {
		return errors.New("exchange: GLB exceeds uint32 length")
	}
	out := bufio.NewWriter(w)
	var header [12]byte
	copy(header[:4], "glTF")
	binary.LittleEndian.PutUint32(header[4:], 2)
	binary.LittleEndian.PutUint32(header[8:], uint32(size))
	if _, e = out.Write(header[:]); e != nil {
		return e
	}
	chunk := func(kind uint32, data []byte, pad byte) error {
		n := len(data)
		padding := (4 - n%4) % 4
		var h [8]byte
		binary.LittleEndian.PutUint32(h[:], uint32(n+padding))
		binary.LittleEndian.PutUint32(h[4:], kind)
		if _, e := out.Write(h[:]); e != nil {
			return e
		}
		if _, e := out.Write(data); e != nil {
			return e
		}
		for i := 0; i < padding; i++ {
			if e := out.WriteByte(pad); e != nil {
				return e
			}
		}
		return nil
	}
	if e = chunk(0x4e4f534a, js, ' '); e != nil {
		return e
	}
	if len(bin) > 0 {
		if e = chunk(0x004e4942, bin, 0); e != nil {
			return e
		}
	}
	return out.Flush()
}

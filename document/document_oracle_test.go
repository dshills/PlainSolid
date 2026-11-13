package document_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"plainsolid/document"
)

type countReader struct {
	r io.Reader
	n int
}

func (r *countReader) Read(p []byte) (int, error) { n, e := r.r.Read(p); r.n += n; return n, e }

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }
func TestReadByteBudgetOracle(t *testing.T) {
	const input = `{"version":1,"state":{"LengthUnit":1}}`
	for _, limit := range []int64{int64(len(input)), int64(len(input) + 1)} {
		d, e := document.Read(strings.NewReader(input), limit, 0)
		if e != nil {
			t.Fatal(limit, e)
		}
		s, _ := d.Snapshot()
		if e = s.Validate(); e != nil {
			t.Fatal(e)
		}
	}
	r := &countReader{r: strings.NewReader(strings.Repeat("x", 1000))}
	if _, e := document.Read(r, 10, 0); e == nil {
		t.Fatal("limit accepted")
	}
	if r.n != 11 {
		t.Fatalf("read %d bytes for a 10-byte budget", r.n)
	}
	for _, limit := range []int64{-1, math.MaxInt64} {
		if _, e := document.Read(strings.NewReader(input), limit, 0); e == nil {
			t.Fatal(limit)
		}
	}
	sentinel := errors.New("reader failed")
	if _, e := document.Read(failingReader{sentinel}, 100, 0); !errors.Is(e, sentinel) {
		t.Fatal(e)
	}
}
func TestReadDocumentInvalidOracle(t *testing.T) {
	for _, s := range []string{`{"version":1,"state":{"LengthUnit":1,"Unknown":[]}}`, `{"version":1,"state":{"LengthUnit":0}}`, `{"version":2,"state":{"LengthUnit":1}}`, `{"version":1,"state":{"LengthUnit":1}} {}`, `{"version":1,"state":{"LengthUnit":1,"Assets":{"mesh":{"Vertices":[{"X":1}],"Triangles":[[0,1,2]]}}}}`} {
		if _, e := document.Read(strings.NewReader(s), 4096, 0); e == nil {
			t.Fatalf("accepted %s", s)
		}
	}
	if _, e := document.Read(bytes.NewReader(nil), 100, 0); e == nil {
		t.Fatal("empty input")
	}
}
func TestNilStreamOracle(t *testing.T) {
	if _, e := document.Read(nil, 100, 0); e == nil {
		t.Fatal("nil reader")
	}
	d, e := document.New(document.State{LengthUnit: 1}, 0)
	if e != nil {
		t.Fatal(e)
	}
	if e = d.Write(nil); e == nil {
		t.Fatal("nil writer")
	}
}

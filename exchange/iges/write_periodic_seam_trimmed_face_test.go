package iges

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"plainsolid/geom"
)

func TestWritePeriodicSeamTrimmedFace(t *testing.T) {
	raw := exactPeriodicSeamFixture(t)
	labeled := replaceField(string(raw), 'D', 2, 7, "SEAMFACE")
	labeled = replaceField(labeled, 'D', 26, 8, "17")
	ctx := context.Background()
	for _, input := range [][]byte{raw, []byte(labeled)} {
		f, err := ReadPeriodicSeamTrimmedFace(ctx, bytes.NewReader(input), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		var first, second bytes.Buffer
		if err := WritePeriodicSeamTrimmedFace(ctx, &first, f); err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(input, first.Bytes()) || f.OriginalIGES() != string(input) {
			t.Fatal("expected a separate canonical carrier and the exact original source")
		}
		comparePeriodicTrimRecords(t, input, first.Bytes())
		again, err := ReadPeriodicSeamTrimmedFace(ctx, bytes.NewReader(first.Bytes()), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		if err := WritePeriodicSeamTrimmedFace(ctx, &second, again); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
			t.Fatal("canonical split face did not roundtrip byte-for-byte", err)
		}
		again.original = f.original
		if !reflect.DeepEqual(again, f) {
			t.Fatal("canonical output changed source ownership, chart winding, or native geometry")
		}
	}
}

func comparePeriodicTrimRecords(t *testing.T, input, output []byte) {
	t.Helper()
	before := periodicTrimmedRecords(t, input)
	after := periodicTrimmedRecords(t, output)
	if before.unit != after.unit || len(before.entities) != len(after.entities) {
		t.Fatal("canonical output changed file units or directory ownership")
	}
	for i, old := range before.entities {
		q := after.entities[i]
		if q.id != old.id || q.kind != old.kind || q.label != old.label || q.subscript != old.subscript {
			t.Fatal("canonical output changed directory IDs or metadata", old.id, q.id)
		}
		if len(q.values) != len(old.values)+2 || q.values[len(q.values)-2].text != "0" || q.values[len(q.values)-1].text != "0" {
			t.Fatal("canonical output changed the finite record layouts", old.id)
		}
		for j, a := range old.values {
			x, err := igesPlanarExact(a)
			if err != nil {
				t.Fatal(err)
			}
			y, err := igesPlanarExact(q.values[j])
			if err != nil || x.Cmp(y) != 0 {
				t.Fatal("canonical output changed an expanded weight, pole, station or owner link", old.id, j)
			}
		}
	}
}

func TestWritePeriodicSeamTrimmedFaceReject(t *testing.T) {
	ctx := context.Background()
	raw := exactPeriodicSeamFixture(t)
	read := func(data []byte) PeriodicSeamTrimmedFace {
		t.Helper()
		f, err := ReadPeriodicSeamTrimmedFace(ctx, bytes.NewReader(data), Limits{})
		if err != nil {
			t.Fatal("negative writer control must first pass exact source proof", err)
		}
		return f
	}
	refuse := func(f PeriodicSeamTrimmedFace, detail string) {
		t.Helper()
		out := bytes.NewBufferString("untouched")
		err := WritePeriodicSeamTrimmedFace(ctx, out, f)
		if !errors.Is(err, geom.ErrUnrepresentable) || !strings.Contains(err.Error(), detail) || out.String() != "untouched" {
			t.Fatal("unsafe or altered exact face did not fail atomically", err)
		}
	}
	for _, mutate := range []func(*PeriodicSeamTrimmedFace){
		func(f *PeriodicSeamTrimmedFace) { f.Edges[2].UVSourceID = f.Edges[1].UVSourceID },
		func(f *PeriodicSeamTrimmedFace) { f.Edges[2].UVStart[0].Add(f.Edges[2].UVStart[0], big.NewRat(1, 100)) },
		func(f *PeriodicSeamTrimmedFace) { f.Edges[2].ModelParameter.Lower.SetInt64(2) },
		func(f *PeriodicSeamTrimmedFace) { f.ProjectionOrientation = -1 },
		func(f *PeriodicSeamTrimmedFace) { f.Cells[1].Patch = f.Cells[0].Patch },
	} {
		f := read(raw)
		mutate(&f)
		refuse(f, "no longer matches its original proof")
	}
	for _, tc := range []struct {
		lo, hi, error string
	}{
		{"2." + strings.Repeat("0", 68) + "1", "3.", "bounded exact decimal"},
		{"2.", "2.0000000000000004", "collapse in binary64 transfer"},
	} {
		source := editPeriodicTrimWriterSource(t, raw, 25, func(values []string) []string {
			for i := 0; i < 3; i++ {
				values[7+i], values[10+i] = tc.lo, tc.hi
			}
			values[25], values[26] = tc.lo, tc.hi
			return values
		})
		refuse(read(source), tc.error)
	}
	// A degree elevation times a positive affine homogeneous weight
	// factor keeps this entire line exact in normalized parameter.
	almostPolynomial := editPeriodicTrimWriterSource(t, raw, 23, func([]string) []string {
		return strings.Split("126,2,2,0,0,0,0,0.,0.,0.,1.,1.,1.,0.999999999999,1.,1.000000000001,5.46875,1.,3.,5.46875,1.,2.000000000001,5.46875,1.,1.,0.,1.,0.,0.,0.", ",")
	})
	refuse(read(almostPolynomial), "rational weights lost")
	f := read(raw)
	f.original = string(periodicTrimmedFixture(t, true))
	refuse(f, "17 source owners")
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	var result bytes.Buffer
	f = read(raw)
	if err := WritePeriodicSeamTrimmedFace(cancelled, &result, f); !errors.Is(err, context.Canceled) || result.Len() != 0 {
		t.Fatal("cancelled writer published prefix", err)
	}
	if err := WritePeriodicSeamTrimmedFace(ctx, periodicTrimShortWriter{}, f); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal("short write was reported as success", err)
	}
}

type periodicTrimShortWriter struct{}

func (periodicTrimShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func editPeriodicTrimWriterSource(t *testing.T, raw []byte, id int, edit func([]string) []string) []byte {
	t.Helper()
	source := periodicTrimmedRecords(t, raw)
	var a topologyAuthor
	root, outer := 0, 10500
	for _, r := range source.entities {
		values := make([]string, len(r.values))
		for i, q := range r.values {
			values[i] = q.text
		}
		if r.id == id {
			values = edit(values)
		}
		a.add(r.kind, 0, joinTopologyValues(values))
		emitted := &a.records[len(a.records)-1]
		if r.kind == 144 {
			emitted.status = &root
		}
		if r.kind == 142 {
			emitted.status = &outer
		}
	}
	changed, err := a.file(source.unit)
	if err != nil {
		t.Fatal(err)
	}
	return changed
}

func TestWritePeriodicSeamTrimmedFacePinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_PERIODIC_128_TRIM_IGES")
	if oracle == "" {
		t.Skip("optional pinned OCCT exact type-144 face reader")
	}
	raw := exactPeriodicSeamFixture(t)
	ctx := context.Background()
	f, err := ReadPeriodicSeamTrimmedFace(ctx, bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := WritePeriodicSeamTrimmedFace(ctx, &out, f); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	before, after := filepath.Join(dir, "original.iges"), filepath.Join(dir, "canonical.iges")
	if err := os.WriteFile(before, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(after, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	original := periodicTrimmedOracle(t, oracle, "read-edges", before)
	got := periodicTrimmedOracle(t, oracle, "read-edges", after)
	if !got.Valid || !got.PeriodicU || got.Topology != [4]int{1, 1, 6, 6} || len(got.Edges) != 6 ||
		len(got.Samples) != 5 || !reflect.DeepEqual(got, original) {
		t.Fatal("pinned reader did not preserve the six complete original edges and physical chart jets", got)
	}
	var doubledArea float64
	for _, e := range got.Edges {
		doubledArea += e.UVStart[0]*e.UVEnd[1] - e.UVEnd[0]*e.UVStart[1]
	}
	if doubledArea != 2 || f.ProjectionAxes != [2]int{1, 2} || f.ProjectionOrientation != 1 {
		t.Fatal("pinned reader changed the lifted wire winding or orientation")
	}
}

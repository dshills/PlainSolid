package step

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"
)

type mappedShortWriter struct{}

func (mappedShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
func TestEditedMappedPlanarGraph(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/occt-multiple-mapped-planar-boxes.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadMappedPlanarMaterials(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	uses := []MappedPlanarOccurrence{}
	for _, id := range a.MapOrder {
		uses = append(uses, a.Materials[id].Occurrences...)
	}
	for _, test := range []string{"nil", "duplicate", "target", "missing", "nan", "name"} {
		t.Run(test, func(t *testing.T) {
			edits := append([]MappedPlanarOccurrence(nil), uses...)
			source := a.Source
			switch test {
			case "nil":
				source = nil
			case "duplicate":
				edits[1] = edits[0]
			case "target":
				edits[0].TargetID++
			case "missing":
				edits = edits[1:]
			case "nan":
				edits[0].Transform[0][0] = 0
			case "name":
				edits[0].Name = string([]byte{255})
			}
			var dst bytes.Buffer
			if e := WriteEditedMappedPlanarMaterials(&dst, source, "new", edits, Limits{}); e == nil || dst.Len() != 0 {
				t.Fatal(e, dst.Len())
			}
		})
	}
	if e = WriteEditedMappedPlanarMaterials(mappedShortWriter{}, a.Source, "new", uses, Limits{}); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
	// Non-short decimal binary inputs are emitted as exact finite decimals.
	uses[0].Transform[0][3] = .1
	var dst bytes.Buffer
	if e = WriteEditedMappedPlanarMaterials(&dst, a.Source, "decimal", uses, Limits{}); e != nil {
		t.Fatal(e)
	}
	b, e := ReadMappedPlanarMaterials(&dst, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, m := range b.Materials {
		for _, u := range m.Occurrences {
			if u.ItemID == uses[0].ItemID {
				found = u.Transform == uses[0].Transform
			}
		}
	}
	if !found {
		t.Fatal("represented translation lost")
	}
}

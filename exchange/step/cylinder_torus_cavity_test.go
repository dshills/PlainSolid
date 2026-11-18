package step

import (
	"bytes"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCylinderTorusCavitySTEP(t *testing.T) {
	t.Parallel()
	b := CylinderTorusCavityBody{"cavity", 5, 2, 8, -3, 4}
	var out bytes.Buffer
	if e := WriteCylinderTorusCavity(&out, b, .001); e != nil {
		t.Fatal(e)
	}
	for _, token := range []string{"BREP_WITH_VOIDS", "ORIENTED_CLOSED_SHELL", "TOROIDAL_SURFACE", "CYLINDRICAL_SURFACE"} {
		if !strings.Contains(out.String(), token) {
			t.Fatal(token)
		}
	}
	f, e := Parse(bytes.NewReader(out.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	shells := 0
	for _, en := range f.Entities {
		for _, r := range en.Records {
			if r.Type == "CLOSED_SHELL" {
				shells++
			}
			if r.Type == "ORIENTED_CLOSED_SHELL" && r.Args[3].Text != "F" {
				t.Fatal("inward")
			}
		}
	}
	if shells != 2 {
		t.Fatal(shells)
	}
	var again bytes.Buffer
	WriteCylinderTorusCavity(&again, b, .001)
	if !bytes.Equal(out.Bytes(), again.Bytes()) {
		t.Fatal("nondeterministic")
	}
	for _, c := range []float64{7, 1 << 21} {
		bad := b
		bad.CylinderRadius = c
		var dst bytes.Buffer
		if e := WriteCylinderTorusCavity(&dst, bad, .001); e == nil || dst.Len() != 0 {
			t.Fatal("atomic domain")
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_CAVITY_STEP"); oracle != "" {
		reloaded, e := ReadCylinderTorusCavity(bytes.NewReader(out.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		var reexport bytes.Buffer
		if e = reloaded.WriteSTEP(&reexport); e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(out.Bytes(), reexport.Bytes()) {
			t.Fatal("paired output")
		}
		path := filepath.Join(t.TempDir(), "cavity.step")
		os.WriteFile(path, out.Bytes(), 0600)
		if log, e := exec.Command(oracle, path).CombinedOutput(); e != nil {
			t.Fatal(e, string(log))
		}
	}
}

func TestCylinderTorusCavityPairedImport(t *testing.T) {
	t.Parallel()
	b := CylinderTorusCavityBody{"observed", 5, 2, 8, -3, 4}
	var raw bytes.Buffer
	WriteCylinderTorusCavity(&raw, b, .001)
	imported, e := ReadCylinderTorusCavity(bytes.NewReader(raw.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if imported.Source() != b {
		t.Fatal("source")
	}
	var again bytes.Buffer
	if e = imported.WriteSTEP(&again); e != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("roundtrip", e)
	}
	f, _ := Parse(bytes.NewReader(raw.Bytes()), Limits{})
	for _, typ := range []string{"ORIENTED_CLOSED_SHELL", "PCURVE", "TOROIDAL_SURFACE", "ADVANCED_FACE", "BREP_WITH_VOIDS"} {
		copy, _ := Parse(bytes.NewReader(raw.Bytes()), Limits{})
		changed := false
		for _, id := range copy.Order {
			en := copy.Entities[id]
			if len(en.Records) != 1 || en.Records[0].Type != typ {
				continue
			}
			r := en.Records[0]
			switch typ {
			case "ORIENTED_CLOSED_SHELL":
				r.Args[3].Text = "T"
			case "PCURVE":
				r.Args[1] = Value{Kind: Reference, Ref: id}
			case "TOROIDAL_SURFACE":
				r.Args[2] = Value{Kind: Number, Text: "5.000000000000000000000000001"}
			case "ADVANCED_FACE":
				if r.Args[3].Text == "T" {
					r.Args[3].Text = "F"
				} else {
					r.Args[3].Text = "T"
				}
			case "BREP_WITH_VOIDS":
				r.Args[2] = Value{Kind: Aggregate}
			}
			en.Records[0] = r
			copy.Entities[id] = en
			changed = true
			break
		}
		if !changed {
			t.Fatal(typ)
		}
		if _, e = ReadCylinderTorusCavity(bytes.NewReader(serializeWrappedProofFile(copy)), Limits{}); e == nil {
			t.Fatal("accepted", typ)
		}
	}
	// A detached, otherwise valid extra geometry entity must not be ignored.
	id := int64(9999)
	f.Entities[id] = Entity{ID: id, Records: []Record{{Type: "CARTESIAN_POINT", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: []Value{{Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}}}}}}}
	f.Order = append(f.Order, id)
	if _, e = ReadCylinderTorusCavity(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
		t.Fatal("detached")
	}
}

func TestCylinderTorusCavityDyadic(t *testing.T) {
	t.Parallel()
	b := CylinderTorusCavityBody{"dyadic", 5.125, 2.25, 8.5, -3.25, 4.5}
	var out bytes.Buffer
	if e := WriteCylinderTorusCavity(&out, b, .001); e != nil {
		t.Fatal(e)
	}
	got, e := ReadCylinderTorusCavity(bytes.NewReader(out.Bytes()), Limits{})
	if e != nil || got.Source() != b {
		t.Fatal(got.Source(), e)
	}
	var round bytes.Buffer
	got.WriteSTEP(&round)
	if !bytes.Equal(round.Bytes(), out.Bytes()) {
		t.Fatal("canonical")
	}
	for _, bad := range []CylinderTorusCavityBody{{"rounded sum", 5, .1, 8, -3, 4}, {"near contact", 5, 2, math.Nextafter(7, 8), -3, 4}, {"rounded height", 5, 2, 8, -3, math.Nextafter(4, 0)}} {
		var dst bytes.Buffer
		if e := WriteCylinderTorusCavity(&dst, bad, .001); e == nil || dst.Len() != 0 {
			t.Fatal("accepted", bad)
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_CAVITY_STEP"); oracle != "" {
		path := filepath.Join(t.TempDir(), "dyadic.step")
		os.WriteFile(path, round.Bytes(), 0600)
		args := []string{path}
		for _, v := range []float64{b.MajorRadius, b.MinorRadius, b.CylinderRadius, b.Lower, b.Upper} {
			args = append(args, strconv.FormatFloat(v, 'g', -1, 64))
		}
		if log, e := exec.Command(oracle, args...).CombinedOutput(); e != nil {
			t.Fatal(e, string(log))
		}
	}
}

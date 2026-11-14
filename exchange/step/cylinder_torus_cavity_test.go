package step

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCylinderTorusCavitySTEP(t *testing.T) {
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
	for _, c := range []float64{7, 8.5} {
		bad := b
		bad.CylinderRadius = c
		var dst bytes.Buffer
		if e := WriteCylinderTorusCavity(&dst, bad, .001); e == nil || dst.Len() != 0 {
			t.Fatal("atomic domain")
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_CAVITY_STEP"); oracle != "" {
		path := filepath.Join(t.TempDir(), "cavity.step")
		os.WriteFile(path, out.Bytes(), 0600)
		if log, e := exec.Command(oracle, path).CombinedOutput(); e != nil {
			t.Fatal(e, string(log))
		}
	}
}

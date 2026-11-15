package step

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const occtPlanarContactPCurves = "testdata/occt-planar-contact-pcurves.step"

func checkPlanarContactPCurveTransfer(t *testing.T, source []byte) {
	t.Helper()
	ctx := context.Background()
	o := VertexContactSTEPOptions{}
	m, proof, err := ReadVertexContactAssembly(ctx, bytes.NewReader(source), o)
	if err != nil || len(m.Bodies) != 3 || len(proof.Contacts) != 1 || proof.SignedVolume.RatString() != "3" {
		t.Fatalf("original OCCT pcurve transfer: %+v %v", proof, err)
	}
	contact := proof.Contacts[0]
	want := [3]float64{1, 1, 1}
	a := m.Bodies[contact.A.Solid].Solid.Vertices()[contact.A.Vertex]
	b := m.Bodies[contact.B.Solid].Solid.Vertices()[contact.B.Vertex]
	if contact.A.Solid != 0 || contact.B.Solid != 1 || [3]float64{a.X, a.Y, a.Z} != want || a != b {
		t.Fatalf("contact did not preserve original point and body incidence: %+v %v %v", contact, a, b)
	}
	var canonical bytes.Buffer
	if _, err := WriteVertexContactAssembly(ctx, &canonical, m, o); err != nil {
		t.Fatal("canonical direct support rewrite:", err)
	}
	m2, proof2, err := ReadVertexContactAssembly(ctx, &canonical, o)
	if err != nil || !sameSTEPContactGeometry(m, proof, m2, proof2) {
		t.Fatalf("canonical rewrite changed transferred contact geometry: %v", err)
	}
}

func TestPlanarVertexContactAssemblySTEPOCCTOriginalPCurves(t *testing.T) {
	source, err := os.ReadFile(occtPlanarContactPCurves)
	if err != nil {
		t.Fatal(err)
	}
	checkPlanarContactPCurveTransfer(t, source)
	for _, c := range []struct{ name, old, changed, message string }{
		{"near-lift", "#39 = CARTESIAN_POINT('',(0.,0.));", "#39 = CARTESIAN_POINT('',(0.,1.E-18));", "does not lift"},
		{"reversed-lift", "#41 = DIRECTION('',(1.,0.));", "#41 = DIRECTION('',(-1.,0.));", "does not lift"},
		{"missing-owner", "#26 = SURFACE_CURVE('',#27,(#31,#43),.PCURVE_S1.);", "#26 = SURFACE_CURVE('',#27,(#31),.PCURVE_S1.);", "requires both"},
		{"detached-owner", "#31 = PCURVE('',#32,#37);", "#1900 = PLANE('',#33);\n#31 = PCURVE('',#1900,#37);", "not a distinct owner"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if !bytes.Contains(source, []byte(c.old)) {
				t.Fatal("original fixture no longer contains the expected entity")
			}
			mutant := bytes.Replace(source, []byte(c.old), []byte(c.changed), 1)
			if m, err := Read(bytes.NewReader(mutant), Limits{}); err != nil || len(m.Bodies) != 3 {
				t.Fatalf("ordinary numeric transfer must admit this control: %v", err)
			}
			m, proof, err := ReadVertexContactAssembly(context.Background(), bytes.NewReader(mutant), VertexContactSTEPOptions{})
			if err == nil || !bytes.Contains([]byte(err.Error()), []byte(c.message)) || m.Bodies != nil || proof.SignedVolume != nil {
				t.Fatalf("changed original owner yielded a partial compound: %+v %v", proof, err)
			}
		})
	}
}

func TestPlanarVertexContactAssemblySTEPOCCTLivePCurves(t *testing.T) {
	author := os.Getenv("PLAINSOLID_OCCT_PLANAR_CONTACT_AUTHOR")
	if author == "" {
		t.Skip("optional pinned OCCT planar contact author")
	}
	file := filepath.Join(t.TempDir(), "independent-planar-contact.step")
	if output, err := exec.Command(author, file).CombinedOutput(); err != nil {
		t.Fatalf("independent STEP author failed: %v\n%s", err, output)
	}
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	checkPlanarContactPCurveTransfer(t, source)
}

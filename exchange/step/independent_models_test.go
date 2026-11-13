package step

import (
	"bytes"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestIndependentModelsPreserveNamesAndRejectDetachedGeometry(t *testing.T) {
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	var source bytes.Buffer
	if e := WriteCylinders(&source, CylinderModel{LengthUnit: .001, Bodies: []CylinderBody{{Name: "A '#42' \\ circle", Frame: f, Radius: 2, Height: 3}}}); e != nil {
		t.Fatal(e)
	}
	var joined bytes.Buffer
	if e := JoinIndependentModels(&joined, [][]byte{source.Bytes(), source.Bytes()}, Limits{}); e != nil {
		t.Fatal(e)
	}
	parts, e := SplitIndependentModels(&joined, Limits{})
	if e != nil || len(parts) != 2 {
		t.Fatal(e, len(parts))
	}
	for _, p := range parts {
		m, e := ReadAnalyticBodies(bytes.NewReader(p), Limits{})
		if e != nil || len(m.Bodies) != 1 || m.Bodies[0].Name != "A '#42' \\ circle" {
			t.Fatal(m, e)
		}
	}
	bad := strings.Replace(source.String(), "ENDSEC;\nEND-ISO-10303-21;", "#999999=CARTESIAN_POINT('',(9.,9.,9.));\nENDSEC;\nEND-ISO-10303-21;", 1)
	if _, e = SplitIndependentModels(strings.NewReader(bad), Limits{}); e == nil {
		t.Fatal("detached geometry dropped")
	}
}

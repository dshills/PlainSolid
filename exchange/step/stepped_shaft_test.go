package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestSteppedShaftSTEP(t *testing.T) {
	f, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	base := trimmed.SteppedShaft{Frame: f, ShaftRadius: 1.5, HeadRadius: 2.75, ShaftLength: 12, HeadHeight: 3}
	socket := base
	r := 2.5 / math.Sqrt(3)
	for i := range 6 {
		a := float64(i) * math.Pi / 3
		socket.Socket = append(socket.Socket, geom.Vec2{X: r * math.Cos(a), Y: r * math.Sin(a)})
	}
	socket.SocketFloor = 1.5
	for _, p := range []trimmed.SteppedShaft{base, socket} {
		m := SteppedShaftModel{Name: "shaft", Body: p, LengthUnit: .001}
		var file bytes.Buffer
		if err := WriteSteppedShaft(&file, m); err != nil {
			t.Fatal(len(p.Socket), err)
		}
		got, err := ReadSteppedShaft(bytes.NewReader(file.Bytes()), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		var again bytes.Buffer
		if err = WriteSteppedShaft(&again, got); err != nil || !bytes.Equal(file.Bytes(), again.Bytes()) {
			t.Fatal(err)
		}
		changed := strings.Replace(file.String(), "CYLINDRICAL_SURFACE", "CONICAL_SURFACE", 1)
		if _, err := ReadSteppedShaft(strings.NewReader(changed), Limits{}); err == nil {
			t.Fatal("accepted substituted support")
		}
		if os.Getenv("PLAINSOLID_OCCT_STEP") != "1" || os.Getenv("PLAINSOLID_OCCT_ORACLE") == "" {
			continue
		}
		dir := t.TempDir()
		path, report := filepath.Join(dir, "shaft.step"), filepath.Join(dir, "measures.json")
		if err := os.WriteFile(path, file.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		stepOracle(t, report, "step-mass-precise", path)
		data, err := os.ReadFile(report)
		if err != nil {
			t.Fatal(err)
		}
		volume := math.Pi * (1.5*1.5*12 + 2.75*2.75*3)
		area := 2 * math.Pi * (1.5*12 + 2.75*3 + 2.75*2.75)
		if len(p.Socket) > 0 {
			volume -= math.Sqrt(3) / 2 * 2.5 * 2.5 * 1.5
			area += 6 * r * 1.5
		}
		var props struct{ Volume, Area float64 }
		if err := json.Unmarshal(data, &props); err != nil || math.Abs(props.Volume-volume) > 1e-7 || math.Abs(props.Area-area) > 1e-7 {
			t.Fatal(props, volume, area, err)
		}
	}
}

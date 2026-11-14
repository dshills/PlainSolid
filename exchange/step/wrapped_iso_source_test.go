package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWrappedIsoSourceDecimals(t *testing.T) {
	raw, err := os.ReadFile("testdata/wrapped-tube.step")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ReadWrappedTube(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"0.50000000000000000001", "0.49999999999999999999", "1.E-999999999"} {
		t.Run(value, func(t *testing.T) {
			bad := strings.Replace(string(raw), "#266 = CARTESIAN_POINT('',(0.,0.5));", "#266 = CARTESIAN_POINT('',(0.,"+value+"));", 1)
			if bad == string(raw) {
				t.Fatal("stale source mutation")
			}
			if _, err := ReadWrappedTube(strings.NewReader(bad), Limits{}, WrappedSurfaceOptions{}); err == nil {
				t.Fatal("accepted altered original coefficient")
			}
		})
	}
}

func TestWrappedIsoSourceKnots(t *testing.T) {
	raw, err := os.ReadFile("testdata/wrapped-tube.step")
	if err != nil {
		t.Fatal(err)
	}
	original := "#264 = B_SPLINE_CURVE_WITH_KNOTS('',2,(#265,#266,#267),.UNSPECIFIED.,.F.\n  ,.F.,(3,3),(-2.,3.),.PIECEWISE_BEZIER_KNOTS.);"
	for _, token := range []string{"3.00000000000000000001", "2.99999999999999999999"} {
		bad := strings.Replace(string(raw), original, strings.Replace(original, "(-2.,3.)", "(-2.,"+token+")", 1), 1)
		if bad == string(raw) {
			t.Fatal("stale knot mutation")
		}
		if _, err := ReadWrappedTube(strings.NewReader(bad), Limits{}, WrappedSurfaceOptions{}); err == nil {
			t.Fatal("accepted altered knot")
		}
	}
}

func TestWrappedIsoSourceNumber(t *testing.T) {
	for _, s := range []string{"0", "0.5", "5.E-1", "-0.125", "2."} {
		if _, err := wrappedIsoSourceNumber(Value{Kind: Number, Text: s}); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	for _, s := range []string{"0.1", "1.00000000000000000001", "1.E-999999999", "1.E999999999"} {
		if _, err := wrappedIsoSourceNumber(Value{Kind: Number, Text: s}); err == nil {
			t.Fatalf("accepted %s", s)
		}
	}
}

func TestWrappedIsoSourceLiveOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_FRUSTUM_HIERARCHY")
	if bin == "" {
		t.Skip("set PLAINSOLID_OCCT_FRUSTUM_HIERARCHY for adaptive original/output transfer")
	}
	raw, e := os.ReadFile("testdata/wrapped-tube.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedTube(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteWrappedTube(&out, m); e != nil {
		t.Fatal(e)
	}
	var reports [2]struct {
		Area, Volume float64
		Solids       int
	}
	for i, b := range [][]byte{raw, out.Bytes()} {
		path := filepath.Join(t.TempDir(), "tube.step")
		if e = os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		result, err := exec.CommandContext(ctx, bin, "read", path).CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("%v: %s", err, result)
		}
		at := bytes.IndexByte(result, '{')
		if at < 0 || json.Unmarshal(result[at:], &reports[i]) != nil {
			t.Fatal(string(result))
		}
		if reports[i].Solids != 1 || reports[i].Volume <= 0 {
			t.Fatal(reports[i])
		}
	}
	if math.Abs(reports[0].Area-reports[1].Area) > 1e-8 || math.Abs(reports[0].Volume-reports[1].Volume) > 1e-8 {
		t.Fatal(reports)
	}
}

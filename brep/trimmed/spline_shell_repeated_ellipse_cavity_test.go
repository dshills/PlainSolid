package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestOCCTTangentEllipseCavityCut(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_TANGENT_CAVITY")
	if bin == "" {
		t.Skip("set PLAINSOLID_OCCT_TANGENT_CAVITY to the independent author")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, fixture := range []struct {
		mode   string
		common float64
	}{
		{"repeated-ellipse-cavity-cut", 20227.0 / 20480},
		{"rotated-ellipse-cavity-cut", 20207.0 / 20480},
	} {
		path := filepath.Join(t.TempDir(), "cavity.jsonl")
		if data, e := exec.CommandContext(ctx, bin, path, fixture.mode).CombinedOutput(); e != nil {
			t.Fatalf("independent OCCT cavity stock Cut: %v %s", e, data)
		}
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		d := json.NewDecoder(bytes.NewReader(raw))
		for _, q := range []struct {
			name   string
			volume float64
		}{{"contained-tangent-common", fixture.common}, {"contained-tangent-fuse", 25.0 / 8},
			{"contained-tangent-cavity", 25.0/8 - fixture.common}} {
			var got struct {
				Name   string
				Volume float64
			}
			if e := d.Decode(&got); e != nil || got.Name != q.name || math.Abs(got.Volume-q.volume) > 1e-10 {
				t.Fatal("independently authored stock and cavity volume", q, got, e)
			}
		}
		if e := d.Decode(new(any)); e != io.EOF {
			t.Fatal("unexpected reference records", e)
		}
	}
}

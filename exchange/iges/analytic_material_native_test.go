package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"testing"
)

func TestAnalyticMaterialNative(t *testing.T) {
	ctx := context.Background()
	for _, fixture := range analyticDispatchFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			raw, e := os.ReadFile("testdata/loop-start-placed-" + fixture.name + ".igs")
			if e != nil {
				t.Fatal(e)
			}
			m, e := ReadAnalyticMaterial(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
			if e != nil {
				t.Fatal(e)
			}
			var b bytes.Buffer
			if e = WriteAnalyticMaterialJSON(ctx, &b, m); e != nil {
				t.Fatal(e)
			}
			p, e := ReadAnalyticMaterialJSON(ctx, bytes.NewReader(b.Bytes()), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			var repeated, source bytes.Buffer
			if e = WriteAnalyticMaterialJSON(ctx, &repeated, p); e != nil || !bytes.Equal(b.Bytes(), repeated.Bytes()) {
				t.Fatal("canonical envelope", e)
			}
			if e = WriteAnalyticMaterialSource(ctx, &source, p); e != nil || !bytes.Equal(raw, source.Bytes()) {
				t.Fatal("source", e)
			}
			for _, mutation := range []string{"tag", "native", "unit", "face", "role", "version", "source", "tolerance"} {
				var q analyticMaterialEnvelope
				if json.Unmarshal(b.Bytes(), &q) != nil {
					t.Fatal("fixture")
				}
				switch mutation {
				case "tag":
					q.Family = "unknown"
				case "native":
					q.Native = json.RawMessage(`{}`)
				case "unit":
					q.LengthUnit *= 1000
				case "face":
					q.NativeFaceSourceIDs[0] = 999
				case "role":
					for k := range q.SourceRoles {
						q.SourceRoles[k] = 999
						break
					}
				case "version":
					q.Version++
				case "source":
					q.Source = ""
				case "tolerance":
					q.Tolerance = -1
				}
				bad, _ := json.Marshal(q)
				v, e := ReadAnalyticMaterialJSON(ctx, bytes.NewReader(bad), Limits{})
				if e == nil || !reflect.DeepEqual(v, AnalyticMaterial{}) {
					t.Fatal("forged", mutation)
				}
			}
			unknown := bytes.TrimSpace(b.Bytes())
			unknown = append(append([]byte(nil), unknown[:len(unknown)-1]...), []byte(`,"IgnoredClaim":true}`)...)
			if _, e = ReadAnalyticMaterialJSON(ctx, bytes.NewReader(unknown), Limits{}); e == nil {
				t.Fatal("unknown field")
			}
			during, cancelDuring := context.WithCancel(ctx)
			if _, e = ReadAnalyticMaterialJSON(during, &analyticCancelReader{Reader: bytes.NewReader(b.Bytes()), cancel: cancelDuring}, Limits{}); e != context.Canceled {
				t.Fatal("read cancellation", e)
			}

			for _, limit := range []Limits{{MaxBytes: int64(b.Len() - 1)}, {MaxEntities: 1}, {MaxRecords: 2}, {MaxValues: 1}} {
				if _, e = ReadAnalyticMaterialJSON(ctx, bytes.NewReader(b.Bytes()), limit); e == nil {
					t.Fatal("limit")
				}
			}
			for _, suffix := range []string{` {}`, ` garbage`} {
				if _, e = ReadAnalyticMaterialJSON(ctx, bytes.NewReader(append(append([]byte(nil), b.Bytes()...), suffix...)), Limits{}); e == nil {
					t.Fatal("trailing")
				}
			}
			canceled, cancel := context.WithCancel(ctx)
			cancel()
			if _, e = ReadAnalyticMaterialJSON(canceled, bytes.NewReader(b.Bytes()), Limits{}); !errors.Is(e, context.Canceled) {
				t.Fatal(e)
			}
			var reject bytes.Buffer
			if e = WriteAnalyticMaterialJSON(canceled, &reject, m); !errors.Is(e, context.Canceled) || reject.Len() != 0 {
				t.Fatal(e)
			}
			if e = WriteAnalyticMaterialJSON(ctx, analyticShortWriter{}, m); e != io.ErrShortWrite {
				t.Fatal(e)
			}
		})
	}
}

type analyticShortWriter struct{}

func (analyticShortWriter) Write(b []byte) (int, error) { return len(b) - 1, nil }
func TestAnalyticMaterialUnsupportedSignatures(t *testing.T) {
	for _, name := range []string{"cylinder-solid.igs", "through-circular-cylinder.igs", "conical45-cylinder-through.igs"} {
		raw, e := os.ReadFile("testdata/" + name)
		if e != nil {

			t.Fatal(e)
		}
		if name == "conical45-cylinder-through.igs" {
			raw = append(raw, raw...)
		}
		if _, e = ReadAnalyticMaterial(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
			t.Fatal("unsupported", name)
		}
	}
	var reject bytes.Buffer
	if e := WriteAnalyticMaterialJSON(context.Background(), &reject, AnalyticMaterial{}); e == nil || reject.Len() != 0 {
		t.Fatal("zero write")
	}
}
func FuzzAnalyticMaterialNative(f *testing.F) {
	for _, fixture := range analyticDispatchFixtures {
		raw, e := os.ReadFile("testdata/loop-start-placed-" + fixture.name + ".igs")
		if e != nil {
			f.Fatal(e)
		}
		m, e := ReadAnalyticMaterial(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9)
		if e != nil {
			f.Fatal(e)
		}
		var b bytes.Buffer
		if e = WriteAnalyticMaterialJSON(context.Background(), &b, m); e != nil {
			f.Fatal(e)
		}
		f.Add(b.Bytes())
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			return
		}
		_, _ = ReadAnalyticMaterialJSON(context.Background(), bytes.NewReader(b), Limits{})
	})
}

type analyticCancelReader struct {
	io.Reader
	cancel context.CancelFunc
}

func (r *analyticCancelReader) Read(b []byte) (int, error) {
	n, e := r.Reader.Read(b)
	r.cancel()
	return n, e
}

package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

type rationalPoseClaimEdit struct {
	name, old, replacement string
}

func checkRationalPosedRepairJSON[M rationalPosedPCurveRepairMaterial](t *testing.T, source []byte, format, otherFormat string,
	reconstruct rationalPosedPCurveSourceReader[M],
	write func(context.Context, io.Writer, M) error,
	read func(context.Context, io.Reader, IndependentMaterialJSONLimits) (M, error),
	stale func(M) M, edits []rationalPoseClaimEdit) {
	t.Helper()
	ctx := context.Background()
	opts := ConvexArcRepairOptions{MaxDistance: 1e-12}
	m, e := reconstruct(ctx, bytes.NewReader(source), Limits{}, opts)
	if e != nil {
		t.Fatal(e)
	}
	var encoded bytes.Buffer
	if e = write(ctx, &encoded, m); e != nil {
		t.Fatal(e)
	}
	raw := encoded.Bytes()
	var envelope rationalPosedPCurveRepairEnvelope
	if e = json.Unmarshal(raw, &envelope); e != nil || envelope.Format != format ||
		envelope.Version != 1 || !bytes.Equal(envelope.Source, source) || envelope.MaxDistance != opts.MaxDistance {
		t.Fatal("source-only record changed its source, tolerance or format", e)
	}
	if bytes.Contains(raw, []byte(`"Native":`)) || bytes.Contains(raw, []byte(`"CorrectedModel":`)) {
		t.Fatal("expected source and claims only")
	}
	out, e := read(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	wantPose, _ := m.Pose().Matrix()
	pose, e := out.Pose().Matrix()
	if e != nil || !reflect.DeepEqual(pose, wantPose) ||
		out.OriginalSTEP() != string(source) || out.CorrectedModelSTEP() != m.CorrectedModelSTEP() ||
		!reflect.DeepEqual(out.Provenance(), m.Provenance()) || !reflect.DeepEqual(out.PCurves(), m.PCurves()) {
		t.Fatal("reloaded material changed exact original/world evidence", e)
	}
	var again bytes.Buffer
	if e = write(ctx, &again, out); e != nil || !bytes.Equal(raw, again.Bytes()) {
		t.Fatal("reloaded proof changed the canonical record", e)
	}
	var indented bytes.Buffer
	if e = json.Indent(&indented, raw, "", "  "); e != nil {
		t.Fatal(e)
	}
	if _, e = read(ctx, bytes.NewReader(indented.Bytes()), IndependentMaterialJSONLimits{}); e != nil {
		t.Fatal("equivalent JSON whitespace should preserve proof", e)
	}

	reject := func(t *testing.T, bad []byte, l IndependentMaterialJSONLimits) {
		t.Helper()
		got, e := read(ctx, bytes.NewReader(bad), l)
		if e == nil || got.OriginalSTEP() != "" {
			t.Fatal("changed record published material", e)
		}
	}
	station, e := json.Marshal(m.FiniteLineOwners()[0].StartParameter)
	if e != nil {
		t.Fatal(e)
	}
	for _, edit := range append([]rationalPoseClaimEdit{
		{"other family", `"Format":"` + format + `"`, `"Format":"` + otherFormat + `"`},
		{"old schema", `"Format":"` + format + `"`, `"Format":"` + posedGroupedPCurveRepairFormat + `"`},
		{"version", `"Version":1`, `"Version":2`},
		{"schema field", `"Version":1`, `"Version":1,"Native":null`},
		{"duplicate field", `"Version":1`, `"Version":1,"Version":1`},
		{"tolerance", `"MaxDistance":1e-12`, `"MaxDistance":2e-12`},
		{"tight tolerance", `"MaxDistance":1e-12`, `"MaxDistance":1e-14`},
		{"pose", `"Pose":[["0","3/5","4/5"`, `"Pose":[["0","1/5","4/5"`},
		{"untrusted rational exponent", `"Pose":[["0","3/5","4/5"`, `"Pose":[["1e999999999","3/5","4/5"`},
		{"reference circle", `"ReferenceCircleID":146`, `"ReferenceCircleID":147`},
		{"repair site", `"Role":"tangent vertex"`, `"Role":"axial line origin"`},
		{"owner bound", `"DistanceSquaredUpper":"0"`, `"DistanceSquaredUpper":"1"`},
		{"finite range", `"StartParameter":` + string(station), `"StartParameter":"999"`},
	}, edits...) {
		t.Run(edit.name, func(t *testing.T) {
			bad := bytes.Replace(raw, []byte(edit.old), []byte(edit.replacement), 1)
			if bytes.Equal(bad, raw) {
				t.Fatal("mutation missed its claimed field")
			}
			reject(t, bad, IndependentMaterialJSONLimits{})
		})
	}
	t.Run("raw source bytes with identical geometry", func(t *testing.T) {
		changed := append(bytes.Clone(source), '\n')
		if _, e := reconstruct(ctx, bytes.NewReader(changed), Limits{}, opts); e != nil {
			t.Fatal("source whitespace must leave geometry valid", e)
		}
		bad := envelope
		bad.Source = changed
		record, e := json.Marshal(bad)
		if e != nil {
			t.Fatal(e)
		}
		reject(t, record, IndependentMaterialJSONLimits{})
	})
	t.Run("changed source owner", func(t *testing.T) {
		bad := envelope
		bad.Source = bytes.Replace(source, []byte("2.6645352591E-17"), []byte("2.6645352591E-6"), 1)
		if bytes.Equal(bad.Source, source) {
			t.Fatal("source owner mutation missed")
		}
		record, e := json.Marshal(bad)
		if e != nil {
			t.Fatal(e)
		}
		reject(t, record, IndependentMaterialJSONLimits{})
	})
	t.Run("budgets and trailing data", func(t *testing.T) {
		reject(t, raw, IndependentMaterialJSONLimits{MaxBytes: int64(len(raw) - 1)})
		reject(t, raw, IndependentMaterialJSONLimits{Source: Limits{MaxBytes: 64}})
		reject(t, raw, IndependentMaterialJSONLimits{Source: Limits{MaxEntities: 4}})
		reject(t, append(bytes.Clone(raw), []byte("{}")...), IndependentMaterialJSONLimits{})
	})
	t.Run("writer reproof and cancellation", func(t *testing.T) {
		var b bytes.Buffer
		if e := write(ctx, &b, stale(m)); e == nil || b.Len() != 0 {
			t.Fatal("writer published an inconsistent proof", e)
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if e := write(canceled, &b, m); !errors.Is(e, context.Canceled) || b.Len() != 0 {
			t.Fatal("writer cancellation", e)
		}
		got, e := read(canceled, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
		if !errors.Is(e, context.Canceled) || got.OriginalSTEP() != "" {
			t.Fatal("reader cancellation", e)
		}
		if e := write(ctx, sideDiskShortWriter{}, m); !errors.Is(e, io.ErrShortWrite) {
			t.Fatal("short write", e)
		}
	})
}

func TestRationalPosedPCurveRepairSourceJSON(t *testing.T) {
	slowtest.Skip(t)
	for _, fixture := range rationalTiltFixtures {
		t.Run("single"+fixture.suffix, func(t *testing.T) {
			checkRationalPosedRepairJSON(t, rationalTiltSource(t, fixture), rationalPosedPCurveRepairFormat, rationalPosedGroupedPCurveRepairFormat,
				RepairRationalPosedPCurveOCCTConvexArcExtrusion, WriteRationalPosedPCurveRepairedOCCTConvexArcJSON,
				ReadRationalPosedPCurveRepairedOCCTConvexArcJSON,
				func(m RationalPosedPCurveRepairedOCCTConvexArcExtrusion) RationalPosedPCurveRepairedOCCTConvexArcExtrusion {
					m.corrected += "\n"
					return m
				}, nil)
		})
	}
	t.Run("grouped", func(t *testing.T) {
		checkRationalPosedRepairJSON(t, rationalTiltGroupedSource(t), rationalPosedGroupedPCurveRepairFormat, rationalPosedPCurveRepairFormat,
			RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion, WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON,
			ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON,
			func(m RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion {
				m.corrected += "\n"
				return m
			}, []rationalPoseClaimEdit{
				{"axial station ends", `"OriginalParameters":["0",`, `"OriginalParameters":["1",`},
				{"finite cap root", `"EndStation":{"version":1`, `"EndStation":{"version":2`},
				{"rational cap one-turn proof", `"ClosedOneTurn":true`, `"ClosedOneTurn":false`},
			})
	})
}

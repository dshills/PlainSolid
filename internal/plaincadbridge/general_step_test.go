package plaincadbridge

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"plainsolid/internal/slowtest"
	"plainsolid/internal/stepmeasure"
)

// generalSTEPMeasured writes a result of the general Boolean and measures
// the file from its entities alone. The measures must be those the bridge
// published for the shape, within the bound that exportStep applies to a
// read-back. It returns the file, and whether step.WriteTrimmedBoundary
// wrote it.
func generalSTEPMeasured(t *testing.T, s stored) ([]byte, bool) {
	t.Helper()
	data, general, err := writeGeneralSTEPBody(s)
	if err != nil {
		t.Fatal(err)
	}
	if !general {
		return data, false
	}
	got, err := stepmeasure.Boundary(data)
	if err != nil {
		t.Fatal(err)
	}
	near := func(a, b float64) bool {
		return math.Abs(a-b) <= math.Max(1e-7, math.Max(math.Abs(a), math.Abs(b))*1e-8)
	}
	want := s.result
	if !near(got.Volume, want.GeometryAssertions.Volume) || !near(got.Area, want.GeometryAssertions.SurfaceArea) {
		t.Fatalf("the file measures volume %.12g and area %.12g, the shape has %.12g and %.12g", got.Volume, got.Area, want.GeometryAssertions.Volume, want.GeometryAssertions.SurfaceArea)
	}
	lo, hi := [3]float64{got.Box.Min.X, got.Box.Min.Y, got.Box.Min.Z}, [3]float64{got.Box.Max.X, got.Box.Max.Y, got.Box.Max.Z}
	for i := range 3 {
		if !near(lo[i], float64(want.Bounds.Min[i])) || !near(hi[i], float64(want.Bounds.Max[i])) {
			t.Fatalf("the file has box %v, the shape has %+v", got.Box, want.Bounds)
		}
	}
	return data, true
}

// TestGeneralStepExport exports results of the general Boolean. The file is
// written by the general writer and measures as the shape does. The export
// itself is accepted only on what the file reads back as through importStep
// on a default bridge. A result bounded by planes reads back, and the
// re-read shape is the exported one. A result with a cylinder has no reader
// yet: importStep recovers named recipes and has none for a general
// boundary of planes and cylinders, so the export is refused, as unsupported
// and naming the reader, and no unverified file is returned.
func TestGeneralStepExport(t *testing.T) {
	type q = map[string]any
	for _, c := range []struct {
		name  string
		tool  q
		reads bool
	}{
		{"planes only", q{"op": "box", "min": vector{2, 1, 1}, "max": vector{6, 3, 3}}, true},
		{"through bore", q{"op": "circle", "origin": vector{2, 2, -1}, "normal": vector{0, 0, 1}, "radius": 1, "height": 6}, false},
		{"blind bore", q{"op": "circle", "origin": vector{2, 2, 2}, "normal": vector{0, 0, 1}, "radius": 1, "height": 6}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := newGeneral()
			shape(t, b, q{"op": "box", "min": vector{}, "max": vector{4, 4, 4}})
			shape(t, b, c.tool)
			cut := ask(t, b, q{"op": "cut", "a": "ps:1", "b": "ps:2"})
			if !cut.OK {
				t.Fatalf("%+v", cut.Error)
			}
			data, general := generalSTEPMeasured(t, b.shapes["ps:3"])
			if !general {
				t.Fatal("a legacy writer took a general result")
			}
			var export struct {
				OK     bool       `json:"ok"`
				Result stepExport `json:"result"`
				Error  *failure   `json:"error"`
			}
			request, _ := json.Marshal(q{"op": "exportStep", "handles": []string{"ps:3"}})
			if err := json.Unmarshal([]byte(b.Call(string(request))), &export); err != nil {
				t.Fatal(err)
			}
			if !c.reads {
				if export.OK || export.Error.Code != "UNSUPPORTED" || !strings.Contains(export.Error.Message, "no STEP reader") {
					t.Fatalf("export ok=%v, failure %+v; want a refusal that names the reader", export.OK, export.Error)
				}
				if again := ask(t, New(), q{"op": "importStep", "source": string(data)}); again.OK {
					t.Fatal("importStep reads the file; the export need not be refused any more")
				}
				return
			}
			if !export.OK {
				t.Fatalf("%+v", export.Error)
			}
			file, err := base64.StdEncoding.DecodeString(export.Result.Base64)
			if err != nil {
				t.Fatal(err)
			}
			again := ask(t, New(), q{"op": "importStep", "source": string(file)})
			if !again.OK {
				t.Fatalf("%+v", again.Error)
			}
			sameShape(t, again.Result, cut.Result)
		})
	}
}

// TestGeneralStepExportTraces replays the Tier 1 acceptance rows on a
// general bridge and, at every exportStep request, takes each result of the
// general Boolean among its handles. Each must be written, and its file
// must measure as the shape does. The outcomes of the requests themselves
// are logged and not required: until importStep has a reader for a general
// boundary, a request for a result with a cylinder is refused at the
// read-back.
func TestGeneralStepExportTraces(t *testing.T) {
	slowtest.Skip(t)
	bodies, legacy, outcomes := 0, 0, map[string]int{}
	for _, row := range generalBooleanRows(t) {
		if !strings.HasPrefix(row.id, "T1") {
			continue
		}
		replay := generalBooleanReplay{open: newGeneral}
		for _, step := range loadGeneralBooleanTrace(t, row.function) {
			var request struct {
				Op      string   `json:"op"`
				Handles []string `json:"handles"`
			}
			export, general := step.Req != nil && json.Unmarshal(step.Req, &request) == nil && request.Op == "exportStep", false
			if export {
				for _, h := range request.Handles {
					s := replay.bridge.shapes[strings.Trim(replay.handles[`"`+h+`"`], `"`)]
					if s.analytic == nil || !generalBody(s.analytic) {
						continue
					}
					general = true
					bodies++
					if _, written := generalSTEPMeasured(t, s); !written {
						legacy++
					}
				}
			}
			reply, issued := replay.issue(t, step)
			switch {
			case !general || !issued:
			case reply.OK:
				outcomes["exported"]++
			default:
				outcomes[row.id+" "+reply.Error.Code+": "+reply.Error.Message]++
			}
		}
	}
	t.Logf("%d results of the general Boolean at exportStep requests: all written and measured as published, %d of them by a legacy writer", bodies, legacy)
	for outcome, n := range outcomes {
		t.Logf("%d requests: %s", n, outcome)
	}
	if bodies == 0 {
		t.Fatal("no result of the general Boolean at any exportStep request")
	}
}

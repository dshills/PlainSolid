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

// generalSTEPExported issues exportStep for one handle and returns the file.
func generalSTEPExported(t *testing.T, b *Bridge, handle string) string {
	t.Helper()
	var export stepExport
	if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "exportStep", "handles": []string{handle}}), &export); err != nil {
		t.Fatal(err)
	}
	file, err := base64.StdEncoding.DecodeString(export.Base64)
	if err != nil {
		t.Fatal(err)
	}
	return string(file)
}

// TestGeneralStepExport exports results of the general Boolean on a general
// bridge. The file is written by the general writer and measures as the
// shape does; the export succeeds, which means the file read back as the
// shape; and importStep on a general bridge gives the shape again. A
// default bridge reads the file of a result bounded by planes, as it reads
// any planar solid, and refuses one with a cylinder as it did before: the
// reader for a general boundary is on the general bridge only.
func TestGeneralStepExport(t *testing.T) {
	type q = map[string]any
	for _, c := range []struct {
		name   string
		tool   q
		planar bool
	}{
		{"planes only", q{"op": "box", "min": vector{2, 1, 1}, "max": vector{6, 3, 3}}, true},
		{"through bore", q{"op": "circle", "origin": vector{2, 2, -1}, "normal": vector{0, 0, 1}, "radius": 1, "height": 6}, false},
		{"blind bore", q{"op": "circle", "origin": vector{2, 2, 2}, "normal": vector{0, 0, 1}, "radius": 1, "height": 6}, false},
		{"oblique bore", q{"op": "circle", "origin": vector{1, 2, -3}, "normal": vector{0.3, 0, 1}, "radius": 0.7, "height": 12}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := newGeneral()
			shape(t, b, q{"op": "box", "min": vector{}, "max": vector{4, 4, 4}})
			shape(t, b, c.tool)
			cut := shape(t, b, q{"op": "cut", "a": "ps:1", "b": "ps:2"})
			if _, general := generalSTEPMeasured(t, b.shapes[cut.Handle]); !general {
				t.Fatal("a legacy writer took a general result")
			}
			file := generalSTEPExported(t, b, cut.Handle)
			sameShape(t, shape(t, newGeneral(), q{"op": "importStep", "source": file}), cut)
			again := ask(t, New(), q{"op": "importStep", "source": file})
			if c.planar {
				if !again.OK {
					t.Fatalf("%+v", again.Error)
				}
				sameShape(t, again.Result, cut)
			} else if again.OK || again.Error.Code != "UNSUPPORTED" {
				t.Fatalf("a default bridge answers ok=%v, %+v; it has no reader for a general boundary", again.OK, again.Error)
			}
		})
	}
}

// TestGeneralStepImported holds a body that a general bridge imported from
// a file of the general writer to what the exported body does: each request
// is issued to the result of a cut and to its re-import, and must have the
// same outcome, with the same assertions where it answers with a shape.
// The requests that must succeed on both are marked.
func TestGeneralStepImported(t *testing.T) {
	type q = map[string]any
	b := newGeneral()
	shape(t, b, q{"op": "box", "min": vector{}, "max": vector{4, 4, 4}})
	shape(t, b, q{"op": "circle", "origin": vector{2, 2, -1}, "normal": vector{0, 0, 1}, "radius": 1, "height": 6})
	cut := shape(t, b, q{"op": "cut", "a": "ps:1", "b": "ps:2"})
	imported := shape(t, b, q{"op": "importStep", "source": generalSTEPExported(t, b, cut.Handle)})
	if body := b.shapes[imported.Handle].analytic; body == nil || !generalBody(body) {
		t.Fatal("the imported body is not stored as a result of the general Boolean is")
	}
	cross := shape(t, b, q{"op": "circle", "origin": vector{-1, 1, 1}, "normal": vector{1, 0, 0}, "radius": 0.4, "height": 2})
	for _, c := range []struct {
		request q
		must    bool
	}{
		{q{"op": "inspect"}, true},
		{q{"op": "mesh"}, true},
		{q{"op": "stl"}, true},
		{q{"op": "clone"}, true},
		{q{"op": "transform", "matrix": [][]float64{{0, 0, 1, 7}, {1, 0, 0, 8}, {0, 1, 0, 9}}}, true},
		{q{"op": "cut", "b": cross.Handle}, true},
		{q{"op": "fuse", "b": cross.Handle}, true},
		{q{"op": "exportStep"}, true},
		{q{"op": "displayEdges", "maxSegments": 1000, "edgeUses": 100}, true},
		{q{"op": "drawingGeometry"}, true},
		{q{"op": "planarFace", "origin": vector{2, 0, 2}, "normal": vector{0, -1, 0}}, true},
		{q{"op": "holeRim", "center": vector{2, 2, 4}, "normal": vector{0, 0, 1}, "radius": 1}, true},
	} {
		var got [2]answer
		for i, handle := range [2]string{cut.Handle, imported.Handle} {
			request := q{}
			for k, v := range c.request {
				request[k] = v
			}
			switch c.request["op"] {
			case "cut", "fuse":
				request["a"] = handle
			case "exportStep":
				request["handles"] = []string{handle}
			default:
				request["handle"] = handle
			}
			got[i] = ask(t, b, request)
		}
		op := c.request["op"]
		if got[0].OK != got[1].OK {
			t.Errorf("%s: the cut result answers ok=%v (%+v), its re-import ok=%v (%+v)", op, got[0].OK, got[0].Error, got[1].OK, got[1].Error)
			continue
		}
		if !got[0].OK {
			if c.must {
				t.Errorf("%s fails on both: %+v", op, got[1].Error)
			} else {
				t.Logf("%s fails on both alike: %s", op, got[1].Error.Message)
			}
			continue
		}
		if got[0].Result.GeometryAssertions.Valid {
			sameShape(t, got[1].Result, got[0].Result)
		}
	}
}

// TestGeneralStepExportTraces replays the Tier 1 acceptance rows on a
// general bridge and, at every exportStep request, takes each result of the
// general Boolean among its handles. Each must be written, and its file
// must measure as the shape does. The request itself must then succeed,
// which means that every file read back, through importStep on a general
// bridge, as a shape with the published volume, area and bounds. A request
// that does not is reported with its reason.
func TestGeneralStepExportTraces(t *testing.T) {
	slowtest.Skip(t)
	bodies, legacy, exported, refused := 0, 0, 0, map[string]int{}
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
				exported++
			default:
				refused[row.id+" "+reply.Error.Code+": "+reply.Error.Message]++
			}
		}
	}
	t.Logf("%d results of the general Boolean at exportStep requests: all written and measured as published, %d of them by a legacy writer; %d requests exported", bodies, legacy, exported)
	for reason, n := range refused {
		t.Errorf("%d requests not exported: %s", n, reason)
	}
	if bodies == 0 {
		t.Fatal("no result of the general Boolean at any exportStep request")
	}
}

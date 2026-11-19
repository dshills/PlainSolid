package plaincadbridge

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"plainsolid/internal/slowtest"
)

// The traces under testdata/general_boolean are the frozen acceptance rows of
// specs/general-boolean/ACCEPTANCE.md: the requests one existing bridge test
// function issues through Bridge.Call, in order, with the responses the
// default bridge gave. This file defines the format by reading it.
//
// One JSON object per line:
//
//	{"new":true}            a new bridge; later requests go to it
//	{"req":…, "ok":…, …}    one request and its recorded response
//
// req is the request as issued. reqText replaces it for input that is not one
// JSON object. code is the failure code when ok is false. handle is the handle
// the response published. result holds what a replay compares. analytic holds
// the expected values the source test asserted with near for this response.
// unchecked names the exclusion of specification section 15.2 that keeps a
// response out of the comparison on a general bridge.

const generalBooleanTraceDir = "testdata/general_boolean"

// generalBooleanSlowLegacy names the rows whose source test is skipped in the
// default tier. Replaying one costs what the source test costs.
var generalBooleanSlowLegacy = map[string]bool{
	"TestPlainCADExactWallSideOpenDualDisk":     true,
	"TestPlainCADOffCenterSideOpenDualDisk":     true,
	"TestPlainCADOffsetSideOpenDualDisk":        true,
	"TestPlainCADSideOpenDualDiskCuts":          true,
	"TestPlainCADUnequalCenterSideOpenDualDisk": true,
	"TestPlainCADUnequalSideOpenDualDiskCuts":   true,
	"TestRetainedPlanarBoreFillRoofAndRedrill":  true,
}

type generalBooleanStep struct {
	New       bool                    `json:"new"`
	Req       json.RawMessage         `json:"req"`
	ReqText   *string                 `json:"reqText"`
	OK        bool                    `json:"ok"`
	Code      string                  `json:"code"`
	Handle    string                  `json:"handle"`
	Result    *generalBooleanResult   `json:"result"`
	Analytic  *generalBooleanAnalytic `json:"analytic"`
	Unchecked string                  `json:"unchecked"`
}

// generalBooleanResult is the compared part of a response: the assertions and
// bounds of a shape, the value of an overlapVolume, the whole pocketFloor
// result, or a marker that a mesh or an STL was returned.
type generalBooleanResult struct {
	Valid       *bool           `json:"valid"`
	Volume      *float64        `json:"volume"`
	SurfaceArea *float64        `json:"surfaceArea"`
	SolidCount  *int            `json:"solidCount"`
	Bounds      *Bounds         `json:"bounds"`
	Value       *float64        `json:"value"`
	PocketFloor json.RawMessage `json:"pocketFloor"`
	Mesh        bool            `json:"mesh"`
	STL         bool            `json:"stl"`
}

type generalBooleanAnalytic struct {
	Volume      []float64 `json:"volume"`
	SurfaceArea []float64 `json:"surfaceArea"`
	Value       []float64 `json:"value"`
}

// generalBooleanReply is a bridge response as a replay sees it.
type generalBooleanReply struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
	Error  *failure        `json:"error"`
	// Handle is the handle this response published, or empty.
	Handle string `json:"-"`
}

func generalBooleanTraceNames(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(generalBooleanTraceDir, "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(paths))
	for i, p := range paths {
		names[i] = strings.TrimSuffix(filepath.Base(p), ".jsonl")
	}
	return names
}

func loadGeneralBooleanTrace(t *testing.T, name string) []generalBooleanStep {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(generalBooleanTraceDir, name+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var steps []generalBooleanStep
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		var step generalBooleanStep
		d := json.NewDecoder(bytes.NewReader(line))
		d.DisallowUnknownFields()
		if err := d.Decode(&step); err != nil {
			t.Fatalf("%s line %d: %v", name, len(steps)+1, err)
		}
		if !step.New && step.Req == nil && step.ReqText == nil {
			t.Fatalf("%s line %d: neither a bridge nor a request", name, len(steps)+1)
		}
		steps = append(steps, step)
	}
	return steps
}

var generalBooleanHandle = regexp.MustCompile(`"ps:[0-9]+"`)

// generalBooleanReplay issues the requests of one trace on bridges made by
// open. It maps each recorded handle to the handle the same request publishes
// in the replay and rewrites later requests through that map.
type generalBooleanReplay struct {
	open    func() *Bridge
	bridge  *Bridge
	handles map[string]string
}

// issue replays one request. issued is false when the step is unchecked and
// uses a handle that the replayed bridge never published: a general bridge
// may fail an unchecked request that the default bridge answered, and a legacy
// result is never substituted for it.
func (r *generalBooleanReplay) issue(t *testing.T, step generalBooleanStep) (reply generalBooleanReply, issued bool) {
	t.Helper()
	if step.New {
		r.bridge, r.handles = r.open(), map[string]string{}
		return reply, false
	}
	if r.bridge == nil {
		t.Fatal("trace issues a request before a bridge")
	}
	input, missing := "", false
	if step.ReqText != nil {
		input = *step.ReqText
	} else {
		input = generalBooleanHandle.ReplaceAllStringFunc(string(step.Req), func(recorded string) string {
			if mapped, ok := r.handles[recorded]; ok {
				return mapped
			}
			missing = true
			return `"ps:unpublished"`
		})
	}
	if missing && step.Unchecked != "" {
		return reply, false
	}
	before := r.bridge.next
	if err := json.Unmarshal([]byte(r.bridge.Call(input)), &reply); err != nil {
		t.Fatal(err)
	}
	if r.bridge.next != before {
		reply.Handle = "ps:" + strconv.FormatUint(r.bridge.next, 10)
		if step.Handle != "" {
			r.handles[strconv.Quote(step.Handle)] = strconv.Quote(reply.Handle)
		}
	}
	return reply, true
}

// generalBooleanClose reports whether two recorded numbers agree within a
// relative bound.
func generalBooleanClose(got, want, relative float64) bool {
	return got == want || math.Abs(got-want) <= relative*math.Max(math.Abs(got), math.Abs(want))
}

// requireGeneralBooleanReply compares a reply with the recorded response:
// outcome, failure code, publication, and every recorded number within the
// relative bound.
func requireGeneralBooleanReply(t *testing.T, step generalBooleanStep, reply generalBooleanReply, relative float64) {
	t.Helper()
	fail := func(format string, args ...any) {
		t.Helper()
		request := string(step.Req)
		if step.ReqText != nil {
			request = *step.ReqText
		}
		if len(request) > 300 {
			request = request[:300] + "…"
		}
		t.Fatalf("request %s: "+format, append([]any{request}, args...)...)
	}
	if reply.OK != step.OK {
		fail("ok %v, recorded %v (%+v)", reply.OK, step.OK, reply.Error)
	}
	if !step.OK {
		if reply.Error == nil || reply.Error.Code != step.Code {
			fail("failure %+v, recorded code %s", reply.Error, step.Code)
		}
		return
	}
	if (reply.Handle != "") != (step.Handle != "") {
		fail("published %q, recorded %q", reply.Handle, step.Handle)
	}
	want := step.Result
	if want == nil {
		return
	}
	number := func(what string, got, recorded float64) {
		t.Helper()
		if !generalBooleanClose(got, recorded, relative) {
			fail("%s %.17g, recorded %.17g", what, got, recorded)
		}
	}
	switch {
	case want.Volume != nil:
		var got Shape
		if err := json.Unmarshal(reply.Result, &got); err != nil {
			fail("%v", err)
		}
		a := got.GeometryAssertions
		if a.Valid != *want.Valid || a.SolidCount != *want.SolidCount {
			fail("valid %v solidCount %d, recorded %v %d", a.Valid, a.SolidCount, *want.Valid, *want.SolidCount)
		}
		number("volume", a.Volume, *want.Volume)
		number("surfaceArea", a.SurfaceArea, *want.SurfaceArea)
		for i := range 3 {
			number("bounds.min", got.Bounds.Min[i], want.Bounds.Min[i])
			number("bounds.max", got.Bounds.Max[i], want.Bounds.Max[i])
		}
	case want.Value != nil:
		var got struct {
			Volume *float64 `json:"volume"`
		}
		if err := json.Unmarshal(reply.Result, &got); err != nil || got.Volume == nil {
			fail("no overlap volume in %s (%v)", reply.Result, err)
		}
		number("overlapVolume", *got.Volume, *want.Value)
	case want.PocketFloor != nil:
		var got, recorded any
		if err := json.Unmarshal(reply.Result, &got); err != nil {
			fail("%v", err)
		}
		if err := json.Unmarshal(want.PocketFloor, &recorded); err != nil {
			fail("%v", err)
		}
		if !reflect.DeepEqual(got, recorded) {
			fail("pocketFloor %s, recorded %s", reply.Result, want.PocketFloor)
		}
	case want.Mesh:
		var got Mesh
		if err := json.Unmarshal(reply.Result, &got); err != nil || len(got.Positions) == 0 || len(got.Indices) == 0 {
			fail("no mesh returned (%v)", err)
		}
	case want.STL:
		var got struct {
			Base64 string `json:"base64"`
		}
		if err := json.Unmarshal(reply.Result, &got); err != nil || got.Base64 == "" {
			fail("no STL returned (%v)", err)
		}
	}
}

// TestGeneralBooleanTraces replays every frozen acceptance row on a default
// bridge and requires the recorded responses, so that the traces are known to
// be a faithful record of what the legacy paths answer.
func TestGeneralBooleanTraces(t *testing.T) {
	names := generalBooleanTraceNames(t)
	if len(names) == 0 {
		t.Fatal("no traces under " + generalBooleanTraceDir)
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			if generalBooleanSlowLegacy[name] {
				slowtest.Skip(t)
			}
			replay := generalBooleanReplay{open: New}
			for _, step := range loadGeneralBooleanTrace(t, name) {
				reply, issued := replay.issue(t, step)
				if step.New {
					continue
				}
				if !issued {
					t.Fatalf("default bridge did not publish a handle that %s uses", step.Req)
				}
				requireGeneralBooleanReply(t, step, reply, 1e-12)
			}
		})
	}
}

package plaincadbridge

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
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
// specs/general-boolean/ACCEPTANCE.md: the bridge operations one existing
// bridge test function performs, in order, as requests, with the responses
// the default bridge gave. This file defines the format by reading it.
//
// One JSON object per line:
//
//	{"new":true}            a new bridge; later requests go to it
//	{"req":…, "ok":…, …}    one request and its recorded response
//
// req is the request as issued. reqText replaces it for input that is not one
// JSON object. source stands for the source member of req when that is STEP
// text of 1 KB or more: it names the file step/<source>.step.gz, which holds
// the text once for every request that sends it, and the reader puts the
// member back. The name is the first 16 hexadecimal digits of the SHA-256 of
// the text. code is the failure code when ok is false. handle is the handle
// the response published. result holds what a replay compares. analytic holds
// the expected values the source test asserted with near for this response.
// unchecked names the exclusion of specification section 15.2 that keeps a
// response out of the comparison on a general bridge.
// direct marks a request that the source test did not send through
// Bridge.Call: it called the bridge method that the request dispatches to,
// and the recorder wrote the equivalent request.

const generalBooleanTraceDir = "testdata/general_boolean"

// generalBooleanSlowLegacy names the rows whose source test is skipped in the
// default tier. Replaying one costs what the source test costs.
var generalBooleanSlowLegacy = map[string]bool{
	"TestBlindCrossedObliqueBoreSTEPAndFurtherCut":   true,
	"TestBlindTripleCrossedBoreScopeAndSTEP":         true,
	"TestClippedCrossedBoreScopeAndSTEP":             true,
	"TestCrossedObliqueBoreCoordinatePlacements":     true,
	"TestMultipleBlindTripleCrossedBoreScopeAndSTEP": true,
	"TestPlainCADExactWallSideOpenDualDisk":          true,
	"TestPlainCADOffCenterSideOpenDualDisk":          true,
	"TestPlainCADOffsetSideOpenDualDisk":             true,
	"TestPlainCADSideOpenDualDiskCuts":               true,
	"TestPlainCADUnequalCenterSideOpenDualDisk":      true,
	"TestPlainCADUnequalSideOpenDualDiskCuts":        true,
	"TestRetainedPlanarBoreFillRoofAndRedrill":       true,
	"TestSideNotchMixedBoreBatchIsAtomic":            true,
}

type generalBooleanStep struct {
	New       bool                    `json:"new"`
	Req       json.RawMessage         `json:"req"`
	ReqText   *string                 `json:"reqText"`
	Source    string                  `json:"source"`
	OK        bool                    `json:"ok"`
	Code      string                  `json:"code"`
	Handle    string                  `json:"handle"`
	Result    *generalBooleanResult   `json:"result"`
	Analytic  *generalBooleanAnalytic `json:"analytic"`
	Unchecked string                  `json:"unchecked"`
	Direct    bool                    `json:"direct"`
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
		if step.Source != "" {
			req, err := generalBooleanWithSource(step.Req, step.Source)
			if err != nil {
				t.Fatalf("%s line %d: %v", name, len(steps)+1, err)
			}
			step.Req = req
		}
		steps = append(steps, step)
	}
	return steps
}

// generalBooleanWithSource returns req with a source member whose value is the
// text stored under name.
func generalBooleanWithSource(req json.RawMessage, name string) (json.RawMessage, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(req, &members); err != nil {
		return nil, fmt.Errorf("source %s needs a request that is an object: %w", name, err)
	}
	if _, ok := members["source"]; ok {
		return nil, fmt.Errorf("source %s: the request has a source already", name)
	}
	packed, err := os.ReadFile(filepath.Join(generalBooleanTraceDir, "step", name+".step.gz"))
	if err != nil {
		return nil, err
	}
	z, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		return nil, fmt.Errorf("source %s: %w", name, err)
	}
	text, err := io.ReadAll(z)
	if err != nil {
		return nil, fmt.Errorf("source %s: %w", name, err)
	}
	value, err := json.Marshal(string(text))
	if err != nil {
		return nil, err
	}
	members["source"] = value
	return json.Marshal(members)
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
		// A bound is a coordinate, which may be zero: it is held to the
		// bound of a length of one or more.
		place := func(what string, got, recorded float64) {
			t.Helper()
			if math.Abs(got-recorded) > relative*math.Max(1, math.Max(math.Abs(got), math.Abs(recorded))) {
				fail("%s %.17g, recorded %.17g", what, got, recorded)
			}
		}
		for i := range 3 {
			place("bounds.min", float64(got.Bounds.Min[i]), float64(want.Bounds.Min[i]))
			place("bounds.max", float64(got.Bounds.Max[i]), float64(want.Bounds.Max[i]))
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
// be a faithful record of what the legacy paths answer. One kind of response
// may differ. Where the record is a legacy path declining an input, as
// legacy_refusals.txt lists it, the default bridge falls back to the general
// Boolean, and a valid result is accepted there as on a general bridge
// (specification A4).
func TestGeneralBooleanTraces(t *testing.T) {
	names := generalBooleanTraceNames(t)
	if len(names) == 0 {
		t.Fatal("no traces under " + generalBooleanTraceDir)
	}
	refusals, pending, row := generalBooleanRefusals(t), generalBooleanPending(t), map[string]string{}
	for _, r := range generalBooleanRows(t) {
		row[r.function] = r.id
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			if generalBooleanSlowLegacy[name] {
				slowtest.Skip(t)
			}
			// Every row opens its own bridges and shares nothing, so the
			// rows run side by side.
			t.Parallel()
			replay := generalBooleanReplay{open: newLegacy}
			for line, step := range loadGeneralBooleanTrace(t, name) {
				reply, issued := replay.issue(t, step)
				if step.New {
					continue
				}
				if !issued {
					t.Fatalf("default bridge did not publish a handle that %s uses", step.Req)
				}
				if refusals[row[name]][line] && !step.OK && reply.OK {
					var got Shape
					if reply.Handle != "" && (json.Unmarshal(reply.Result, &got) != nil || !got.GeometryAssertions.Valid) {
						t.Fatalf("request %s: the result where legacy declined is not valid: %s", step.Req, reply.Result)
					}
					t.Logf("fallback %s line %d: %.160s", row[name], line, reply.Result)
					continue
				}
				// A response of a legacy path is held to its record to
				// the last digits. Where no legacy path answers any
				// more, the general Boolean has, and its result is held
				// to the bound of the acceptance (A3).
				bound := 1e-12
				if s, ok := replay.bridge.shapes[reply.Handle]; ok && s.analytic != nil && s.analytic.general {
					bound = 1e-7
					// A row that the acceptance excludes is excluded
					// here too once the general Boolean answers it.
					if strings.Contains(pending[row[name]], "excluded") && reply.OK == step.OK {
						continue
					}
				}
				requireGeneralBooleanReply(t, step, reply, bound)
			}
		})
	}
}

package plaincadbridge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"plainsolid/exchange"
	"plainsolid/internal/slowtest"
)

// generalBooleanRow is one acceptance row of
// specs/general-boolean/ACCEPTANCE.md.
type generalBooleanRow struct{ id, function, family string }

var generalBooleanRowLine = regexp.MustCompile("^\\| (T[12]-[0-9]+) \\| `(Test[A-Za-z0-9_]+)` \\| [^|]+ \\| [12] \\| ([^|]+) \\|")

func generalBooleanRows(t *testing.T) []generalBooleanRow {
	t.Helper()
	data, err := os.ReadFile("../../specs/general-boolean/ACCEPTANCE.md")
	if err != nil {
		t.Fatal(err)
	}
	var rows []generalBooleanRow
	for _, line := range strings.Split(string(data), "\n") {
		if m := generalBooleanRowLine.FindStringSubmatch(line); m != nil {
			rows = append(rows, generalBooleanRow{m[1], m[2], strings.TrimSpace(m[3])})
		}
	}
	if len(rows) == 0 {
		t.Fatal("no acceptance rows")
	}
	return rows
}

// generalBooleanPending reads the bridge pending list (K7): a row
// identifier, the function, and the reason, one row on a line.
func generalBooleanPending(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile(generalBooleanTraceDir + "/pending.txt")
	if err != nil {
		t.Fatal(err)
	}
	pending := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		if id, rest, _ := strings.Cut(line, " "); id != "" {
			_, pending[id], _ = strings.Cut(rest, " ")
		}
	}
	return pending
}

// generalBooleanRefusals reads legacy_refusals.txt: the recorded failures
// that are the legacy path declining an input it could not handle, by
// row and by the line of the trace, with the legacy message. A recorded
// failure that is not listed is a failure of the request's contract and
// must fail on the general bridge too. One that is listed need not: a
// general result that passes validation is accepted, and logged for the
// oracle run, which is all that checks its values.
func generalBooleanRefusals(t *testing.T) map[string]map[int]bool {
	t.Helper()
	data, err := os.ReadFile(generalBooleanTraceDir + "/legacy_refusals.txt")
	if err != nil {
		t.Fatal(err)
	}
	refusals := map[string]map[int]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var id string
		var step int
		if _, err := fmt.Sscanf(line, "%s %d", &id, &step); err != nil {
			t.Fatalf("legacy_refusals.txt: %q: %v", line, err)
		}
		if refusals[id] == nil {
			refusals[id] = map[int]bool{}
		}
		refusals[id][step] = true
	}
	return refusals
}

// generalBooleanAcceptance holds what one row achieved (A8).
type generalBooleanAcceptance struct {
	volume, area float64 // largest errors over the checked responses with a value
	values, none int     // checked responses that gave a value, and that gave none
	first        string  // the first requirement that did not hold
	failed       int
	kinds        []string              // the distinct failures, without their numbers
	refused      []string              // general results where the legacy path refused
	volumes      map[string]Assertions // by replayed handle
}

var generalBooleanNumber = regexp.MustCompile(`-?[0-9]+(\.[0-9]+)?(e[-+]?[0-9]+)?`)

func (a *generalBooleanAcceptance) fail(step generalBooleanStep, format string, args ...any) {
	a.failed++
	kind := generalBooleanNumber.ReplaceAllString(fmt.Sprintf(format, args...), "N")
	if len(kind) > 140 {
		kind = kind[:140]
	}
	seen := false
	for _, k := range a.kinds {
		seen = seen || k == kind
	}
	if !seen {
		a.kinds = append(a.kinds, kind)
	}
	if a.first == "" {
		request := string(step.Req)
		if len(request) > 160 {
			request = request[:160] + "…"
		}
		a.first = fmt.Sprintf(format, args...) + " at " + request
	}
}

// within is the bound of A3: 1e-7 × max(1, |expected|).
func (a *generalBooleanAcceptance) within(step generalBooleanStep, what string, got, legacy float64, analytic []float64, worst *float64) {
	expected := legacy
	if len(analytic) != 0 {
		expected = analytic[0]
	}
	*worst = math.Max(*worst, generalBooleanError(got, expected))
	for _, want := range append([]float64{legacy}, analytic...) {
		if !(generalBooleanError(got, want) <= 1e-7) {
			a.fail(step, "%s %.12g, expected %.12g", what, got, want)
		}
	}
}

// check compares one checked response with its record as A3 and A4
// require.
func (a *generalBooleanAcceptance) check(step generalBooleanStep, reply generalBooleanReply, declined bool) {
	if declined && !step.OK && reply.OK {
		var got Shape
		if reply.Handle != "" && (json.Unmarshal(reply.Result, &got) != nil || !got.GeometryAssertions.Valid) {
			a.fail(step, "a result where legacy refused is not valid")
		}
		a.refused = append(a.refused, string(reply.Result))
		return
	}
	want := step.Result
	valued := step.OK && want != nil && (want.Volume != nil || want.Value != nil)
	if reply.OK != step.OK {
		if valued {
			a.none++
		}
		message := "succeeded"
		if reply.Error != nil {
			message = reply.Error.Code + ": " + reply.Error.Message
		}
		a.fail(step, "ok %v, recorded %v (%s)", reply.OK, step.OK, message)
		return
	}
	if !step.OK {
		return
	}
	if (reply.Handle != "") != (step.Handle != "") {
		a.fail(step, "published %q, recorded %q", reply.Handle, step.Handle)
	}
	if want == nil {
		return
	}
	analytic := generalBooleanAnalytic{}
	if step.Analytic != nil {
		analytic = *step.Analytic
	}
	switch {
	case want.Volume != nil:
		var got Shape
		if err := json.Unmarshal(reply.Result, &got); err != nil {
			a.fail(step, "%v", err)
			return
		}
		a.values++
		g := got.GeometryAssertions
		a.volumes[reply.Handle] = g
		if !g.Valid || g.SolidCount != *want.SolidCount {
			a.fail(step, "valid %v solidCount %d, recorded solidCount %d", g.Valid, g.SolidCount, *want.SolidCount)
		}
		a.within(step, "volume", g.Volume, *want.Volume, analytic.Volume, &a.volume)
		a.within(step, "surfaceArea", g.SurfaceArea, *want.SurfaceArea, analytic.SurfaceArea, &a.area)
		for i := range 3 {
			if lo, hi := math.Abs(float64(got.Bounds.Min[i]-want.Bounds.Min[i])), math.Abs(float64(got.Bounds.Max[i]-want.Bounds.Max[i])); !(lo <= 1e-7 && hi <= 1e-7) {
				a.fail(step, "bounds %v, recorded %v", got.Bounds, *want.Bounds)
				break
			}
		}
	case want.Value != nil:
		var got struct {
			Volume *float64 `json:"volume"`
		}
		if err := json.Unmarshal(reply.Result, &got); err != nil || got.Volume == nil {
			a.none++
			a.fail(step, "no overlap volume (%v)", err)
			return
		}
		a.values++
		a.within(step, "overlapVolume", *got.Volume, *want.Value, analytic.Value, &a.volume)
	case want.PocketFloor != nil:
		// CO4: the answer equals the recorded legacy answer.
		var got, recorded any
		if err := json.Unmarshal(reply.Result, &got); err != nil {
			a.fail(step, "%v", err)
		} else if err := json.Unmarshal(want.PocketFloor, &recorded); err != nil || !reflect.DeepEqual(got, recorded) {
			a.fail(step, "pocketFloor %s, recorded %s", reply.Result, want.PocketFloor)
		}
	case want.Mesh, want.STL:
		if err := generalBooleanDisplay(reply.Result, want.STL, a.volumes); err != nil {
			a.fail(step, "%v", err)
		}
	}
}

// generalBooleanDisplay applies the mesh checks of the source tests that
// ACCEPTANCE.md names: a mesh response decodes with positions and indices
// and carries the assertions of a shape of the replay; an STL response
// reads back as a closed solid mesh with positive volume.
func generalBooleanDisplay(result json.RawMessage, stl bool, shapes map[string]Assertions) error {
	if !stl {
		var got Mesh
		if err := json.Unmarshal(result, &got); err != nil || len(got.Positions) == 0 || len(got.Indices) == 0 {
			return fmt.Errorf("no mesh returned (%v)", err)
		}
		for _, s := range shapes {
			if s == got.GeometryAssertions {
				return nil
			}
		}
		return fmt.Errorf("the mesh carries assertions %+v of no shape of the row", got.GeometryAssertions)
	}
	var got struct {
		Base64 string `json:"base64"`
	}
	if err := json.Unmarshal(result, &got); err != nil || got.Base64 == "" {
		return fmt.Errorf("no STL returned (%v)", err)
	}
	data, err := base64.StdEncoding.DecodeString(got.Base64)
	if err != nil {
		return err
	}
	m, err := exchange.ReadSTL(bytes.NewReader(data), exchange.Limits{})
	if err != nil {
		return err
	}
	if err := m.ValidateSolid(); err != nil {
		return err
	}
	p, err := m.MassProperties()
	if err != nil {
		return err
	}
	for _, s := range shapes {
		if math.Abs(p.Volume-s.Volume) <= 0.02*s.Volume {
			return nil
		}
	}
	return fmt.Errorf("the STL has volume %g, which is that of no shape of the row", p.Volume)
}

// TestGeneralBooleanAcceptance replays every frozen acceptance row on a
// bridge with the general Boolean selected and compares every checked
// response with its record (A2 to A4). It logs the accuracy each row
// achieves (A8) and its elapsed time (A7). A row in the bridge pending
// list is expected to fail (K7).
func TestGeneralBooleanAcceptance(t *testing.T) { generalBooleanAcceptanceOn(t, newGeneral, false) }

// TestDefaultBridgeAcceptance replays the same rows on the default bridge,
// where the general Boolean answers plain bores and what follows from
// them first and the legacy paths answer the rest (OD-11), and holds
// every checked response to the same comparison with its record. A row
// that the pending list excludes for the general Boolean may pass here,
// where a legacy path still answers it, and no row fails for its time:
// A7 is a bound on the general Boolean, which the replay above measures.
func TestDefaultBridgeAcceptance(t *testing.T) { generalBooleanAcceptanceOn(t, New, true) }

// generalBooleanAcceptanceOn replays every acceptance row on bridges that
// open makes. With mixed set, for a bridge on which legacy paths answer
// some requests, a row in the pending list may pass and the time of a row
// is logged and not judged.
func generalBooleanAcceptanceOn(t *testing.T, open func() *Bridge, mixed bool) {
	rows, pending, refusals := generalBooleanRows(t), generalBooleanPending(t), generalBooleanRefusals(t)
	for _, tier := range []string{"T1", "T2"} {
		t.Run(tier, func(t *testing.T) {
			for _, row := range rows {
				if !strings.HasPrefix(row.id, tier) {
					continue
				}
				t.Run(row.function, func(t *testing.T) {
					// A row listed as taking more than two seconds runs
					// in the slow tier only.
					if strings.Contains(pending[row.id], "time over two seconds") {
						slowtest.Skip(t)
					}
					t.Parallel()
					start := time.Now()
					a := generalBooleanAcceptance{volumes: map[string]Assertions{}}
					replay := generalBooleanReplay{open: open}
					for line, step := range loadGeneralBooleanTrace(t, row.function) {
						reply, issued := replay.issue(t, step)
						if step.New || !issued {
							continue
						}
						if step.Unchecked != "" {
							var got Shape
							if reply.OK && reply.Handle != "" && json.Unmarshal(reply.Result, &got) == nil {
								a.volumes[reply.Handle] = got.GeometryAssertions
							}
							continue
						}
						a.check(step, reply, refusals[row.id][line])
					}
					for _, result := range a.refused {
						t.Logf("general result where legacy refused: %s %.200s", row.id, result)
					}
					elapsed := time.Since(start)
					if a.values != 0 {
						t.Logf("A8 %s vol=%.3g area=%.3g none=%d time=%.2fs family=%q", row.id, a.volume, a.area, a.none, elapsed.Seconds(), row.family)
					} else {
						t.Logf("no value: %s none=%d time=%.2fs family=%q", row.id, a.none, elapsed.Seconds(), row.family)
					}
					// A7 is a bound on elapsed time, and the rows run side
					// by side, so a row near two seconds lands on either
					// side of it from run to run. A row listed for its
					// time alone is therefore logged either way, and a
					// row that is not listed fails for its time only
					// beyond twice the bound. Time fails a row only in
					// the slow tier, where the replay is run by itself:
					// in the default tier it runs beside every other
					// package of the repository, and a row that takes
					// 1.6 s alone has taken 5.5 s there.
					reason, listed := pending[row.id]
					slow := elapsed > 2*time.Second
					if a.first == "" && slow && slowtest.Enabled() && !mixed && (listed || elapsed > 4*time.Second) {
						a.failed, a.first, a.kinds = 1, fmt.Sprintf("A7: the row took %.2fs", elapsed.Seconds()), []string{"A7: over two seconds"}
					}
					switch {
					case a.first != "" && listed:
						t.Logf("pending %s: %d checked responses differ; first: %s\nkinds %s: %s", row.id, a.failed, a.first, row.id, strings.Join(a.kinds, " ;; "))
					case a.first != "":
						t.Errorf("%s: %d checked responses differ; first: %s\nkinds %s: %s", row.id, a.failed, a.first, row.id, strings.Join(a.kinds, " ;; "))
					case listed && strings.HasPrefix(reason, "time over two seconds"):
						t.Logf("pending %s: listed for its time, and took %.2fs in this run", row.id, elapsed.Seconds())
					case listed && mixed:
						t.Logf("%s is in the pending list and passes on this bridge", row.id)
					case listed:
						t.Errorf("%s is in the pending list and passes", row.id)
					case slow:
						t.Logf("A7 %s: not listed, and took %.2fs in this run", row.id, elapsed.Seconds())
					}
				})
			}
		})
	}
}

// TestGeneralBooleanMeasures is the independent check of the closed-form
// volume and area that the general Boolean reports and the adapter
// publishes. It replays every acceptance row on a general bridge and
// integrates each result the bridge publishes with
// trimmed.Solid.MassProperties at the bridge's integration tolerance.
// It runs in the slow tier only, and apart from the acceptance replay
// so that it does not add to the times that replay measures.
func TestGeneralBooleanMeasures(t *testing.T) {
	slowtest.Skip(t)
	var mu sync.Mutex
	results, volume, area := 0, 0., 0.
	t.Cleanup(func() {
		t.Logf("closed form against integration on %d published results: largest difference %.3g in volume, %.3g in area, relative to max(1, value)", results, volume, area)
	})
	for _, row := range generalBooleanRows(t) {
		t.Run(row.id, func(t *testing.T) {
			t.Parallel()
			replay := generalBooleanReplay{open: newGeneral}
			for _, step := range loadGeneralBooleanTrace(t, row.function) {
				reply, issued := replay.issue(t, step)
				if step.New || !issued || !reply.OK || reply.Handle == "" {
					continue
				}
				body := replay.bridge.shapes[reply.Handle].analytic
				if body == nil || !generalBody(body) {
					continue
				}
				p, err := body.solid.MassProperties(context.Background(), generalOptions.Integration)
				if err != nil {
					t.Fatalf("%s: %v", step.Req, err)
				}
				dv, da := generalBooleanError(body.volume, p.Volume), generalBooleanError(body.area, p.Area)
				mu.Lock()
				results, volume, area = results+1, math.Max(volume, dv), math.Max(area, da)
				mu.Unlock()
				if dv > 1e-8 || da > 1e-8 {
					t.Errorf("%.200s: closed form %.12g and %.12g, integration %.12g and %.12g", step.Req, body.volume, body.area, p.Volume, p.Area)
				}
			}
		})
	}
}

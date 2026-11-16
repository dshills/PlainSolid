package exchange_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestVariableTransverseClippedEnvelopeExchange(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		op                            string
		nodes, faces, edges, vertices int
		volume, area                  float64
	}{
		{"common", 1, 13, 28, 17, 11.970700327015948, 31.65510819561157},
		{"common", 4, 16, 40, 26, 11.970698738855068, 31.655053074646922},
		{"common", 5, 17, 44, 29, 11.970702661843006, 31.655113388373966},
		{"fuse", 1, 20, 45, 27, 16.2795523966779, 39.74461928518421},
		{"fuse", 5, 24, 61, 39, 16.278283135323818, 39.73393378191109},
	} {
		t.Run(fmt.Sprintf("%s/nodes%d", c.op, c.nodes), func(t *testing.T) {
			recipe, err := os.ReadFile(fmt.Sprintf("testdata/native_clipped_variable_%s_%d.json", c.op, c.nodes))
			if err != nil {
				t.Fatal(err)
			}
			const name, units = "Variable clipped envelope", .001
			nativeOptions := trimmed.PolynomialCubicCrossingTangencyEnvelopeOptions{}
			sopts := step.ClippedPolynomialFuseSTEPOptions{PoleTolerance: 1e-14}
			iopts := iges.ClippedPolynomialFuseIGESOptions{PoleTolerance: 1e-14}
			var source trimmed.Solid
			var writeStep, writeIGES func(io.Writer) error
			var verifyStep, verifyIGES func(io.Reader, float64) error
			if c.op == "common" {
				r, err := trimmed.ReadClippedPolynomialTransverseCommonJSON(ctx, bytes.NewReader(recipe), trimmed.JSONLimits{}, nativeOptions)
				if err != nil {
					t.Fatal(err)
				}
				source = r.Boundary()
				writeStep = func(w io.Writer) error {
					return step.WriteClippedPolynomialTransverseCommon(ctx, w, r, name, units, step.Limits{}, sopts)
				}
				writeIGES = func(w io.Writer) error {
					return iges.WriteClippedPolynomialTransverseCommon(ctx, w, r, units, iges.Limits{}, iopts)
				}
				verifyStep = func(input io.Reader, u float64) error {
					return step.VerifyClippedPolynomialTransverseCommonSTEP(ctx, input, r, name, u, step.Limits{}, sopts)
				}
				verifyIGES = func(input io.Reader, u float64) error {
					return iges.VerifyClippedPolynomialTransverseCommonIGES(ctx, input, r, u, iges.Limits{}, iopts)
				}
			} else {
				r, err := trimmed.ReadClippedPolynomialFuseJSON(ctx, bytes.NewReader(recipe), trimmed.JSONLimits{}, nativeOptions)
				if err != nil {
					t.Fatal(err)
				}
				source = r.Boundary()
				writeStep = func(w io.Writer) error {
					return step.WriteClippedPolynomialFuse(ctx, w, r, name, units, step.Limits{}, sopts)
				}
				writeIGES = func(w io.Writer) error {
					return iges.WriteClippedPolynomialFuse(ctx, w, r, units, iges.Limits{}, iopts)
				}
				verifyStep = func(input io.Reader, u float64) error {
					return step.VerifyClippedPolynomialFuseSTEP(ctx, input, r, name, u, step.Limits{}, sopts)
				}
				verifyIGES = func(input io.Reader, u float64) error {
					return iges.VerifyClippedPolynomialFuseIGES(ctx, input, r, u, iges.Limits{}, iopts)
				}
			}
			nodePoints := exactVariableEnvelopeContacts(t, recipe, c.nodes)
			for _, fm := range []struct {
				ext, reader string
				write       func(io.Writer) error
				verify      func(io.Reader, float64) error
			}{
				{"step", os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_STEP_COMPONENTS"), writeStep, verifyStep},
				{"igs", os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_IGES"), writeIGES, verifyIGES},
			} {
				var raw bytes.Buffer
				if err := fm.write(&raw); err != nil {
					t.Fatal(fm.ext, err)
				}
				if err := fm.verify(bytes.NewReader(raw.Bytes()), units); err != nil {
					t.Fatal(fm.ext, err)
				}
				if err := fm.verify(bytes.NewReader(raw.Bytes()), 1); !errors.Is(err, geom.ErrConvergence) {
					t.Fatalf("%s accepted a different physical unit: %v", fm.ext, err)
				}
				if fm.reader != "" {
					independent := readPinnedExactComponents(t, ctx, fm.ext, raw.Bytes(), fm.reader)
					if independent.Solids != 1 || independent.IndexedVertices != c.vertices || len(independent.Components) != 1 {
						t.Fatalf("independent %s readback changed the single solid or its exact vertices", fm.ext)
					}
					body := independent.Components[0]
					if body.Faces != c.faces || body.Edges != c.edges || body.Shells != 1 ||
						body.Volume <= 0 || math.Abs(body.Volume-c.volume) > 1e-8 || math.Abs(body.Area-c.area) > 1e-8 {
						t.Fatalf("independent %s material changed: %d faces, %d edges, volume %.16g, area %.16g",
							fm.ext, body.Faces, body.Edges, body.Volume, body.Area)
					}
					checkVariableEnvelopeReadbackVertices(t, fm.ext, body, source, nodePoints)
				}
			}
		})
	}
}

func exactVariableEnvelopeContacts(t *testing.T, recipe []byte, count int) [][3]float64 {
	t.Helper()
	var native struct {
		Ledger struct {
			Contacts []struct{ Point [3]string }
		}
	}
	if err := json.Unmarshal(recipe, &native); err != nil || len(native.Ledger.Contacts) != count {
		t.Fatalf("original receipt contact ledger: %v, count %d", err, len(native.Ledger.Contacts))
	}
	out := make([][3]float64, count)
	for i, c := range native.Ledger.Contacts {
		for axis, s := range c.Point {
			q, ok := new(big.Rat).SetString(s)
			if !ok {
				t.Fatalf("invalid original rational contact: %q", s)
			}
			v, err := geom.RepresentedExactScalar(q)
			if err != nil {
				t.Fatal(err)
			}
			out[i][axis] = v
		}
	}
	return out
}

func checkVariableEnvelopeReadbackVertices(t *testing.T, format string, body pinnedExactComponentReport, source trimmed.Solid, nodes [][3]float64) {
	t.Helper()
	want := map[[3]float64]bool{}
	for _, v := range source.Vertices() {
		want[[3]float64{v.X, v.Y, v.Z}] = true
	}
	index := map[[3]float64]int{}
	for i, v := range body.Vertices {
		if !want[v] {
			t.Fatalf("independent %s changed original vertex %v", format, v)
		}
		index[v] = i
		delete(want, v)
	}
	if len(want) != 0 {
		t.Fatalf("independent %s omitted %d original vertices", format, len(want))
	}
	for _, p := range nodes {
		id, ok := index[p]
		rays := 0
		for _, e := range body.EdgeVertices {
			if e[0] == id || e[1] == id {
				rays++
			}
		}
		if !ok || rays != 4 {
			t.Fatalf("independent %s lost the four original rays of exact node %v", format, p)
		}
	}
}

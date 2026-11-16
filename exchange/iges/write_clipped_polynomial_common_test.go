package iges

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestWriteClippedPolynomialCommon(t *testing.T) {
	ctx := context.Background()
	opts := ClippedPolynomialCommonIGESOptions{PoleTolerance: 1e-14}
	for _, layout := range []string{"transverse", "mixed"} {
		t.Run(layout, func(t *testing.T) {
			input, err := os.Open("../testdata/native_clipped_polynomial_common_" + layout + ".json")
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			common, err := trimmed.ReadClippedPolynomialCommonJSON(ctx, input, trimmed.JSONLimits{}, trimmed.PolynomialCubicCrossingTangencyEnvelopeOptions{})
			if err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := WriteClippedPolynomialCommon(ctx, &output, common, .001, Limits{}, opts); err != nil {
				t.Fatal(err)
			}
			raw := output.Bytes()
			graph, err := readTopologySource(bytes.NewReader(raw), Limits{}, true)
			if err != nil || graph.unit != .001 {
				t.Fatalf("original IGES units %g: %v", graph.unit, err)
			}
			body := common.Boundary()
			verify := func(data []byte, unit float64) error {
				return VerifyClippedPolynomialCommonIGES(ctx, bytes.NewReader(data), common, unit, Limits{}, opts)
			}
			if layout == "mixed" {
				original := body
				body, err = common.SubdivideClippedCommonLens(ctx, geom.GraphPointContactOptions{})
				if err != nil || len(body.Vertices()) != len(original.Vertices()) || len(body.Faces()) != len(original.Faces())+1 ||
					len(body.Edges()) != len(original.Edges())+1 {
					t.Fatal("mixed transfer must split just one original lens without changing its vertices", err)
				}
				var unsplit bytes.Buffer
				if err := writeExactBezierSingleIGES(ctx, &unsplit, original, .001, Limits{}, opts); err != nil {
					t.Fatal(err)
				}
				if err := verify(unsplit.Bytes(), .001); !errors.Is(err, geom.ErrConvergence) {
					t.Fatalf("original nonportable cusp wire was accepted: %v", err)
				}
				oldGraph, err := readTopologySource(bytes.NewReader(unsplit.Bytes()), Limits{}, true)
				if err != nil {
					t.Fatal(err)
				}
				var refused bytes.Buffer
				short := opts
				short.Proof.MaxWork = 1
				if err := WriteClippedPolynomialCommon(ctx, &refused, common, .001, Limits{}, short); err == nil || refused.Len() != 0 {
					t.Fatalf("exhausted cusp proof published bytes: %v", err)
				}
				if err := WriteClippedPolynomialCommon(ctx, &refused, common, .001, Limits{MaxEntities: len(oldGraph.entities)}, opts); err == nil || refused.Len() != 0 {
					t.Fatalf("rejected cusp split published part of a solid: %v", err)
				}
			} else {
				const before = "a0edad01ea1d65edf4577239331570fca4f6c20a60dc012747a6f1cfefdc973e"
				if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != before {
					t.Fatalf("unchanged transverse export hash %s, want %s", got, before)
				}
			}
			loop := checkClippedCommonIGESGraph(t, graph, body)
			if err := verify(bytes.Replace(raw, []byte("PlainSolid exact"), []byte("PlainSolid check"), 1), .001); err != nil {
				t.Fatal("harmless start annotation", err)
			}
			if err := verify(raw, 1); !errors.Is(err, geom.ErrConvergence) {
				t.Fatalf("changed original material units accepted: %v", err)
			}
			changed := bytes.Clone(raw)
			replaceExactBezierIGESParameterDigit(t, changed, loop, 5)
			if _, err := readTopologySource(bytes.NewReader(changed), Limits{}, true); err != nil {
				t.Fatal("changed coedge direction must retain valid IGES record syntax", err)
			}
			if err := verify(changed, .001); !errors.Is(err, geom.ErrConvergence) {
				t.Fatalf("changed curve ownership accepted: %v", err)
			}
			var refused bytes.Buffer
			if err := WriteClippedPolynomialCommon(ctx, &refused, &trimmed.AlignedPolynomialEnvelope{}, .001, Limits{}, opts); !errors.Is(err, geom.ErrInvalid) || refused.Len() != 0 {
				t.Fatalf("unproved native input published bytes: %v", err)
			}
			if err := WriteClippedPolynomialCommon(ctx, &refused, common, .001, Limits{MaxControlPoints: 4}, opts); err == nil || refused.Len() != 0 {
				t.Fatalf("exhausted control-point budget published bytes: %v", err)
			}
			if oracle := os.Getenv("PLAINSOLID_OCCT_CLIPPED_COMMON_IGES"); oracle != "" {
				checkOCCTClippedCommonIGES(t, ctx, oracle, raw, body)
			}
		})
	}
}

func checkClippedCommonIGESGraph(t *testing.T, graph topologySource, body trimmed.Solid) int {
	t.Helper()
	byType := map[int][]topologyRecord{}
	byID := map[int]topologyRecord{}
	for _, e := range graph.entities {
		byType[e.kind] = append(byType[e.kind], e)
		byID[e.id] = e
	}
	faces, edges, points := body.Faces(), body.Edges(), body.Vertices()
	for kind, count := range map[int]int{
		126: len(edges) * 3, 128: len(faces), 186: 1, 502: 1, 504: 1,
		508: len(faces), 510: len(faces), 514: 1, 402: 0,
	} {
		if len(byType[kind]) != count {
			t.Fatalf("IGES type %d count %d, want %d", kind, len(byType[kind]), count)
		}
	}
	vertices := byType[502][0].values
	if len(vertices) != 2+3*len(points) {
		t.Fatal("changed exact source vertices")
	}
	for i, p := range points {
		record, err := igesPlanarCoordinates(vertices[2+3*i : 5+3*i])
		if err != nil {
			t.Fatal(err)
		}
		for axis, coord := range [...]float64{p.X, p.Y, p.Z} {
			if record[axis].Cmp(new(big.Rat).SetFloat64(coord)) != 0 {
				t.Fatalf("vertex %d coordinate %d changed from source", i, axis)
			}
		}
	}
	limits, err := (Limits{}).defaults()
	if err != nil {
		t.Fatal(err)
	}
	usedPcurves := make(map[int]bool)
	edgeOwners := make([]int, len(edges))
	for i, face := range faces {
		use, err := topologyInts(byType[510][i], 5)
		if err != nil || use[2] != 1 || use[4] != byType[508][i].id || len(face.Holes) != 0 {
			t.Fatalf("face %d changed original loop incidence: %v", i, err)
		}
		rec := byID[use[1]]
		support, err := decodeSurface(rec.values, limits.MaxControlPoints)
		if rec.kind != 128 || err != nil {
			t.Fatalf("face %d changed supporting chart: %v", i, err)
		}
		if spline, ok := face.Surface.(*geom.BSplineSurface); ok && !reflect.DeepEqual(spline.Data(), support.Surface.Geometry.Data()) {
			t.Fatalf("face %d changed original spline data", i)
		}
		dir := byType[508][i]
		loop, err := topologyInts(dir, 2+7*len(face.Outer))
		if err != nil || loop[1] != len(face.Outer) {
			t.Fatalf("face %d changed owner count: %v", i, err)
		}
		for j, native := range face.Outer {
			owner := loop[2+7*j : 2+7*(j+1)]
			pc := byID[owner[6]]
			if owner[0] != 0 || owner[1] != byType[504][0].id || owner[2] != native.Edge+1 ||
				owner[3] != igesSplineFaceOrientation(native.Reversed) || owner[4] != 1 || pc.kind != 126 || usedPcurves[pc.id] {
				t.Fatalf("face %d edge %d changed signed owner incidence", i, j)
			}
			edgeOwners[native.Edge]++
			usedPcurves[pc.id] = true
			curve, err := decode(126, pc.values, limits.MaxControlPoints)
			if err != nil {
				t.Fatal(err)
			}
			checkClippedCommonIGESOwner(t, i, j, face, native, support, curve)
		}
	}
	for i, owners := range edgeOwners {
		if owners != 2 {
			t.Fatalf("edge %d lost its second owner", i)
		}
	}
	return byType[508][0].id
}

func checkClippedCommonIGESOwner(t *testing.T, faceIndex, useIndex int, face trimmed.Face, use trimmed.Use, support, curve Entity) {
	t.Helper()
	for _, clock := range []float64{0, .5, 1} {
		// IGES stores the pcurve in shared model-edge direction; the
		// signed loop flag restores the source owner's traversal.
		nativeClock := clock
		if use.Reversed {
			nativeClock = 1 - clock
		}
		nativeUV := geom.Vec2{
			X: use.UVStart.X*(1-nativeClock) + use.UVEnd.X*nativeClock,
			Y: use.UVStart.Y*(1-nativeClock) + use.UVEnd.Y*nativeClock,
		}
		if use.Curve != nil {
			uv, err := use.Curve.EvalUV(nativeClock)
			if err != nil {
				t.Fatal(err)
			}
			nativeUV = uv
		}
		uv, err := curve.Eval(clock)
		if err != nil || uv.Z != 0 || math.Hypot(uv.X-nativeUV.X, uv.Y-nativeUV.Y) > 1e-13 {
			t.Fatalf("face %d use %d changed original UV at %g: got %v, want %v: %v", faceIndex, useIndex, clock, uv, nativeUV, err)
		}
		actual, err := support.Surface.Geometry.Eval(uv.X, uv.Y)
		if err != nil {
			t.Fatal(err)
		}
		want, err := face.Surface.Eval(nativeUV.X, nativeUV.Y)
		if err != nil || actual.Distance(want) > 1e-12 {
			t.Fatalf("face %d use %d changed original model incidence at %g: %v", faceIndex, useIndex, clock, err)
		}
	}
}

func checkOCCTClippedCommonIGES(t *testing.T, ctx context.Context, oracle string, raw []byte, body trimmed.Solid) occtExactBezierIGESComponent {
	t.Helper()
	result := readOCCTExactBezierIGES(t, ctx, oracle, raw)
	points := body.Vertices()
	if result.TransferredRoots != 1 || result.Solids != 1 || len(result.Components) != 1 ||
		result.Faces != len(body.Faces()) || result.IndexedVertices != len(points) {
		t.Fatalf("independent IGES material counts: %+v", result)
	}
	part := result.Components[0]
	if part.Faces != result.Faces || part.Edges != len(body.Edges()) || part.Shells != 1 || len(part.Vertices) != len(points) {
		t.Fatalf("independent IGES shell counts: %+v", part)
	}
	actual := make(map[[3]float64]bool, len(points))
	for _, p := range part.Vertices {
		actual[p] = true
	}
	for _, p := range points {
		if !actual[[3]float64{p.X, p.Y, p.Z}] {
			t.Fatalf("independent IGES vertex changed from source: %v", p)
		}
	}
	want, err := body.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if err != nil || math.Abs(part.Volume-want.Volume) > 1e-7 || math.Abs(part.Area-want.Area) > 1e-7 {
		t.Fatalf("independent IGES volume %.16g area %.16g; native volume %.16g area %.16g: %v", part.Volume, part.Area, want.Volume, want.Area, err)
	}
	t.Logf("OCCT transferred one valid solid: %d faces, %d edges, %d exact original vertices, volume %.16g, area %.16g",
		part.Faces, part.Edges, result.IndexedVertices, part.Volume, part.Area)
	return part
}

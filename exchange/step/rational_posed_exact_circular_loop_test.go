package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	rigidplacement "plainsolid/brep/placement"
	"plainsolid/geom"
)

type rationalWorldCircularLoopFixture struct {
	name     string
	material rationalPosedPCurveRepairMaterial
	check    func(testing.TB, float64, float64, [3]float64)
}

func rationalWorldCircularLoopFixtures(t testing.TB) []rationalWorldCircularLoopFixture {
	t.Helper()
	ctx := context.Background()
	opts := ConvexArcRepairOptions{MaxDistance: 1e-12}
	var cases []rationalWorldCircularLoopFixture
	for _, one := range rationalTiltFixtures {
		m, e := RepairRationalPosedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(rationalTiltSource(t, one)), Limits{}, opts)
		if e != nil {
			t.Fatal(e)
		}
		cases = append(cases, rationalWorldCircularLoopFixture{
			name: "single" + one.suffix, material: m,
			check: func(t testing.TB, volume, area float64, c [3]float64) {
				checkRationalTiltMeasures(t, volume, area, c, one)
			},
		})
	}
	m, e := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(rationalTiltGroupedSource(t)), Limits{}, opts)
	if e != nil {
		t.Fatal(e)
	}
	return append(cases, rationalWorldCircularLoopFixture{"grouped", m, checkRationalTiltGroupedMeasures})
}

func encodeRationalWorldCircularLoop(t testing.TB, m rationalPosedPCurveRepairMaterial, reverse bool) []byte {
	t.Helper()
	d := m.LocalDescriptor()
	d.Frame, _ = geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if reverse {
		d.Height = -d.Height
	}
	var out bytes.Buffer
	if e := WriteRationalPosedExactCircularLoopExtrusion(context.Background(), &out, d, m.Pose()); e != nil {
		t.Fatal(e)
	}
	return out.Bytes()
}

func TestRationalPosedExactCircularLoopStrictRoundTrip(t *testing.T) {
	cases := rationalWorldCircularLoopFixtures(t)
	ctx := context.Background()
	var originals [][]byte
	for _, tc := range cases {
		originals = append(originals, encodeRationalWorldCircularLoop(t, tc.material, false))
	}
	var joined bytes.Buffer
	if e := JoinIndependentModels(&joined, originals, Limits{}); e != nil {
		t.Fatal(e)
	}
	parts, e := SplitIndependentModels(&joined, Limits{})
	if e != nil || len(parts) != len(cases) {
		t.Fatal("standard rooted STEP packaging", len(parts), e)
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, e := ReadExactCircularLoopExtrusion(ctx, bytes.NewReader(parts[i]), Limits{}); e == nil {
				t.Fatal("old signed-coordinate schema accepted the rational world graph")
			}
			m, e := ReadRationalPosedExactCircularLoopExtrusion(ctx, bytes.NewReader(parts[i]), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			want, _ := tc.material.Pose().Matrix()
			got, e := m.Pose.Matrix()
			if e != nil || !reflect.DeepEqual(got, want) || frameTransform(m.Descriptor.Frame) != geom.Identity() {
				t.Fatal("strict reader changed exact world placement", got, want, e)
			}
			actual, e := prismNativeBytes(m.Native)
			if e != nil {
				t.Fatal(e)
			}
			expected, e := prismNativeBytes(tc.material.Solid())
			if e != nil || !bytes.Equal(actual, expected) {
				t.Fatal("strict reader changed exact product-local material", e)
			}
			if _, e = simple(m.Source, m.Body, "MANIFOLD_SOLID_BREP", 2); e != nil {
				t.Fatal("lost retained world graph", e)
			}
			var again bytes.Buffer
			if e = WriteRationalPosedExactCircularLoopExtrusion(ctx, &again, m.Descriptor, m.Pose); e != nil || !bytes.Equal(again.Bytes(), originals[i]) {
				t.Fatal("canonical world STEP changed on re-export", e)
			}
		})
	}
	t.Run("negative station height", func(t *testing.T) {
		raw := encodeRationalWorldCircularLoop(t, cases[1].material, true)
		m, e := ReadRationalPosedExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{})
		if e != nil || m.Descriptor.Height != -cases[1].material.LocalDescriptor().Height {
			t.Fatal("wall station lost negative height", e)
		}
		got, _ := m.Pose.Matrix()
		want, _ := cases[1].material.Pose().Matrix()
		if !reflect.DeepEqual(got, want) {
			t.Fatal("first cap was mistaken for pose origin", got, want)
		}
	})
}

func TestRationalPosedExactCircularLoopStrictRejection(t *testing.T) {
	ctx := context.Background()
	m := rationalWorldCircularLoopFixtures(t)[0].material
	raw := encodeRationalWorldCircularLoop(t, m, false)
	for _, edit := range []struct {
		name string
		run  func(*File) error
	}{
		{"legacy schema", func(f *File) error {
			return setExactCircularLoopProductIdentifier(f, rationalPosedExactCircularLoopProduct, localExactCircularLoopProduct)
		}},
		{"inexact support direction", func(f *File) error {
			for _, id := range f.Order {
				p, e := simple(f, id, "DIRECTION", 2)
				if e == nil {
					p.Args[1].Items[0].Text = "0.8000000000001"
					return nil
				}
			}
			return geom.ErrDomain
		}},
		{"perturbed world vertex", func(f *File) error {
			for _, id := range f.Order {
				v, e := simple(f, id, "VERTEX_POINT", 2)
				if e != nil {
					continue
				}
				p, e := simple(f, v.Args[1].Ref, "CARTESIAN_POINT", 2)
				if e != nil {
					return e
				}
				x, e := liftSourceRat(p.Args[1].Items[0])
				if e != nil {
					return e
				}
				p.Args[1].Items[0].Text = x.Add(x, big.NewRat(1, 1000000000000000)).FloatString(20)
				return nil
			}
			return geom.ErrDomain
		}},
	} {
		t.Run(edit.name, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			if e = edit.run(f); e != nil {
				t.Fatal(e)
			}
			bad, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			got, e := ReadRationalPosedExactCircularLoopExtrusion(ctx, bytes.NewReader(bad), Limits{})
			if e == nil || got.Source != nil {
				t.Fatal("strict reader accepted malformed world source", e)
			}
		})
	}
	if _, e := ReadRationalPosedExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{MaxBytes: 64}); e == nil {
		t.Fatal("source byte budget ignored")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	var out bytes.Buffer
	if e := WriteRationalPosedExactCircularLoopExtrusion(canceled, &out, m.LocalDescriptor(), m.Pose()); !errors.Is(e, context.Canceled) || out.Len() != 0 {
		t.Fatal("canceled writer", e)
	}
	if e := WriteRationalPosedExactCircularLoopExtrusion(ctx, sideDiskShortWriter{}, m.LocalDescriptor(), m.Pose()); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal("short writer", e)
	}
	z := new(big.Rat)
	nonDecimal, e := rigidplacement.NewRationalRigid([3][4]*big.Rat{
		{big.NewRat(5, 13), z, big.NewRat(12, 13), z},
		{z, big.NewRat(1, 1), z, z},
		{big.NewRat(-12, 13), z, big.NewRat(5, 13), z},
	}, 512)
	if e != nil {
		t.Fatal(e)
	}
	if e = WriteRationalPosedExactCircularLoopExtrusion(ctx, &out, m.LocalDescriptor(), nonDecimal); e == nil || out.Len() != 0 {
		t.Fatal("writer rounded a nonterminating pose into STEP or emitted partial output", e)
	}
}

func TestRationalPosedExactCircularLoopLive(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_ARC_ORIGINAL to the pinned author helper")
	}
	for _, tc := range rationalWorldCircularLoopFixtures(t) {
		t.Run(tc.name, func(t *testing.T) {
			raw := encodeRationalWorldCircularLoop(t, tc.material, false)
			path := filepath.Join(t.TempDir(), "canonical-world.step")
			if e := os.WriteFile(path, raw, 0600); e != nil {
				t.Fatal(e)
			}
			out, e := exec.Command(oracle, "read", path).CombinedOutput()
			if e != nil {
				t.Fatalf("pinned OCCT transfer: %v\n%s", e, out)
			}
			var got struct {
				Volume, Area  float64
				Centroid      [3]float64
				Faces, Solids int
			}
			if e = json.Unmarshal(bytes.TrimSpace(out), &got); e != nil ||
				got.Faces != len(tc.material.Solid().Faces()) || got.Solids != 1 {
				t.Fatalf("invalid canonical world material: %s (%v)", out, e)
			}
			tc.check(t, got.Volume, got.Area, got.Centroid)
		})
	}
}

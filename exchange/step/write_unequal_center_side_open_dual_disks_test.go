package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func unequalCenterSideDiskDescriptor(t *testing.T, through, swap, posed bool) UnequalCenterSideOpenDualDiskDescriptor {
	t.Helper()
	frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if posed {
		frame, _ = geom.NewFrame(geom.Vec3{X: 60, Z: 6}, geom.Vec3{X: -1}, geom.Vec3{Z: -1})
	}
	depth := 3.
	if through {
		depth = 6
	}
	d := SideOpenDualDiskDescriptor{Name: "unequalCenter external-center walls", Frame: frame, Max: geom.Vec2{X: 60, Y: 10}, Discs: [2]trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 7, Y: -3}, Radius: 5}, {Center: geom.Vec2{X: 30, Y: -20}, Radius: 29}}, Height: 6, Depth: depth, LengthUnit: .001, Tolerance: 1e-9}
	if swap {
		d.Discs[0], d.Discs[1] = d.Discs[1], d.Discs[0]
	}
	return UnequalCenterSideOpenDualDiskDescriptor{SideOpenDualDiskDescriptor: d, Join: [2]*big.Rat{big.NewRat(10, 1), big.NewRat(1, 1)}}
}
func TestWriteUnequalCenterSideOpenDualDiskProduct(t *testing.T) {
	for _, through := range []bool{false, true} {
		for _, swap := range []bool{false, true} {
			for _, posed := range []bool{false, true} {
				d := unequalCenterSideDiskDescriptor(t, through, swap, posed)
				var b bytes.Buffer
				if e := WriteUnequalCenterSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
					t.Fatal(through, swap, posed, e)
				}
				f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
				if e != nil {
					t.Fatal(e)
				}
				counts := map[string]int{}
				radii := map[float64]int{}
				for _, en := range f.Entities {
					for _, r := range en.Records {
						counts[r.Type]++
						if r.Type == "PRODUCT" && r.Args[0].Text != unequalCenterSideDiskProductID {
							t.Fatal("missing registered unequalCenter class")
						}
						if r.Type == "CYLINDRICAL_SURFACE" {
							radius, e := cylinderSupportNumber(r.Args[2])
							if e != nil {
								t.Fatal(e)
							}
							radii[radius]++
						}
					}
				}
				wantFaces := 16
				if through {
					wantFaces = 9
				}
				if counts["ADVANCED_FACE"] != wantFaces || counts["CIRCLE"] != 4 || counts["PCURVE"] != 0 || counts["SURFACE_CURVE"] != 0 || radii[5] != 1 || radii[29] != 1 {
					t.Fatal(counts, radii)
				}
				if _, e = ReadGeneralSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, 1e-9); e == nil {
					t.Fatal("ordinary general reader widened")
				}
				if _, e = ReadUnequalSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, 1e-9); e == nil {
					t.Fatal("between-centers reader widened")
				}
				if _, e = ReadSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, 1e-9); e == nil {
					t.Fatal("equal reader widened")
				}
				if _, e = ReadSideOpenDualDiskProductWithOptions(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, 1e-9, SideOpenDualDiskReadOptions{AllowReversedModelLines: true, AllowRephasedModelCircles: true, AllowOppositeModelCircleNormals: true, AllowOppositePlaneNormals: true}); e == nil {
					t.Fatal("equal reader options widened")
				}
				var repeated bytes.Buffer
				if e = WriteUnequalCenterSideOpenDualDiskProduct(context.Background(), &repeated, d); e != nil || !bytes.Equal(repeated.Bytes(), b.Bytes()) {
					t.Fatal("nondeterministic", e)
				}
			}
		}
	}
}
func TestOCCTWriteUnequalCenterSideOpenDualDiskProduct(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_OVERLAPPING_SIDE_POCKETS")
	if oracle == "" {
		t.Skip("optional independent OCCT helper")
	}
	type measures struct {
		Volume, Area float64
		Center       [3]float64
		Cylinders    int
	}
	run := func(mode, path string) measures {
		out, e := exec.Command(oracle, mode, path).CombinedOutput()
		if e != nil {
			t.Fatalf("%s: %v %s", mode, e, out)
		}
		start := bytes.LastIndex(out, []byte("{\"Faces\""))
		if start < 0 {
			t.Fatal(string(out))
		}
		var m measures
		if e = json.Unmarshal(bytes.TrimSpace(out[start:]), &m); e != nil {
			t.Fatal(e)
		}
		return m
	}
	for _, through := range []bool{false, true} {
		mode := "unequal-center-side"
		if through {
			mode = "unequal-center-through"
		}
		reference := run(mode, filepath.Join(t.TempDir(), "independent.step"))
		for _, swap := range []bool{false, true} {
			for _, posed := range []bool{false, true} {
				d := unequalCenterSideDiskDescriptor(t, through, swap, posed)
				var b bytes.Buffer
				if e := WriteUnequalCenterSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
					t.Fatal(e)
				}
				path := filepath.Join(t.TempDir(), "native.step")
				if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
					t.Fatal(e)
				}
				got := run("read", path)
				expected := d.Frame.Origin().Add(d.Frame.XAxis().Scale(reference.Center[0])).Add(d.Frame.YAxis().Scale(reference.Center[1])).Add(d.Frame.ZAxis().Scale(6 - reference.Center[2]))
				actual := geom.Vec3{X: got.Center[0], Y: got.Center[1], Z: got.Center[2]}
				if got.Cylinders != 2 || math.Abs(got.Volume-reference.Volume) > 1e-7 || math.Abs(got.Area-reference.Area) > 1e-7 || actual.Sub(expected).Length() > 1e-8 {
					t.Fatal(through, swap, posed, got, reference, expected)
				}
				t.Logf("through=%v swap=%v posed=%v volume=%.12g area=%.12g", through, swap, posed, got.Volume, got.Area)
			}
		}
	}
}
func TestWriteUnequalCenterSideOpenDualDiskRejects(t *testing.T) {
	for _, edit := range []func(*UnequalCenterSideOpenDualDiskDescriptor){
		func(d *UnequalCenterSideOpenDualDiskDescriptor) { d.Join[0] = nil },
		func(d *UnequalCenterSideOpenDualDiskDescriptor) {
			d.Join[0] = big.NewRat(10000000000001, 1000000000000)
		},
		func(d *UnequalCenterSideOpenDualDiskDescriptor) {
			d.Join = [2]*big.Rat{big.NewRat(1659, 409), big.NewRat(-2880, 409)}
		},
		func(d *UnequalCenterSideOpenDualDiskDescriptor) { d.Discs[1].Center.Y = -3 },
		func(d *UnequalCenterSideOpenDualDiskDescriptor) { d.Depth = 7 },
		func(d *UnequalCenterSideOpenDualDiskDescriptor) { d.LengthUnit = .0254 },
		func(d *UnequalCenterSideOpenDualDiskDescriptor) {
			d.Join[0] = new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 4097))
		},
	} {
		d := unequalCenterSideDiskDescriptor(t, false, false, false)
		edit(&d)
		var b bytes.Buffer
		if e := WriteUnequalCenterSideOpenDualDiskProduct(context.Background(), &b, d); e == nil || b.Len() != 0 {
			t.Fatal("invalid/partial", e)
		}
	}
	d := unequalCenterSideDiskDescriptor(t, false, false, false)
	var b bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := WriteUnequalCenterSideOpenDualDiskProduct(ctx, &b, d); e != context.Canceled {
		t.Fatal(e)
	}
	if e := WriteUnequalCenterSideOpenDualDiskProduct(context.Background(), sideDiskShortWriter{}, d); e != io.ErrShortWrite {
		t.Fatal(e)
	}
	if e := WriteUnequalCenterSideOpenDualDiskProduct(context.Background(), &b, d); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadOffsetSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, 1e-9); e == nil {
		t.Fatal("common ordinate reader widened")
	}
	if _, e := ReadExactWallSideOpenDualDiskProduct(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, 1e-9); e == nil {
		t.Fatal("side-centered reader widened")
	}
}

package offset

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"os/exec"
	"strings"
	"testing"

	"plainsolid/geom"
	"plainsolid/solid"
)

func volumeBox(x0, x1, y0, y1, z0, z1 float64) geom.Box {
	return geom.Box{Min: geom.Vec3{X: x0, Y: y0, Z: z0}, Max: geom.Vec3{X: x1, Y: y1, Z: z1}}
}

func volumeCells(t *testing.T, boxes ...geom.Box) solid.Solid {
	t.Helper()
	s, err := solid.New(boxes...)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func checkOffsetVolume(t *testing.T, result VolumeResult, err error, want int64) {
	t.Helper()
	if err != nil || result.ExactVolume == nil || result.ExactVolume.Cmp(big.NewRat(want, 1)) != 0 {
		t.Fatalf("wrong exact volume, wanted %d: %v %v", want, result.ExactVolume, err)
	}
	if want == 0 {
		if len(result.Cells) != 0 || len(result.Solid.Vertices()) != 0 {
			t.Fatal("a lower dimensional remnant entered the closed solid")
		}
		return
	}
	p, err := result.Solid.MassProperties()
	if err != nil || math.Abs(p.Volume-float64(want)) > 1e-10 || len(result.Cells) == 0 || result.MaxDisplacement != 0 {
		t.Fatalf("native topology lost the exact represented set: %v %v", p, err)
	}
}

func TestOrthogonalVolumeOffsetStepAndCavity(t *testing.T) {
	ctx := context.Background()
	step := volumeCells(t, volumeBox(0, 4, 0, 2, 0, 2), volumeBox(0, 2, 0, 2, 2, 4))
	brep, err := step.BRep(solid.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		distance float64
		volume   int64
	}{{.5, 63}, {-.5, 5}, {0, 24}} {
		// Independently integrate the two XZ arms of the step and their
		// intersection, then multiply by the changed Y thickness.
		r, err := FromOrthogonalVolume(ctx, brep, c.distance, VolumeOptions{})
		checkOffsetVolume(t, r, err, c.volume)
	}
	outer, inner := volumeCells(t, volumeBox(0, 10, 0, 10, 0, 10)), volumeCells(t, volumeBox(3, 7, 3, 7, 3, 7))
	cavity, err := outer.Difference(inner)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		distance       float64
		volume         int64
		boundaryShells int
	}{{1, 1720, 2}, {-1, 296, 2}, {2, 2744, 1}, {-1.5, 0, 0}} {
		// A positive offset grows the outer box and shrinks the void;
		// an inward offset applies the opposite change to each shell.
		r, err := ComputeVolume(ctx, cavity, c.distance, VolumeOptions{})
		checkOffsetVolume(t, r, err, c.volume)
		ss, err := r.Solid.Shells()
		if err != nil || len(ss) != c.boundaryShells {
			t.Fatal("offset cavity shell count", c.distance, len(ss), err)
		}
	}
}

func TestOrthogonalVolumeOffsetTopologicalTransitions(t *testing.T) {
	ctx := context.Background()
	left := volumeBox(0, 2, 0, 3, 0, 3)
	right := volumeBox(4, 6, 0, 3, 0, 3)
	neck := volumeBox(2, 4, 1, 2, 1, 2)
	joined := volumeCells(t, left, neck, right)
	r, err := ComputeVolume(ctx, joined, -.5, VolumeOptions{})
	checkOffsetVolume(t, r, err, 8)
	if s, err := r.Solid.Shells(); err != nil || len(s) != 2 || len(r.Cells) != 2 {
		t.Fatal("collapsed neck must split into two positive solids", len(s), len(r.Cells), err)
	}
	// Two expanded boxes meeting only at a vertical edge still form a
	// regularized set, but its boundary cannot be published as a manifold.
	struck := volumeCells(t, volumeBox(0, 1, 0, 1, 0, 1), volumeBox(2, 3, 2, 3, 0, 1))
	if r, err := ComputeVolume(ctx, struck, .5, VolumeOptions{}); err == nil || r.ExactVolume != nil || len(r.Cells) != 0 {
		t.Fatal("nonmanifold offset published a partial or invalid result", err)
	}
}

func TestOrthogonalVolumeOffsetExactCoordinatesAndRefusal(t *testing.T) {
	ctx := context.Background()
	input := volumeCells(t, volumeBox(0, 1, 0, 1, 0, 1))
	result, err := ComputeVolume(ctx, input, .1, VolumeOptions{})
	if err != nil || result.ExactVolume == nil || len(result.Cells) != 1 {
		t.Fatal("positive represented distance was not bounded", err)
	}
	d := new(big.Rat).SetFloat64(.1)
	length := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(big.NewRat(2, 1), d))
	want := new(big.Rat).Mul(length, new(big.Rat).Mul(length, length))
	if want.Cmp(result.ExactVolume) != 0 || result.MaxDisplacement <= 0 || result.Cells[0].Max[0].Cmp(new(big.Rat).Add(big.NewRat(1, 1), d)) != 0 {
		t.Fatal("exact decimal volume or station was rounded", result.ExactVolume, result.MaxDisplacement)
	}
	for _, c := range []struct {
		o   VolumeOptions
		err error
	}{{VolumeOptions{MaxGridEntries: 63}, ErrLimit}, {VolumeOptions{MaxWork: 12}, ErrLimit}, {VolumeOptions{MaxCoordinateError: 1e-20}, ErrPrecision}} {
		if r, err := ComputeVolume(ctx, input, .1, c.o); !errors.Is(err, c.err) || r.ExactVolume != nil || len(r.Cells) != 0 {
			t.Fatal("short limit returned partial material", c, err)
		}
	}
	stopped, cancel := context.WithCancel(ctx)
	cancel()
	if r, err := ComputeVolume(stopped, input, .1, VolumeOptions{}); !errors.Is(err, context.Canceled) || r.ExactVolume != nil {
		t.Fatal("canceled offset returned partial material", err)
	}
}

func TestOrthogonalVolumeOffsetPinnedOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_ORTHOGONAL_VOLUME_OFFSET")
	if oracle == "" {
		t.Skip("pinned OCCT orthogonal volume author not configured")
	}
	ctx := context.Background()
	stepped := volumeCells(t, volumeBox(0, 4, 0, 2, 0, 2), volumeBox(0, 2, 0, 2, 2, 4))
	outer := volumeCells(t, volumeBox(0, 10, 0, 10, 0, 10))
	inner := volumeCells(t, volumeBox(3, 7, 3, 7, 3, 7))
	cavity, err := outer.Difference(inner)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name     string
		source   solid.Solid
		distance float64
		wall     bool
		open     bool
		bodies   int
	}{
		{"step-outward", stepped, .5, false, false, 1}, {"step-inward", stepped, -.5, false, false, 1},
		{"cavity-outward", cavity, 1, false, false, 1}, {"cavity-inward", cavity, -1, false, false, 1},
		{"step-wall", stepped, .5, true, false, 1}, {"cavity-wall", cavity, 1, true, false, 2},
		{"step-open-top", stepped, .5, true, true, 1}, {"cavity-open-top", cavity, 1, true, true, 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			out, err := exec.Command(oracle, c.name).CombinedOutput()
			if err != nil {
				t.Fatal("pinned OCCT author failed", err, string(out))
			}
			var valid, bodies, shells int
			var volume, area float64
			if n, err := fmt.Fscan(strings.NewReader(string(out)), &valid, &bodies, &shells, &volume, &area); err != nil || n != 5 || valid != 1 || bodies != c.bodies {
				t.Fatal("unexpected valid-body count in pinned OCCT", string(out), err)
			}
			op := ComputeVolume
			if c.wall {
				op = ThickenVolume
			}
			if c.open {
				op = func(ctx context.Context, source solid.Solid, thickness float64, options VolumeOptions) (VolumeResult, error) {
					return ThickenVolumeOpenPlane(ctx, source, thickness, VolumeOpenPlane{Axis: 2, High: true}, options)
				}
			}
			result, err := op(ctx, c.source, c.distance, VolumeOptions{})
			if err != nil {
				t.Fatal(err)
			}
			p, err := result.Solid.MassProperties()
			if err != nil || math.Abs(p.Volume-volume) > 1e-10 || math.Abs(p.SurfaceArea-area) > 1e-10 {
				t.Fatal("native offset differs from independent OCCT material", p, string(out), err)
			}
			ss, err := result.Solid.Shells()
			if err != nil || len(ss) != shells {
				t.Fatal("native offset differs from independent OCCT shell count", len(ss), shells, err)
			}
			positive := 0
			for _, sh := range ss {
				mass, err := sh.MassProperties()
				if err != nil {
					t.Fatal(err)
				}
				if mass.Volume > 0 {
					positive++
				}
			}
			if positive != bodies {
				t.Fatal("native offset has a different number of positively oriented material shells", positive, bodies)
			}
			t.Logf("pinned valid body count %d, shells %d, volume %.16g, area %.16g", bodies, shells, volume, area)
		})
	}
}

func TestOrthogonalVolumeClosedWallLayer(t *testing.T) {
	ctx := context.Background()
	box := volumeCells(t, volumeBox(0, 6, 0, 6, 0, 6))
	step := volumeCells(t, volumeBox(0, 4, 0, 2, 0, 2), volumeBox(0, 2, 0, 2, 2, 4))
	neck := volumeCells(t, volumeBox(0, 2, 0, 3, 0, 3), volumeBox(2, 4, 1, 2, 1, 2), volumeBox(4, 6, 0, 3, 0, 3))
	outer := volumeCells(t, volumeBox(0, 10, 0, 10, 0, 10))
	inner := volumeCells(t, volumeBox(3, 7, 3, 7, 3, 7))
	cavity, err := outer.Difference(inner)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name      string
		source    solid.Solid
		thickness float64
		volume    int64
		shells    int
	}{
		{"box", box, 1, 152, 2}, {"full collapse", box, 3, 216, 1},
		{"step", step, .5, 19, 2}, {"split core", neck, .5, 30, 3}, {"separate cavity walls", cavity, 1, 640, 4},
	} {
		t.Run(c.name, func(t *testing.T) {
			original, err := c.source.BRep(solid.Options{})
			if err != nil {
				t.Fatal(err)
			}
			r, err := ThickenOrthogonalVolume(ctx, original, c.thickness, VolumeOptions{})
			checkOffsetVolume(t, r, err, c.volume)
			ss, err := r.Solid.Shells()
			if err != nil || len(ss) != c.shells {
				t.Fatal("wrong number of closed inward or outward wall shells", len(ss), err)
			}
		})
	}
	for _, options := range []VolumeOptions{{MaxGridEntries: 20}, {MaxWork: 10}} {
		if r, err := ThickenVolume(ctx, box, .5, options); !errors.Is(err, ErrLimit) || r.ExactVolume != nil || len(r.Cells) != 0 {
			t.Fatal("short closed wall budget returned partial material", err)
		}
	}
	if r, err := ThickenVolume(ctx, box, 0, VolumeOptions{}); !errors.Is(err, ErrInvalid) || r.ExactVolume != nil {
		t.Fatal("zero thickness published a wall layer", err)
	}
}

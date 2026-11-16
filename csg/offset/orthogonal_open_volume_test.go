package offset

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/geom"
	"plainsolid/solid"
)

func TestOrthogonalOpenVolumeWallsOnEachSupport(t *testing.T) {
	ctx := context.Background()
	box := volumeCells(t, volumeBox(0, 6, 0, 6, 0, 6))
	for axis := range 3 {
		for _, high := range []bool{false, true} {
			r, err := ThickenVolumeOpenPlane(ctx, box, 1, VolumeOpenPlane{Axis: axis, High: high}, VolumeOptions{})
			// The open cavity of the 6³ stock has volume 4×4×5.
			checkOffsetVolume(t, r, err, 136)
			ss, err := r.Solid.Shells()
			if err != nil || len(ss) != 1 {
				t.Fatal("the lip should connect the inner and outer cup walls", axis, high, len(ss), err)
			}
		}
	}
	separated := volumeCells(t, volumeBox(0, 4, 0, 4, 0, 4), volumeBox(6, 10, 0, 4, 0, 4))
	r, err := ThickenVolumeOpenPlane(ctx, separated, 1, VolumeOpenPlane{Axis: 2, High: true}, VolumeOptions{})
	checkOffsetVolume(t, r, err, 104)
	ss, err := r.Solid.Shells()
	if err != nil || len(ss) != 2 {
		t.Fatal("two openings on the selected support should retain two solids", len(ss), err)
	}
}

func TestOrthogonalOpenVolumeLipsAndUnaffectedSupports(t *testing.T) {
	ctx := context.Background()
	steps := volumeCells(t, volumeBox(0, 4, 0, 2, 0, 2), volumeBox(0, 2, 0, 2, 2, 4))
	stock, err := steps.BRep(solid.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		high bool
		want int64
	}{{true, 37}, {false, 35}} {
		r, err := ThickenOrthogonalVolumeOpenPlane(ctx, stock, .5, VolumeOpenPlane{Axis: 2, High: c.high}, VolumeOptions{})
		// The upper mouth cuts 1×1×.5 from the closed wall of volume
		// 19. The lower cap's broader mouth cuts 3×1×.5.
		if err != nil || r.ExactVolume == nil || r.ExactVolume.Cmp(big.NewRat(c.want, 2)) != 0 {
			t.Fatal("opening did not retain the original step", c, r.ExactVolume, err)
		}
		p, err := r.Solid.MassProperties()
		if err != nil || math.Abs(p.Volume-float64(c.want)/2) > 1e-10 {
			t.Fatal("native lip lost positive exact material", p, err)
		}
		if c.high {
			cells, err := solid.FromOrthogonal(r.Solid, solid.Options{})
			if err != nil || cells.Contains(geom.Vec3{X: 1, Y: 1, Z: 3.75}) ||
				!cells.Contains(geom.Vec3{X: .25, Y: 1, Z: 3.75}) || !cells.Contains(geom.Vec3{X: 3, Y: 1, Z: 1.75}) {
				t.Fatal("opening changed the lip or punched through the unselected lower top face", err)
			}
		}
	}
	outer := volumeCells(t, volumeBox(0, 10, 0, 10, 0, 10))
	inner := volumeCells(t, volumeBox(3, 7, 3, 7, 3, 7))
	cavity, err := outer.Difference(inner)
	if err != nil {
		t.Fatal(err)
	}
	r, err := ThickenVolumeOpenPlane(ctx, cavity, 1, VolumeOpenPlane{Axis: 2, High: true}, VolumeOptions{})
	checkOffsetVolume(t, r, err, 576)
	ss, err := r.Solid.Shells()
	if err != nil || len(ss) != 3 {
		t.Fatal("opening the exterior should preserve the separate internal wall", len(ss), err)
	}
}

func TestOrthogonalOpenVolumeRefusesAnAbsentMouth(t *testing.T) {
	ctx := context.Background()
	box := volumeCells(t, volumeBox(0, 6, 0, 6, 0, 6))
	withTip := volumeCells(t, volumeBox(0, 6, 0, 6, 0, 6), volumeBox(2.75, 3.25, 2.75, 3.25, 6, 8))
	for _, c := range []struct {
		solid     solid.Solid
		thickness float64
	}{{box, 3}, {withTip, 1}} {
		r, err := ThickenVolumeOpenPlane(ctx, c.solid, c.thickness, VolumeOpenPlane{Axis: 2, High: true}, VolumeOptions{})
		if !errors.Is(err, ErrNoOpenCore) || r.ExactVolume != nil || len(r.Cells) != 0 {
			t.Fatal("no complete mouth should publish a result", err)
		}
	}
	if r, err := ThickenVolumeOpenPlane(ctx, box, 1, VolumeOpenPlane{Axis: -1}, VolumeOptions{}); !errors.Is(err, ErrInvalid) || r.ExactVolume != nil {
		t.Fatal("invalid selected support published a result", err)
	}
}

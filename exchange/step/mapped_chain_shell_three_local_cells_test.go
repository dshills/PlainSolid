package step

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"strings"
	"testing"
	"time"

	"plainsolid/geom"
)

func TestOCCTMappedChainRectangularThreeLocalSupportKnotCells(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(threeLocalCellMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	verifyLocalCellMappedChain(t, context.Background(), raw, 3)
}

func TestOCCTMappedChainRectangularThreeLocalSupportKnotCellsClosedDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw, err := os.ReadFile(threeLocalCellMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, threeLocalCellMappedChainOptions())
	if err != nil {
		t.Fatal(err)
	}
	verifyMappedChainCutDisplay(t, ctx, m)
}

func TestOCCTMappedChainRectangularThreeLocalSupportKnotCellsRejectsInvalidSource(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw, err := os.ReadFile(threeLocalCellMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	o := threeLocalCellMappedChainOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	older := localCellMappedChainOptions()
	older.AllowUpToSixSupportKnots = true
	if bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, older); err == nil || len(bad.Crossings) != 0 {
		t.Fatal("coenabled older gates must retain their original scope", err)
	}
	north := 0
	if !m.Shell.IndependentOwnerBases()[0] {
		north = 1
	}
	for _, tc := range []struct {
		name, diagnosis string
		want            error
		edit            func(*File)
	}{
		{"incomplete stream at second cell endpoint", "complete local support event partition", geom.ErrConvergence, func(f *File) {
			moveLocalSouthCrease(t, f, m.FaceIDs[1-north], .5)
		}},
		{"complete raw event moved into incompatible first cell", "one owner-side crossing per original edge", geom.ErrConvergence, func(f *File) {
			moveLocalSouthCrease(t, f, m.FaceIDs[1-north], 369.0/2048)
		}},
		{"third shared knot loses one representable raw stop", "not representable", geom.ErrUnrepresentable, func(f *File) {
			_, knots, _, err := mappedChainStepSplineParts(f, m.Chain[2].Owners[north].BasisID)
			if err != nil {
				t.Fatal(err)
			}
			knots[1].Items[1].Text = "863."
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := Parse(bytes.NewReader(raw), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			tc.edit(f)
			bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, o)
			if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), tc.diagnosis) || len(bad.Crossings) != 0 || len(bad.Native.ChainEdges) != 0 {
				t.Fatal("the exact local partition must be complete before any repair is published", err)
			}
		})
	}
}

func TestOCCTMappedChainRectangularThreeLocalSupportKnotCellsLive(t *testing.T) {
	t.Parallel()
	author, reader := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_THREE_CELLS_AUTHOR"), os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if author == "" || reader == "" {
		t.Skip("optional pinned OCCT three-cell author and STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "occt-authored-three-support-cells.step")
	if out, err := exec.CommandContext(ctx, author, "write-three-support-cells", path).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT author failed: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	canonical := verifyLocalCellMappedChain(t, ctx, raw, 3)
	verifyPinnedMappedChainTransfer(t, ctx, reader, raw, canonical, mappedChainThreeLocalCellVolume, mappedChainThreeLocalCellArea)
}

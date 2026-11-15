package step

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"testing"

	"plainsolid/brep/trimmed"
)

func TestMappedChainSupportKnotNativeRecordCompatibilityV8(t *testing.T) {
	ctx := context.Background()
	// Native and canonical STEP bytes from the original v7 through v9 proofs,
	// captured before their support-cut bounds or cell scopes were extended.
	for _, tc := range []struct {
		path, nativeSHA256, stepSHA256 string
		options                        MappedChainShellSTEPOptions
	}{
		{mixedBasisUnsplitMappedChainPath, "ab561af2b387b267a6d0fcc3a008a364a45f8fdbd7911a3a4bc74f519a556412",
			"ecc8b0b80565da5f3eb34585060f9c5ac6ed9d3a358ca9c83ab2b0995a553137", unsplitMappedChainMixedBasisOptions()},
		{fourKnotMappedChainPath, "2e773fa632fc5810d95adf8bd44467de0bb49c3d18e0b06276fe0712eaad2475",
			"c248b639ad9b8206b4f45736786cfea2db97823e1d6cdbf38bc08fd660a9ab24", fourKnotMappedChainOptions()},
		{mappedChainMultipleKnotFixtures[0].path, "2a226db61cfb2c87ebc41b9edf3029b326f4af9890434a0a5227dd2367fc8ff7",
			"da0b3a75e7d4ab0e6abed45134f5564826ae5ec78e6475fcc9551fef18deb543", multipleKnotMappedChainOptions()},
		{mappedChainMultipleKnotFixtures[1].path, "d7593e2f36cb22b8a86a80a9ffdedf1e40125734209c15225417bc2a28def9db",
			"f44e264c5747e1c8d928542d753efe4383b4702876302c1b063de3acfe768168", multipleKnotMappedChainOptions()},
		{localCellMappedChainPath, "bcfa334018e5954bfa51756b6a1d7a9bf3c2036c8e480c3ee9eb9a56e5582c7f",
			"32a7789cea5621a0d9eaf2bfce4356bc2a1b8968e1cf7fae5675341307a06ba0", localCellMappedChainOptions()},
	} {
		raw, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		tc.options.AllowUpToSixSupportKnots = true
		tc.options.AllowMultipleSupportKnotCells = true
		tc.options.AllowThreeSupportKnotCells = true
		m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, tc.options)
		if err != nil {
			t.Fatal(err)
		}
		var native, step bytes.Buffer
		if err := m.Shell.WriteJSON(ctx, &native, trimmed.MappedChainShellNativeOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := WriteMappedChainRectangularOriginalShell(ctx, &step, m, Limits{}, tc.options); err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(native.Bytes())) != tc.nativeSHA256 || fmt.Sprintf("%x", sha256.Sum256(step.Bytes())) != tc.stepSHA256 {
			t.Fatal("the same original source must produce the same native and canonical bytes with the extended gate", tc.path)
		}
	}
}

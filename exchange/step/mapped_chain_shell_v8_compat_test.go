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
	// Native and canonical STEP bytes from the earlier inferred v3/v4 and the v7 through v10 proofs,
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
		{threeLocalCellMappedChainPath, "de6e1057c4c5b36ca61292318f3e042a5e48ba98003c4f5a335e598191505158",
			"4dda9544dd51f94b86551bb1fa3ddd5a524b46998e871153d0476d2563968b14", threeLocalCellMappedChainOptions()},
		{separatedCellMappedChainPath, "dfe9c4727e60353c1c2e41699befde7b279c30e439d0ac92279c2dee1cc93b3f",
			"882efc77ce9618c84825fc06e05b0f36e4cefb64a8bab3acac00b72a77af94ae", separatedCellMappedChainOptions()},
		{sameCellAsynchronousMappedChainPath, "5622e54178f369b3396916c5b90b7340632b310fa41a89c9ae52e997ed9af285",
			"e8b8a4648dfaeaa820dbfc6a250947f3387c1c913cf4e4bfb6bf2d297e56de38", sameCellAsynchronousOptions()},
		{inferredSharedMappedChainPath, "c2f34ccab09964dbc1b32661ecdc25a24ad21d6a65742e0bc6c868725db222e5",
			"1b02f5358b051bdd8c4900bf171acefd5dec395eafbfc0eb146ab9ff64e8bb0f", inferredSharedMappedChainOptions()},
		{inferredSharedPairMappedChainPath, "75dc81e678a1dd71d09dca4541f33f5492a1d670eff27a04ce51947c59d72662",
			"de6c246e34b515a006535c12f0c436bde6a66f553164aea874c2d1cd99969fd5", inferredSharedMappedChainOptions()},
	} {
		raw, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		tc.options.AllowTwoInteriorRoofKnots = true
		tc.options.AllowUpToSixSupportKnots = true
		tc.options.AllowMultipleSupportKnotCells = true
		tc.options.AllowThreeSupportKnotCells = true
		tc.options.AllowSeparatedSupportKnotCells = true
		tc.options.AllowSameCellAsynchronousSupportKnots = true
		tc.options.AllowInferredSharedSupportKnot = true
		tc.options.AllowThreeSupportKnots = true
		m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, tc.options)
		if err != nil {
			t.Fatal(tc.path, err)
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

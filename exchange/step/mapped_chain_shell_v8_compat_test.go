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
	// Native and canonical STEP bytes from the earlier inferred v3/v4 and the v7 through v12 proofs,
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
		{inferredMixedMappedChainPath, "820ca5d596527763c59c7c81a1ea943fee277e10b439f82faa47dbf4abebb577",
			"45d9b12f26bb6cd2ed90e6641048b35d05a192297af31b184d3ce91c3dc91536", inferredMixedMappedChainOptions()},
		{nonplanarInferredMappedChainPath, "0c398cd75d8b83d56a694ecf8745eb08b4c9f0a485db1161bd1b2fed603fc23b",
			"0456659adcf714e3b1a054f1e250d6ed59e7beb202707244868b8313bff25249", inferredMixedMappedChainOptions()},
		{nonplanarRationalInferredMappedChainPath, "e898be9ddebe2f5579e22d49bed23fe1420be2ea4264e621c4cc2be109624c12",
			"a7cbab04880ec766865c31cf08274bf116b7034dfd9fa63012b4eb695159fc7f", inferredMixedMappedChainOptions()},
		{nonplanarIndependentOwnerClocksPath, "1d53420939aaddacf3e4e23905826aaf1d72f869714941f8e3b9efafba841749",
			"53f68e6d037fbb0e7dc7ca96cfcf46b3f2d36b99ccb96c09ade8bbfea211f4c5", inferredMixedMappedChainOptions()},
		{nonplanarRationalRoofsPath, "4014e76e802e67e3958b8f3a259df278772df87ba268857950e79c64e8ee8ac7",
			"2ad2219943df5378b5e3fe153a07ac45c108a63bfbecb9ee95811c96d5013948", inferredMixedMappedChainOptions()},
		{joinedRationalCellsPath, "31911ab957b8f1d0ef4ac1ac4bec5d16ee6c3764576bc1698e9e920b5ebf2fe4",
			"1d3a6165ac4b61a45e2259d7aec1066f084f181a060420221f445913f6fe0656", joinedRationalCellOptions()},
		{joinedIndependentRoofsPath, "6c8c36dc397fe6b0f0d2ccd960b587ac94ccfbf8a57e8d6d213261ac1d305f25",
			"7130079d677d382266dda1a9be4b8ae16bcbe3d9dd2ae07a95ade7ebbe1e10e2", joinedRationalCellOptions()},
		{threeJoinedRoofPath, "41e3887cb210ac13073c170cac6648633cf9ab5306dc91decc0dec3b40b43d39",
			"634997f655acc45748473c9e4d5ff5836009ec076b8ec3285f01c34996bc6ac6", threeJoinedRoofOptions()},
		{asymmetricJoinedRoofPath, "813a0c47c3d0d8f45f132b2b92abba1661bcc0a68951cdeb9f90257c63bace9d",
			"b319d7c8059b526da02090507520f13b18909dfca3a214885918ac2ba4c2e25a", threeJoinedRoofOptions()},
		{kinkedJoinedRoofPath, "f8ed5fd55caba5adec291c6b2e2b2b744bcc7524754e031aa2592cf92f0786d9",
			"6af6fce562879b1a1fa47e0b8e08845c9e3e47e234f7f61aadcc781c18d11b05", threeJoinedRoofOptions()},
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
		tc.options.AllowJoinedSupportKnotCells = true
		tc.options.AllowThreeJoinedSupportKnotCells = true
		tc.options.AllowOrderedJoinedSupportKnotCells = true
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

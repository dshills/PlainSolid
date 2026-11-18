#!/usr/bin/env bash
set -euo pipefail

# Compile the test binaries and run a bounded set of representative
# operations. The qualification workflow runs the complete suites.
tests=(
  FuzzSurfaceParameters
  TestAP242PaletteOwnershipReject
  TestAnalyticSTEPDocumentPersistence
  TestAutomaticAffineChartCurvedSection
  TestBRepRoundTrip
  TestBoxMeshSTLAndOwnership
  TestBoxTopology
  TestCanonicalWrappedCylinderMaterial
  TestCombinedOccurrenceColorsReject
  TestCurvedSharedTrim
  TestExportRoundTrip
  TestExtrudeRoundedRectangleWithCircularFeatures
  TestHoledTrim
  TestIndependentFixture
  TestLineCylinderContacts
  TestMassProperties
  TestNativeRoundTrip
  TestOCCTDeepSHUOReject
  TestPlainCADSideNotchSTEPRoundTrip
  TestRationalPosedExactCircularLoopStrictRoundTrip
  TestRoundTrip
  TestRoundTrips
  TestSignedHoledPrismsAndPlacement
  TestStrictRequestsAndLimits
  TestTessellateCertifiedConvexFootprint
  TestTransactions
  TestWriteExactPeriodicSurface128
  TestWriteExactPeriodicSurface128Reject
)
filter=$(IFS='|'; printf '^(%s)$' "${tests[*]}")
# The filter names every case, so enable any that sit in the slow tier.
export PLAINSOLID_SLOW=1
log=$(mktemp)
trap 'rm -f -- "$log"' EXIT
go test ./... -count=1 -timeout=90s -v -run "$filter" | tee "$log"

for name in "${tests[@]}"; do
  if ! grep -q "^--- PASS: $name (" "$log"; then
    printf 'Missing passing smoke case: %s\n' "$name" >&2
    exit 1
  fi
done

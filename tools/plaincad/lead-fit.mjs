export const leadFitTest = "modelingGeometry.test.ts";
export const leadFitTestPrefix = "native lead screw and nut";
export const leadFitDeadlineMs = 45_000;
export const leadFitCases = [
  "cuts 1 start(s), right-hand trapezoidal threads and verifies coaxial fit",
  "cuts 2 start(s), left-hand acme threads and verifies coaxial fit",
  "cuts 4 start(s), right-hand trapezoidal threads and verifies coaxial fit",
].map(title => `${leadFitTestPrefix} ${title}`);

export function stageLeadFitProviderChecks(original, staged) {
  const opening = `describe("${leadFitTestPrefix}", () => {`;
  const closing = `\n    }, ${leadFitDeadlineMs},\n  );\n});`;
  const assertion = '          expect(shaft.geometryAssertions).toMatchObject({ solidCount: 1, valid: true });';
  const caseSource = source => {
    const parts = source.split(opening);
    if (parts.length !== 2) throw new Error("Cannot identify the original PlainCAD lead-fit tests.");
    const end = parts[1].indexOf(closing);
    if (end < 0) throw new Error("The original PlainCAD lead-fit case deadline changed.");
    return opening + parts[1].slice(0, end) + closing;
  };
  const originalCases = caseSource(original);
  const stagedCases = caseSource(staged);
  if (originalCases !== stagedCases || stagedCases.split(assertion).length !== 2)
    throw new Error("The staged PlainCAD lead-fit geometry tests differ from the original.");
  const providerChecks = [
    '          expect(shaft.geometrySource).toBe("plainsolid");',
    '          expect(sleeve.geometrySource).toBe("plainsolid");',
  ].join("\n");
  const adaptedCases = stagedCases.replace(assertion, `${providerChecks}\n${assertion}`);
  return { source: staged.replace(stagedCases, adaptedCases), originalCases, adaptedCases };
}

export function passedLeadFitCases(tests) {
  const selected = tests.filter(test => test.title.startsWith(`${leadFitTestPrefix} `));
  return selected.length === leadFitCases.length && leadFitCases.every(title =>
    selected.some(test => test.file === `src/tests/${leadFitTest}` && test.title === title &&
      test.status === "passed" && Number.isFinite(test.durationMs) &&
      test.durationMs >= 0 && test.durationMs < leadFitDeadlineMs));
}

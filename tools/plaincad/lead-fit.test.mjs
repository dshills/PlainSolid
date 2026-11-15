import assert from "node:assert/strict";
import test from "node:test";
import { leadFitCases, passedLeadFitCases, stageLeadFitProviderChecks } from "./lead-fit.mjs";

test("lead-fit setup preserves the original geometry checks and deadline", () => {
  const before = `const provider = "opencascade";
describe("native lead screw and nut", () => {
  it("a lead case", () => {
          expect(shaft.geometryAssertions).toMatchObject({ solidCount: 1, valid: true });
    }, 45000,
  );
});
`;
  const staged = before.replace('"opencascade"', '"plainsolid"');
  const result = stageLeadFitProviderChecks(before, staged);
  for (const part of ["shaft", "sleeve"])
    assert.ok(result.adaptedCases.includes(`expect(${part}.geometrySource).toBe("plainsolid")`));
  assert.equal(result.source.replace(/^.*geometrySource.*\n/gm, ""), staged);
  assert.throws(() => stageLeadFitProviderChecks(before, staged.replace("45000", "60000")), /deadline/);
  assert.throws(() => stageLeadFitProviderChecks(before, staged.replace("solidCount: 1", "solidCount: 2")), /differ/);
});

test("lead-fit report requires each original start count and a duration within its deadline", () => {
  const passing = leadFitCases.map(title => ({
    file: "src/tests/modelingGeometry.test.ts", title, status: "passed", durationMs: 1234,
  }));
  assert.ok(passedLeadFitCases(passing));
  assert.ok(!passedLeadFitCases(passing.slice(1)));
  assert.ok(!passedLeadFitCases([passing[0], passing[0], passing[2]]));
  assert.ok(!passedLeadFitCases([{ ...passing[0], status: "pending" }, ...passing.slice(1)]));
  assert.ok(!passedLeadFitCases([{ ...passing[0], durationMs: 45000 }, ...passing.slice(1)]));
});

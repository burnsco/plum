import { describe, expect, it } from "vitest";
import { getLibraryActivity } from "./libraryActivity";

describe("getLibraryActivity", () => {
  it("keeps showing analyzing while enrichment is running", () => {
    expect(
      getLibraryActivity({
        scanPhase: "completed",
        enrichmentPhase: "running",
        enriching: true,
      }),
    ).toBe("analyzing");
  });

  it("shows queued analyzer work separately from active analysis", () => {
    expect(
      getLibraryActivity({
        scanPhase: "completed",
        enrichmentPhase: "queued",
        enriching: false,
      }),
    ).toBe("analyze-queued");
  });

  it("hides analyzing when identify has already failed", () => {
    expect(
      getLibraryActivity({
        scanPhase: "completed",
        enrichmentPhase: "running",
        enriching: true,
        identifyPhase: "failed",
      }),
    ).toBeUndefined();
  });

  it("shows queued identify separately from active identify", () => {
    expect(
      getLibraryActivity({
        scanPhase: "completed",
        identifyPhase: "queued",
      }),
    ).toBe("identify-queued");

    expect(
      getLibraryActivity({
        scanPhase: "completed",
        identifyPhase: "identifying",
      }),
    ).toBe("identifying");
  });

  it("treats local queued identify as waiting work", () => {
    expect(
      getLibraryActivity({
        scanPhase: "completed",
        localIdentifyPhase: "queued",
      }),
    ).toBe("identify-queued");
  });
});

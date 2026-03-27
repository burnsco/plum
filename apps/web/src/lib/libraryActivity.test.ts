import { describe, expect, it } from "vitest";
import { getLibraryActivity } from "./libraryActivity";

describe("getLibraryActivity", () => {
  it("keeps showing finishing while enrichment is running", () => {
    expect(
      getLibraryActivity({
        scanPhase: "completed",
        enriching: true,
      }),
    ).toBe("finishing");
  });

  it("hides finishing when identify has already failed", () => {
    expect(
      getLibraryActivity({
        scanPhase: "completed",
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

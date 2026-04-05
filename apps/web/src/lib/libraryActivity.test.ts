import { describe, expect, it } from "vitest";
import type { LibraryScanStatus } from "@/api";
import { getLibraryActivity, isLibraryScanProcessing } from "./libraryActivity";

function baseStatus(overrides: Partial<LibraryScanStatus>): LibraryScanStatus {
  return {
    libraryId: 1,
    phase: "completed",
    enrichmentPhase: "idle",
    enriching: false,
    identifyPhase: "idle",
    identified: 0,
    identifyFailed: 0,
    processed: 0,
    added: 0,
    updated: 0,
    removed: 0,
    unmatched: 0,
    skipped: 0,
    identifyRequested: false,
    estimatedItems: 0,
    queuePosition: 0,
    ...overrides,
  };
}

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

describe("isLibraryScanProcessing", () => {
  it("is true for queued and active scan, analysis, and identify work", () => {
    expect(isLibraryScanProcessing(baseStatus({ phase: "queued" }))).toBe(true);
    expect(isLibraryScanProcessing(baseStatus({ phase: "scanning" }))).toBe(true);
    expect(isLibraryScanProcessing(baseStatus({ enrichmentPhase: "queued" }))).toBe(true);
    expect(isLibraryScanProcessing(baseStatus({ enrichmentPhase: "running" }))).toBe(true);
    expect(isLibraryScanProcessing(baseStatus({ identifyPhase: "queued" }))).toBe(true);
    expect(isLibraryScanProcessing(baseStatus({ identifyPhase: "identifying" }))).toBe(true);
  });

  it("is false when the library is idle", () => {
    expect(isLibraryScanProcessing(baseStatus({ phase: "completed" }))).toBe(false);
  });
});

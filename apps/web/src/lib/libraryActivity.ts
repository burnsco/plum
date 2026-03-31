export type LibraryActivity =
  | "importing"
  | "analyze-queued"
  | "analyzing"
  | "identify-queued"
  | "identifying";

export function getEnrichmentPhase(options: {
  enrichmentPhase?: string;
  enriching?: boolean;
}): "idle" | "queued" | "running" {
  if (options.enrichmentPhase === "queued" || options.enrichmentPhase === "running") {
    return options.enrichmentPhase;
  }
  if (options.enriching) {
    return "running";
  }
  return "idle";
}

export function getLibraryActivity(options: {
  scanPhase?: string;
  enrichmentPhase?: string;
  enriching?: boolean;
  identifyPhase?: string;
  localIdentifyPhase?: string;
}): LibraryActivity | undefined {
  const enrichmentPhase = getEnrichmentPhase(options);
  const backendIdentifying = options.identifyPhase === "identifying";
  const backendIdentifyQueued = options.identifyPhase === "queued";
  const localIdentifying =
    options.localIdentifyPhase === "identifying" ||
    options.localIdentifyPhase === "soft-reveal";
  const localIdentifyQueued = options.localIdentifyPhase === "queued";

  if (backendIdentifying || localIdentifying) {
    return "identifying";
  }
  if (backendIdentifyQueued || localIdentifyQueued) {
    return "identify-queued";
  }
  if (options.identifyPhase === "failed" || options.localIdentifyPhase === "identify-failed") {
    return undefined;
  }
  if (options.scanPhase === "queued" || options.scanPhase === "scanning") {
    return "importing";
  }
  if (options.scanPhase === "completed" && enrichmentPhase === "queued") {
    return "analyze-queued";
  }
  if (options.scanPhase === "completed" && enrichmentPhase === "running") {
    return "analyzing";
  }
  return undefined;
}

export function getLibraryActivityLabel(activity: LibraryActivity | undefined): string | undefined {
  switch (activity) {
    case "importing":
      return "Importing";
    case "analyze-queued":
      return "Waiting for analyzer";
    case "analyzing":
      return "Analyzing media";
    case "identify-queued":
      return "Queued for identify";
    case "identifying":
      return "Identifying";
    default:
      return undefined;
  }
}

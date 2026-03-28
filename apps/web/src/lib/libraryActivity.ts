export type LibraryActivity =
  | "importing"
  | "analyzing"
  | "identify-queued"
  | "identifying";

export function getLibraryActivity(options: {
  scanPhase?: string;
  enriching?: boolean;
  identifyPhase?: string;
  localIdentifyPhase?: string;
}): LibraryActivity | undefined {
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
  if (options.scanPhase === "completed" && options.enriching) {
    return "analyzing";
  }
  return undefined;
}

export function getLibraryActivityLabel(activity: LibraryActivity | undefined): string | undefined {
  switch (activity) {
    case "importing":
      return "Importing";
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

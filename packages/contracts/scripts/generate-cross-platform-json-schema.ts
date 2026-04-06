/**
 * Emits Draft 2020-12 JSON Schema derived from Effect schemas in src/index.ts.
 * Run from package root: `bun run generate:json-schema`
 *
 * Android and other clients can use this file for codegen or manual alignment;
 * TypeScript remains the authoring source in @plum/contracts.
 */
import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { Schema } from "effect";

import {
  CreatePlaybackSessionPayloadSchema,
  HomeDashboardSchema,
  PlaybackSessionSchema,
  PlaybackSessionUpdateEventSchema,
  PlumWebSocketCommandSchema,
  PlumWebSocketEventSchema,
  UpdateMediaProgressPayloadSchema,
  UpdatePlaybackSessionAudioPayloadSchema,
} from "../src/index";

const __dirname = dirname(fileURLToPath(import.meta.url));

/** One root object so shared $defs (e.g. MediaItem) are deduplicated. */
const CrossPlatformWireBundle = Schema.Struct({
  homeDashboard: HomeDashboardSchema,
  playbackSession: PlaybackSessionSchema,
  createPlaybackSessionPayload: CreatePlaybackSessionPayloadSchema,
  updatePlaybackSessionAudioPayload: UpdatePlaybackSessionAudioPayloadSchema,
  updateMediaProgressPayload: UpdateMediaProgressPayloadSchema,
  playbackSessionUpdateEvent: PlaybackSessionUpdateEventSchema,
  plumWebSocketEvent: PlumWebSocketEventSchema,
  plumWebSocketCommand: PlumWebSocketCommandSchema,
});

const outDir = join(__dirname, "../generated/json-schema");
const outPath = join(outDir, "cross-platform-wire.draft2020-12.json");

const doc = Schema.toJsonSchemaDocument(CrossPlatformWireBundle, {
  additionalProperties: false,
});

const payload: Record<string, unknown> = {
  $schema: "https://json-schema.org/draft/2020-12/schema",
  title: "Plum cross-platform wire shapes (subset)",
  description:
    "Generated from @plum/contracts Effect schemas. Top-level properties are documentation roots; the server may expose these as separate endpoints or WebSocket payloads.",
  ...doc.schema,
};
if (Object.keys(doc.definitions).length > 0) {
  payload.$defs = doc.definitions;
}

mkdirSync(outDir, { recursive: true });
writeFileSync(outPath, `${JSON.stringify(payload, null, 2)}\n`, "utf-8");
console.log(`Wrote ${outPath}`);

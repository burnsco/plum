import { Schema } from "effect";

/**
 * Library type determines identification/scan behavior and which category table is used.
 * TV and anime use TMDB for episodes; movie uses TMDB; music does not.
 */
export type LibraryType = "tv" | "movie" | "music" | "anime";

export const LibraryTypeSchema = Schema.Literals(["tv", "movie", "music", "anime"]);

/**
 * Media item type stored per item; matches library type for identification.
 */
export type MediaType = "tv" | "movie" | "music" | "anime";

export const MediaTypeSchema = Schema.Literals(["tv", "movie", "music", "anime"]);

export type MatchStatus = "identified" | "local" | "unmatched";

export const MatchStatusSchema = Schema.Literals(["identified", "local", "unmatched"]);

export type IdentifyState = "queued" | "identifying" | "failed";

export const IdentifyStateSchema = Schema.Literals(["queued", "identifying", "failed"]);

export interface Subtitle {
  id: number;
  title: string;
  language: string;
  format: string;
  logicalId?: string;
  forced?: boolean;
  default?: boolean;
  hearingImpaired?: boolean;
}

export const SubtitleSchema = Schema.Struct({
  id: Schema.Number,
  title: Schema.String,
  language: Schema.String,
  format: Schema.String,
  logicalId: Schema.optional(Schema.String),
  forced: Schema.optional(Schema.Boolean),
  default: Schema.optional(Schema.Boolean),
  hearingImpaired: Schema.optional(Schema.Boolean),
});

export interface EmbeddedSubtitle {
  streamIndex: number;
  language: string;
  title: string;
  codec?: string;
  logicalId?: string;
  supported?: boolean;
  forced?: boolean;
  default?: boolean;
  hearingImpaired?: boolean;
  /** Playback session only: false for bitmap subs that need server-side burn-in. */
  vttEligible?: boolean;
  /** Playback session: raw PGS stream for clients like Android TV / ExoPlayer. */
  pgsBinaryEligible?: boolean;
  /** Playback session: raw ASS stream available for clients that render ASS natively (e.g. JASSUB). */
  assEligible?: boolean;
  /** Playback session only: server-classified delivery modes for this logical subtitle. */
  deliveryModes?: ReadonlyArray<EmbeddedSubtitleDeliveryOption>;
  preferredWebDeliveryMode?: EmbeddedSubtitleDeliveryMode;
  preferredAndroidDeliveryMode?: EmbeddedSubtitleDeliveryMode;
}

export type EmbeddedSubtitleDeliveryMode =
  | "hls_vtt"
  | "direct_vtt"
  | "ass"
  | "pgs_binary"
  | "burn_in";

export const EmbeddedSubtitleDeliveryModeSchema = Schema.Literals([
  "hls_vtt",
  "direct_vtt",
  "ass",
  "pgs_binary",
  "burn_in",
]);

export interface EmbeddedSubtitleDeliveryOption {
  mode: EmbeddedSubtitleDeliveryMode;
  requiresReload: boolean;
}

export const EmbeddedSubtitleDeliveryOptionSchema = Schema.Struct({
  mode: EmbeddedSubtitleDeliveryModeSchema,
  requiresReload: Schema.Boolean,
});

export const EmbeddedSubtitleSchema = Schema.Struct({
  streamIndex: Schema.Number,
  language: Schema.String,
  title: Schema.String,
  codec: Schema.optional(Schema.String),
  logicalId: Schema.optional(Schema.String),
  supported: Schema.optional(Schema.Boolean),
  forced: Schema.optional(Schema.Boolean),
  default: Schema.optional(Schema.Boolean),
  hearingImpaired: Schema.optional(Schema.Boolean),
  vttEligible: Schema.optional(Schema.Boolean),
  pgsBinaryEligible: Schema.optional(Schema.Boolean),
  assEligible: Schema.optional(Schema.Boolean),
  deliveryModes: Schema.optional(Schema.Array(EmbeddedSubtitleDeliveryOptionSchema)),
  preferredWebDeliveryMode: Schema.optional(EmbeddedSubtitleDeliveryModeSchema),
  preferredAndroidDeliveryMode: Schema.optional(EmbeddedSubtitleDeliveryModeSchema),
});

export interface EmbeddedAudioTrack {
  streamIndex: number;
  language: string;
  title: string;
}

export const EmbeddedAudioTrackSchema = Schema.Struct({
  streamIndex: Schema.Number,
  language: Schema.String,
  title: Schema.String,
});

export interface LibraryBrowseItem {
  id: number;
  library_id?: number;
  title: string;
  path: string;
  duration: number;
  type: MediaType;
  match_status?: MatchStatus;
  identify_state?: IdentifyState;
  tmdb_id?: number;
  tvdb_id?: string;
  overview?: string;
  poster_path?: string;
  backdrop_path?: string;
  poster_url?: string;
  backdrop_url?: string;
  show_poster_path?: string;
  show_poster_url?: string;
  release_date?: string;
  show_vote_average?: number;
  /** Series IMDb user rating from `shows` (TV/anime browse rows only). */
  show_imdb_rating?: number;
  vote_average?: number;
  imdb_id?: string;
  imdb_rating?: number;
  artist?: string;
  album?: string;
  album_artist?: string;
  disc_number?: number;
  track_number?: number;
  release_year?: number;
  progress_seconds?: number;
  progress_percent?: number;
  remaining_seconds?: number;
  completed?: boolean;
  last_watched_at?: string;
  /** Set for TV/anime episodes; 0 when not applicable. */
  season?: number;
  episode?: number;
  metadata_review_needed?: boolean;
  metadata_confirmed?: boolean;
  /** Path to generated frame thumbnail (video episodes); served at /api/media/:id/thumbnail. */
  thumbnail_path?: string;
  /** Stable Plum-served thumbnail URL when available. */
  thumbnail_url?: string;
  missing?: boolean;
  missing_since?: string;
  intro_start_seconds?: number;
  intro_end_seconds?: number;
  intro_locked?: boolean;
  credits_start_seconds?: number;
  credits_end_seconds?: number;
}

export const LibraryBrowseItemSchema = Schema.Struct({
  id: Schema.Number,
  library_id: Schema.optional(Schema.Number),
  title: Schema.String,
  path: Schema.String,
  duration: Schema.Number,
  type: MediaTypeSchema,
  match_status: Schema.optional(MatchStatusSchema),
  identify_state: Schema.optional(IdentifyStateSchema),
  tmdb_id: Schema.optional(Schema.Number),
  tvdb_id: Schema.optional(Schema.String),
  overview: Schema.optional(Schema.String),
  poster_path: Schema.optional(Schema.String),
  backdrop_path: Schema.optional(Schema.String),
  poster_url: Schema.optional(Schema.String),
  backdrop_url: Schema.optional(Schema.String),
  show_poster_path: Schema.optional(Schema.String),
  show_poster_url: Schema.optional(Schema.String),
  release_date: Schema.optional(Schema.String),
  show_vote_average: Schema.optional(Schema.Number),
  show_imdb_rating: Schema.optional(Schema.Number),
  vote_average: Schema.optional(Schema.Number),
  imdb_id: Schema.optional(Schema.String),
  imdb_rating: Schema.optional(Schema.Number),
  artist: Schema.optional(Schema.String),
  album: Schema.optional(Schema.String),
  album_artist: Schema.optional(Schema.String),
  disc_number: Schema.optional(Schema.Number),
  track_number: Schema.optional(Schema.Number),
  release_year: Schema.optional(Schema.Number),
  progress_seconds: Schema.optional(Schema.Number),
  progress_percent: Schema.optional(Schema.Number),
  remaining_seconds: Schema.optional(Schema.Number),
  completed: Schema.optional(Schema.Boolean),
  last_watched_at: Schema.optional(Schema.String),
  season: Schema.optional(Schema.Number),
  episode: Schema.optional(Schema.Number),
  metadata_review_needed: Schema.optional(Schema.Boolean),
  metadata_confirmed: Schema.optional(Schema.Boolean),
  thumbnail_path: Schema.optional(Schema.String),
  thumbnail_url: Schema.optional(Schema.String),
  missing: Schema.optional(Schema.Boolean),
  missing_since: Schema.optional(Schema.String),
  intro_start_seconds: Schema.optional(Schema.Number),
  intro_end_seconds: Schema.optional(Schema.Number),
  intro_locked: Schema.optional(Schema.Boolean),
  credits_start_seconds: Schema.optional(Schema.Number),
  credits_end_seconds: Schema.optional(Schema.Number),
});

/** Full library media row (playback, detail) — browse fields plus required metadata and track lists. */
export type MediaItem = Omit<
  LibraryBrowseItem,
  | "library_id"
  | "tmdb_id"
  | "overview"
  | "poster_path"
  | "backdrop_path"
  | "release_date"
  | "vote_average"
> & {
  library_id: number;
  tmdb_id: number;
  overview: string;
  poster_path: string;
  backdrop_path: string;
  release_date: string;
  vote_average: number;
  subtitles?: Subtitle[];
  embeddedSubtitles?: EmbeddedSubtitle[];
  embeddedAudioTracks?: EmbeddedAudioTrack[];
  duplicate?: boolean;
  duplicate_count?: number;
};

export const MediaItemSchema = LibraryBrowseItemSchema.pipe(
  Schema.fieldsAssign({
    library_id: Schema.Number,
    tmdb_id: Schema.Number,
    overview: Schema.String,
    poster_path: Schema.String,
    backdrop_path: Schema.String,
    release_date: Schema.String,
    vote_average: Schema.Number,
    subtitles: Schema.optional(Schema.Array(SubtitleSchema)),
    embeddedSubtitles: Schema.optional(Schema.Array(EmbeddedSubtitleSchema)),
    embeddedAudioTracks: Schema.optional(Schema.Array(EmbeddedAudioTrackSchema)),
    duplicate: Schema.optional(Schema.Boolean),
    duplicate_count: Schema.optional(Schema.Number),
  }),
);

export interface LibraryMediaPage {
  items: LibraryBrowseItem[];
  next_offset?: number;
  has_more: boolean;
  total?: number;
}

export const LibraryMediaPageSchema = Schema.Struct({
  items: Schema.Array(LibraryBrowseItemSchema),
  next_offset: Schema.optional(Schema.Number),
  has_more: Schema.Boolean,
  total: Schema.optional(Schema.Number),
});

export interface ShowSeasonEpisodes {
  seasonNumber: number;
  label: string;
  episodes: LibraryBrowseItem[];
}

export const ShowSeasonEpisodesSchema = Schema.Struct({
  seasonNumber: Schema.Number,
  label: Schema.String,
  episodes: Schema.Array(LibraryBrowseItemSchema),
});

export interface ShowEpisodesResponse {
  seasons: ShowSeasonEpisodes[];
}

export const ShowEpisodesResponseSchema = Schema.Struct({
  seasons: Schema.Array(ShowSeasonEpisodesSchema),
});

export interface UpdateMediaProgressPayload {
  position_seconds: number;
  duration_seconds: number;
  completed?: boolean;
}

export const UpdateMediaProgressPayloadSchema = Schema.Struct({
  position_seconds: Schema.Number,
  duration_seconds: Schema.Number,
  completed: Schema.optional(Schema.Boolean),
});

/** Body for `PUT /api/libraries/{id}/shows/{showKey}/watched`. */
export const MarkShowWatchedPayloadSchema = Schema.Union([
  Schema.Struct({
    mode: Schema.Literal("all"),
  }),
  Schema.Struct({
    mode: Schema.Literal("season"),
    season: Schema.Number,
  }),
  Schema.Struct({
    mode: Schema.Literal("up_to"),
    season: Schema.Number,
    episode: Schema.Number,
  }),
]);

export type MarkShowWatchedPayload = Schema.Schema.Type<typeof MarkShowWatchedPayloadSchema>;

export interface SetContinueWatchingVisibilityPayload {
  hidden: boolean;
}

export const SetContinueWatchingVisibilityPayloadSchema = Schema.Struct({
  hidden: Schema.Boolean,
});

export type PlaybackSessionStatus = "starting" | "ready" | "error" | "closed";

export const PlaybackSessionStatusSchema = Schema.Literals([
  "starting",
  "ready",
  "error",
  "closed",
]);

export type PlaybackDelivery = "direct" | "remux" | "transcode";

export const PlaybackDeliverySchema = Schema.Literals(["direct", "remux", "transcode"]);

export interface ClientPlaybackCapabilities {
  supportsNativeHls: boolean;
  supportsMseHls: boolean;
  videoCodecs: string[];
  audioCodecs: string[];
  containers: string[];
}

export const ClientPlaybackCapabilitiesSchema = Schema.Struct({
  supportsNativeHls: Schema.Boolean,
  supportsMseHls: Schema.Boolean,
  videoCodecs: Schema.Array(Schema.String),
  audioCodecs: Schema.Array(Schema.String),
  containers: Schema.Array(Schema.String),
});

export interface CreatePlaybackSessionPayload {
  audioIndex?: number;
  clientCapabilities?: ClientPlaybackCapabilities;
  /** Image-based embedded subtitle stream index (e.g. PGS); forces transcode with burn-in. */
  burnEmbeddedSubtitleStreamIndex?: number;
}

export const CreatePlaybackSessionPayloadSchema = Schema.Struct({
  audioIndex: Schema.optional(Schema.Number),
  clientCapabilities: Schema.optional(ClientPlaybackCapabilitiesSchema),
  burnEmbeddedSubtitleStreamIndex: Schema.optional(Schema.Number),
});

export interface UpdatePlaybackSessionAudioPayload {
  audioIndex: number;
}

export const UpdatePlaybackSessionAudioPayloadSchema = Schema.Struct({
  audioIndex: Schema.Number,
});

export interface UpdatePlaybackSessionSeekPayload {
  positionSeconds: number;
}

export const UpdatePlaybackSessionSeekPayloadSchema = Schema.Struct({
  positionSeconds: Schema.Number,
});

export interface DirectPlaybackSession {
  delivery: "direct";
  mediaId: number;
  audioIndex?: number;
  status: PlaybackSessionStatus;
  streamUrl: string;
  durationSeconds: number;
  streamOffsetSeconds?: number;
  subtitles?: Subtitle[];
  embeddedSubtitles?: EmbeddedSubtitle[];
  embeddedAudioTracks?: EmbeddedAudioTrack[];
  burnEmbeddedSubtitleStreamIndex?: number;
  error?: string;
  intro_start_seconds?: number;
  intro_end_seconds?: number;
  credits_start_seconds?: number;
  credits_end_seconds?: number;
}

export interface HlsPlaybackSession {
  sessionId: string;
  delivery: "remux" | "transcode";
  mediaId: number;
  revision: number;
  audioIndex: number;
  status: PlaybackSessionStatus;
  streamUrl: string;
  durationSeconds: number;
  streamOffsetSeconds?: number;
  subtitles?: Subtitle[];
  embeddedSubtitles?: EmbeddedSubtitle[];
  embeddedAudioTracks?: EmbeddedAudioTrack[];
  burnEmbeddedSubtitleStreamIndex?: number;
  error?: string;
  intro_start_seconds?: number;
  intro_end_seconds?: number;
  credits_start_seconds?: number;
  credits_end_seconds?: number;
}

export type PlaybackSession = DirectPlaybackSession | HlsPlaybackSession;

export const DirectPlaybackSessionSchema = Schema.Struct({
  delivery: Schema.Literal("direct"),
  mediaId: Schema.Number,
  audioIndex: Schema.optional(Schema.Number),
  status: PlaybackSessionStatusSchema,
  streamUrl: Schema.String,
  durationSeconds: Schema.Number,
  streamOffsetSeconds: Schema.optional(Schema.Number),
  subtitles: Schema.optional(Schema.Array(SubtitleSchema)),
  embeddedSubtitles: Schema.optional(Schema.Array(EmbeddedSubtitleSchema)),
  embeddedAudioTracks: Schema.optional(Schema.Array(EmbeddedAudioTrackSchema)),
  burnEmbeddedSubtitleStreamIndex: Schema.optional(Schema.Number),
  error: Schema.optional(Schema.String),
  intro_start_seconds: Schema.optional(Schema.Number),
  intro_end_seconds: Schema.optional(Schema.Number),
  credits_start_seconds: Schema.optional(Schema.Number),
  credits_end_seconds: Schema.optional(Schema.Number),
});

export const HlsPlaybackSessionSchema = Schema.Struct({
  sessionId: Schema.String,
  delivery: Schema.Literals(["remux", "transcode"]),
  mediaId: Schema.Number,
  revision: Schema.Number,
  audioIndex: Schema.Number,
  status: PlaybackSessionStatusSchema,
  streamUrl: Schema.String,
  durationSeconds: Schema.Number,
  streamOffsetSeconds: Schema.optional(Schema.Number),
  subtitles: Schema.optional(Schema.Array(SubtitleSchema)),
  embeddedSubtitles: Schema.optional(Schema.Array(EmbeddedSubtitleSchema)),
  embeddedAudioTracks: Schema.optional(Schema.Array(EmbeddedAudioTrackSchema)),
  burnEmbeddedSubtitleStreamIndex: Schema.optional(Schema.Number),
  error: Schema.optional(Schema.String),
  intro_start_seconds: Schema.optional(Schema.Number),
  intro_end_seconds: Schema.optional(Schema.Number),
  credits_start_seconds: Schema.optional(Schema.Number),
  credits_end_seconds: Schema.optional(Schema.Number),
});

export const PlaybackSessionSchema = Schema.Union([
  DirectPlaybackSessionSchema,
  HlsPlaybackSessionSchema,
]);

export interface PlaybackTrackMetadata {
  subtitles?: Subtitle[];
  embeddedSubtitles?: EmbeddedSubtitle[];
  embeddedAudioTracks?: EmbeddedAudioTrack[];
}

export const PlaybackTrackMetadataSchema = Schema.Struct({
  subtitles: Schema.optional(Schema.Array(SubtitleSchema)),
  embeddedSubtitles: Schema.optional(Schema.Array(EmbeddedSubtitleSchema)),
  embeddedAudioTracks: Schema.optional(Schema.Array(EmbeddedAudioTrackSchema)),
});

export interface ContinueWatchingEntry {
  kind: "movie" | "show";
  media: MediaItem;
  show_key?: string;
  show_title?: string;
  episode_label?: string;
  remaining_seconds: number;
}

export const ContinueWatchingEntrySchema = Schema.Struct({
  kind: Schema.Literals(["movie", "show"]),
  media: MediaItemSchema,
  show_key: Schema.optional(Schema.String),
  show_title: Schema.optional(Schema.String),
  episode_label: Schema.optional(Schema.String),
  remaining_seconds: Schema.Number,
});

export interface RecentlyAddedEntry {
  kind: "movie" | "show" | "episode";
  media: MediaItem;
  show_key?: string;
  show_title?: string;
  episode_label?: string;
}

export const RecentlyAddedEntrySchema = Schema.Struct({
  kind: Schema.Literals(["movie", "show", "episode"]),
  media: MediaItemSchema,
  show_key: Schema.optional(Schema.String),
  show_title: Schema.optional(Schema.String),
  episode_label: Schema.optional(Schema.String),
});

export interface HomeDashboard {
  continueWatching: ContinueWatchingEntry[];
  /** Merged on the web client for notifications; optional from API. */
  recentlyAdded?: RecentlyAddedEntry[];
  recentlyAddedTvEpisodes?: RecentlyAddedEntry[];
  recentlyAddedTvShows?: RecentlyAddedEntry[];
  recentlyAddedMovies?: RecentlyAddedEntry[];
  recentlyAddedAnimeEpisodes?: RecentlyAddedEntry[];
  recentlyAddedAnimeShows?: RecentlyAddedEntry[];
}

export const HomeDashboardSchema = Schema.Struct({
  continueWatching: Schema.Array(ContinueWatchingEntrySchema),
  recentlyAdded: Schema.optional(Schema.Array(RecentlyAddedEntrySchema)),
  recentlyAddedTvEpisodes: Schema.optional(Schema.Array(RecentlyAddedEntrySchema)),
  recentlyAddedTvShows: Schema.optional(Schema.Array(RecentlyAddedEntrySchema)),
  recentlyAddedMovies: Schema.optional(Schema.Array(RecentlyAddedEntrySchema)),
  recentlyAddedAnimeEpisodes: Schema.optional(Schema.Array(RecentlyAddedEntrySchema)),
  recentlyAddedAnimeShows: Schema.optional(Schema.Array(RecentlyAddedEntrySchema)),
});

export interface Library {
  id: number;
  name: string;
  type: LibraryType;
  path: string;
  user_id: number;
  preferred_audio_language?: string;
  preferred_subtitle_language?: string;
  subtitles_enabled_by_default?: boolean;
  watcher_enabled?: boolean;
  watcher_mode?: "auto" | "poll";
  scan_interval_minutes?: number;
}

export const LibrarySchema = Schema.Struct({
  id: Schema.Number,
  name: Schema.String,
  type: LibraryTypeSchema,
  path: Schema.String,
  user_id: Schema.Number,
  preferred_audio_language: Schema.optional(Schema.String),
  preferred_subtitle_language: Schema.optional(Schema.String),
  subtitles_enabled_by_default: Schema.optional(Schema.Boolean),
  watcher_enabled: Schema.optional(Schema.Boolean),
  watcher_mode: Schema.optional(Schema.Literals(["auto", "poll"])),
  scan_interval_minutes: Schema.optional(Schema.Number),
});

export interface UnidentifiedLibrarySummary {
  library_id: number;
  name: string;
  type: LibraryType;
  count: number;
}

export const UnidentifiedLibrarySummarySchema = Schema.Struct({
  library_id: Schema.Number,
  name: Schema.String,
  type: LibraryTypeSchema,
  count: Schema.Number,
});

export interface UnidentifiedLibrariesResponse {
  libraries: UnidentifiedLibrarySummary[];
}

export const UnidentifiedLibrariesResponseSchema = Schema.Struct({
  libraries: Schema.Array(UnidentifiedLibrarySummarySchema),
});

export interface UpdateLibraryPlaybackPreferencesPayload {
  preferred_audio_language: string;
  preferred_subtitle_language: string;
  subtitles_enabled_by_default: boolean;
  watcher_enabled?: boolean;
  watcher_mode?: "auto" | "poll";
  scan_interval_minutes?: number;
}

export const UpdateLibraryPlaybackPreferencesPayloadSchema = Schema.Struct({
  preferred_audio_language: Schema.String,
  preferred_subtitle_language: Schema.String,
  subtitles_enabled_by_default: Schema.Boolean,
  watcher_enabled: Schema.optional(Schema.Boolean),
  watcher_mode: Schema.optional(Schema.Literals(["auto", "poll"])),
  scan_interval_minutes: Schema.optional(Schema.Number),
});

export interface CreateLibraryPayload {
  name: string;
  type: LibraryType;
  path: string;
  watcher_enabled?: boolean;
  watcher_mode?: "auto" | "poll";
  scan_interval_minutes?: number;
}

export const CreateLibraryPayloadSchema = Schema.Struct({
  name: Schema.String,
  type: LibraryTypeSchema,
  path: Schema.String,
  watcher_enabled: Schema.optional(Schema.Boolean),
  watcher_mode: Schema.optional(Schema.Literals(["auto", "poll"])),
  scan_interval_minutes: Schema.optional(Schema.Number),
});

export interface CredentialsPayload {
  email: string;
  password: string;
}

export const CredentialsPayloadSchema = Schema.Struct({
  email: Schema.String,
  password: Schema.String,
});

export interface User {
  id: number;
  email: string;
  is_admin: boolean;
}

export const UserSchema = Schema.Struct({
  id: Schema.Number,
  email: Schema.String,
  is_admin: Schema.Boolean,
});

/** Response from POST /api/auth/quick-connect (admin). */
export interface QuickConnectCodeResponse {
  code: string;
  expiresAt: string;
}

export const QuickConnectCodeResponseSchema = Schema.Struct({
  code: Schema.String,
  expiresAt: Schema.String,
});

export interface SetupStatus {
  hasAdmin: boolean;
  libraryDefaults: {
    tv: string;
    movie: string;
    anime: string;
    music: string;
  };
}

export const SetupStatusSchema = Schema.Struct({
  hasAdmin: Schema.Boolean,
  libraryDefaults: Schema.Struct({
    tv: Schema.String,
    movie: Schema.String,
    anime: Schema.String,
    music: Schema.String,
  }),
});

export interface ScanLibraryResult {
  added: number;
  updated: number;
  removed: number;
  unmatched: number;
  skipped: number;
}

export const ScanLibraryResultSchema = Schema.Struct({
  added: Schema.Number,
  updated: Schema.Number,
  removed: Schema.Number,
  unmatched: Schema.Number,
  skipped: Schema.Number,
});

export type LibraryScanPhase = "idle" | "queued" | "scanning" | "completed" | "failed";

export const LibraryScanPhaseSchema = Schema.Literals([
  "idle",
  "queued",
  "scanning",
  "completed",
  "failed",
]);

export type LibraryEnrichmentPhase = "idle" | "queued" | "running";

export const LibraryEnrichmentPhaseSchema = Schema.Literals(["idle", "queued", "running"]);

export type LibraryIdentifyPhase = "idle" | "queued" | "identifying" | "completed" | "failed";

export const LibraryIdentifyPhaseSchema = Schema.Literals([
  "idle",
  "queued",
  "identifying",
  "completed",
  "failed",
]);

export type LibraryScanActivityStage =
  | "queued"
  | "discovery"
  | "enrichment"
  | "identify"
  | "failed";

export const LibraryScanActivityStageSchema = Schema.Literals([
  "queued",
  "discovery",
  "enrichment",
  "identify",
  "failed",
]);

export type LibraryScanActivityPhase = "discovery" | "enrichment" | "identify";

export const LibraryScanActivityPhaseSchema = Schema.Literals([
  "discovery",
  "enrichment",
  "identify",
]);

export type LibraryScanActivityTarget = "directory" | "file";

export const LibraryScanActivityTargetSchema = Schema.Literals(["directory", "file"]);

export interface LibraryScanActivityEntry {
  phase: LibraryScanActivityPhase;
  target: LibraryScanActivityTarget;
  relativePath: string;
  at: string;
}

export const LibraryScanActivityEntrySchema = Schema.Struct({
  phase: LibraryScanActivityPhaseSchema,
  target: LibraryScanActivityTargetSchema,
  relativePath: Schema.String,
  at: Schema.String,
});

export interface LibraryScanActivity {
  stage: LibraryScanActivityStage;
  current?: LibraryScanActivityEntry;
  recent: readonly LibraryScanActivityEntry[];
}

export const LibraryScanActivitySchema = Schema.Struct({
  stage: LibraryScanActivityStageSchema,
  current: Schema.optional(LibraryScanActivityEntrySchema),
  recent: Schema.Array(LibraryScanActivityEntrySchema),
});

export interface LibraryScanStatus {
  libraryId: number;
  phase: LibraryScanPhase;
  enrichmentPhase: LibraryEnrichmentPhase;
  enriching: boolean;
  identifyPhase: LibraryIdentifyPhase;
  identified: number;
  identifyFailed: number;
  processed: number;
  added: number;
  updated: number;
  removed: number;
  unmatched: number;
  skipped: number;
  identifyRequested: boolean;
  queuedAt?: string;
  estimatedItems: number;
  queuePosition: number;
  error?: string;
  retryCount?: number;
  maxRetries?: number;
  nextRetryAt?: string;
  lastError?: string;
  nextScheduledAt?: string;
  startedAt?: string;
  finishedAt?: string;
  activity?: LibraryScanActivity;
}

export const LibraryScanStatusSchema = Schema.Struct({
  libraryId: Schema.Number,
  phase: LibraryScanPhaseSchema,
  enrichmentPhase: LibraryEnrichmentPhaseSchema,
  enriching: Schema.Boolean,
  identifyPhase: LibraryIdentifyPhaseSchema,
  identified: Schema.Number,
  identifyFailed: Schema.Number,
  processed: Schema.Number,
  added: Schema.Number,
  updated: Schema.Number,
  removed: Schema.Number,
  unmatched: Schema.Number,
  skipped: Schema.Number,
  identifyRequested: Schema.Boolean,
  queuedAt: Schema.optional(Schema.String),
  estimatedItems: Schema.Number,
  queuePosition: Schema.Number,
  error: Schema.optional(Schema.String),
  retryCount: Schema.optional(Schema.Number),
  maxRetries: Schema.optional(Schema.Number),
  nextRetryAt: Schema.optional(Schema.String),
  lastError: Schema.optional(Schema.String),
  nextScheduledAt: Schema.optional(Schema.String),
  startedAt: Schema.optional(Schema.String),
  finishedAt: Schema.optional(Schema.String),
  activity: Schema.optional(LibraryScanActivitySchema),
});

export interface IdentifyResult {
  identified: number;
  failed: number;
}

export const IdentifyResultSchema = Schema.Struct({
  identified: Schema.Number,
  failed: Schema.Number,
});

export interface CastMember {
  name: string;
  character?: string;
  order?: number;
  profile_path?: string;
}

export const CastMemberSchema = Schema.Struct({
  name: Schema.String,
  character: Schema.optional(Schema.String),
  order: Schema.optional(Schema.Number),
  profile_path: Schema.optional(Schema.String),
});

export interface SeriesDetails {
  name: string;
  overview: string;
  poster_path: string;
  backdrop_path: string;
  poster_url?: string;
  backdrop_url?: string;
  first_air_date: string;
  imdb_id?: string;
  imdb_rating?: number;
  genres: string[];
  cast: CastMember[];
  runtime?: number;
  number_of_seasons?: number;
  number_of_episodes?: number;
}

export const SeriesDetailsSchema = Schema.Struct({
  name: Schema.String,
  overview: Schema.String,
  poster_path: Schema.String,
  backdrop_path: Schema.String,
  poster_url: Schema.optional(Schema.String),
  backdrop_url: Schema.optional(Schema.String),
  first_air_date: Schema.String,
  imdb_id: Schema.optional(Schema.String),
  imdb_rating: Schema.optional(Schema.Number),
  genres: Schema.Array(Schema.String),
  cast: Schema.Array(CastMemberSchema),
  runtime: Schema.optional(Schema.Number),
  number_of_seasons: Schema.optional(Schema.Number),
  number_of_episodes: Schema.optional(Schema.Number),
});

export interface MovieDetails {
  media_id: number;
  library_id: number;
  title: string;
  source_path?: string;
  overview: string;
  poster_path?: string;
  poster_url?: string;
  backdrop_path?: string;
  backdrop_url?: string;
  release_date?: string;
  vote_average?: number;
  imdb_id?: string;
  imdb_rating?: number;
  runtime?: number;
  /** From `playback_progress`; drives resume in the player when starting from the detail page. */
  progress_seconds?: number;
  progress_percent?: number;
  completed?: boolean;
  subtitles?: Subtitle[];
  embeddedSubtitles?: EmbeddedSubtitle[];
  embeddedAudioTracks?: EmbeddedAudioTrack[];
  genres: string[];
  cast: CastMember[];
}

export const MovieDetailsSchema = Schema.Struct({
  media_id: Schema.Number,
  library_id: Schema.Number,
  title: Schema.String,
  source_path: Schema.optional(Schema.String),
  overview: Schema.String,
  poster_path: Schema.optional(Schema.String),
  poster_url: Schema.optional(Schema.String),
  backdrop_path: Schema.optional(Schema.String),
  backdrop_url: Schema.optional(Schema.String),
  release_date: Schema.optional(Schema.String),
  vote_average: Schema.optional(Schema.Number),
  imdb_id: Schema.optional(Schema.String),
  imdb_rating: Schema.optional(Schema.Number),
  runtime: Schema.optional(Schema.Number),
  progress_seconds: Schema.optional(Schema.Number),
  progress_percent: Schema.optional(Schema.Number),
  completed: Schema.optional(Schema.Boolean),
  subtitles: Schema.optional(Schema.Array(SubtitleSchema)),
  embeddedSubtitles: Schema.optional(Schema.Array(EmbeddedSubtitleSchema)),
  embeddedAudioTracks: Schema.optional(Schema.Array(EmbeddedAudioTrackSchema)),
  genres: Schema.Array(Schema.String),
  cast: Schema.Array(CastMemberSchema),
});

export interface ShowDetails {
  library_id: number;
  show_key: string;
  name: string;
  overview: string;
  poster_path?: string;
  poster_url?: string;
  backdrop_path?: string;
  backdrop_url?: string;
  first_air_date?: string;
  vote_average?: number;
  imdb_id?: string;
  imdb_rating?: number;
  runtime?: number;
  number_of_seasons: number;
  number_of_episodes: number;
  genres: string[];
  cast: CastMember[];
}

export const ShowDetailsSchema = Schema.Struct({
  library_id: Schema.Number,
  show_key: Schema.String,
  name: Schema.String,
  overview: Schema.String,
  poster_path: Schema.optional(Schema.String),
  poster_url: Schema.optional(Schema.String),
  backdrop_path: Schema.optional(Schema.String),
  backdrop_url: Schema.optional(Schema.String),
  first_air_date: Schema.optional(Schema.String),
  vote_average: Schema.optional(Schema.Number),
  imdb_id: Schema.optional(Schema.String),
  imdb_rating: Schema.optional(Schema.Number),
  runtime: Schema.optional(Schema.Number),
  number_of_seasons: Schema.Number,
  number_of_episodes: Schema.Number,
  genres: Schema.Array(Schema.String),
  cast: Schema.Array(CastMemberSchema),
});

export type SearchResultKind = "movie" | "show";

export const SearchResultKindSchema = Schema.Literals(["movie", "show"]);

export type SearchMatchReason = "title" | "actor";

export const SearchMatchReasonSchema = Schema.Literals(["title", "actor"]);

export interface SearchResult {
  kind: SearchResultKind;
  library_id: number;
  library_name: string;
  library_type: LibraryType;
  title: string;
  subtitle?: string;
  poster_path?: string;
  poster_url?: string;
  imdb_rating?: number;
  match_reason: SearchMatchReason;
  matched_actor?: string;
  href: string;
  genres?: string[];
}

export const SearchResultSchema = Schema.Struct({
  kind: SearchResultKindSchema,
  library_id: Schema.Number,
  library_name: Schema.String,
  library_type: LibraryTypeSchema,
  title: Schema.String,
  subtitle: Schema.optional(Schema.String),
  poster_path: Schema.optional(Schema.String),
  poster_url: Schema.optional(Schema.String),
  imdb_rating: Schema.optional(Schema.Number),
  match_reason: SearchMatchReasonSchema,
  matched_actor: Schema.optional(Schema.String),
  href: Schema.String,
  genres: Schema.optional(Schema.Array(Schema.String)),
});

export interface SearchFacetValue {
  value: string;
  label: string;
  count: number;
}

export const SearchFacetValueSchema = Schema.Struct({
  value: Schema.String,
  label: Schema.String,
  count: Schema.Number,
});

export interface SearchFacets {
  libraries: SearchFacetValue[];
  types: SearchFacetValue[];
  genres: SearchFacetValue[];
}

export const SearchFacetsSchema = Schema.Struct({
  libraries: Schema.Array(SearchFacetValueSchema),
  types: Schema.Array(SearchFacetValueSchema),
  genres: Schema.Array(SearchFacetValueSchema),
});

export interface SearchResponse {
  query: string;
  results: SearchResult[];
  total: number;
  facets: SearchFacets;
}

export const SearchResponseSchema = Schema.Struct({
  query: Schema.String,
  results: Schema.Array(SearchResultSchema),
  total: Schema.Number,
  facets: SearchFacetsSchema,
});

export interface SeriesSearchResult {
  Title: string;
  Overview: string;
  PosterURL: string;
  BackdropURL: string;
  ReleaseDate: string;
  VoteAverage: number;
  Provider: string;
  ExternalID: string;
}

export const SeriesSearchResultSchema = Schema.Struct({
  Title: Schema.String,
  Overview: Schema.String,
  PosterURL: Schema.String,
  BackdropURL: Schema.String,
  ReleaseDate: Schema.String,
  VoteAverage: Schema.Number,
  Provider: Schema.String,
  ExternalID: Schema.String,
});

export interface MovieSearchResult {
  Title: string;
  Overview: string;
  PosterURL: string;
  BackdropURL: string;
  ReleaseDate: string;
  VoteAverage: number;
  Provider: string;
  ExternalID: string;
}

export const MovieSearchResultSchema = Schema.Struct({
  Title: Schema.String,
  Overview: Schema.String,
  PosterURL: Schema.String,
  BackdropURL: Schema.String,
  ReleaseDate: Schema.String,
  VoteAverage: Schema.Number,
  Provider: Schema.String,
  ExternalID: Schema.String,
});

export type DiscoverMediaType = "movie" | "tv";

export const DiscoverMediaTypeSchema = Schema.Literals(["movie", "tv"]);

export type DiscoverBrowseCategory =
  | "trending"
  | "popular-movies"
  | "popular-tv"
  | "now-playing"
  | "upcoming"
  | "on-the-air"
  | "top-rated";

export const DiscoverBrowseCategorySchema = Schema.Literals([
  "trending",
  "popular-movies",
  "popular-tv",
  "now-playing",
  "upcoming",
  "on-the-air",
  "top-rated",
]);

/** Canonical browse shelf order for Discover; IDs match server TMDB browse routes. */
export const DISCOVER_BROWSE_CATEGORY_ORDER: readonly DiscoverBrowseCategory[] = [
  "trending",
  "popular-movies",
  "now-playing",
  "upcoming",
  "popular-tv",
  "on-the-air",
  "top-rated",
];

export type MediaStackServiceKind = "radarr" | "sonarr-tv";

export const MediaStackServiceKindSchema = Schema.Literals(["radarr", "sonarr-tv"]);

export interface DiscoverLibraryMatch {
  library_id: number;
  library_name: string;
  library_type: LibraryType;
  kind: "movie" | "show";
  show_key?: string;
}

export const DiscoverLibraryMatchSchema = Schema.Struct({
  library_id: Schema.Number,
  library_name: Schema.String,
  library_type: LibraryTypeSchema,
  kind: Schema.Literals(["movie", "show"]),
  show_key: Schema.optional(Schema.String),
});

export type DiscoverAcquisitionState = "not_added" | "added" | "downloading" | "available";

export const DiscoverAcquisitionStateSchema = Schema.Literals([
  "not_added",
  "added",
  "downloading",
  "available",
]);

export interface DiscoverAcquisition {
  state: DiscoverAcquisitionState;
  source?: MediaStackServiceKind;
  can_add?: boolean;
  is_configured?: boolean;
}

export const DiscoverAcquisitionSchema = Schema.Struct({
  state: DiscoverAcquisitionStateSchema,
  source: Schema.optional(MediaStackServiceKindSchema),
  can_add: Schema.optional(Schema.Boolean),
  is_configured: Schema.optional(Schema.Boolean),
});

export interface DiscoverItem {
  media_type: DiscoverMediaType;
  tmdb_id: number;
  title: string;
  overview?: string;
  poster_path?: string;
  backdrop_path?: string;
  release_date?: string;
  first_air_date?: string;
  vote_average?: number;
  library_matches?: DiscoverLibraryMatch[];
  acquisition?: DiscoverAcquisition;
}

export const DiscoverItemSchema = Schema.Struct({
  media_type: DiscoverMediaTypeSchema,
  tmdb_id: Schema.Number,
  title: Schema.String,
  overview: Schema.optional(Schema.String),
  poster_path: Schema.optional(Schema.String),
  backdrop_path: Schema.optional(Schema.String),
  release_date: Schema.optional(Schema.String),
  first_air_date: Schema.optional(Schema.String),
  vote_average: Schema.optional(Schema.Number),
  library_matches: Schema.optional(Schema.Array(DiscoverLibraryMatchSchema)),
  acquisition: Schema.optional(DiscoverAcquisitionSchema),
});

export interface DiscoverShelf {
  id: string;
  title: string;
  items: DiscoverItem[];
}

export const DiscoverShelfSchema = Schema.Struct({
  id: Schema.String,
  title: Schema.String,
  items: Schema.Array(DiscoverItemSchema),
});

export interface DiscoverResponse {
  shelves: DiscoverShelf[];
}

export const DiscoverResponseSchema = Schema.Struct({
  shelves: Schema.Array(DiscoverShelfSchema),
});

export interface DiscoverSearchResponse {
  movies: DiscoverItem[];
  tv: DiscoverItem[];
}

export const DiscoverSearchResponseSchema = Schema.Struct({
  movies: Schema.Array(DiscoverItemSchema),
  tv: Schema.Array(DiscoverItemSchema),
});

export interface DiscoverGenre {
  id: number;
  name: string;
}

export const DiscoverGenreSchema = Schema.Struct({
  id: Schema.Number,
  name: Schema.String,
});

export interface DiscoverGenresResponse {
  movie_genres: DiscoverGenre[];
  tv_genres: DiscoverGenre[];
}

export const DiscoverGenresResponseSchema = Schema.Struct({
  movie_genres: Schema.Array(DiscoverGenreSchema),
  tv_genres: Schema.Array(DiscoverGenreSchema),
});

export interface DiscoverBrowseResponse {
  items: DiscoverItem[];
  page: number;
  total_pages: number;
  total_results: number;
  media_type?: DiscoverMediaType;
  genre?: DiscoverGenre;
  category?: DiscoverBrowseCategory;
}

export const DiscoverBrowseResponseSchema = Schema.Struct({
  items: Schema.Array(DiscoverItemSchema),
  page: Schema.Number,
  total_pages: Schema.Number,
  total_results: Schema.Number,
  media_type: Schema.optional(DiscoverMediaTypeSchema),
  genre: Schema.optional(DiscoverGenreSchema),
  category: Schema.optional(DiscoverBrowseCategorySchema),
});

export interface DiscoverTitleVideo {
  name: string;
  site: string;
  key: string;
  type: string;
  official?: boolean;
}

export const DiscoverTitleVideoSchema = Schema.Struct({
  name: Schema.String,
  site: Schema.String,
  key: Schema.String,
  type: Schema.String,
  official: Schema.optional(Schema.Boolean),
});

export interface DiscoverTitleDetails {
  media_type: DiscoverMediaType;
  tmdb_id: number;
  title: string;
  overview: string;
  poster_path?: string;
  backdrop_path?: string;
  release_date?: string;
  first_air_date?: string;
  vote_average?: number;
  imdb_id?: string;
  imdb_rating?: number;
  status?: string;
  genres: string[];
  runtime?: number;
  number_of_seasons?: number;
  number_of_episodes?: number;
  videos: DiscoverTitleVideo[];
  library_matches?: DiscoverLibraryMatch[];
  acquisition?: DiscoverAcquisition;
}

export const DiscoverTitleDetailsSchema = Schema.Struct({
  media_type: DiscoverMediaTypeSchema,
  tmdb_id: Schema.Number,
  title: Schema.String,
  overview: Schema.String,
  poster_path: Schema.optional(Schema.String),
  backdrop_path: Schema.optional(Schema.String),
  release_date: Schema.optional(Schema.String),
  first_air_date: Schema.optional(Schema.String),
  vote_average: Schema.optional(Schema.Number),
  imdb_id: Schema.optional(Schema.String),
  imdb_rating: Schema.optional(Schema.Number),
  status: Schema.optional(Schema.String),
  genres: Schema.Array(Schema.String),
  runtime: Schema.optional(Schema.Number),
  number_of_seasons: Schema.optional(Schema.Number),
  number_of_episodes: Schema.optional(Schema.Number),
  videos: Schema.Array(DiscoverTitleVideoSchema),
  library_matches: Schema.optional(Schema.Array(DiscoverLibraryMatchSchema)),
  acquisition: Schema.optional(DiscoverAcquisitionSchema),
});

export interface MediaStackServiceSettings {
  baseUrl: string;
  apiKey: string;
  qualityProfileId: number;
  rootFolderPath: string;
  searchOnAdd: boolean;
}

export const MediaStackServiceSettingsSchema = Schema.Struct({
  baseUrl: Schema.String,
  apiKey: Schema.String,
  qualityProfileId: Schema.Number,
  rootFolderPath: Schema.String,
  searchOnAdd: Schema.Boolean,
});

export interface MediaStackSettings {
  radarr: MediaStackServiceSettings;
  sonarrTv: MediaStackServiceSettings;
}

export const MediaStackSettingsSchema = Schema.Struct({
  radarr: MediaStackServiceSettingsSchema,
  sonarrTv: MediaStackServiceSettingsSchema,
});

export interface MediaStackRootFolderOption {
  path: string;
}

export const MediaStackRootFolderOptionSchema = Schema.Struct({
  path: Schema.String,
});

export interface MediaStackQualityProfileOption {
  id: number;
  name: string;
}

export const MediaStackQualityProfileOptionSchema = Schema.Struct({
  id: Schema.Number,
  name: Schema.String,
});

export interface MediaStackServiceValidationResult {
  configured: boolean;
  reachable: boolean;
  errorMessage?: string;
  rootFolders: MediaStackRootFolderOption[];
  qualityProfiles: MediaStackQualityProfileOption[];
}

export const MediaStackServiceValidationResultSchema = Schema.Struct({
  configured: Schema.Boolean,
  reachable: Schema.Boolean,
  errorMessage: Schema.optional(Schema.String),
  rootFolders: Schema.Array(MediaStackRootFolderOptionSchema),
  qualityProfiles: Schema.Array(MediaStackQualityProfileOptionSchema),
});

export interface MediaStackValidationResult {
  radarr: MediaStackServiceValidationResult;
  sonarrTv: MediaStackServiceValidationResult;
}

export const MediaStackValidationResultSchema = Schema.Struct({
  radarr: MediaStackServiceValidationResultSchema,
  sonarrTv: MediaStackServiceValidationResultSchema,
});

export interface ServerEnvSecretsPresent {
  tmdb_api_key: boolean;
  tvdb_api_key: boolean;
  omdb_api_key: boolean;
  fanart_api_key: boolean;
}

export const ServerEnvSecretsPresentSchema = Schema.Struct({
  tmdb_api_key: Schema.Boolean,
  tvdb_api_key: Schema.Boolean,
  omdb_api_key: Schema.Boolean,
  fanart_api_key: Schema.Boolean,
});

export interface ServerEnvSettingsResponse {
  env_file_path: string;
  env_file_existed: boolean;
  env_file_writable: boolean;
  plum_addr: string;
  plum_database_url: string;
  musicbrainz_contact_url: string;
  secrets_present: ServerEnvSecretsPresent;
  restart_recommended: boolean;
  help: string;
}

export const ServerEnvSettingsResponseSchema = Schema.Struct({
  env_file_path: Schema.String,
  env_file_existed: Schema.Boolean,
  env_file_writable: Schema.Boolean,
  plum_addr: Schema.String,
  plum_database_url: Schema.String,
  musicbrainz_contact_url: Schema.String,
  secrets_present: ServerEnvSecretsPresentSchema,
  restart_recommended: Schema.Boolean,
  help: Schema.String,
});

/** Partial body for PUT /api/settings/server-env. Omit a field to leave it unchanged; use *_clear to remove API keys. */
export interface ServerEnvSettingsUpdate {
  plum_addr?: string;
  plum_database_url?: string;
  musicbrainz_contact_url?: string;
  tmdb_api_key?: string;
  tvdb_api_key?: string;
  omdb_api_key?: string;
  fanart_api_key?: string;
  tmdb_api_key_clear?: boolean;
  tvdb_api_key_clear?: boolean;
  omdb_api_key_clear?: boolean;
  fanart_api_key_clear?: boolean;
}

export const ServerEnvSettingsUpdateSchema = Schema.Struct({
  plum_addr: Schema.optional(Schema.String),
  plum_database_url: Schema.optional(Schema.String),
  musicbrainz_contact_url: Schema.optional(Schema.String),
  tmdb_api_key: Schema.optional(Schema.String),
  tvdb_api_key: Schema.optional(Schema.String),
  omdb_api_key: Schema.optional(Schema.String),
  fanart_api_key: Schema.optional(Schema.String),
  tmdb_api_key_clear: Schema.optional(Schema.Boolean),
  tvdb_api_key_clear: Schema.optional(Schema.Boolean),
  omdb_api_key_clear: Schema.optional(Schema.Boolean),
  fanart_api_key_clear: Schema.optional(Schema.Boolean),
});

export interface DownloadItem {
  id: string;
  title: string;
  media_type: DiscoverMediaType;
  source: MediaStackServiceKind;
  status_text: string;
  progress?: number;
  size_left_bytes?: number;
  eta_seconds?: number;
  error_message?: string;
}

export const DownloadItemSchema = Schema.Struct({
  id: Schema.String,
  title: Schema.String,
  media_type: DiscoverMediaTypeSchema,
  source: MediaStackServiceKindSchema,
  status_text: Schema.String,
  progress: Schema.optional(Schema.Number),
  size_left_bytes: Schema.optional(Schema.Number),
  eta_seconds: Schema.optional(Schema.Number),
  error_message: Schema.optional(Schema.String),
});

export interface DownloadsResponse {
  configured: boolean;
  items: DownloadItem[];
}

export const DownloadsResponseSchema = Schema.Struct({
  configured: Schema.Boolean,
  items: Schema.Array(DownloadItemSchema),
});

/** Body for POST /api/downloads/remove — removes the queue entry in Radarr or Sonarr (same id as list items). */
export interface RemoveDownloadPayload {
  id: string;
}

export const RemoveDownloadPayloadSchema = Schema.Struct({
  id: Schema.String,
});

export interface ShowActionResult {
  updated: number;
}

export const ShowActionResultSchema = Schema.Struct({
  updated: Schema.Number,
});

/** Response from POST /api/libraries/{id}/playback-tracks/refresh (202 Accepted). Work runs in the background. */
export interface LibraryPlaybackTracksRefreshResult {
  accepted: boolean;
  libraryId: number;
}

export const LibraryPlaybackTracksRefreshResultSchema = Schema.Struct({
  accepted: Schema.Boolean,
  libraryId: Schema.Number,
});

export interface ShowRefreshPayload {
  showKey: string;
}

export const ShowRefreshPayloadSchema = Schema.Struct({
  showKey: Schema.String,
});

export interface ShowConfirmPayload {
  showKey: string;
}

export const ShowConfirmPayloadSchema = Schema.Struct({
  showKey: Schema.String,
});

export interface ShowIdentifyPayload {
  showKey: string;
  provider: string;
  externalId: string;
  tmdbId?: number;
}

export const ShowIdentifyPayloadSchema = Schema.Struct({
  showKey: Schema.String,
  provider: Schema.String,
  externalId: Schema.String,
  tmdbId: Schema.optional(Schema.Number),
});

export interface MovieIdentifyPayload {
  mediaId: number;
  provider: string;
  externalId: string;
  tmdbId?: number;
}

export const MovieIdentifyPayloadSchema = Schema.Struct({
  mediaId: Schema.Number,
  provider: Schema.String,
  externalId: Schema.String,
  tmdbId: Schema.optional(Schema.Number),
});

export type VaapiDecodeCodec =
  | "h264"
  | "hevc"
  | "mpeg2"
  | "vc1"
  | "vp8"
  | "vp9"
  | "av1"
  | "hevc10bit"
  | "vp910bit";

export const VaapiDecodeCodecSchema = Schema.Literals([
  "h264",
  "hevc",
  "mpeg2",
  "vc1",
  "vp8",
  "vp9",
  "av1",
  "hevc10bit",
  "vp910bit",
]);

export type HardwareEncodeFormat = "h264" | "hevc" | "av1";

export const HardwareEncodeFormatSchema = Schema.Literals(["h264", "hevc", "av1"]);

/** FFmpeg `tonemap_opencl` algorithm names (experimental server setting). */
export type OpenCLToneMapAlgorithm = "hable" | "reinhard" | "mobius" | "linear" | "gamma" | "clip";

export const OpenCLToneMapAlgorithmSchema = Schema.Literals([
  "hable",
  "reinhard",
  "mobius",
  "linear",
  "gamma",
  "clip",
]);

export interface TranscodingSettings {
  vaapiEnabled: boolean;
  vaapiDevicePath: string;
  decodeCodecs: Record<VaapiDecodeCodec, boolean>;
  hardwareEncodingEnabled: boolean;
  encodeFormats: Record<HardwareEncodeFormat, boolean>;
  preferredHardwareEncodeFormat: HardwareEncodeFormat;
  allowSoftwareFallback: boolean;
  crf: number;
  audioBitrate: string;
  audioChannels: number;
  threads: number;
  keyframeInterval: number;
  maxBitrate: string;
  /** Experimental: HDR→SDR via FFmpeg `tonemap_opencl` when the source looks HDR. Default off. */
  openclToneMappingEnabled: boolean;
  openclToneMapAlgorithm: OpenCLToneMapAlgorithm;
  /** Highlight desaturation (FFmpeg `desat`, typically 0–1; capped server-side). */
  openclToneMapDesat: number;
}

export const VaapiDecodeCodecFlagsSchema = Schema.Struct({
  h264: Schema.Boolean,
  hevc: Schema.Boolean,
  mpeg2: Schema.Boolean,
  vc1: Schema.Boolean,
  vp8: Schema.Boolean,
  vp9: Schema.Boolean,
  av1: Schema.Boolean,
  hevc10bit: Schema.Boolean,
  vp910bit: Schema.Boolean,
});

export const HardwareEncodeFormatFlagsSchema = Schema.Struct({
  h264: Schema.Boolean,
  hevc: Schema.Boolean,
  av1: Schema.Boolean,
});

export const TranscodingSettingsSchema = Schema.Struct({
  vaapiEnabled: Schema.Boolean,
  vaapiDevicePath: Schema.String,
  decodeCodecs: VaapiDecodeCodecFlagsSchema,
  hardwareEncodingEnabled: Schema.Boolean,
  encodeFormats: HardwareEncodeFormatFlagsSchema,
  preferredHardwareEncodeFormat: HardwareEncodeFormatSchema,
  allowSoftwareFallback: Schema.Boolean,
  crf: Schema.Number,
  audioBitrate: Schema.String,
  audioChannels: Schema.Number,
  threads: Schema.Number,
  keyframeInterval: Schema.Number,
  maxBitrate: Schema.String,
  openclToneMappingEnabled: Schema.Boolean,
  openclToneMapAlgorithm: OpenCLToneMapAlgorithmSchema,
  openclToneMapDesat: Schema.Number,
});

export interface TranscodingSettingsWarning {
  code: string;
  message: string;
}

export const TranscodingSettingsWarningSchema = Schema.Struct({
  code: Schema.String,
  message: Schema.String,
});

export interface TranscodingSettingsResponse {
  settings: TranscodingSettings;
  warnings: TranscodingSettingsWarning[];
}

export const TranscodingSettingsResponseSchema = Schema.Struct({
  settings: TranscodingSettingsSchema,
  warnings: Schema.Array(TranscodingSettingsWarningSchema),
});

export type MetadataArtworkProvider = "fanart" | "tmdb" | "tvdb" | "omdb";

export const MetadataArtworkProviderSchema = Schema.Literals(["fanart", "tmdb", "tvdb", "omdb"]);

export interface MetadataArtworkProviderStatus {
  provider: MetadataArtworkProvider;
  enabled: boolean;
  available: boolean;
  reason?: string;
}

export const MetadataArtworkProviderStatusSchema = Schema.Struct({
  provider: MetadataArtworkProviderSchema,
  enabled: Schema.Boolean,
  available: Schema.Boolean,
  reason: Schema.optional(Schema.String),
});

export interface ShowMetadataArtworkFetchers {
  fanart: boolean;
  tmdb: boolean;
  tvdb: boolean;
}

export const ShowMetadataArtworkFetchersSchema = Schema.Struct({
  fanart: Schema.Boolean,
  tmdb: Schema.Boolean,
  tvdb: Schema.Boolean,
});

export interface EpisodeMetadataArtworkFetchers {
  tmdb: boolean;
  tvdb: boolean;
  omdb: boolean;
}

export const EpisodeMetadataArtworkFetchersSchema = Schema.Struct({
  tmdb: Schema.Boolean,
  tvdb: Schema.Boolean,
  omdb: Schema.Boolean,
});

export interface MetadataArtworkSettings {
  movies: ShowMetadataArtworkFetchers;
  shows: ShowMetadataArtworkFetchers;
  seasons: ShowMetadataArtworkFetchers;
  episodes: EpisodeMetadataArtworkFetchers;
}

export const MetadataArtworkSettingsSchema = Schema.Struct({
  movies: ShowMetadataArtworkFetchersSchema,
  shows: ShowMetadataArtworkFetchersSchema,
  seasons: ShowMetadataArtworkFetchersSchema,
  episodes: EpisodeMetadataArtworkFetchersSchema,
});

export interface MetadataArtworkSettingsResponse {
  settings: MetadataArtworkSettings;
  provider_availability: MetadataArtworkProviderStatus[];
}

export const MetadataArtworkSettingsResponseSchema = Schema.Struct({
  settings: MetadataArtworkSettingsSchema,
  provider_availability: Schema.Array(MetadataArtworkProviderStatusSchema),
});

export interface PosterCandidate {
  id: string;
  provider: MetadataArtworkProvider;
  label: string;
  image_url: string;
  source_url: string;
  selected: boolean;
}

export const PosterCandidateSchema = Schema.Struct({
  id: Schema.String,
  provider: MetadataArtworkProviderSchema,
  label: Schema.String,
  image_url: Schema.String,
  source_url: Schema.String,
  selected: Schema.Boolean,
});

export interface PosterCandidatesResponse {
  candidates: PosterCandidate[];
  provider_availability: MetadataArtworkProviderStatus[];
  has_custom_selection: boolean;
}

export const PosterCandidatesResponseSchema = Schema.Struct({
  candidates: Schema.Array(PosterCandidateSchema),
  provider_availability: Schema.Array(MetadataArtworkProviderStatusSchema),
  has_custom_selection: Schema.Boolean,
});

export interface SetPosterSelectionPayload {
  source_url: string;
}

export const SetPosterSelectionPayloadSchema = Schema.Struct({
  source_url: Schema.String,
});

export interface WelcomeEvent {
  type: "welcome";
  message: string;
}

export interface PongEvent {
  type: "pong";
}

/**
 * Interval (milliseconds) at which video clients SHOULD persist in-session playback progress
 * (e.g. `UpdateMediaProgressPayload.progress_seconds`) so resume points stay aligned when
 * switching devices. Web `PlaybackDock` and Android `PlumPlayerController` must match this value.
 */
export const PLAYBACK_PROGRESS_HEARTBEAT_MS = 10_000;

export interface AttachPlaybackSessionCommand {
  action: "attach_playback_session";
  sessionId: string;
}

export interface DetachPlaybackSessionCommand {
  action: "detach_playback_session";
  sessionId: string;
}

/**
 * WebSocket `playback_session_update` payload. Must match Go
 * `transcoder.PlaybackSessionState.MarshalWSPayload` and Android
 * `PlaybackSessionUpdateEventJson`.
 */
export interface PlaybackSessionUpdateEvent {
  type: "playback_session_update";
  sessionId: string;
  delivery: "remux" | "transcode" | "direct";
  mediaId: number;
  /** Omitted for direct delivery and zero revision (matches Go `json:",omitempty"`). */
  revision?: number;
  audioIndex: number;
  status: PlaybackSessionStatus;
  streamUrl: string;
  durationSeconds: number;
  streamOffsetSeconds?: number;
  burnEmbeddedSubtitleStreamIndex?: number;
  error?: string;
  intro_start_seconds?: number;
  intro_end_seconds?: number;
  credits_start_seconds?: number;
  credits_end_seconds?: number;
}

export interface LibraryScanUpdateEvent {
  type: "library_scan_update";
  scan: LibraryScanStatus;
}

/** Emitted when library rows are hidden or reshaped without a terminal scan phase change (e.g. immediate missing-file mark after delete). */
export interface LibraryCatalogChangedEvent {
  type: "library_catalog_changed";
  libraryId: number;
}

export type PlumWebSocketEvent =
  | WelcomeEvent
  | PongEvent
  | PlaybackSessionUpdateEvent
  | LibraryScanUpdateEvent
  | LibraryCatalogChangedEvent;

export type PlumWebSocketCommand = AttachPlaybackSessionCommand | DetachPlaybackSessionCommand;

export const WelcomeEventSchema = Schema.Struct({
  type: Schema.Literal("welcome"),
  message: Schema.String,
});

export const PongEventSchema = Schema.Struct({
  type: Schema.Literal("pong"),
});

export const AttachPlaybackSessionCommandSchema = Schema.Struct({
  action: Schema.Literal("attach_playback_session"),
  sessionId: Schema.String,
});

export const DetachPlaybackSessionCommandSchema = Schema.Struct({
  action: Schema.Literal("detach_playback_session"),
  sessionId: Schema.String,
});

export const PlaybackSessionUpdateEventSchema = Schema.Struct({
  type: Schema.Literal("playback_session_update"),
  sessionId: Schema.String,
  delivery: Schema.Literals(["remux", "transcode", "direct"]),
  mediaId: Schema.Number,
  revision: Schema.optional(Schema.Number),
  audioIndex: Schema.Number,
  status: PlaybackSessionStatusSchema,
  streamUrl: Schema.String,
  durationSeconds: Schema.Number,
  streamOffsetSeconds: Schema.optional(Schema.Number),
  burnEmbeddedSubtitleStreamIndex: Schema.optional(Schema.Number),
  error: Schema.optional(Schema.String),
  intro_start_seconds: Schema.optional(Schema.Number),
  intro_end_seconds: Schema.optional(Schema.Number),
  credits_start_seconds: Schema.optional(Schema.Number),
  credits_end_seconds: Schema.optional(Schema.Number),
});

export interface PatchMediaIntroPayload {
  intro_start_seconds?: number;
  intro_end_seconds?: number;
  intro_locked?: boolean;
  clear_intro?: boolean;
  credits_start_seconds?: number;
  credits_end_seconds?: number;
  clear_credits?: boolean;
}

export const PatchMediaIntroPayloadSchema = Schema.Struct({
  intro_start_seconds: Schema.optional(Schema.Number),
  intro_end_seconds: Schema.optional(Schema.Number),
  intro_locked: Schema.optional(Schema.Boolean),
  clear_intro: Schema.optional(Schema.Boolean),
  credits_start_seconds: Schema.optional(Schema.Number),
  credits_end_seconds: Schema.optional(Schema.Number),
  clear_credits: Schema.optional(Schema.Boolean),
});

export const LibraryScanUpdateEventSchema = Schema.Struct({
  type: Schema.Literal("library_scan_update"),
  scan: LibraryScanStatusSchema,
});

export const LibraryCatalogChangedEventSchema = Schema.Struct({
  type: Schema.Literal("library_catalog_changed"),
  libraryId: Schema.Number,
});

export const PlumWebSocketEventSchema = Schema.Union([
  WelcomeEventSchema,
  PongEventSchema,
  PlaybackSessionUpdateEventSchema,
  LibraryScanUpdateEventSchema,
  LibraryCatalogChangedEventSchema,
]);

/** Known admin maintenance task identifiers (server + client). */
export type AdminMaintenanceTaskId =
  | "optimize_database"
  | "clean_transcode"
  | "clean_logs"
  | "delete_cache"
  | "scan_all_media"
  | "extract_chapter_images"
  | "check_metadata_updates";

export const AdminMaintenanceTaskIdSchema = Schema.Literals([
  "optimize_database",
  "clean_transcode",
  "clean_logs",
  "delete_cache",
  "scan_all_media",
  "extract_chapter_images",
  "check_metadata_updates",
]);

export interface AdminMaintenanceScheduleTask {
  intervalHours: number;
}

export interface AdminMaintenanceScheduleResponse {
  tasks: Record<string, AdminMaintenanceScheduleTask>;
  lastRun: Record<string, string>;
}

export const AdminMaintenanceScheduleTaskSchema = Schema.Struct({
  intervalHours: Schema.Number,
});

export const AdminMaintenanceScheduleResponseSchema = Schema.Struct({
  tasks: Schema.Record(Schema.String, AdminMaintenanceScheduleTaskSchema),
  lastRun: Schema.Record(Schema.String, Schema.String),
});

export interface AdminMaintenanceRunRequest {
  task: AdminMaintenanceTaskId;
}

export const AdminMaintenanceRunRequestSchema = Schema.Struct({
  task: AdminMaintenanceTaskIdSchema,
});

export interface AdminMaintenanceRunResponse {
  task: string;
  accepted: boolean;
  detail?: string;
  error?: string;
}

export const AdminMaintenanceRunResponseSchema = Schema.Struct({
  task: Schema.String,
  accepted: Schema.Boolean,
  detail: Schema.optional(Schema.String),
  error: Schema.optional(Schema.String),
});

export interface AdminActivePlaybackSession {
  sessionId: string;
  userId: number;
  userEmail: string;
  mediaId: number;
  title: string;
  libraryId: number;
  kind: string;
  delivery: string;
  status: string;
  durationSeconds: number;
}

export const AdminActivePlaybackSessionSchema = Schema.Struct({
  sessionId: Schema.String,
  userId: Schema.Number,
  userEmail: Schema.String,
  mediaId: Schema.Number,
  title: Schema.String,
  libraryId: Schema.Number,
  kind: Schema.String,
  delivery: Schema.String,
  status: Schema.String,
  durationSeconds: Schema.Number,
});

export interface AdminActivePlaybackResponse {
  sessions: AdminActivePlaybackSession[];
}

export const AdminActivePlaybackResponseSchema = Schema.Struct({
  sessions: Schema.Array(AdminActivePlaybackSessionSchema),
});

export interface AdminLogsResponse {
  lines: string[];
  source: string;
  hint?: string;
}

export const AdminLogsResponseSchema = Schema.Struct({
  lines: Schema.Array(Schema.String),
  source: Schema.String,
  hint: Schema.optional(Schema.String),
});

export const PlumWebSocketCommandSchema = Schema.Union([
  AttachPlaybackSessionCommandSchema,
  DetachPlaybackSessionCommandSchema,
]);

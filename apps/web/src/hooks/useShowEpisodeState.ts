import { useMemo } from "react";
import type { MediaItem, ShowEpisodesResponse, ShowSeasonEpisodes } from "@/api";
import { shouldShowProgress } from "@/lib/progress";
import { sortEpisodes } from "@/lib/showGrouping";

export type EpisodesBySeason = {
  map: Map<number, MediaItem[]>;
  labels: Map<number, string>;
  seasons: number[];
};

export type ShowEpisodeState = {
  episodes: MediaItem[];
  sortedEpisodes: MediaItem[];
  resumeEpisode: MediaItem | null;
  episodesBySeason: EpisodesBySeason;
};

const EMPTY_SEASONS: EpisodesBySeason = {
  map: new Map(),
  labels: new Map(),
  seasons: [],
};

export function useShowEpisodeState(
  episodesData: ShowEpisodesResponse | undefined | null,
): ShowEpisodeState {
  return useMemo<ShowEpisodeState>(() => {
    if (!episodesData?.seasons) {
      return {
        episodes: [],
        sortedEpisodes: [],
        resumeEpisode: null,
        episodesBySeason: EMPTY_SEASONS,
      };
    }

    const episodes: MediaItem[] = [];
    const map = new Map<number, MediaItem[]>();
    const labels = new Map<number, string>();
    for (const s of episodesData.seasons as ShowSeasonEpisodes[]) {
      const seasonEpisodes = s.episodes as MediaItem[];
      map.set(s.seasonNumber, seasonEpisodes);
      labels.set(s.seasonNumber, s.label);
      for (const e of seasonEpisodes) episodes.push(e);
    }
    const seasons = [...map.keys()].toSorted((a, b) => a - b);

    const sortedEpisodes = [...episodes];
    sortEpisodes(sortedEpisodes);

    let resumeEpisode: MediaItem | null = null;
    if (sortedEpisodes.length > 0) {
      const inProgress = sortedEpisodes
        .filter((ep) => shouldShowProgress(ep))
        .toSorted((a, b) => (b.last_watched_at ?? "").localeCompare(a.last_watched_at ?? ""))[0];
      if (inProgress) {
        resumeEpisode = inProgress;
      } else {
        const lastWatched = sortedEpisodes
          .filter((ep) => ep.last_watched_at)
          .toSorted((a, b) => (b.last_watched_at ?? "").localeCompare(a.last_watched_at ?? ""))[0];
        if (lastWatched) {
          const idx = sortedEpisodes.findIndex((ep) => ep.id === lastWatched.id);
          if (idx >= 0 && idx + 1 < sortedEpisodes.length) {
            resumeEpisode = sortedEpisodes[idx + 1] ?? null;
          }
        }
      }
    }

    return {
      episodes,
      sortedEpisodes,
      resumeEpisode,
      episodesBySeason: { map, labels, seasons },
    };
  }, [episodesData]);
}

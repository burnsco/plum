import type { MetadataArtworkProviderStatus, MetadataArtworkSettings as MetadataArtworkSettingsShape } from "@plum/contracts";
import { Button } from "@/components/ui/button";
import { CheckboxCard } from "./settingsControls";
import {
  episodeArtworkProviderOptions,
  movieArtworkProviderOptions,
  showArtworkProviderOptions,
} from "./settingsOptions";

type MetadataAvailabilityMap = Map<string, MetadataArtworkProviderStatus>;

type MetadataArtworkTabProps = {
  metadataArtworkQuery: {
    isLoading: boolean;
    isError: boolean;
    error: Error | null;
    data?: { provider_availability?: MetadataArtworkProviderStatus[] } | null;
  };
  metadataArtworkForm: MetadataArtworkSettingsShape | null;
  metadataArtworkAvailabilityByProvider: MetadataAvailabilityMap;
  metadataArtworkSaveMessage: string | null;
  metadataArtworkDirty: boolean;
  metadataArtworkSaving: boolean;
  handleSaveMetadataArtwork: () => void;
  setArtworkField: (
    section: keyof Pick<MetadataArtworkSettingsShape, "movies" | "shows" | "seasons">,
    key: keyof MetadataArtworkSettingsShape["shows"],
    checked: boolean,
  ) => void;
  setEpisodeArtworkField: (
    key: keyof MetadataArtworkSettingsShape["episodes"],
    checked: boolean,
  ) => void;
};

export function SettingsMetadataTab(props: MetadataArtworkTabProps) {
  const metadataArtworkForm = props.metadataArtworkForm;

  if (props.metadataArtworkQuery.isLoading || metadataArtworkForm == null) {
    return (
      <div className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4">
        <p className="text-sm text-(--plum-muted)">Loading metadata artwork settings…</p>
      </div>
    );
  }

  if (props.metadataArtworkQuery.isError) {
    return (
      <div className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4">
        <p className="text-sm text-red-300">
          {props.metadataArtworkQuery.error?.message || "Failed to load metadata artwork settings."}
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-3 rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4 shadow-[0_20px_45px_rgba(0,0,0,0.35)] md:flex-row md:items-end md:justify-between">
        <div>
          <h2 className="text-xl font-semibold text-(--plum-text)">Metadata artwork</h2>
          <p className="mt-1 max-w-2xl text-sm text-(--plum-muted)">
            Control which image fetchers Plum uses for movies, shows, seasons, and episodes.
            Provider order is fixed; these toggles only enable or disable each step.
          </p>
        </div>
        <Button onClick={props.handleSaveMetadataArtwork} disabled={props.metadataArtworkSaving}>
          {props.metadataArtworkSaving ? "Saving…" : "Save settings"}
        </Button>
      </div>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(18rem,1fr)]">
        <div className="flex flex-col gap-6">
          <div className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4">
            <h3 className="text-base font-medium text-(--plum-text)">Movies</h3>
            <p className="mt-1 text-sm text-(--plum-muted)">
              Automatic order: Fanart, then TMDB, then TVDB.
            </p>
            <div className="mt-5 grid gap-3 md:grid-cols-2">
              {movieArtworkProviderOptions.map((option) => {
                const availability = props.metadataArtworkAvailabilityByProvider.get(option.key);
                return (
                  <CheckboxCard
                    key={`movies-${option.key}`}
                    checked={metadataArtworkForm.movies[option.key]}
                    label={option.label}
                    description={
                      availability && !availability.available && availability.reason
                        ? `${option.description} ${availability.reason}.`
                        : option.description
                    }
                    disabled={availability?.available === false}
                    onChange={(checked) => props.setArtworkField("movies", option.key, checked)}
                  />
                );
              })}
            </div>
          </div>

          <div className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4">
            <h3 className="text-base font-medium text-(--plum-text)">Shows</h3>
            <p className="mt-1 text-sm text-(--plum-muted)">
              Automatic order: Fanart, then TMDB, then TVDB.
            </p>
            <div className="mt-5 grid gap-3 md:grid-cols-2">
              {showArtworkProviderOptions.map((option) => {
                const availability = props.metadataArtworkAvailabilityByProvider.get(option.key);
                return (
                  <CheckboxCard
                    key={`shows-${option.key}`}
                    checked={metadataArtworkForm.shows[option.key]}
                    label={option.label}
                    description={
                      availability && !availability.available && availability.reason
                        ? `${option.description} ${availability.reason}.`
                        : option.description
                    }
                    disabled={availability?.available === false}
                    onChange={(checked) => props.setArtworkField("shows", option.key, checked)}
                  />
                );
              })}
            </div>
          </div>

          <div className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4">
            <h3 className="text-base font-medium text-(--plum-text)">Seasons</h3>
            <p className="mt-1 text-sm text-(--plum-muted)">
              Automatic order: Fanart, then TMDB, then TVDB.
            </p>
            <div className="mt-5 grid gap-3 md:grid-cols-2">
              {showArtworkProviderOptions.map((option) => {
                const availability = props.metadataArtworkAvailabilityByProvider.get(option.key);
                return (
                  <CheckboxCard
                    key={`seasons-${option.key}`}
                    checked={metadataArtworkForm.seasons[option.key]}
                    label={option.label}
                    description={
                      availability && !availability.available && availability.reason
                        ? `${option.description} ${availability.reason}.`
                        : option.description
                    }
                    disabled={availability?.available === false}
                    onChange={(checked) => props.setArtworkField("seasons", option.key, checked)}
                  />
                );
              })}
            </div>
          </div>

          <div className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4">
            <h3 className="text-base font-medium text-(--plum-text)">Episodes</h3>
            <p className="mt-1 text-sm text-(--plum-muted)">
              Automatic order: TMDB, TVDB, then OMDb.
            </p>
            <div className="mt-5 grid gap-3 md:grid-cols-2">
              {episodeArtworkProviderOptions.map((option) => {
                const availability = props.metadataArtworkAvailabilityByProvider.get(option.key);
                return (
                  <CheckboxCard
                    key={`episodes-${option.key}`}
                    checked={metadataArtworkForm.episodes[option.key]}
                    label={option.label}
                    description={
                      availability && !availability.available && availability.reason
                        ? `${option.description} ${availability.reason}.`
                        : option.description
                    }
                    disabled={availability?.available === false}
                    onChange={(checked) => props.setEpisodeArtworkField(option.key, checked)}
                  />
                );
              })}
            </div>
          </div>
        </div>

        <aside className="flex flex-col gap-4">
          <div className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-5">
            <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-(--plum-muted)">
              Provider status
            </h3>
            <ul className="mt-3 space-y-3">
              {(props.metadataArtworkQuery.data?.provider_availability ?? []).map((provider) => (
                <li
                  key={provider.provider}
                  className={`rounded-md border p-3 text-sm ${
                    provider.available
                      ? "border-emerald-500/30 bg-emerald-500/10 text-emerald-100"
                      : "border-(--plum-border) bg-(--plum-panel-alt)/60 text-(--plum-muted)"
                  }`}
                >
                  <div className="font-medium uppercase tracking-[0.12em]">{provider.provider}</div>
                  <div className="mt-1">
                    {provider.available ? "Available" : provider.reason || "Unavailable"}
                  </div>
                </li>
              ))}
            </ul>
          </div>

          <div className="rounded-(--radius-lg) border border-(--plum-border) bg-(--plum-panel)/80 p-5">
            <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-(--plum-muted)">
              Save status
            </h3>
            <p
              className={`mt-3 text-sm ${
                props.metadataArtworkSaveMessage?.includes("saved")
                  ? "text-emerald-300"
                  : props.metadataArtworkSaveMessage
                    ? "text-red-300"
                    : "text-(--plum-muted)"
              }`}
            >
              {props.metadataArtworkSaveMessage ??
                (props.metadataArtworkDirty
                  ? "Unsaved changes."
                  : "Saved settings are active for future metadata refreshes.")}
            </p>
          </div>
        </aside>
      </div>
    </div>
  );
}

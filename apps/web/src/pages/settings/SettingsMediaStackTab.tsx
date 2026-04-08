import type { UseQueryResult } from "@tanstack/react-query";
import type { MediaStackServiceValidationResult, MediaStackSettings as MediaStackSettingsShape } from "@plum/contracts";
import { Button } from "@/components/ui/button";
import { MediaStackArrQualityProfileCard, MediaStackServiceCard } from "./settingsControls";

type MediaStackValidation = {
  radarr: MediaStackServiceValidationResult;
  sonarrTv: MediaStackServiceValidationResult;
};

type MediaStackTabBaseProps = {
  mediaStackQuery: Pick<UseQueryResult<MediaStackSettingsShape>, "isLoading" | "isError" | "error">;
  mediaStackForm: MediaStackSettingsShape | null;
  mediaStackValidation: MediaStackValidation | null;
  mediaStackSaveMessage: string | null;
  mediaStackSaveTone: "success" | "warning" | "error";
  mediaStackDirty: boolean;
  mediaStackSaving: boolean;
  handleValidateMediaStack: (options: { applyDefaults: boolean }) => void;
  handleSaveMediaStack: () => void;
  setMediaStackServiceField: <
    S extends keyof MediaStackSettingsShape,
    K extends keyof MediaStackSettingsShape[S],
  >(
    service: S,
    key: K,
    value: MediaStackSettingsShape[S][K],
  ) => void;
};

export function SettingsMediaStackTab(props: MediaStackTabBaseProps) {
  const error = props.mediaStackQuery.error;
  return (
    <section className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4 shadow-[0_20px_45px_rgba(0,0,0,0.35)]">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <h2 className="text-xl font-semibold text-(--plum-text)">Media stack</h2>
          <p className="mt-1 max-w-2xl text-sm text-(--plum-muted)">
            Connect Radarr and Sonarr TV so Discover can add titles directly and Downloads can show
            live queue progress.
          </p>
        </div>
        <div className="flex flex-wrap gap-3">
          <Button
            variant="outline"
            onClick={() => void props.handleValidateMediaStack({ applyDefaults: true })}
            disabled={props.mediaStackForm == null || props.mediaStackQuery.isLoading}
          >
            {props.mediaStackQuery.isLoading ? "Validating..." : "Validate & load defaults"}
          </Button>
          <Button
            onClick={props.handleSaveMediaStack}
            disabled={props.mediaStackForm == null || !props.mediaStackDirty || props.mediaStackSaving}
          >
            {props.mediaStackSaving ? "Saving..." : "Save settings"}
          </Button>
        </div>
      </div>

      {props.mediaStackQuery.isLoading || props.mediaStackForm == null ? (
        <p className="mt-5 text-sm text-(--plum-muted)">Loading media stack settings...</p>
      ) : props.mediaStackQuery.isError ? (
        <p className="mt-5 text-sm text-red-300">{error?.message || "Failed to load media stack settings."}</p>
      ) : (
        <div className="mt-6 grid gap-4 xl:grid-cols-2">
          <MediaStackServiceCard
            title="Radarr"
            description="Movie adds always route here in v1."
            service={props.mediaStackForm.radarr}
            validation={props.mediaStackValidation?.radarr ?? null}
            onChange={(key, value) => props.setMediaStackServiceField("radarr", key, value)}
          />
          <MediaStackServiceCard
            title="Sonarr TV"
            description="TV show adds always route here in v1."
            service={props.mediaStackForm.sonarrTv}
            validation={props.mediaStackValidation?.sonarrTv ?? null}
            onChange={(key, value) => props.setMediaStackServiceField("sonarrTv", key, value)}
          />
        </div>
      )}

      <p
        className={`mt-4 text-sm ${
          props.mediaStackSaveMessage == null
            ? "text-(--plum-muted)"
            : props.mediaStackSaveTone === "success"
              ? "text-emerald-300"
              : props.mediaStackSaveTone === "warning"
                ? "text-amber-200"
                : props.mediaStackSaveMessage
                  ? "text-red-300"
                  : "text-(--plum-muted)"
        }`}
      >
        {props.mediaStackSaveMessage ??
          (props.mediaStackDirty
            ? "Unsaved changes."
            : "Direct adds always search immediately after Plum hands the title to Radarr or Sonarr TV.")}
      </p>
    </section>
  );
}

export function SettingsArrProfilesTab(props: MediaStackTabBaseProps) {
  const error = props.mediaStackQuery.error;
  return (
    <section className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4 shadow-[0_20px_45px_rgba(0,0,0,0.35)]">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <h2 className="text-xl font-semibold text-(--plum-text)">Sonarr / Radarr profiles</h2>
          <p className="mt-1 max-w-2xl text-sm text-(--plum-muted)">
            Pick the default quality profiles used when Discover adds a movie to Radarr or a series to
            Sonarr TV. Set the base URL and API key on the Media stack tab first. Lists load
            automatically when you open this tab; use Refresh if you changed Arr outside Plum.{" "}
            <span className="text-(--plum-muted)">
              Refreshing only updates the dropdowns and does not replace your saved choices.
            </span>
          </p>
        </div>
        <div className="flex flex-wrap gap-3">
          <Button
            variant="outline"
            onClick={() => void props.handleValidateMediaStack({ applyDefaults: false })}
            disabled={props.mediaStackForm == null || props.mediaStackQuery.isLoading}
          >
            {props.mediaStackQuery.isLoading ? "Refreshing…" : "Refresh profile lists"}
          </Button>
          <Button
            onClick={props.handleSaveMediaStack}
            disabled={props.mediaStackForm == null || !props.mediaStackDirty || props.mediaStackSaving}
          >
            {props.mediaStackSaving ? "Saving..." : "Save profile defaults"}
          </Button>
        </div>
      </div>

      {props.mediaStackQuery.isLoading || props.mediaStackForm == null ? (
        <p className="mt-5 text-sm text-(--plum-muted)">Loading media stack settings...</p>
      ) : props.mediaStackQuery.isError ? (
        <p className="mt-5 text-sm text-red-300">{error?.message || "Failed to load media stack settings."}</p>
      ) : (
        <div className="mt-6 grid gap-4 xl:grid-cols-2">
          <MediaStackArrQualityProfileCard
            title="Radarr (movies)"
            description="Default quality profile for new movie adds."
            service={props.mediaStackForm.radarr}
            validation={props.mediaStackValidation?.radarr ?? null}
            idPrefix="arr-profiles-radarr"
            qualityLabel="Default movie quality profile (Radarr)"
            onChange={(profileId) => props.setMediaStackServiceField("radarr", "qualityProfileId", profileId)}
          />
          <MediaStackArrQualityProfileCard
            title="Sonarr TV (shows)"
            description="Default quality profile for new series adds."
            service={props.mediaStackForm.sonarrTv}
            validation={props.mediaStackValidation?.sonarrTv ?? null}
            idPrefix="arr-profiles-sonarr"
            qualityLabel="Default TV quality profile (Sonarr)"
            onChange={(profileId) =>
              props.setMediaStackServiceField("sonarrTv", "qualityProfileId", profileId)
            }
          />
        </div>
      )}

      <p
        className={`mt-4 text-sm ${
          props.mediaStackSaveMessage == null
            ? "text-(--plum-muted)"
            : props.mediaStackSaveTone === "success"
              ? "text-emerald-300"
              : props.mediaStackSaveTone === "warning"
                ? "text-amber-200"
                : props.mediaStackSaveMessage
                  ? "text-red-300"
                  : "text-(--plum-muted)"
        }`}
      >
        {props.mediaStackSaveMessage ??
          (props.mediaStackDirty
            ? "Unsaved changes."
            : "Saved defaults are used the next time a title is sent to Radarr or Sonarr TV.")}
      </p>
    </section>
  );
}

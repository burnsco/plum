import type {
  MediaStackServiceValidationResult,
  MediaStackSettings as MediaStackSettingsShape,
} from "@plum/contracts";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

export function Toggle({
  label,
  checked,
  onChange,
  description,
}: {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  description?: string;
}) {
  return (
    <label className="inline-flex cursor-pointer items-start gap-3 text-sm text-(--plum-text)">
      <input
        type="checkbox"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        className="sr-only"
      />
      <span
        aria-hidden
        className={`relative mt-0.5 inline-flex h-5 w-9 shrink-0 rounded-full border-2 border-transparent transition-colors duration-200 ${
          checked ? "bg-(--plum-accent)" : "bg-(--plum-panel-alt)"
        }`}
      >
        <span
          className={`inline-block size-4 rounded-full bg-white shadow-sm transition-transform duration-200 ${
            checked ? "translate-x-4" : "translate-x-0"
          }`}
        />
      </span>
      <span className="flex flex-col">
        <span>{label}</span>
        {description ? (
          <span className="text-xs text-(--plum-muted)">{description}</span>
        ) : null}
      </span>
    </label>
  );
}

export function CheckboxCard({
  checked,
  label,
  description,
  disabled,
  onChange,
}: {
  checked: boolean;
  label: string;
  description: string;
  disabled?: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <label
      className={cn(
        "flex gap-3 rounded-(--radius-md) border p-3 transition-colors",
        disabled
          ? "cursor-not-allowed border-(--plum-border) bg-(--plum-panel-alt)/60 opacity-70"
          : checked
            ? "cursor-pointer border-(--plum-accent-soft) bg-(--plum-accent-subtle)"
            : "cursor-pointer border-(--plum-border) bg-(--plum-panel-alt)/60 hover:border-(--plum-accent-soft)",
      )}
    >
      <input
        type="checkbox"
        checked={checked}
        aria-label={label}
        disabled={disabled}
        onChange={(event) => onChange(event.target.checked)}
        className="mt-1 size-4 shrink-0 rounded border-(--plum-border) bg-(--plum-panel-alt) accent-(--plum-accent)"
      />
      <span className="flex min-w-0 flex-col">
        <span className="text-sm font-medium text-(--plum-text)">{label}</span>
        <span className="text-xs text-(--plum-muted)">{description}</span>
      </span>
    </label>
  );
}

export function MediaStackArrQualityProfileCard({
  title,
  description,
  service,
  validation,
  qualityLabel,
  idPrefix,
  onChange,
}: {
  title: string;
  description: string;
  service: MediaStackSettingsShape["radarr"];
  validation: MediaStackServiceValidationResult | null;
  qualityLabel: string;
  idPrefix: string;
  onChange: (qualityProfileId: number) => void;
}) {
  const reachable = validation?.reachable === true;

  return (
    <article className="rounded-(--radius-lg) border border-(--plum-border) bg-(--plum-panel-alt)/60 p-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h3 className="text-base font-medium text-(--plum-text)">{title}</h3>
          <p className="mt-1 text-sm text-(--plum-muted)">{description}</p>
        </div>
        <span
          className={`rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-[0.16em] ${
            reachable
              ? "bg-emerald-500/12 text-emerald-200"
              : validation?.configured
                ? "bg-amber-500/12 text-amber-200"
                : "bg-(--plum-panel) text-(--plum-muted)"
          }`}
        >
          {reachable ? "Connected" : validation?.configured ? "Needs attention" : "Not configured"}
        </span>
      </div>

      <div className="mt-5">
        <label
          htmlFor={`${idPrefix}-quality-profile`}
          className="mb-2 block text-sm font-medium text-(--plum-text)"
        >
          {qualityLabel}
        </label>
        <select
          id={`${idPrefix}-quality-profile`}
          value={service.qualityProfileId}
          onChange={(event) => onChange(Number.parseInt(event.target.value, 10) || 0)}
          className="flex h-9 w-full rounded-(--radius-md) border border-(--plum-border) bg-(--plum-panel) px-3 py-1 text-sm text-(--plum-text) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--plum-bg)"
        >
          <option value={0}>Select a quality profile</option>
          {(validation?.qualityProfiles ?? []).map((profile) => (
            <option key={profile.id} value={profile.id}>
              {profile.name}
            </option>
          ))}
        </select>
      </div>

      <p
        className={`mt-4 text-sm ${
          validation?.errorMessage ? "text-amber-200" : "text-(--plum-muted)"
        }`}
      >
        {validation?.errorMessage ??
          "Profile names load when you open this tab (after URL and API key are set on Media stack). Use Refresh profile lists to update."}
      </p>
    </article>
  );
}

export function MediaStackServiceCard({
  title,
  description,
  service,
  validation,
  onChange,
}: {
  title: string;
  description: string;
  service: MediaStackSettingsShape["radarr"];
  validation: MediaStackServiceValidationResult | null;
  onChange: <K extends keyof MediaStackSettingsShape["radarr"]>(
    key: K,
    value: MediaStackSettingsShape["radarr"][K],
  ) => void;
}) {
  const reachable = validation?.reachable === true;
  const serviceId = title.toLowerCase().replace(/[^a-z0-9]+/g, "-");

  return (
    <article className="rounded-(--radius-lg) border border-(--plum-border) bg-(--plum-panel-alt)/60 p-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h3 className="text-base font-medium text-(--plum-text)">{title}</h3>
          <p className="mt-1 text-sm text-(--plum-muted)">{description}</p>
        </div>
        <span
          className={`rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-[0.16em] ${
            reachable
              ? "bg-emerald-500/12 text-emerald-200"
              : validation?.configured
                ? "bg-amber-500/12 text-amber-200"
                : "bg-(--plum-panel) text-(--plum-muted)"
          }`}
        >
          {reachable ? "Connected" : validation?.configured ? "Needs attention" : "Not configured"}
        </span>
      </div>

      <div className="mt-5 grid gap-4">
        <div>
          <label
            htmlFor={`${serviceId}-base-url`}
            className="mb-2 block text-sm font-medium text-(--plum-text)"
            title="Root URL you use in the browser for this app (include http/https, no trailing path)."
          >
            Base URL
          </label>
          <Input
            id={`${serviceId}-base-url`}
            value={service.baseUrl}
            onChange={(event) => onChange("baseUrl", event.target.value)}
            placeholder="http://127.0.0.1:7878"
          />
        </div>

        <div>
          <label
            htmlFor={`${serviceId}-api-key`}
            className="mb-2 block text-sm font-medium text-(--plum-text)"
            title="From Settings → General → Security → API key in Radarr or Sonarr."
          >
            API key
          </label>
          <Input
            id={`${serviceId}-api-key`}
            type="password"
            value={service.apiKey}
            onChange={(event) => onChange("apiKey", event.target.value)}
            placeholder="Paste the service API key"
          />
        </div>

        <div className="grid gap-4 md:grid-cols-2">
          <div>
            <label
              htmlFor={`${serviceId}-root-folder`}
              className="mb-2 block text-sm font-medium text-(--plum-text)"
              title="Library path on the Arr host where new downloads are imported."
            >
              Root folder
            </label>
            <select
              id={`${serviceId}-root-folder`}
              value={service.rootFolderPath}
              onChange={(event) => onChange("rootFolderPath", event.target.value)}
              className="flex h-9 w-full rounded-(--radius-md) border border-(--plum-border) bg-(--plum-panel) px-3 py-1 text-sm text-(--plum-text) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--plum-bg)"
            >
              <option value="">Select a root folder</option>
              {(validation?.rootFolders ?? []).map((folder) => (
                <option key={folder.path} value={folder.path}>
                  {folder.path}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label
              htmlFor={`${serviceId}-quality-profile`}
              className="mb-2 block text-sm font-medium text-(--plum-text)"
              title="Default profile when Discover adds a title. Refresh lists after validating the connection."
            >
              Quality profile
            </label>
            <select
              id={`${serviceId}-quality-profile`}
              value={service.qualityProfileId}
              onChange={(event) =>
                onChange("qualityProfileId", Number.parseInt(event.target.value, 10) || 0)
              }
              className="flex h-9 w-full rounded-(--radius-md) border border-(--plum-border) bg-(--plum-panel) px-3 py-1 text-sm text-(--plum-text) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--plum-bg)"
            >
              <option value={0}>Select a quality profile</option>
              {(validation?.qualityProfiles ?? []).map((profile) => (
                <option key={profile.id} value={profile.id}>
                  {profile.name}
                </option>
              ))}
            </select>
          </div>
        </div>
      </div>

      <p
        className={`mt-4 text-sm ${
          validation?.errorMessage ? "text-amber-200" : "text-(--plum-muted)"
        }`}
      >
        {validation?.errorMessage ??
          "Use Validate & load defaults to pull the live root folders and quality profiles from this service."}
      </p>
    </article>
  );
}

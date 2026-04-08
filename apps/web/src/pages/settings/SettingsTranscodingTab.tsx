import type {
  HardwareEncodeFormat,
  TranscodingSettings as TranscodingSettingsShape,
  TranscodingSettingsWarning,
  VaapiDecodeCodec,
} from "@plum/contracts";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { CheckboxCard, Toggle } from "./settingsControls";
import {
  decodeCodecOptions,
  encodeFormatOptions,
  openclTonemapAlgorithmOptions,
} from "./settingsOptions";

type TranscodingSettingsTabProps = {
  settingsQuery: {
    isLoading: boolean;
    isError: boolean;
    error: Error | null;
  };
  form: TranscodingSettingsShape | null;
  warnings: TranscodingSettingsWarning[];
  saveMessage: string | null;
  dirty: boolean;
  saving: boolean;
  handleSave: () => void;
  setField: <K extends keyof TranscodingSettingsShape>(
    key: K,
    value: TranscodingSettingsShape[K],
  ) => void;
  setDecodeCodec: (key: VaapiDecodeCodec, checked: boolean) => void;
  setEncodeFormat: (key: HardwareEncodeFormat, checked: boolean) => void;
};

export function SettingsTranscodingTab(props: TranscodingSettingsTabProps) {
  const form = props.form;

  if (props.settingsQuery.isLoading || form == null) {
    return (
      <div className="rounded-(--radius-lg) border border-(--plum-border) bg-(--plum-panel)/80 p-4">
        <p className="text-sm text-(--plum-muted)">Loading transcoding settings…</p>
      </div>
    );
  }

  if (props.settingsQuery.isError) {
    return (
      <div className="rounded-(--radius-lg) border border-(--plum-border) bg-(--plum-panel)/80 p-4">
        <p className="text-sm text-red-300">
          {props.settingsQuery.error?.message || "Failed to load transcoding settings."}
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-2 rounded-(--radius-lg) border border-(--plum-border) bg-(--plum-panel)/80 p-4 shadow-[0_20px_45px_rgba(0,0,0,0.35)] md:flex-row md:items-end md:justify-between">
        <div>
          <h2 className="text-xl font-semibold text-(--plum-text)">Transcoding</h2>
          <p className="mt-1 max-w-2xl text-sm text-(--plum-muted)">
            Configure server-wide VAAPI decode and hardware encode behavior for future transcode
            jobs.
          </p>
        </div>
        <Button onClick={props.handleSave} disabled={props.saving}>
          {props.saving ? "Saving…" : "Save settings"}
        </Button>
      </div>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(18rem,1fr)]">
        <div className="flex flex-col gap-6">
          <div className="rounded-(--radius-lg) border border-(--plum-border) bg-(--plum-panel)/80 p-4">
            <div className="flex items-start justify-between gap-4">
              <div>
                <h3 className="text-base font-medium text-(--plum-text)">Video Acceleration API</h3>
                <p className="mt-1 text-sm text-(--plum-muted)">
                  Enable VAAPI on the server and choose which source codecs are allowed to use it for
                  decode.
                </p>
              </div>
                <Toggle
                  label="Enable VAAPI"
                  checked={form.vaapiEnabled}
                onChange={(checked) => props.setField("vaapiEnabled", checked)}
              />
            </div>

            <div className="mt-5 space-y-5">
              <div>
                <label className="mb-2 block text-sm font-medium text-(--plum-text)" htmlFor="vaapi-device">
                  VAAPI device
                </label>
                <Input
                  id="vaapi-device"
                  value={form.vaapiDevicePath}
                  onChange={(event) => props.setField("vaapiDevicePath", event.target.value)}
                  placeholder="/dev/dri/renderD128"
                />
                <p className="mt-2 text-xs text-(--plum-muted)">
                  Default render node for Intel/AMD VAAPI on Linux hosts.
                </p>
              </div>

              <div>
                <div className="mb-3">
                  <h4 className="text-sm font-medium text-(--plum-text)">Decode codecs</h4>
                  <p className="mt-1 text-xs text-(--plum-muted)">
                    Each codec can be enabled or disabled independently. Disabled codecs stay on
                    software decode.
                  </p>
                </div>
                <div className="grid gap-3 md:grid-cols-2">
                  {decodeCodecOptions.map((option) => (
                    <CheckboxCard
                      key={option.key}
                      checked={form.decodeCodecs[option.key]}
                      label={option.label}
                      description={option.description}
                      onChange={(checked) => props.setDecodeCodec(option.key, checked)}
                    />
                  ))}
                </div>
              </div>
            </div>
          </div>

          <div className="rounded-(--radius-lg) border border-(--plum-border) bg-(--plum-panel)/80 p-4">
            <div className="flex items-start justify-between gap-4">
              <div>
                <h3 className="text-base font-medium text-(--plum-text)">Hardware encoding</h3>
                <p className="mt-1 text-sm text-(--plum-muted)">
                  Use VAAPI encoders when possible, with automatic software fallback if the hardware
                  path fails.
                </p>
              </div>
                <Toggle
                  label="Enable hardware encoding"
                  checked={form.hardwareEncodingEnabled}
                onChange={(checked) => props.setField("hardwareEncodingEnabled", checked)}
              />
            </div>

            <div className="mt-5 space-y-5">
              <div>
                <div className="mb-3">
                  <h4 className="text-sm font-medium text-(--plum-text)">Allowed output formats</h4>
                  <p className="mt-1 text-xs text-(--plum-muted)">
                    H.264 is enabled by default. HEVC and AV1 stay opt-in for compatibility and host
                    support reasons.
                  </p>
                </div>
                <div className="grid gap-3 md:grid-cols-3">
                  {encodeFormatOptions.map((option) => (
                    <CheckboxCard
                      key={option.key}
                      checked={form.encodeFormats[option.key]}
                      label={option.label}
                      description={option.description}
                      onChange={(checked) => props.setEncodeFormat(option.key, checked)}
                    />
                  ))}
                </div>
              </div>

              <div>
                <label
                  className="mb-2 block text-sm font-medium text-(--plum-text)"
                  htmlFor="preferred-encode-format"
                >
                  Preferred hardware encode format
                </label>
                  <select
                    id="preferred-encode-format"
                  value={form.preferredHardwareEncodeFormat}
                  onChange={(event) =>
                    props.setField(
                      "preferredHardwareEncodeFormat",
                      event.target.value as HardwareEncodeFormat,
                    )
                  }
                  className="flex h-9 w-full rounded-(--radius-md) border border-(--plum-border) bg-(--plum-panel) px-3 py-1 text-sm text-(--plum-text) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--plum-bg)"
                >
                  {encodeFormatOptions.map((option) => (
                    <option
                      key={option.key}
                      value={option.key}
                      disabled={!form.encodeFormats[option.key]}
                    >
                      {option.label}
                    </option>
                  ))}
                </select>
                <p className="mt-2 text-xs text-(--plum-muted)">
                  Plum will try this hardware output format first, then retry in software if enabled
                  below.
                </p>
              </div>

              <Toggle
                label="Allow automatic software fallback"
                checked={form.allowSoftwareFallback}
                onChange={(checked) => props.setField("allowSoftwareFallback", checked)}
                description="When hardware transcoding fails, retry with software-safe FFmpeg settings."
              />
            </div>
          </div>

          <div className="rounded-(--radius-lg) border border-amber-500/25 bg-amber-500/[0.06] p-4">
            <div className="flex items-start justify-between gap-4">
              <div>
                <h3 className="text-base font-medium text-(--plum-text)">
                  Experimental: OpenCL tone mapping
                </h3>
                <p className="mt-1 text-sm text-(--plum-muted)">
                  When enabled, Plum may insert FFmpeg{" "}
                  <code className="rounded bg-black/30 px-1 py-0.5 text-xs">tonemap_opencl</code> for
                  sources that look HDR (PQ / HLG transfer, or 10-bit BT.2020). Requires
                  OpenCL-capable drivers and a matching FFmpeg build. Does not apply when burning in
                  PGS subtitles.
                </p>
              </div>
              <Toggle
                label="Enable OpenCL tone map"
                checked={form.openclToneMappingEnabled}
                onChange={(checked) => props.setField("openclToneMappingEnabled", checked)}
              />
            </div>

            <div className={`mt-5 space-y-5 ${form.openclToneMappingEnabled ? "" : "pointer-events-none opacity-50"}`}>
              <div>
                <label
                  className="mb-2 block text-sm font-medium text-(--plum-text)"
                  htmlFor="opencl-tonemap-algorithm"
                >
                  Tonemap curve
                </label>
                <select
                  id="opencl-tonemap-algorithm"
                  value={form.openclToneMapAlgorithm}
                  onChange={(event) =>
                    props.setField(
                      "openclToneMapAlgorithm",
                      event.target.value as TranscodingSettingsShape["openclToneMapAlgorithm"],
                    )
                  }
                  className="flex h-9 w-full rounded-(--radius-md) border border-(--plum-border) bg-(--plum-panel) px-3 py-1 text-sm text-(--plum-text) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--plum-bg)"
                >
                  {openclTonemapAlgorithmOptions.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
                <p className="mt-2 text-xs text-(--plum-muted)">
                  {
                    openclTonemapAlgorithmOptions.find((o) => o.value === form.openclToneMapAlgorithm)
                      ?.description
                  }
                </p>
              </div>

              <div>
                <label
                  className="mb-2 block text-sm font-medium text-(--plum-text)"
                  htmlFor="opencl-tonemap-desat"
                >
                  Highlight desaturation
                </label>
                  <Input
                    id="opencl-tonemap-desat"
                  type="number"
                  min={0}
                  max={4}
                  step={0.05}
                  value={form.openclToneMapDesat}
                  onChange={(event) => {
                    const n = Number.parseFloat(event.target.value);
                    if (Number.isFinite(n)) {
                      props.setField("openclToneMapDesat", n);
                    }
                  }}
                />
                <p className="mt-2 text-xs text-(--plum-muted)">
                  Passed to FFmpeg as <code className="rounded bg-black/30 px-1 py-0.5 text-xs">desat</code>{" "}
                  (0–4). Try around 0.5 unless you want a more saturated HDR look.
                </p>
              </div>
            </div>
          </div>
        </div>

        <aside className="flex flex-col gap-4">
          <div className="rounded-(--radius-lg) border border-(--plum-border) bg-(--plum-panel)/80 p-5">
            <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-(--plum-muted)">
              Host warnings
            </h3>
            {props.warnings.length === 0 ? (
              <p className="mt-3 text-sm text-(--plum-muted)">
                No capability warnings reported for the current server configuration.
              </p>
            ) : (
              <ul className="mt-3 space-y-3">
                {props.warnings.map((warning) => (
                  <li
                    key={warning.code}
                    className="rounded-(--radius-md) border border-amber-500/30 bg-amber-500/10 p-3 text-sm text-amber-100"
                  >
                    {warning.message}
                  </li>
                ))}
              </ul>
            )}
          </div>

          <div className="rounded-(--radius-lg) border border-(--plum-border) bg-(--plum-panel)/80 p-5">
            <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-(--plum-muted)">
              Save status
            </h3>
            <p
              className={`mt-3 text-sm ${
                props.saveMessage?.includes("saved")
                  ? "text-emerald-300"
                  : props.saveMessage
                    ? "text-red-300"
                    : "text-(--plum-muted)"
              }`}
            >
              {props.saveMessage ?? (props.dirty ? "Unsaved changes." : "Saved settings are active for future jobs.")}
            </p>
          </div>
        </aside>
      </div>
    </div>
  );
}

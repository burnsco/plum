import { useEffect, useMemo, useState } from "react";
import type { ServerEnvSettingsUpdate } from "@plum/contracts";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useServerEnvSettings, useUpdateServerEnvSettings } from "@/queries";
import { cn } from "@/lib/utils";
import { AlertCircle, HelpCircle } from "lucide-react";

function FieldHint({ text }: { text: string }) {
  return (
    <span
      className="inline-flex shrink-0 cursor-help text-(--plum-muted) hover:text-(--plum-text-secondary)"
      title={text}
    >
      <HelpCircle className="size-3.5" strokeWidth={2} aria-hidden />
      <span className="sr-only">{text}</span>
    </span>
  );
}

function LabelWithHint({
  htmlFor,
  label,
  hint,
}: {
  htmlFor: string;
  label: string;
  hint: string;
}) {
  return (
    <label
      htmlFor={htmlFor}
      className="mb-2 flex items-center gap-1.5 text-sm font-medium text-(--plum-text)"
    >
      <span>{label}</span>
      <FieldHint text={hint} />
    </label>
  );
}

function SecretFieldRow({
  id,
  label,
  hint,
  configured,
  value,
  onChange,
  onClear,
  clearRequested,
}: {
  id: string;
  label: string;
  hint: string;
  configured: boolean;
  value: string;
  onChange: (v: string) => void;
  onClear: () => void;
  clearRequested: boolean;
}) {
  return (
    <div className="rounded-lg border border-(--plum-border) bg-(--plum-panel-alt)/50 p-4">
      <LabelWithHint htmlFor={id} label={label} hint={hint} />
      <Input
        id={id}
        type="password"
        autoComplete="off"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={configured ? "Leave blank to keep the saved key" : "Paste API key"}
        disabled={clearRequested}
        className={cn(clearRequested && "opacity-50")}
      />
      <div className="mt-2 flex flex-wrap items-center gap-3 text-xs text-(--plum-muted)">
        {configured ? (
          <span className="text-emerald-200/90">A key is already stored.</span>
        ) : (
          <span>Not configured yet.</span>
        )}
        <button
          type="button"
          className="text-(--plum-accent) underline-offset-2 hover:underline disabled:opacity-40"
          disabled={!configured && !value}
          onClick={onClear}
        >
          {clearRequested ? "Undo remove" : "Remove key"}
        </button>
      </div>
    </div>
  );
}

export function ServerEnvSettingsTab() {
  const query = useServerEnvSettings();
  const update = useUpdateServerEnvSettings();
  const [statusMsg, setStatusMsg] = useState<{ tone: "ok" | "err"; text: string } | null>(null);

  const [plumAddr, setPlumAddr] = useState("");
  const [dbURL, setDbURL] = useState("");
  const [musicBrainz, setMusicBrainz] = useState("");
  const [tmdb, setTmdb] = useState("");
  const [tvdb, setTvdb] = useState("");
  const [omdb, setOmdb] = useState("");
  const [fanart, setFanart] = useState("");
  const [clearTmdb, setClearTmdb] = useState(false);
  const [clearTvdb, setClearTvdb] = useState(false);
  const [clearOmdb, setClearOmdb] = useState(false);
  const [clearFanart, setClearFanart] = useState(false);

  useEffect(() => {
    const d = query.data;
    if (!d) return;
    setPlumAddr(d.plum_addr);
    setDbURL(d.plum_database_url);
    setMusicBrainz(d.musicbrainz_contact_url);
    setTmdb("");
    setTvdb("");
    setOmdb("");
    setFanart("");
    setClearTmdb(false);
    setClearTvdb(false);
    setClearOmdb(false);
    setClearFanart(false);
  }, [query.data]);

  const dirty = useMemo(() => {
    const d = query.data;
    if (!d) return false;
    if (plumAddr !== d.plum_addr || dbURL !== d.plum_database_url || musicBrainz !== d.musicbrainz_contact_url) {
      return true;
    }
    if (tmdb.trim() || tvdb.trim() || omdb.trim() || fanart.trim()) return true;
    if (clearTmdb || clearTvdb || clearOmdb || clearFanart) return true;
    return false;
  }, [
    query.data,
    plumAddr,
    dbURL,
    musicBrainz,
    tmdb,
    tvdb,
    omdb,
    fanart,
    clearTmdb,
    clearTvdb,
    clearOmdb,
    clearFanart,
  ]);

  useEffect(() => {
    if (dirty && statusMsg) setStatusMsg(null);
  }, [dirty, statusMsg]);

  const handleSave = async () => {
    const d = query.data;
    if (!d) return;
    setStatusMsg(null);
    const payload: ServerEnvSettingsUpdate = {};
    if (plumAddr !== d.plum_addr) payload.plum_addr = plumAddr;
    if (dbURL !== d.plum_database_url) payload.plum_database_url = dbURL;
    if (musicBrainz !== d.musicbrainz_contact_url) payload.musicbrainz_contact_url = musicBrainz;
    if (tmdb.trim()) payload.tmdb_api_key = tmdb.trim();
    if (tvdb.trim()) payload.tvdb_api_key = tvdb.trim();
    if (omdb.trim()) payload.omdb_api_key = omdb.trim();
    if (fanart.trim()) payload.fanart_api_key = fanart.trim();
    if (clearTmdb) payload.tmdb_api_key_clear = true;
    if (clearTvdb) payload.tvdb_api_key_clear = true;
    if (clearOmdb) payload.omdb_api_key_clear = true;
    if (clearFanart) payload.fanart_api_key_clear = true;
    try {
      const res = await update.mutateAsync(payload);
      const extra = res.restart_recommended
        ? " Restart the Plum server for listen address or database path changes to take effect."
        : "";
      setStatusMsg({ tone: "ok", text: res.help + extra });
    } catch (e) {
      setStatusMsg({
        tone: "err",
        text: e instanceof Error ? e.message : "Save failed.",
      });
    }
  };

  if (query.isLoading) {
    return (
      <div className="rounded-xl border border-(--plum-border) bg-(--plum-panel)/80 p-8">
        <p className="text-sm text-(--plum-muted)">Loading environment settings…</p>
      </div>
    );
  }

  if (query.isError) {
    return (
      <div className="rounded-xl border border-(--plum-border) bg-(--plum-panel)/80 p-8">
        <p className="text-sm text-red-300">{query.error.message || "Failed to load settings."}</p>
      </div>
    );
  }

  const d = query.data!;

  return (
    <div className="flex flex-col gap-6">
      <section className="rounded-xl border border-(--plum-border) bg-gradient-to-br from-(--plum-panel)/95 to-(--plum-panel-alt)/40 p-6 shadow-[0_24px_50px_rgba(0,0,0,0.35)]">
        <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
          <div>
            <h2 className="text-xl font-semibold tracking-tight text-(--plum-text)">Server &amp; environment</h2>
            <p className="mt-2 max-w-2xl text-sm leading-relaxed text-(--plum-muted)">
              Edit the same variables you would set in your <code className="text-(--plum-text-secondary)">.env</code>{" "}
              file. Saving writes the file on the server host and updates metadata keys for the running process.
              Radarr and Sonarr fields stay on the Media stack tab; saving there also syncs{" "}
              <code className="text-(--plum-text-secondary)">PLUM_RADARR_*</code> and{" "}
              <code className="text-(--plum-text-secondary)">PLUM_SONARR_TV_*</code>.
            </p>
          </div>
          <Button onClick={() => void handleSave()} disabled={!dirty || update.isPending}>
            {update.isPending ? "Saving…" : "Save to .env"}
          </Button>
        </div>

        <div
          className={cn(
            "mt-5 flex flex-col gap-3 rounded-lg border p-4 text-sm md:flex-row md:items-center",
            d.env_file_writable
              ? "border-emerald-500/25 bg-emerald-500/5 text-emerald-100/90"
              : "border-amber-500/30 bg-amber-500/10 text-amber-100",
          )}
        >
          {d.env_file_writable ? null : <AlertCircle className="size-5 shrink-0 text-amber-200" aria-hidden />}
          <div className="min-w-0 flex-1">
            <p className="font-medium text-(--plum-text)">Target file</p>
            <p className="mt-1 break-all font-mono text-xs text-(--plum-muted)">{d.env_file_path}</p>
            <p className="mt-2 text-xs leading-snug text-(--plum-muted)">{d.help}</p>
          </div>
        </div>

        {statusMsg ? (
          <p
            className={`mt-3 text-sm ${statusMsg.tone === "ok" ? "text-emerald-300" : "text-red-300"}`}
          >
            {statusMsg.text}
          </p>
        ) : null}
      </section>

      <div className="grid gap-6 lg:grid-cols-2">
        <section className="rounded-xl border border-(--plum-border) bg-(--plum-panel)/80 p-3">
          <h3 className="text-sm font-semibold uppercase tracking-[0.14em] text-(--plum-muted)">Process &amp; database</h3>
          <p className="mt-2 text-xs text-(--plum-muted)">
            Changing these updates the file only. Restart the Plum server so the new listen address or SQLite path take
            effect.
          </p>
          <div className="mt-5 space-y-4">
            <div>
              <LabelWithHint
                htmlFor="env-plum-addr"
                label="PLUM_ADDR"
                hint="Host and port the Go server binds to, e.g. :8080 or 127.0.0.1:8080."
              />
              <Input id="env-plum-addr" value={plumAddr} onChange={(e) => setPlumAddr(e.target.value)} placeholder=":8080" />
            </div>
            <div>
              <LabelWithHint
                htmlFor="env-db-url"
                label="PLUM_DATABASE_URL"
                hint="Path to the SQLite database file (or connection string). Written as PLUM_DATABASE_URL; legacy PLUM_DB_PATH lines are removed when you save."
              />
              <Input
                id="env-db-url"
                value={dbURL}
                onChange={(e) => setDbURL(e.target.value)}
                placeholder="./data/plum.db"
              />
            </div>
            <div>
              <LabelWithHint
                htmlFor="env-mb"
                label="MUSICBRAINZ_CONTACT_URL"
                hint="Contact URL required by MusicBrainz for API etiquette (often your project or email URL)."
              />
              <Input
                id="env-mb"
                value={musicBrainz}
                onChange={(e) => setMusicBrainz(e.target.value)}
                placeholder="https://github.com/you/plum"
              />
            </div>
          </div>
        </section>

        <section className="rounded-xl border border-(--plum-border) bg-(--plum-panel)/80 p-3">
          <h3 className="text-sm font-semibold uppercase tracking-[0.14em] text-(--plum-muted)">Metadata API keys</h3>
          <p className="mt-2 text-xs text-(--plum-muted)">
            Keys are stored in plaintext in .env like other self-hosted apps. They are not shown back after save.
          </p>
          <div className="mt-5 grid gap-4">
            <SecretFieldRow
              id="env-tmdb"
              label="TMDB_API_KEY"
              hint="Required for Discover, external shelves, and most TV/movie identification."
              configured={d.secrets_present.tmdb_api_key}
              value={tmdb}
              onChange={setTmdb}
              clearRequested={clearTmdb}
              onClear={() => {
                setClearTmdb((c) => !c);
                if (!clearTmdb) setTmdb("");
              }}
            />
            <SecretFieldRow
              id="env-tvdb"
              label="TVDB_API_KEY"
              hint="Optional TVDB v4 key for artwork and some TV metadata when enabled in artwork settings."
              configured={d.secrets_present.tvdb_api_key}
              value={tvdb}
              onChange={setTvdb}
              clearRequested={clearTvdb}
              onClear={() => {
                setClearTvdb((c) => !c);
                if (!clearTvdb) setTvdb("");
              }}
            />
            <SecretFieldRow
              id="env-omdb"
              label="OMDB_API_KEY"
              hint="Optional OMDb key for episode artwork when an IMDb id is known."
              configured={d.secrets_present.omdb_api_key}
              value={omdb}
              onChange={setOmdb}
              clearRequested={clearOmdb}
              onClear={() => {
                setClearOmdb((c) => !c);
                if (!clearOmdb) setOmdb("");
              }}
            />
            <SecretFieldRow
              id="env-fanart"
              label="FANART_API_KEY"
              hint="Optional fanart.tv key for high-quality background and logo artwork."
              configured={d.secrets_present.fanart_api_key}
              value={fanart}
              onChange={setFanart}
              clearRequested={clearFanart}
              onClear={() => {
                setClearFanart((c) => !c);
                if (!clearFanart) setFanart("");
              }}
            />
          </div>
        </section>
      </div>

    </div>
  );
}

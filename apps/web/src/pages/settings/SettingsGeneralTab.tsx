import { Button } from "@/components/ui/button";

export function SettingsGeneralTab({
  userEmail,
  quickConnect,
  quickConnectBusy,
  quickConnectErr,
  onGenerateQuickConnect,
}: {
  userEmail: string;
  quickConnect: { code: string; expiresAt: string } | null;
  quickConnectBusy: boolean;
  quickConnectErr: string | null;
  onGenerateQuickConnect: () => void;
}) {
  return (
    <section className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4 shadow-[0_20px_45px_rgba(0,0,0,0.35)]">
      <div className="flex flex-col gap-2">
        <h2 className="text-xl font-semibold text-(--plum-text)">General</h2>
        <p className="max-w-2xl text-sm text-(--plum-muted)">
          Account shortcuts and devices that sign in outside the browser.
        </p>
      </div>

      <div className="mt-6 rounded-md border border-(--plum-border) bg-(--plum-panel-alt)/60 p-4">
        <h3 className="flex items-center gap-2 text-sm font-semibold text-(--plum-text)">Quick connect</h3>
        <p className="mt-1.5 text-xs leading-snug text-(--plum-muted)">
          TV sign-in for <span className="text-(--plum-text-secondary)">{userEmail}</span>. On the TV,
          use this server&apos;s URL and &quot;Sign in with TV code&quot;. Code expires in 15 minutes,
          one use.
        </p>
        <Button
          type="button"
          variant="secondary"
          size="sm"
          className="mt-3"
          disabled={quickConnectBusy}
          onClick={onGenerateQuickConnect}
        >
          {quickConnectBusy ? "Generating…" : quickConnect ? "New code" : "Generate code"}
        </Button>
        {quickConnectErr ? <p className="mt-2 text-xs text-red-400">{quickConnectErr}</p> : null}
        {quickConnect ? (
          <div className="mt-3 rounded-md border border-(--plum-border) bg-(--plum-panel) p-2">
            <p className="text-[10px] font-medium uppercase tracking-wider text-(--plum-muted)">Code</p>
            <p className="mt-0.5 font-mono text-xl font-semibold tracking-[0.25em] text-(--plum-text)">
              {quickConnect.code}
            </p>
            <p className="mt-1 text-[11px] text-(--plum-muted)">
              Expires {new Date(quickConnect.expiresAt).toLocaleString()} · once
            </p>
          </div>
        ) : null}
      </div>
    </section>
  );
}

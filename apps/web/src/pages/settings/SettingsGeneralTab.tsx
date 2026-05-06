export function SettingsGeneralTab({ userEmail }: { userEmail: string }) {
  return (
    <section className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4 shadow-[0_20px_45px_rgba(0,0,0,0.35)]">
      <div className="flex flex-col gap-2">
        <h2 className="text-xl font-semibold text-(--plum-text)">General</h2>
        <p className="max-w-2xl text-sm text-(--plum-muted)">
          Account details for the signed-in browser session.
        </p>
      </div>

      <div className="mt-6 rounded-md border border-(--plum-border) bg-(--plum-panel-alt)/60 p-4">
        <h3 className="text-sm font-semibold text-(--plum-text)">Account</h3>
        <p className="mt-1.5 text-sm text-(--plum-text-secondary)">{userEmail}</p>
      </div>
    </section>
  );
}

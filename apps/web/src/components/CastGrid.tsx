import { Link } from "react-router-dom";
import { resolveCastProfileUrl } from "@plum/shared";
import { BASE_URL } from "@/api";

export type CastGridMember = {
  name: string;
  character?: string | null;
  profile_path?: string | null;
};

type CastGridProps = {
  members: CastGridMember[];
  /** When true and there are no members, the section is omitted entirely. */
  hideWhenEmpty?: boolean;
  emptyMessage?: string;
};

export function CastGrid({
  members,
  hideWhenEmpty,
  emptyMessage = "No cast metadata yet.",
}: CastGridProps) {
  if (hideWhenEmpty && members.length === 0) {
    return null;
  }

  return (
    <section className="rounded-(--radius-xl) border border-(--plum-border) bg-(--plum-panel) p-5">
      <h2 className="text-lg font-semibold text-(--plum-text)">Cast</h2>
      {members.length === 0 ? (
        <p className="mt-3 text-sm text-(--plum-muted)">{emptyMessage}</p>
      ) : (
        <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {members.map((member) => {
            const headshot = resolveCastProfileUrl(undefined, member.profile_path ?? undefined, "w185", BASE_URL);
            const initial = member.name.trim().charAt(0).toUpperCase() || "?";
            return (
              <Link
                key={`${member.name}-${member.character ?? ""}`}
                to={`/search?q=${encodeURIComponent(member.name)}`}
                className="flex gap-3 rounded-lg border border-(--plum-border) bg-(--plum-panel-alt) p-3 transition-colors hover:border-(--plum-accent)/50 hover:bg-(--plum-panel)"
              >
                {headshot ? (
                  <img
                    src={headshot}
                    alt=""
                    className="h-[4.5rem] w-12 shrink-0 rounded-md object-cover object-top"
                  />
                ) : (
                  <div
                    className="flex h-[4.5rem] w-12 shrink-0 items-center justify-center rounded-md bg-(--plum-border) text-sm font-semibold text-(--plum-muted)"
                    aria-hidden
                  >
                    {initial}
                  </div>
                )}
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-semibold text-(--plum-text)">{member.name}</div>
                  {member.character ? (
                    <div className="text-xs text-(--plum-muted)">{member.character}</div>
                  ) : null}
                </div>
              </Link>
            );
          })}
        </div>
      )}
    </section>
  );
}

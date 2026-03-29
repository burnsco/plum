import { useEffect, useState, type ReactNode } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { useAuthActions, useAuthState } from "@/contexts/AuthContext";
import { Button } from "@/components/ui/button";
import { LibraryActivityCenter } from "@/components/LibraryActivityCenter";
import { Input } from "@/components/ui/input";
import { HorizontalScrollRail } from "@/components/ui/page";
import { useLibraries } from "@/queries";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";
import { Search, Settings, User } from "lucide-react";

export function TopBar() {
  const { user } = useAuthState();
  const { logout } = useAuthActions();
  const navigate = useNavigate();
  const location = useLocation();
  const { data: libraries = [] } = useLibraries();
  const activeQuery =
    location.pathname === "/search" ? new URLSearchParams(location.search).get("q") ?? "" : "";
  const [searchValue, setSearchValue] = useState(activeQuery);
  const activeLibraryId = location.pathname.startsWith("/library/")
    ? Number.parseInt(location.pathname.split("/")[2] ?? "", 10)
    : null;
  const isHomeRoute = location.pathname === "/";
  const isDiscoverRoute = location.pathname === "/discover" || location.pathname.startsWith("/discover/");

  useEffect(() => {
    if (location.pathname === "/search") {
      setSearchValue(activeQuery);
    }
  }, [activeQuery, location.pathname]);

  useEffect(() => {
    const trimmed = searchValue.trim();
    const timeoutId = window.setTimeout(() => {
      if (trimmed.length >= 2) {
        const params = new URLSearchParams();
        params.set("q", trimmed);
        navigate(`/search?${params.toString()}`, { replace: location.pathname === "/search" });
        return;
      }
      if (location.pathname === "/search" && activeQuery) {
        navigate("/search", { replace: true });
      }
    }, 300);
    return () => window.clearTimeout(timeoutId);
  }, [activeQuery, location.pathname, navigate, searchValue]);

  return (
    <header className="sticky top-0 z-40 shrink-0 border-b border-[var(--plum-border)] bg-[var(--plum-bg)]">
      <div className="mx-auto flex w-full max-w-[var(--page-max-width)] flex-col gap-3 px-4 py-3 md:h-16 md:flex-row md:items-center md:gap-4 md:px-6 xl:px-8">
        <div className="flex items-center justify-between gap-3 md:justify-start">
          <Link
            to="/"
            className="flex items-center rounded-[var(--radius-md)] px-1 py-1 transition-opacity hover:opacity-90"
            aria-label="Plum home"
          >
            <img
              src="/logo.svg"
              alt=""
              aria-hidden="true"
              className="h-12 w-12 shrink-0 object-contain md:h-14 md:w-14"
            />
          </Link>

          <div className="flex items-center gap-2 md:hidden">
            <LibraryActivityCenter />
            <Link to="/settings">
              <Button variant="icon" size="icon" aria-label="Settings">
                <Settings className="size-5" />
              </Button>
            </Link>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="icon" size="icon" aria-label="Profile">
                  <User className="size-5" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-56">
                {user?.email && (
                  <div className="truncate px-2 py-1.5 text-sm text-[var(--plum-muted)]">
                    {user.email}
                  </div>
                )}
                <DropdownMenuItem
                  onSelect={() => logout()}
                  className="text-[var(--plum-accent)] focus:text-[var(--plum-accent)]"
                >
                  Sign out
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>

        <div className="flex md:flex-1 md:justify-center">
          <div className="relative w-full max-w-xl">
            <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[var(--plum-muted)]" />
            <Input
              type="search"
              value={searchValue}
              onChange={(event) => setSearchValue(event.target.value)}
              placeholder="Search libraries, shows, and movies"
              className="h-10 border-[var(--plum-border)]/90 bg-[var(--plum-panel-alt)]/85 pl-9"
            />
          </div>
        </div>

        <HorizontalScrollRail
          label="mobile navigation"
          className="md:hidden"
          contentClassName="flex gap-2 overflow-x-auto px-10"
        >
          <MobileNavLink to="/" active={isHomeRoute}>
            Home
          </MobileNavLink>
          <MobileNavLink to="/discover" active={isDiscoverRoute}>
            Discover
          </MobileNavLink>
          {libraries.map((library) => {
            const active =
              activeLibraryId === library.id ||
              location.pathname.startsWith(`/library/${library.id}/`);
            return (
              <MobileNavLink key={library.id} to={`/library/${library.id}`} active={active}>
                {library.name}
              </MobileNavLink>
            );
          })}
        </HorizontalScrollRail>

        <div className="hidden items-center gap-2 md:flex">
          <LibraryActivityCenter />

          <Link to="/settings">
            <Button variant="icon" size="icon" aria-label="Settings">
              <Settings className="size-5" />
            </Button>
          </Link>

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="icon" size="icon" aria-label="Profile">
                <User className="size-5" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
              {user?.email && (
                <div className="truncate px-2 py-1.5 text-sm text-[var(--plum-muted)]">
                  {user.email}
                </div>
              )}
              <DropdownMenuItem
                onSelect={() => logout()}
                className="text-[var(--plum-accent)] focus:text-[var(--plum-accent)]"
              >
                Sign out
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
    </header>
  );
}

function MobileNavLink({
  to,
  active,
  children,
}: {
  to: string;
  active: boolean;
  children: ReactNode;
}) {
  return (
    <Link
      to={to}
      className={cn(
        "shrink-0 rounded-full border px-3 py-1.5 text-xs font-medium transition-colors",
        active
          ? "border-[color-mix(in_srgb,var(--plum-accent)_28%,var(--plum-border))] bg-[var(--plum-accent-soft)] text-[var(--plum-accent)]"
          : "border-[var(--plum-border)] bg-[var(--plum-panel-alt)]/80 text-[var(--plum-muted)] hover:border-[color-mix(in_srgb,var(--plum-accent)_20%,var(--plum-border))] hover:text-[var(--plum-text)]",
      )}
    >
      {children}
    </Link>
  );
}

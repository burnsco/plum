import { LibraryActivityCenter } from "@/components/LibraryActivityCenter";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { HorizontalScrollRail } from "@/components/ui/page";
import { useAuthActions, useAuthState } from "@/contexts/AuthContext";
import { cn } from "@/lib/utils";
import { useLibraries } from "@/queries";
import { Search, Settings, User } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";

/** Plum logo icon — purple plum fruit on a dark button */
function PlumLogoButton() {
  return (
    <Link
      to="/"
      className="flex items-center justify-center shrink-0 rounded-[var(--radius-lg)] transition-opacity hover:opacity-85 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--plum-ring)]"
      aria-label="Plum home"
    >
      <svg
        width="42"
        height="42"
        viewBox="0 0 42 42"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        {/* Dark rounded-square background */}
        <rect width="42" height="42" rx="10" fill="#1a1a1a" />

        {/* Purple plum fruit body */}
        <ellipse cx="21" cy="23" rx="12" ry="11" fill="url(#plumGrad)" />

        {/* Highlight gloss */}
        <ellipse cx="17.5" cy="19.5" rx="4" ry="3" fill="rgba(255,255,255,0.18)" />

        {/* Stem */}
        <path d="M21 12 C21 12, 22 9, 25 8 C24 10, 23 11, 21 12Z" fill="#5a3e1b" />

        {/* Leaf */}
        <path d="M21 12 C21 12, 24 10, 27 11 C25 13, 22 13, 21 12Z" fill="#4ade80" opacity="0.85" />

        <defs>
          <radialGradient id="plumGrad" cx="38%" cy="35%" r="65%" gradientUnits="objectBoundingBox">
            <stop offset="0%" stopColor="#d97bff" />
            <stop offset="55%" stopColor="#a855f7" />
            <stop offset="100%" stopColor="#7c22d4" />
          </radialGradient>
        </defs>
      </svg>
    </Link>
  );
}

export function TopBar() {
  const { user } = useAuthState();
  const { logout } = useAuthActions();
  const navigate = useNavigate();
  const location = useLocation();
  const { data: libraries = [] } = useLibraries();
  const activeQuery =
    location.pathname === "/search" ? (new URLSearchParams(location.search).get("q") ?? "") : "";
  const [searchValue, setSearchValue] = useState(activeQuery);
  const activeLibraryId = location.pathname.startsWith("/library/")
    ? Number.parseInt(location.pathname.split("/")[2] ?? "", 10)
    : null;
  const isHomeRoute = location.pathname === "/";
  const isDiscoverRoute =
    location.pathname === "/discover" || location.pathname.startsWith("/discover/");
  const isDownloadsRoute =
    location.pathname === "/downloads" || location.pathname.startsWith("/downloads/");

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
    <header className="sticky top-0 z-40 shrink-0 border-b border-[rgba(181,123,255,0.1)] bg-[rgba(10,8,18,0.97)] shadow-[0_1px_0_rgba(181,123,255,0.06),0_12px_32px_rgba(0,0,0,0.5)] backdrop-blur-xl" style={{backdropFilter: "blur(24px) saturate(1.5)"}}>
      <div className="mx-auto flex w-full max-w-[var(--page-max-width)] flex-col gap-3 px-4 py-3 md:h-16 md:flex-row md:items-center md:gap-4 md:px-6 xl:px-8">
        <div className="flex items-center justify-between gap-3 md:justify-start">
          <PlumLogoButton />

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
              className="h-10 border-[rgba(181,123,255,0.12)] bg-[rgba(181,123,255,0.05)] pl-9 placeholder:text-[var(--plum-muted)] focus-visible:border-[rgba(181,123,255,0.32)] focus-visible:ring-0 focus-visible:shadow-[0_0_0_3px_rgba(139,92,246,0.15)]"
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
          <MobileNavLink to="/downloads" active={isDownloadsRoute}>
            Downloads
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
          ? "border-[rgba(255,255,255,0.18)] bg-[rgba(255,255,255,0.08)] text-[var(--plum-text)]"
          : "border-[var(--plum-border)] bg-transparent text-[var(--plum-muted)] hover:border-[rgba(255,255,255,0.14)] hover:text-[var(--plum-text)]",
      )}
    >
      {children}
    </Link>
  );
}

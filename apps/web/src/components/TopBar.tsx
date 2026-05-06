import { LibraryActivityCenter } from "@/components/LibraryActivityCenter";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { approveQuickConnectCode } from "@/api";
import { HorizontalScrollRail } from "@/components/ui/page";
import { useAuthActions, useAuthState } from "@/contexts/AuthContext";
import { cn } from "@/lib/utils";
import { useLibraries, useUnidentifiedLibrarySummaries } from "@/queries";
import { Link2, Search, Settings, User } from "lucide-react";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";

/** Plum logo icon — purple plum fruit on a dark button */
function PlumLogoButton() {
  return (
    <Link
      to="/"
      className="flex items-center justify-center shrink-0 rounded-lg transition-opacity hover:opacity-85 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring)"
      aria-label="Plum home"
    >
      <img
        src="/logo.svg"
        alt="Plum"
        width={42}
        height={42}
        className="block h-10 w-10 object-contain"
      />
    </Link>
  );
}

export function TopBar() {
  const { user } = useAuthState();
  const { logout } = useAuthActions();
  const navigate = useNavigate();
  const location = useLocation();
  const { data: libraries = [] } = useLibraries();
  const { data: unidentifiedData } = useUnidentifiedLibrarySummaries();
  const unidentifiedEntries = unidentifiedData?.libraries ?? [];
  const activeQuery =
    location.pathname === "/search" ? (new URLSearchParams(location.search).get("q") ?? "") : "";
  const [searchValue, setSearchValue] = useState(activeQuery);
  const activeLibraryId = location.pathname.startsWith("/library/")
    ? Number.parseInt(location.pathname.split("/")[2] ?? "", 10)
    : null;
  const unidentifiedOnlyMobile = new URLSearchParams(location.search).get("unidentified") === "1";
  const isHomeRoute = location.pathname === "/";
  const isDiscoverRoute =
    location.pathname === "/discover" || location.pathname.startsWith("/discover/");
  const isDownloadsRoute =
    location.pathname === "/downloads" || location.pathname.startsWith("/downloads/");
  const isSettingsRoute = location.pathname === "/settings";

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
    <header
      className="sticky top-0 z-40 shrink-0 border-b border-(--plum-chrome-border) bg-(--plum-topbar-bg) shadow-[0_12px_32px_rgba(0,0,0,0.35)] backdrop-blur-xl"
      style={{ backdropFilter: "blur(24px) saturate(1.5)" }}
    >
      <div className="mx-auto flex w-full max-w-(--page-max-width) items-center gap-3 px-4 py-3 md:h-16 md:gap-4 md:px-6 xl:px-8 flex-wrap">
        <div className="flex items-center justify-between gap-3 md:justify-start">
          <PlumLogoButton />

          <div className="flex items-center gap-2 md:hidden">
            <LibraryActivityCenter />
            <SettingsButton isActive={isSettingsRoute} />
            <UserMenu email={user?.email} onSignOut={logout} />
          </div>
        </div>

        <div className="flex flex-1 min-w-0 justify-center">
          <div className="relative flex-1 min-w-0 max-w-xl">
            <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-(--plum-muted)" />
            <Input
              type="search"
              value={searchValue}
              onChange={(event) => setSearchValue(event.target.value)}
              placeholder="Search libraries, shows, and movies"
              className="h-8 md:h-10 border-(--plum-chrome-border) bg-(--plum-field-fill) pl-8 md:pl-9 placeholder:text-(--plum-muted) focus-visible:border-(--plum-border-strong) focus-visible:ring-0 focus-visible:shadow-[0_0_0_3px_var(--plum-accent-subtle)]"
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
          {unidentifiedEntries.map((entry) => {
            const active =
              activeLibraryId === entry.library_id &&
              unidentifiedOnlyMobile &&
              !location.pathname.includes("/movie/") &&
              !location.pathname.includes("/show/");
            return (
              <MobileNavLink
                key={`unidentified-${entry.library_id}`}
                to={`/library/${entry.library_id}?unidentified=1`}
                active={active}
              >
                {entry.name} ({entry.count})
              </MobileNavLink>
            );
          })}
          {libraries.map((library) => {
            const isLibraryRoot = location.pathname === `/library/${library.id}`;
            const isLibrarySubroute = location.pathname.startsWith(`/library/${library.id}/`);
            const active =
              isLibrarySubroute ||
              (activeLibraryId === library.id && !(isLibraryRoot && unidentifiedOnlyMobile));
            return (
              <MobileNavLink key={library.id} to={`/library/${library.id}`} active={active}>
                {library.name}
              </MobileNavLink>
            );
          })}
        </HorizontalScrollRail>

        <div className="hidden items-center gap-2 md:flex">
          <LibraryActivityCenter />
          <SettingsButton isActive={isSettingsRoute} />
          <UserMenu email={user?.email} onSignOut={logout} />
        </div>
      </div>
    </header>
  );
}

function SettingsButton({ isActive }: { isActive: boolean }) {
  return (
    <Link to="/settings" aria-current={isActive ? "page" : undefined}>
      <Button
        variant="icon"
        size="icon"
        aria-label="Settings"
        className={cn(
          isActive &&
            "border-[rgba(255,255,255,0.18)] bg-[rgba(255,255,255,0.1)] text-(--plum-text) hover:border-[rgba(255,255,255,0.22)] hover:bg-[rgba(255,255,255,0.14)] hover:text-(--plum-text)",
        )}
      >
        <Settings className="size-5" />
      </Button>
    </Link>
  );
}

function UserMenu({ email, onSignOut }: { email?: string | null; onSignOut: () => void }) {
  const quickConnectInputRef = useRef<HTMLInputElement>(null);
  const [quickConnectOpen, setQuickConnectOpen] = useState(false);
  const [quickConnectCode, setQuickConnectCode] = useState("");
  const [quickConnectBusy, setQuickConnectBusy] = useState(false);
  const [quickConnectMessage, setQuickConnectMessage] = useState<string | null>(null);

  const normalizedQuickConnectCode = quickConnectCode
    .toUpperCase()
    .replace(/[^0-9A-Z]/g, "")
    .slice(0, 6);

  useEffect(() => {
    if (quickConnectOpen) {
      quickConnectInputRef.current?.focus();
    }
  }, [quickConnectOpen]);

  async function approveCode() {
    if (quickConnectBusy) return;
    setQuickConnectMessage(null);
    if (normalizedQuickConnectCode.length !== 6) {
      setQuickConnectMessage("Enter the 6-character code shown on the TV.");
      return;
    }
    setQuickConnectBusy(true);
    try {
      await approveQuickConnectCode({ code: normalizedQuickConnectCode });
      setQuickConnectMessage("TV connected.");
      window.setTimeout(() => {
        setQuickConnectOpen(false);
        setQuickConnectCode("");
        setQuickConnectMessage(null);
      }, 900);
    } catch (error) {
      setQuickConnectMessage(error instanceof Error ? error.message : "Quick connect failed.");
    } finally {
      setQuickConnectBusy(false);
    }
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="icon" size="icon" aria-label="Profile">
            <User className="size-5" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-56">
          {email && (
            <div className="truncate px-2 py-1.5 text-sm text-(--plum-muted)">{email}</div>
          )}
          <DropdownMenuItem
            onSelect={() => {
              setQuickConnectOpen(true);
              setQuickConnectMessage(null);
            }}
          >
            <Link2 className="mr-2 size-4" />
            Quick connect
          </DropdownMenuItem>
          <DropdownMenuItem
            onSelect={() => onSignOut()}
            className="text-(--plum-accent) focus:text-(--plum-accent)"
          >
            Sign out
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <Dialog open={quickConnectOpen} onOpenChange={setQuickConnectOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Quick connect</DialogTitle>
            <DialogDescription>
              Enter the 6-character code shown on the TV to connect it to this account.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-3">
            <Input
              ref={quickConnectInputRef}
              value={normalizedQuickConnectCode}
              onChange={(event) => setQuickConnectCode(event.target.value)}
              maxLength={6}
              inputMode="text"
              aria-label="TV quick connect code"
              className="h-12 font-mono text-lg font-semibold tracking-[0.25em] uppercase"
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  void approveCode();
                }
              }}
            />
            {quickConnectMessage ? (
              <p className="text-sm text-(--plum-muted)">{quickConnectMessage}</p>
            ) : null}
            <div className="flex justify-end gap-2">
              <Button
                type="button"
                variant="secondary"
                onClick={() => setQuickConnectOpen(false)}
                disabled={quickConnectBusy}
              >
                Cancel
              </Button>
              <Button type="button" onClick={() => void approveCode()} disabled={quickConnectBusy}>
                {quickConnectBusy ? "Connecting..." : "Connect TV"}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </>
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
          ? "border-[rgba(255,255,255,0.18)] bg-[rgba(255,255,255,0.08)] text-(--plum-text)"
          : "border-(--plum-border) bg-transparent text-(--plum-muted) hover:border-[rgba(255,255,255,0.14)] hover:text-(--plum-text)",
      )}
    >
      {children}
    </Link>
  );
}

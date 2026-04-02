import { useCallback, useEffect, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "./ui/dialog";
import { Button } from "./ui/button";
import { Input } from "./ui/input";

export interface IdentifySearchResult {
  Title: string;
  Overview: string;
  PosterURL: string;
  BackdropURL: string;
  ReleaseDate: string;
  VoteAverage: number;
  Provider: string;
  ExternalID: string;
}

export interface IdentifySearchDialogProps<Result extends IdentifySearchResult> {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  searchPlaceholder: string;
  initialQuery: string;
  search: (query: string) => Promise<readonly Result[]>;
  choose: (result: Result) => Promise<void>;
}

export function IdentifySearchDialog<Result extends IdentifySearchResult>({
  open,
  onOpenChange,
  title,
  description,
  searchPlaceholder,
  initialQuery,
  search,
  choose,
}: IdentifySearchDialogProps<Result>) {
  const [query, setQuery] = useState(initialQuery);
  const [results, setResults] = useState<readonly Result[]>([]);
  const [loading, setLoading] = useState(false);
  const [identifying, setIdentifying] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setQuery(initialQuery);
      setResults([]);
      setError(null);
      setIdentifying(null);
    }
  }, [initialQuery, open]);

  const doSearch = useCallback(async () => {
    const trimmed = query.trim();
    if (!trimmed) {
      setResults([]);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const data = await search(trimmed);
      setResults(data);
    } catch (searchError) {
      setError(searchError instanceof Error ? searchError.message : "Search failed");
      setResults([]);
    } finally {
      setLoading(false);
    }
  }, [query, search]);

  useEffect(() => {
    if (!open || !query.trim()) return;
    const timeout = setTimeout(doSearch, 300);
    return () => clearTimeout(timeout);
  }, [doSearch, open, query]);

  async function handleChoose(result: Result) {
    const key = `${result.Provider}:${result.ExternalID}`;
    setIdentifying(key);
    setError(null);
    try {
      await choose(result);
    } catch (chooseError) {
      setError(chooseError instanceof Error ? chooseError.message : "Identify failed");
    } finally {
      setIdentifying(null);
    }
  }

  const year = (result: Result) =>
    result.ReleaseDate ? new Date(result.ReleaseDate).getFullYear() : "";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent onClose={() => onOpenChange(false)}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <DialogDescription>{description}</DialogDescription>
        <div className="flex gap-2">
          <Input
            type="search"
            placeholder={searchPlaceholder}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={(event) => event.key === "Enter" && void doSearch()}
            className="flex-1"
          />
          <Button type="button" variant="outline" onClick={() => void doSearch()} disabled={loading}>
            {loading ? "Searching..." : "Search"}
          </Button>
        </div>
        {error ? (
          <p className="text-sm text-red-500" role="alert">
            {error}
          </p>
        ) : null}
        <div className="max-h-[60vh] overflow-y-auto space-y-2">
          {results.length === 0 && !loading && query.trim() && !error ? (
            <p className="text-sm text-(--plum-muted)">No results.</p>
          ) : null}
          {results.map((result) => {
            const key = `${result.Provider}:${result.ExternalID}`;
            return (
              <div
                key={key}
                className="flex items-center gap-3 rounded-md border border-(--plum-border) bg-(--plum-panel) p-2"
              >
                <img
                  src={result.PosterURL || "/placeholder-poster.svg"}
                  alt=""
                  className="h-18 w-12 rounded-sm object-cover"
                />
                <div className="min-w-0 flex-1">
                  <div className="truncate font-medium">{result.Title}</div>
                  {year(result) ? (
                    <div className="text-sm text-(--plum-muted)">{year(result)}</div>
                  ) : null}
                </div>
                <Button
                  type="button"
                  size="sm"
                  onClick={() => void handleChoose(result)}
                  disabled={identifying !== null}
                >
                  {identifying === key ? "Updating..." : "Choose"}
                </Button>
              </div>
            );
          })}
        </div>
      </DialogContent>
    </Dialog>
  );
}

import { Navigate, Outlet, useParams } from "react-router-dom";
import { DetailViewSkeleton } from "@/components/loading/PlumLoadingSkeletons";
import { useLibraries } from "@/queries";

/**
 * Ensures `/library/:libraryId/*` uses a real library id; replaces unknown or non-numeric ids
 * with the first library or the dashboard.
 */
export function LibrarySectionLayout() {
  const { libraryId: param } = useParams();
  const { data: libraries, isPending, isFetched } = useLibraries();
  const libs = libraries ?? [];

  const parsed = param ? parseInt(param, 10) : NaN;
  const idValid = Number.isFinite(parsed) && libs.some((l) => l.id === parsed);

  if (isPending && libs.length === 0) {
    return <DetailViewSkeleton />;
  }

  if (libs.length > 0) {
    if (!idValid) {
      return <Navigate to={`/library/${libs[0]!.id}`} replace />;
    }
    return <Outlet />;
  }

  if (isFetched && libs.length === 0) {
    return <Navigate to="/" replace />;
  }

  return <p className="text-sm text-(--plum-muted)">Loading…</p>;
}

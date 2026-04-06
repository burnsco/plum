import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { SCAN_SENSITIVE_STALE_MS } from "./queries";
import { Component, type ErrorInfo, type ReactNode, Suspense, lazy, useState } from "react";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import "./App.css";
import { MainLayout } from "./components/MainLayout";
import { AuthProvider, useAuthActions, useAuthState } from "./contexts/AuthContext";
import { IdentifyQueueProvider } from "./contexts/IdentifyQueueContext";
import { PlayerProvider } from "./contexts/PlayerContext";
import { ScanQueueProvider } from "./contexts/ScanQueueProvider";
import { WsProvider } from "./contexts/WsContext";

const Dashboard = lazy(() => import("./pages/Dashboard").then(m => ({ default: m.Dashboard })));
const Discover = lazy(() => import("./pages/Discover").then(m => ({ default: m.Discover })));
const DiscoverBrowse = lazy(() => import("./pages/DiscoverBrowse").then(m => ({ default: m.DiscoverBrowse })));
const DiscoverDetail = lazy(() => import("./pages/DiscoverDetail").then(m => ({ default: m.DiscoverDetail })));
const Downloads = lazy(() => import("./pages/Downloads").then(m => ({ default: m.Downloads })));
const Home = lazy(() => import("./pages/Home").then(m => ({ default: m.Home })));
const Login = lazy(() => import("./pages/Login").then(m => ({ default: m.Login })));
const MovieDetail = lazy(() => import("./pages/MovieDetail").then(m => ({ default: m.MovieDetail })));
const Onboarding = lazy(() => import("./pages/Onboarding").then(m => ({ default: m.Onboarding })));
const SearchPage = lazy(() => import("./pages/Search").then(m => ({ default: m.SearchPage })));
const Settings = lazy(() => import("./pages/Settings").then(m => ({ default: m.Settings })));
const ShowDetail = lazy(() => import("./pages/ShowDetail").then(m => ({ default: m.ShowDetail })));

class ErrorBoundary extends Component<
  { children: ReactNode },
  { error: Error | null }
> {
  state = { error: null };
  static getDerivedStateFromError(error: Error) {
    return { error };
  }
  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("Uncaught render error:", error, info.componentStack);
  }
  render() {
    if (this.state.error) {
      return (
        <main className="auth-screen">
          <div className="auth-card">
            <p style={{ fontWeight: 600, marginBottom: 8 }}>Something went wrong</p>
            <p className="auth-muted" style={{ marginBottom: 16, fontFamily: "monospace", fontSize: 12 }}>
              {(this.state.error as Error).message}
            </p>
            <button
              className="auth-button"
              onClick={() => window.location.reload()}
            >
              Reload
            </button>
          </div>
        </main>
      );
    }
    return this.props.children;
  }
}

function AppRouter() {
  const { hasAdmin, user, loading } = useAuthState();
  const { refreshSetupStatus } = useAuthActions();

  const handleGoToHome = () => {
    refreshSetupStatus().catch(() => {});
  };

  if (loading) {
    return (
      <main className="auth-screen">
        <div className="auth-card">
          <p className="auth-muted">Loading…</p>
        </div>
      </main>
    );
  }

  if (!hasAdmin) {
    return (
      <Suspense fallback={null}>
        <Onboarding onGoToHome={handleGoToHome} />
      </Suspense>
    );
  }

  if (!user) {
    return (
      <Suspense fallback={null}>
        <Login />
      </Suspense>
    );
  }

  return (
    <BrowserRouter>
      <WsProvider>
        <ScanQueueProvider>
          <IdentifyQueueProvider>
            <PlayerProvider>
              <Suspense fallback={null}>
                <Routes>
                  <Route path="/" element={<MainLayout />}>
                    <Route index element={<Dashboard />} />
                    <Route path="discover" element={<Discover />} />
                    <Route path="discover/browse" element={<DiscoverBrowse />} />
                    <Route path="discover/:mediaType/:tmdbId" element={<DiscoverDetail />} />
                    <Route path="downloads" element={<Downloads />} />
                    <Route path="search" element={<SearchPage />} />
                    <Route path="library/:libraryId" element={<Home />} />
                    <Route path="library/:libraryId/movie/:mediaId" element={<MovieDetail />} />
                    <Route path="library/:libraryId/show/:showKey" element={<ShowDetail />} />
                    <Route path="settings" element={<Settings />} />
                  </Route>
                </Routes>
              </Suspense>
            </PlayerProvider>
          </IdentifyQueueProvider>
        </ScanQueueProvider>
      </WsProvider>
    </BrowserRouter>
  );
}

function App() {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: SCAN_SENSITIVE_STALE_MS,
            retry: import.meta.env.MODE === "test" ? false : 3,
          },
        },
      }),
  );

  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <AppRouter />
        </AuthProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  );
}

export default App;

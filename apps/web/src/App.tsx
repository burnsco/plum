import { useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import { AuthProvider, useAuthActions, useAuthState } from "./contexts/AuthContext";
import { IdentifyQueueProvider } from "./contexts/IdentifyQueueContext";
import { PlayerProvider } from "./contexts/PlayerContext";
import { ScanQueueProvider } from "./contexts/ScanQueueProvider";
import { WsProvider } from "./contexts/WsContext";
import { MainLayout } from "./components/MainLayout";
import { Dashboard } from "./pages/Dashboard";
import { Discover } from "./pages/Discover";
import { DiscoverBrowse } from "./pages/DiscoverBrowse";
import { DiscoverDetail } from "./pages/DiscoverDetail";
import { Downloads } from "./pages/Downloads";
import { Home } from "./pages/Home";
import { Login } from "./pages/Login";
import { MovieDetail } from "./pages/MovieDetail";
import { Onboarding } from "./pages/Onboarding";
import { SearchPage } from "./pages/Search";
import { Settings } from "./pages/Settings";
import { ShowDetail } from "./pages/ShowDetail";
import "./App.css";

function AppRouter({ queryClient }: { queryClient: QueryClient }) {
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
    return <Onboarding onGoToHome={handleGoToHome} />;
  }

  if (!user) {
    return <Login />;
  }

  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <WsProvider>
          <ScanQueueProvider>
            <IdentifyQueueProvider>
              <PlayerProvider>
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
              </PlayerProvider>
            </IdentifyQueueProvider>
          </ScanQueueProvider>
        </WsProvider>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

function App() {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 60_000,
            retry: import.meta.env.MODE === "test" ? false : 3,
          },
        },
      }),
  );

  return (
    <AuthProvider>
      <AppRouter queryClient={queryClient} />
    </AuthProvider>
  );
}

export default App;

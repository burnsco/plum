import { AuthProvider, useAuthActions, useAuthState } from './contexts/AuthContext'
import { Home } from './pages/Home'
import { Login } from './pages/Login'
import { Onboarding } from './pages/Onboarding'
import './App.css'

function AppRouter() {
  const { hasAdmin, user, loading } = useAuthState()
  const { refreshSetupStatus } = useAuthActions()

  const handleGoToHome = () => {
    refreshSetupStatus().catch(() => {})
  }

  if (loading) {
    return (
      <div className="auth-screen">
        <div className="auth-card">
          <p className="auth-muted">Loading…</p>
        </div>
      </div>
    )
  }

  if (!hasAdmin) {
    return <Onboarding onGoToHome={handleGoToHome} />
  }

  if (!user) {
    return <Login />
  }

  return <Home />
}

function App() {
  return (
    <AuthProvider>
      <AppRouter />
    </AuthProvider>
  )
}

export default App

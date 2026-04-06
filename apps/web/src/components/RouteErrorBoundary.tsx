import { Component, type ErrorInfo, type ReactNode } from "react";
import { Link } from "react-router-dom";

type Props = { children: ReactNode };
type State = { error: Error | null };

/**
 * Isolates render failures in main content so the shell (nav, player) stays mounted.
 */
export class RouteErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error("Route render error:", error, info.componentStack);
  }

  render(): ReactNode {
    if (this.state.error) {
      return (
        <div className="rounded-lg border border-(--plum-border) bg-(--plum-card-bg) p-6">
          <p style={{ fontWeight: 600, marginBottom: 8 }}>This page could not be displayed</p>
          <p className="auth-muted" style={{ marginBottom: 16, fontFamily: "monospace", fontSize: 12 }}>
            {(this.state.error as Error).message}
          </p>
          <Link to="/" className="link-button">
            Back home
          </Link>
        </div>
      );
    }
    return this.props.children;
  }
}

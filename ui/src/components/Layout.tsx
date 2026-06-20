import { Link } from "react-router-dom";
import type { ReactNode } from "react";
import { useAuth } from "../lib/auth";

// Layout is the app shell: sticky header + centered main. Mirrors the old
// base.html, with "Sign out" clearing the local token instead of POST /logout.
export function Layout({ children }: { children: ReactNode }) {
  const { logout } = useAuth();
  return (
    <div className="min-h-full">
      <header className="sticky top-0 z-20 border-b border-line bg-ink/85 backdrop-blur">
        <div className="mx-auto flex max-w-5xl items-center justify-between px-5 py-3">
          <Link to="/" className="group flex items-center gap-2.5">
            <span className="grid h-6 w-6 place-items-center rounded-md bg-go/15 ring-1 ring-go/40">
              <span className="h-2 w-2 rounded-[2px] bg-go shadow-[0_0_8px_var(--color-go)]" />
            </span>
            <span className="font-display text-lg font-semibold tracking-tight">edp</span>
            <span className="hidden font-mono text-[11px] text-faint sm:inline">easy deploy platform</span>
          </Link>
          <nav className="flex items-center gap-1 text-sm">
            <Link to="/" className="rounded-md px-3 py-1.5 text-dim transition-colors hover:bg-raised hover:text-fg">
              Environments
            </Link>
            <Link to="/env/new" className="btn btn-sm ml-1">
              New environment
            </Link>
            <button onClick={logout} className="btn-ghost ml-1 rounded-md px-3 py-1.5 text-sm">
              Sign out
            </button>
          </nav>
        </div>
      </header>
      <main className="mx-auto max-w-5xl px-5 py-8">{children}</main>
    </div>
  );
}

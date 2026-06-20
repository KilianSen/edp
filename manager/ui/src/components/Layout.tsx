import type { ReactNode } from "react";
import { Link, NavLink } from "react-router-dom";
import { useAuth } from "../lib/auth";

export default function Layout({ children }: { children: ReactNode }) {
  const { logout } = useAuth();
  const link = "px-3 py-1.5 rounded-md text-sm";
  const active = "bg-[#1b2236] text-[#2dd4bf]";
  const idle = "text-[#8595b6] hover:text-[#e7ebf4]";

  return (
    <div className="min-h-screen">
      <header className="flex items-center gap-2 border-b border-[#262e42] px-5 py-3">
        <Link to="/" className="mr-3 font-semibold tracking-tight">
          edp-manager
        </Link>
        <nav className="flex gap-1">
          <NavLink to="/" end className={({ isActive }) => `${link} ${isActive ? active : idle}`}>
            Environments
          </NavLink>
        </nav>
        {/* instance registry is configuration, not the organizing principle */}
        <NavLink to="/instances" className={({ isActive }) => `ml-auto ${link} ${isActive ? active : idle}`}>
          Instances
        </NavLink>
        <button
          onClick={logout}
          className="rounded-md border border-[#33405d] px-3 py-1.5 text-sm text-[#8595b6] hover:text-[#e7ebf4]"
        >
          Log out
        </button>
      </header>
      <main className="mx-auto max-w-5xl px-5 py-6">{children}</main>
    </div>
  );
}

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { api, clearToken, getToken, setToken, setUnauthorizedHandler } from "./api";

interface AuthCtx {
  authed: boolean;
  login: (password: string) => Promise<boolean>; // returns first_run
  logout: () => void;
}

const Ctx = createContext<AuthCtx | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [authed, setAuthed] = useState<boolean>(!!getToken());

  const logout = useCallback(() => {
    clearToken();
    setAuthed(false);
  }, []);

  const login = useCallback(async (password: string) => {
    const { token, first_run } = await api.login(password);
    setToken(token);
    setAuthed(true);
    return first_run;
  }, []);

  // A 401 from any request (e.g. token rotated/invalidated) bounces to login.
  useEffect(() => {
    setUnauthorizedHandler(logout);
  }, [logout]);

  const value = useMemo(() => ({ authed, login, logout }), [authed, login, logout]);
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useAuth(): AuthCtx {
  const v = useContext(Ctx);
  if (!v) throw new Error("useAuth outside AuthProvider");
  return v;
}

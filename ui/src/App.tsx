import { Route, Routes } from "react-router-dom";
import { useAuth } from "./lib/auth";
import { Layout } from "./components/Layout";
import { Login } from "./pages/Login";
import { Dashboard } from "./pages/Dashboard";
import { EnvForm } from "./pages/EnvForm";
import { EnvDetail } from "./pages/EnvDetail";
import { HookForm } from "./pages/HookForm";
import { HookDetail } from "./pages/HookDetail";
import { Import } from "./pages/Import";

export function App() {
  const { authed } = useAuth();
  if (!authed) return <Login />;

  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/env/new" element={<EnvForm />} />
        <Route path="/env/import" element={<Import />} />
        <Route path="/env/:id" element={<EnvDetail />} />
        <Route path="/env/:id/edit" element={<EnvForm />} />
        <Route path="/env/:envId/hooks/new" element={<HookForm />} />
        <Route path="/timed-hooks/:id" element={<HookDetail />} />
        <Route path="/timed-hooks/:hookId/edit" element={<HookForm />} />
        <Route path="*" element={<Dashboard />} />
      </Routes>
    </Layout>
  );
}

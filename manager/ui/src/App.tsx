import { Navigate, Route, Routes } from "react-router-dom";
import { useAuth } from "./lib/auth";
import Layout from "./components/Layout";
import Login from "./pages/Login";
import Dashboard from "./pages/Dashboard";
import EnvDetail from "./pages/EnvDetail";
import EnvForm from "./pages/EnvForm";
import Instances from "./pages/Instances";

export default function App() {
  const { authed } = useAuth();

  if (!authed) {
    return (
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    );
  }

  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/env/new" element={<EnvForm />} />
        <Route path="/i/:instanceId/env/:envId/edit" element={<EnvForm />} />
        <Route path="/i/:instanceId/env/:envId" element={<EnvDetail />} />
        <Route path="/instances" element={<Instances />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Layout>
  );
}

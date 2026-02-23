import { Routes, Route } from "react-router";
import { Layout } from "@/components/layout/Layout";
import { DashboardPage } from "@/pages/DashboardPage";

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<DashboardPage />} />
        {/* Phase 2: AttackPathPage, TaintPathsPage, RepositoriesPage */}
      </Route>
    </Routes>
  );
}

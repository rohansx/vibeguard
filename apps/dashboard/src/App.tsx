import { Routes, Route } from "react-router";
import { Layout } from "@/components/layout/Layout";
import { DashboardPage } from "@/pages/DashboardPage";
import { RulesPage } from "@/pages/RulesPage";
import { DocumentsPage } from "@/pages/DocumentsPage";
import { ScansPage } from "@/pages/ScansPage";

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<DashboardPage />} />
        <Route path="/rules" element={<RulesPage />} />
        <Route path="/documents" element={<DocumentsPage />} />
        <Route path="/scans" element={<ScansPage />} />
      </Route>
    </Routes>
  );
}

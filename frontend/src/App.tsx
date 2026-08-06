import { useEffect } from "react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { Toaster } from "sonner";

import { EventsOn } from "./wailsjs/runtime/runtime";
import DashboardPage from "./pages/DashboardPage";
import JobDetailPage from "./pages/JobDetailPage";

// 桌面 WebView 无 URL 栏，必须用 MemoryRouter
export default function App() {
  const qc = useQueryClient();

  // 事件驱动刷新：后端在抓取完成 / 状态变更后 emit jobs:changed
  useEffect(() => {
    const off = EventsOn("jobs:changed", () => {
      qc.invalidateQueries();
    });
    return () => off();
  }, [qc]);

  return (
    <MemoryRouter>
      <Routes>
        <Route path="/" element={<DashboardPage />} />
        <Route path="/jobs/:id" element={<JobDetailPage />} />
      </Routes>
      <Toaster position="bottom-right" richColors />
    </MemoryRouter>
  );
}

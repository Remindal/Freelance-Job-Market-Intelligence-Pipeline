import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  // 桌面 WebView 以非 http 协议加载产物，资源必须用相对路径
  base: "./",
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});

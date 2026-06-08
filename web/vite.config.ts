import path from "node:path";
import { createHash } from "node:crypto";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const webRoot = path.dirname(fileURLToPath(import.meta.url));
const appVersion = process.env.VITE_APP_VERSION || process.env.npm_package_version || "0.0.0-dev";
const buildFileSuffix = appVersion.replace(/[^a-zA-Z0-9]/g, "") || "dev";

function hashName(value: string) {
  return createHash("sha256").update(value).digest("hex").slice(0, 12);
}

function chunkKey(chunk: { name?: string; facadeModuleId?: string | null; moduleIds?: string[] }) {
  const modules = [...(chunk.moduleIds || [])].sort().join("|");
  return modules || chunk.facadeModuleId || chunk.name || "chunk";
}

export default defineConfig({
  plugins: [react()],
  define: {
    __APP_VERSION__: JSON.stringify(appVersion),
  },
  resolve: {
    alias: {
      "@": path.resolve(webRoot, "src"),
    },
  },
  server: {
    host: "0.0.0.0",
  },
  build: {
    outDir: "../internal/web/dist",
    emptyOutDir: true,
    rollupOptions: {
      output: {
        codeSplitting: false,
        entryFileNames: (chunk) => `assets/e${buildFileSuffix}${hashName(chunkKey(chunk))}.js`,
        chunkFileNames: (chunk) => `assets/c${buildFileSuffix}${hashName(chunkKey(chunk))}.js`,
        assetFileNames: (asset) => {
          const sourceNames = [
            ...("names" in asset && Array.isArray(asset.names) ? asset.names : []),
            ...("originalFileNames" in asset && Array.isArray(asset.originalFileNames) ? asset.originalFileNames : []),
          ].join("|");
          return `assets/a${buildFileSuffix}${hashName(sourceNames || asset.name || "asset")}[extname]`;
        },
      },
    },
  },
});

/// <reference types="vite/client" />

interface Window {
  EDP_UI_CONFIG?: { apiBase?: string };
}

interface ImportMetaEnv {
  readonly VITE_EDP_API?: string;
}
interface ImportMeta {
  readonly env: ImportMetaEnv;
}

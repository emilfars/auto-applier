/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Extension id the web app may message via externally_connectable (demo). */
  readonly VITE_EXTENSION_ID?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}

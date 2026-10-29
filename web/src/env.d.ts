interface ImportMetaEnv {
  readonly VITE_AWS_REGION: string;
  readonly VITE_IDENTITY_POOL_ID: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}

function required(name: keyof ImportMetaEnv): string {
  const value = import.meta.env[name];
  if (!value) {
    throw new Error(
      `${name} is not set. Copy web/.env.example to web/.env and fill it in from ` +
        "`terraform -chdir=infra output -raw env_file`, then restart the dev server.",
    );
  }
  return value;
}

export const config = {
  awsRegion: required("VITE_AWS_REGION"),
  identityPoolId: required("VITE_IDENTITY_POOL_ID"),
};

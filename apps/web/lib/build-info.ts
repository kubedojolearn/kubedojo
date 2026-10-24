/** Build metadata injected at image build time (see apps/web/Dockerfile). Never contains secrets. */
export function getBuildInfo() {
  return {
    version: process.env.KUBEDOJO_VERSION ?? "dev",
    commit: process.env.KUBEDOJO_COMMIT ?? "unknown",
  };
}

package kubectl

// Manifests from files (create -f, apply -f, delete -f) read the lab editor's virtual files,
// which arrive with Part 13. Until then these commands say so instead of pretending to work.

const filesNotYet = "error: -f reads manifests from the lab's YAML editor, which isn't available yet. For now, use kubectl create deployment|namespace|configmap|secret|service, kubectl run and kubectl expose."

func createFromFile(_ flags, _ Env, _ *Parsed, _ []string) (Result, *Failure) {
	return Result{}, reject(CodeUnsupported, filesNotYet)
}

func deleteFromFile(_ flags, _ Env, _ *Parsed, _ []string) (Result, *Failure) {
	return Result{}, reject(CodeUnsupported, filesNotYet)
}

func runApply(_ []string, _ Env, _ *Parsed) (Result, *Failure) {
	return Result{}, reject(CodeUnsupported, filesNotYet)
}

export default function Home() {
  return (
    <main className="mx-auto flex w-full max-w-2xl flex-1 flex-col justify-center gap-4 px-6 py-24">
      <h1 className="text-4xl font-semibold tracking-tight">KubeDojo</h1>
      <p className="text-lg">Master Kubernetes, one command at a time.</p>
      <p className="text-sm opacity-80">
        Development build. The learning experience is under construction; see the project README for the roadmap.
      </p>
    </main>
  );
}

import type { Me } from "../types";

export function LoginScreen({ me, title }: { me: Me; title: string }) {
  return (
    <main className="flex min-h-screen items-center justify-center px-4">
      <div className="rise w-full max-w-xs text-center">
        <img src="/favicon.svg" alt="" width={48} height={48} className="mx-auto rounded-xl" />
        <h1 className="mt-4 text-xl font-semibold text-ink">{title}</h1>
        <p className="mt-1 text-[13px] text-ink-2">Sign in to open the dashboard.</p>
        <div className="mt-6 space-y-2">
          {me.providers.map((provider) => (
            <a
              key={provider.key}
              href={`/login/${encodeURIComponent(provider.key)}?redirect=%2F`}
              className="block rounded-lg bg-accent px-4 py-2.5 text-[14px] font-medium text-accent-ink transition-opacity hover:opacity-90"
            >
              Sign in with {provider.key}
            </a>
          ))}
          {me.providers.length === 0 && (
            <p className="text-[13px] text-down">No sign-in providers are configured on the server.</p>
          )}
        </div>
      </div>
    </main>
  );
}

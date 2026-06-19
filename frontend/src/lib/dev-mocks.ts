/** Explicit opt-in for runtime mock fallback; disabled in production and demo paths. */
export function allowsMockFallback(): boolean {
  return (
    import.meta.env.DEV && import.meta.env.VITE_ALLOW_MOCK_FALLBACK === "true"
  );
}

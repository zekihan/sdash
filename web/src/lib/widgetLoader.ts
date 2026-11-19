export function createWidgetLoader<T>(request: () => Promise<T>, apply: (data: T) => void): () => Promise<void> {
  let latestRequest = 0;
  let latestApplied = 0;

  return async () => {
    const sequence = ++latestRequest;
    try {
      const data = await request();
      if (sequence > latestApplied) {
        latestApplied = sequence;
        apply(data);
      }
    } catch {
      // Preserve the last successful snapshot on request failure.
    }
  };
}

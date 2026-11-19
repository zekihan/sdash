/**
 * Subsequence fuzzy scorer: query characters must appear in order.
 * Bonuses for prefix and word-boundary matches, penalty for gaps.
 * Returns a nonnegative score for a match and -1 otherwise.
 */
export function fuzzyScore(query: string, candidate: string): number {
  const q = query.toLowerCase();
  const c = candidate.toLowerCase();
  if (!q) return 0;
  if (c.startsWith(q)) return Math.max(0, 1000 - c.length);

  let qi = 0;
  let score = 0;
  let lastMatch = -2;

  for (let ci = 0; ci < c.length && qi < q.length; ci++) {
    if (c[ci] !== q[qi]) continue;
    score += 10;
    if (ci === lastMatch + 1) score += 6;
    const prev = c[ci - 1];
    if (ci === 0 || prev === " " || prev === "-" || prev === "_" || prev === "/") score += 8;
    lastMatch = ci;
    qi++;
  }

  if (qi < q.length) return -1;
  return Math.max(0, score - (c.length - q.length) * 0.2);
}

/** Best score across several fields; -1 when nothing matches. */
export function fuzzyAny(query: string, fields: (string | undefined)[]): number {
  let best = -1;
  for (const field of fields) {
    if (!field) continue;
    const score = fuzzyScore(query, field);
    if (score > best) best = score;
  }
  return best;
}

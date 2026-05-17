const uniqueTags = (items: string[]): string[] => {
  const seen = new Set<string>();
  const out: string[] = [];
  items.forEach((item) => {
    const tag = item.trim();
    if (!tag || seen.has(tag)) return;
    seen.add(tag);
    out.push(tag);
  });
  return out;
};

export const normalizeTags = (tags: string[]): string[] => uniqueTags(tags);

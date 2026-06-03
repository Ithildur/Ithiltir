export const toggleId = (ids: number[], id: number): number[] =>
  ids.includes(id) ? ids.filter((item) => item !== id) : [...ids, id];

export const toggleVisibleIds = (selectedIds: number[], visibleIds: number[]): number[] => {
  const visible = [...new Set(visibleIds)];
  if (visible.length === 0) return selectedIds;

  const selected = new Set(selectedIds);
  const allVisibleSelected = visible.every((id) => selected.has(id));
  for (const id of visible) {
    if (allVisibleSelected) selected.delete(id);
    else selected.add(id);
  }
  return Array.from(selected);
};

export const pruneIds = (selectedIds: number[], validIds: Iterable<number>): number[] => {
  const valid = new Set(validIds);
  const next: number[] = [];
  let changed = false;
  for (const id of selectedIds) {
    if (valid.has(id)) next.push(id);
    else changed = true;
  }
  return changed ? next : selectedIds;
};

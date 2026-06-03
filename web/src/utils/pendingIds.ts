export const pendingIds = (ids: number[], id: number, pending: boolean): number[] => {
  if (pending) return ids.includes(id) ? ids : [...ids, id];
  return ids.filter((item) => item !== id);
};

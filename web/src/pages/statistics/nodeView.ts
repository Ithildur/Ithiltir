import type { NodeView } from '@app-types/frontMetrics';
import { fetchFrontMetrics } from '@lib/frontApi';

export const loadStatisticsNodeView = async (
  serverId: number,
  signal?: AbortSignal,
): Promise<NodeView | null> => {
  const nodes = await fetchFrontMetrics({ signal });
  return nodes.find((node) => Number(node.node.id) === serverId) ?? null;
};

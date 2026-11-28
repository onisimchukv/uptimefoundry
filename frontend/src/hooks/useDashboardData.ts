'use client';

import { useQuery } from '@tanstack/react-query';
import { api } from '../lib/api';

export function useDashboardData(refetchInterval?: number) {
  return useQuery({
    queryKey: ['dashboard'],
    queryFn: async () => {
      const [topology, metrics, incidents, investigations] = await Promise.all([
        api.getTopology({ format: 'graph', depth: 2 }),
        api.getMetrics({
          service: '*',
          metrics: ['cpu_usage', 'memory_usage', 'disk_usage'],
          startTime: new Date(Date.now() - 3600000).toISOString(),
          endTime: new Date().toISOString(),
          resolution: '1m',
        }),
        api.getIncidents({ limit: 10 }),
        api.getInvestigations(),
      ]);
      return { topology, metrics, incidents, investigations };
    },
    refetchInterval: refetchInterval || false,
  });
}

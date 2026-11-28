'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../lib/api';
import type { Metric, LogEntry } from '../types';

export function useMetrics(refetchInterval?: number) {
  return useQuery<{ data: Metric[]; error: Error | null }, Error>({
    queryKey: ['metrics', { limit: 10 }],
    queryFn: async () => {
      const response = await api.getMetrics({
        metrics: ['system.cpu_usage', 'system.memory_usage'],
        startTime: new Date(Date.now() - 3600000).toISOString(),
        endTime: new Date().toISOString(),
        resolution: '1m',
      });
      return { data: response.data?.series.map(s => ({
        name: s.metric,
        value: s.values[s.values.length - 1][1],
        unit: 'percent',
        timestamp: new Date(s.values[s.values.length - 1][0]).toISOString(),
        labels: s.labels,
      })) || [], error: null };
    },
    refetchInterval: refetchInterval || false,
  });
}

export function useLogs() {
  return useQuery<{ data: LogEntry[]; error: Error | null }, Error>({
    queryKey: ['logs', { limit: 100 }],
    queryFn: async () => {
      const response = await api.getLogs({
        service: undefined,
        limit: 100,
        startTime: new Date(Date.now() - 3600000).toISOString(),
        endTime: new Date().toISOString(),
      });
      return { data: response.data?.entries || [], error: null };
    },
  });
}

export function useMetricsUpdate() {
  const queryClient = useQueryClient();
  
  return {
    updateMetrics: async (metrics: Metric[]) => {
      // This is a no-op since there's no updateMetrics API
      queryClient.setQueryData(['metrics'], { data: metrics, error: null });
    },
  };
}

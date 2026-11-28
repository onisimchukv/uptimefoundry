'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../lib/api';
import type { Topology } from '../types';

export function useTopology(refetchInterval?: number) {
  return useQuery<{ data: Topology; error: Error | null }, Error>({
    queryKey: ['topology'],
    queryFn: async () => {
      const response = await api.getTopology({ 
        format: 'tree', 
        depth: 2 
      });
      // Map TopologyResponse to Topology format
      return {
        data: {
          nodes: response.data?.nodes || [],
          edges: response.data?.edges || [],
          metadata: response.data?.metadata || {
            clusterId: '',
            lastUpdated: new Date().toISOString(),
            version: '1.0.0'
          }
        },
        error: null
      };
    },
    refetchInterval: refetchInterval || false,
  });
}

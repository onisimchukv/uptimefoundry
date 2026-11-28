'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../lib/api';
import type { Investigation } from '../types';

export function useInvestigations(refetchInterval?: number) {
  return useQuery<{ data: Investigation[]; error: Error | null }, Error>({
    queryKey: ['investigations'],
    queryFn: async () => {
      const response = await api.getInvestigations({ limit: 10 });
      return { data: response.data?.investigations || [], error: null };
    },
    refetchInterval: refetchInterval || false,
  });
}

export function useInvestigation(id: string | undefined) {
  return useQuery<{ data: Investigation | undefined; error: Error | null }, Error>({
    queryKey: ['investigation', id],
    queryFn: async () => {
      const response = await api.getInvestigation(id!);
      return { data: response.data?.result ? (response.data.result as Investigation) : undefined, error: null };
    },
    enabled: !!id,
  });
}

export function useInvestigationUpdate() {
  const queryClient = useQueryClient();
  
  return {
    startInvestigation: async (incidentId: string) => {
      return await api.triggerInvestigation({ incidentId });
    },
  };
}

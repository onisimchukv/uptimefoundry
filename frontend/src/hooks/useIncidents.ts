'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../lib/api';
import type { Incident } from '../types';

export function useIncidents(refetchInterval?: number) {
  return useQuery<{ data: Incident[]; error: Error | null }, Error>({
    queryKey: ['incidents'],
    queryFn: async () => {
      const response = await api.getIncidents({ limit: 10 });
      return { data: response.data?.incidents || [], error: null };
    },
    refetchInterval: refetchInterval || false,
  });
}

export function useIncident(id: string | undefined) {
  const { data: incidents } = useIncidents();
  
  return useQuery<{ data: Incident | undefined; error: Error | null }, Error>({
    queryKey: ['incident', id],
    queryFn: () => {
      const incident = incidents?.data?.find(i => i.id === id);
      return { data: incident, error: null };
    },
    enabled: !!id,
  });
}

export function useIncidentUpdate() {
  const queryClient = useQueryClient();
  
  return {
    updateIncident: async (incident: Incident) => {
      await api.updateIncident(incident.id, incident);
      const current = queryClient.getQueryData<{ data: Incident[]; error: Error | null }>(['incidents']);
      queryClient.setQueryData(['incidents'], { data: current?.data || [], error: null });
    },
    
    deleteIncident: async (id: string) => {
      await api.deleteIncident(id);
      const current = queryClient.getQueryData<{ data: Incident[]; error: Error | null }>(['incidents']);
      queryClient.setQueryData(['incidents'], {
        data: current?.data?.filter(i => i.id !== id) || [],
        error: null,
      });
    },
  };
}

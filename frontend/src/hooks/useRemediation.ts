import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getRemediation, executeRemediation, cancelRemediation } from '../lib/api';
import type { RemediationAction } from '../types';

export function useRemediation(remediationId: string) {
  const queryClient = useQueryClient();

  const { data, isLoading, error } = useQuery<RemediationAction | undefined, Error>({
    queryKey: ['remediation', remediationId],
    queryFn: async () => {
      const response = await getRemediation(remediationId);
      return response.data;
    },
  });

  const executeMutation = useMutation({
    mutationFn: ({
      remediationId,
      targetId,
      mode,
    }: {
      remediationId: string;
      targetId?: string;
      mode: 'single' | 'all';
    }) => executeRemediation(remediationId, targetId, mode),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['remediation', remediationId] });
    },
  });

  const cancelMutation = useMutation({
    mutationFn: (remediationId: string) => cancelRemediation(remediationId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['remediation', remediationId] });
    },
  });

  return {
    remediation: data,
    isLoading,
    error,
    execute: executeMutation.mutate,
    executing: executeMutation.isPending,
    executeError: executeMutation.error,
    cancel: cancelMutation.mutate,
    cancelling: cancelMutation.isPending,
    cancelError: cancelMutation.error,
  };
}

export function useRemediationList() {
  const queryClient = useQueryClient();

  const { data, isLoading, error } = useQuery<RemediationAction[] | undefined, Error>({
    queryKey: ['remediations'],
    queryFn: async () => {
      // This would be implemented as a list endpoint in the future
      // For now, return an empty array
      return [];
    },
  });

  return {
    remediations: data || [],
    isLoading,
    error,
  };
}

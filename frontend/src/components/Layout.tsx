import React from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { api } from '../lib/api';
import { cn } from '../utils/cn';
import type { Incident } from '../types';

interface NavItem {
  path: string;
  label: string;
  icon?: React.ReactNode;
}

interface LayoutProps {
  children: React.ReactNode;
  sidebarItems?: NavItem[];
  user?: {
    name: string;
    email: string;
    role: string;
  };
}

export const Layout: React.FC<LayoutProps> = ({
  children,
  sidebarItems = [
    { path: '/dashboard', label: 'Dashboard', icon: '📊' },
    { path: '/topology', label: 'Topology', icon: '🕸️' },
    { path: '/incidents', label: 'Incidents', icon: '🚨' },
    { path: '/investigations', label: 'Investigations', icon: '🤖' },
    { path: '/settings', label: 'Settings', icon: '⚙️' },
  ],
  user,
}) => {
  const pathname = usePathname();

  // Fetch dashboard data using React Query
  const { data: dashboardData, isLoading } = useQuery({
    queryKey: ['dashboard'],
    queryFn: async () => {
      const data = await api.getDashboard();
      return data;
    },
    staleTime: 3000,
    refetchInterval: 3000,
  });

  const topology = dashboardData?.data?.topology;
  const incidents = dashboardData?.data?.incidents;

  return (
    <div className="flex flex-col h-screen font-sans bg-gray-50">
      {/* Sidebar */}
      <aside className="w-64 bg-slate-900 text-white border-r border-gray-700 flex flex-col">
        <div className="border-b border-gray-700 p-4">
          <h1 className="text-2xl font-bold text-sky-400">UptimeFoundry</h1>
        </div>

        <nav className="flex-1 overflow-y-auto p-4">
          {sidebarItems.map((item) => {
            const isActive = pathname === item.path;
            return (
              <Link
                key={item.path}
                href={item.path}
                className={`flex items-center gap-3 px-3 py-3 text-sm rounded-lg transition-colors ${
                  isActive
                    ? 'bg-sky-700 text-white border-l-4 border-sky-400'
                    : 'text-gray-400 hover:bg-slate-700 hover:text-white'
                }`}
              >
                <span className="text-xl">{item.icon}</span>
                <span>{item.label}</span>
              </Link>
            );
          })}
        </nav>

        {user && (
          <div className="border-t border-gray-700 p-4 flex items-center gap-3">
            <div className="w-10 h-10 bg-sky-700 rounded-full flex items-center justify-center text-white font-semibold">
              {user.name.charAt(0).toUpperCase()}
            </div>
            <div className="flex-1 min-w-0">
              <span className="block font-semibold text-sm truncate">{user.name}</span>
              <span className="block text-xs text-gray-400 truncate">{user.email}</span>
              <span className="inline-block mt-1 bg-sky-700 text-xs px-2 py-0.5 rounded-full text-white uppercase">
                {user.role}
              </span>
            </div>
          </div>
        )}
      </aside>

      {/* Main Content */}
      <main className="flex-1 flex flex-col bg-gray-50">
        <header className="bg-white border-b border-gray-200 p-3 flex items-center justify-between">
          <div>
            <h2 className="text-xl font-semibold text-slate-900">UptimeFoundry Console</h2>
          </div>
          <div className="flex items-center gap-4">
            <div className="relative">
              <select className="px-3 py-2 border border-gray-300 rounded-lg text-sm bg-white cursor-pointer">
                <option value="cluster-1">Production Cluster 1</option>
                <option value="cluster-2">Production Cluster 2</option>
                <option value="staging">Staging</option>
              </select>
            </div>
            <div className="flex items-center gap-2 text-xs text-gray-600">
              <span className="w-2 h-2 bg-green-500 rounded-full"></span>
              <span>Connected</span>
            </div>
          </div>
        </header>

        <div className="flex-1 overflow-auto p-6">
          {children}
        </div>
      </main>

      {/* Status Bar */}
      <footer className="bg-slate-900 text-gray-400 p-2 flex gap-6 text-xs border-t border-gray-700">
        <span className="flex items-center gap-2">
          <span>🔄</span>
          <span>Last update: {isLoading ? 'Loading...' : 'Just now'}</span>
        </span>
        <span className="flex items-center gap-2">
          <span>🖥️</span>
          <span>Nodes: {topology?.nodes?.length || 0}</span>
        </span>
        <span className="flex items-center gap-2">
          <span>⚠️</span>
          <span>Incidents: {(incidents?.recent?.filter((i: Incident) => i.status === 'open' || i.status === 'investigating').length || 0)}</span>
        </span>
      </footer>
    </div>
  );
};

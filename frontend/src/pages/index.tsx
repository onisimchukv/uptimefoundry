'use client';

import React, { useState } from 'react';
import { QueryProvider } from '../components/providers/QueryProvider';
import { Layout } from '../components/Layout';
import { TopologyGraph } from '../components/TopologyGraph';
import { MonitoringDashboard } from '../components/MonitoringDashboard';
import { IncidentManagement } from '../components/IncidentManagement';
import { AIInvestigationViewer } from '../components/AIInvestigationViewer';
import { RemediationControl } from '../components/RemediationControl';

export default function Home() {
  const [activeTab, setActiveTab] = useState<'topology' | 'monitoring' | 'incidents' | 'investigations'>('topology');

  return (
    <QueryProvider>
      <Layout>
        <div className="flex h-screen">
          {/* Sidebar */}
          <aside className="w-64 bg-gray-900 text-white flex flex-col">
            <div className="p-4 border-b border-gray-700">
              <h1 className="text-xl font-bold">AIFoundry Console</h1>
            </div>
            <nav className="flex-1 p-4">
              <button
                onClick={() => setActiveTab('topology')}
                className={`w-full text-left px-4 py-2 rounded mb-2 ${
                  activeTab === 'topology' ? 'bg-blue-600' : 'hover:bg-gray-800'
                }`}
              >
                Topology
              </button>
              <button
                onClick={() => setActiveTab('monitoring')}
                className={`w-full text-left px-4 py-2 rounded mb-2 ${
                  activeTab === 'monitoring' ? 'bg-blue-600' : 'hover:bg-gray-800'
                }`}
              >
                Monitoring
              </button>
              <button
                onClick={() => setActiveTab('incidents')}
                className={`w-full text-left px-4 py-2 rounded mb-2 ${
                  activeTab === 'incidents' ? 'bg-blue-600' : 'hover:bg-gray-800'
                }`}
              >
                Incidents
              </button>
              <button
                onClick={() => setActiveTab('investigations')}
                className={`w-full text-left px-4 py-2 rounded mb-2 ${
                  activeTab === 'investigations' ? 'bg-blue-600' : 'hover:bg-gray-800'
                }`}
              >
                Investigations
              </button>
            </nav>
          </aside>

          {/* Main Content */}
          <main className="flex-1 bg-gray-100 p-6 overflow-auto">
            {activeTab === 'topology' && <TopologyGraph />}

            {activeTab === 'monitoring' && <MonitoringDashboard />}

            {activeTab === 'incidents' && <IncidentManagement />}

            {activeTab === 'investigations' && <AIInvestigationViewer />}
          </main>
        </div>
      </Layout>
    </QueryProvider>
  );
}

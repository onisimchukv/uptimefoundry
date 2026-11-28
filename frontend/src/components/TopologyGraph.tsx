'use client';

import React, { useEffect, useRef, useState } from 'react';
import { ForceGraph2D } from 'react-force-graph';
import { useQuery } from '@tanstack/react-query';
import { api } from '../lib/api';
import type { Node, Edge, NodeType, NodeStatus } from '../types';

export const TopologyGraph: React.FC = () => {
  // Fetch topology data using React Query
  const { data: topology, isLoading, error } = useQuery({
    queryKey: ['topology'],
    queryFn: async () => {
      const data = await api.getTopology({ format: 'graph', depth: 2 });
      return data;
    },
    staleTime: 3000,
    refetchInterval: 3000,
  });

  if (isLoading) {
    return (
      <div className="card">
        <div className="spinner"></div>
        <p style={{ textAlign: 'center', marginTop: 16 }}>Loading topology...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="card">
        <div className="error-state">
          <div className="error-state-icon">❌</div>
          <div className="error-state-title">Failed to Load Topology</div>
          <p>{error instanceof Error ? error.message : 'An unknown error occurred'}</p>
        </div>
      </div>
    );
  }
  const fgRef = useRef<any>(null);
  const [dimensions, setDimensions] = useState({ width: 0, height: 0 });

  useEffect(() => {
    const handleResize = () => {
      if (fgRef.current) {
        fgRef.current.width = window.innerWidth;
        fgRef.current.height = window.innerHeight;
      }
      setDimensions({ width: window.innerWidth, height: window.innerHeight });
    };

    handleResize();
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  const nodeColor = (node: Node) => {
    switch (node.type) {
      case 'host':
        return '#4fc3f7';
      case 'service':
        return '#64b5f6';
      case 'swarm':
        return '#90caf9';
      case 'container':
        return '#bbdefb';
      case 'network':
        return '#ff9800';
      case 'storage':
        return '#8bc34a';
      case 'external':
        return '#9e9e9e';
      default:
        return '#616161';
    }
  };

  const nodeLabel = (node: Node) => {
    if (!node.metadata) return node.name;
    return node.name;
  };

  const onNodeClick = (node: Node, event: MouseEvent) => {
    console.log('Clicked node:', node);
  };

  const onEdgeClick = (edge: Edge, event: MouseEvent) => {
    console.log('Clicked edge:', edge);
  };

  const getNodeSize = (node: Node) => {
    switch (node.type) {
      case 'host':
        return 30;
      case 'service':
        return 20;
      case 'swarm':
        return 25;
      case 'container':
        return 15;
      case 'network':
        return 22;
      case 'storage':
        return 20;
      case 'external':
        return 18;
      default:
        return 15;
    }
  };

  const nodeFont = (node: Node) => {
    return {
      face: 'Arial',
      fontSize: 10,
      fill: '#000',
      fontWeight: 400,
    };
  };

  const edgeColor = (edge: Edge) => {
    switch (edge.type) {
      case 'depends_on':
        return '#666';
      case 'communicates_with':
        return '#999';
      case 'owned_by':
        return '#4fc3f7';
      case 'routes_to':
        return '#64b5f6';
      case 'stores_data':
        return '#8bc34a';
      default:
        return '#ccc';
    }
  };

  const edgeWidth = (edge: Edge) => {
    return 1.5;
  };

  const nodeLabelColor = (node: Node) => {
    return {
      fill: '#000',
      fontSize: 11,
      fontWeight: 500,
    };
  };

  if (isLoading) {
    return (
      <div className="card">
        <div className="spinner"></div>
        <p style={{ textAlign: 'center', marginTop: 16 }}>Loading topology...</p>
      </div>
    );
  }

  if (!topology || !topology.data || topology.data.nodes.length === 0) {
    return (
      <div className="card">
        <div className="empty-state">
          <div className="empty-state-icon">🕸️</div>
          <div className="empty-state-title">No Topology Data</div>
          <p>Topology data is not available at this time.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="card" style={{ height: '600px', position: 'relative' }}>
      <ForceGraph2D
        ref={fgRef}
        graphData={{
          nodes: topology.data.nodes.map((node: Node) => ({
            ...node,
            label: nodeLabel(node),
            val: getNodeSize(node),
            color: nodeColor(node),
          })),
          links: topology.data.edges.map((edge: Edge) => ({
            id: edge.id,
            source: edge.source,
            target: edge.target,
            type: edge.type,
            color: edgeColor(edge),
            width: edgeWidth(edge),
          })),
        }}
        width={dimensions.width}
        height={dimensions.height}
        nodeLabel={nodeLabel}
        nodeColor={nodeColor}
        linkColor={edgeColor}
        linkWidth={edgeWidth}
        linkDirectionalArrowLength={6}
        linkDirectionalArrowRelPos={1}
        onNodeClick={onNodeClick}
        onLinkClick={onEdgeClick}
        onNodeRightClick={(node: Node, event: MouseEvent) => {
          event.stopPropagation();
          console.log('Right clicked node:', node);
        }}
        onLinkRightClick={(link: Edge, event: MouseEvent) => {
          event.stopPropagation();
          console.log('Right clicked edge:', link);
        }}
        onEngineStop={() => {
          console.log('Engine stopped');
        }}
        onEngineTick={() => {
          console.log('Tick');
        }}
        backgroundColor="transparent"
      />
      
      {/* Legend */}
      <div style={{
        position: 'absolute',
        bottom: 16,
        right: 16,
        background: 'rgba(255, 255, 255, 0.95)',
        padding: 12,
        borderRadius: 6,
        boxShadow: '0 2px 8px rgba(0,0,0,0.15)',
        fontSize: '0.8rem',
      }}>
        <strong style={{ display: 'block', marginBottom: 8 }}>Node Types</strong>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          {[
            { type: 'host', color: '#4fc3f7' },
            { type: 'service', color: '#64b5f6' },
            { type: 'swarm', color: '#90caf9' },
            { type: 'container', color: '#bbdefb' },
            { type: 'network', color: '#ff9800' },
            { type: 'storage', color: '#8bc34a' },
            { type: 'external', color: '#9e9e9e' },
          ].map((item) => (
            <div key={item.type} style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <div style={{
                width: 12,
                height: 12,
                borderRadius: '50%',
                background: item.color,
              }} />
              <span>{item.type}</span>
            </div>
          ))}
        </div>
        
        <strong style={{ display: 'block', marginTop: 12, marginBottom: 8 }}>Status</strong>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          {[
            { status: 'healthy', color: '#4caf50' },
            { status: 'degraded', color: '#ff9800' },
            { status: 'unhealthy', color: '#f44336' },
            { status: 'maintenance', color: '#9c27b0' },
            { status: 'offline', color: '#616161' },
          ].map((item) => (
            <div key={item.status} style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <div style={{
                width: 10,
                height: 10,
                borderRadius: '50%',
                background: item.color,
              }} />
              <span>{item.status}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

export default TopologyGraph;

// Local type definitions for react-force-graph
import React from 'react';

declare module 'react-force-graph' {
  interface ForceGraphProps {
    // Keep all existing props
    [key: string]: any;
  }

  const ForceGraph: React.FC<ForceGraphProps> & {
    new (): any;
  };

  export default ForceGraph;
  export { ForceGraph };
}

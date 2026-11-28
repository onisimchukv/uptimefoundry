// WebSocket client for real-time updates

import { io, Socket } from 'socket.io-client';
import { v4 as uuidv4 } from 'uuid';
import type {
  WebSocketMessage,
  TopologyUpdateEvent,
  MetricsUpdateEvent,
  IncidentUpdateEvent,
  InvestigationUpdateEvent,
  ActionStatusEvent,
} from '../types/api';

// Configuration
const WS_URL = process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8080/ws';

class WebSocketClient {
  private socket: Socket | null = null;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectDelay = 1000;
  private eventHandlers: Map<string, Set<Function>> = new Map();
  private isConnecting = false;

  constructor() {
    this.connect();
  }

  private connect(): void {
    if (this.isConnecting || this.socket?.connected) {
      return;
    }

    this.isConnecting = true;
    this.socket = io(WS_URL, {
      transports: ['websocket'],
      reconnection: true,
      reconnectionAttempts: this.maxReconnectAttempts,
      reconnectionDelay: this.reconnectDelay,
      reconnectionDelayMax: 5000,
      timeout: 20000,
    });

    this.socket.on('connect', () => {
      console.log('[WebSocket] Connected');
      this.isConnecting = false;
      this.reconnectAttempts = 0;
    });

    this.socket.on('disconnect', (reason) => {
      console.log('[WebSocket] Disconnected:', reason);
      this.isConnecting = false;
    });

    this.socket.on('reconnecting', () => {
      console.log('[WebSocket] Reconnecting...');
      this.isConnecting = true;
    });

    this.socket.on('error', (error) => {
      console.error('[WebSocket] Error:', error);
    });

    this.socket.on('message', (message: WebSocketMessage) => {
      this.handleMessage(message);
    });
  }

  private handleMessage(message: WebSocketMessage): void {
    const handlers = this.eventHandlers.get(message.type);
    if (handlers) {
      handlers.forEach((handler) => {
        try {
          handler(message);
        } catch (error) {
          console.error(`[WebSocket] Error handling ${message.type}:`, error);
        }
      });
    }
  }

  // Subscribe to topology updates
  subscribeTopology(onUpdate: (data: TopologyUpdateEvent) => void): () => void {
    this.ensureHandlers('topology:update', onUpdate);
    return this.unsubscribe('topology:update');
  }

  // Subscribe to metrics updates
  subscribeMetrics(onUpdate: (data: MetricsUpdateEvent) => void): () => void {
    this.ensureHandlers('metrics:update', onUpdate);
    return this.unsubscribe('metrics:update');
  }

  // Subscribe to incident updates
  subscribeIncidents(onUpdate: (data: IncidentUpdateEvent) => void): () => void {
    this.ensureHandlers('incident:update', onUpdate);
    return this.unsubscribe('incident:update');
  }

  // Subscribe to investigation updates
  subscribeInvestigations(
    onUpdate: (data: InvestigationUpdateEvent) => void
  ): () => void {
    this.ensureHandlers('investigation:update', onUpdate);
    return this.unsubscribe('investigation:update');
  }

  // Subscribe to action status updates
  subscribeActionStatus(
    onUpdate: (data: ActionStatusEvent) => void
  ): () => void {
    this.ensureHandlers('action:status', onUpdate);
    return this.unsubscribe('action:status');
  }

  private ensureHandlers(eventType: string, handler: Function): void {
    if (!this.eventHandlers.has(eventType)) {
      this.eventHandlers.set(eventType, new Set());
    }
    this.eventHandlers.get(eventType)!.add(handler);
  }

  private unsubscribe(eventType: string): () => void {
    const handlers = this.eventHandlers.get(eventType);
    if (handlers) {
      const handlerToRemove = Array.from(handlers).find(
        (h: Function) => h.toString() === h.toString()
      );
      if (handlerToRemove) {
        handlers.delete(handlerToRemove);
      }
    }
    return () => {};
  }

  // Send a message to the server
  send(type: string, payload: unknown): void {
    if (this.socket?.connected) {
      this.socket.emit('message', {
        type,
        payload,
        timestamp: new Date().toISOString(),
        correlationId: uuidv4(),
      });
    }
  }

  // Disconnect
  disconnect(): void {
    if (this.socket) {
      this.socket.disconnect();
      this.socket = null;
    }
  }

  // Check connection status
  isConnected(): boolean {
    return this.socket?.connected ?? false;
  }
}

// Singleton instance
export const wsClient = new WebSocketClient();

export default wsClient;

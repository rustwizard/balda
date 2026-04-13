import { Centrifuge } from 'centrifuge';
import type { CentrifugoEvent } from '../types';

export type EventHandler = (event: CentrifugoEvent) => void;

export class CentrifugoClient {
  private centrifuge: Centrifuge | null = null;
  private handlers: EventHandler[] = [];
  private subscriptions: Map<string, any> = new Map();

  connect(wsUrl: string, token: string) {
    this.centrifuge = new Centrifuge(wsUrl, {
      token,
    });

    this.centrifuge.on('connecting', () => {
      console.log('[centrifugo] connecting');
    });

    this.centrifuge.on('connected', () => {
      console.log('[centrifugo] connected');
    });

    this.centrifuge.on('disconnected', () => {
      console.log('[centrifugo] disconnected');
    });

    this.centrifuge.connect();
  }

  subscribe(channel: string, token: string) {
    if (!this.centrifuge) return;
    if (this.subscriptions.has(channel)) return;

    const sub = this.centrifuge.newSubscription(channel, { token });

    sub.on('publication', (ctx) => {
      const data = ctx.data as CentrifugoEvent;
      console.log('[centrifugo]', channel, data);
      this.handlers.forEach((h) => h(data));
    });

    sub.subscribe();
    this.subscriptions.set(channel, sub);
  }

  unsubscribe(channel: string) {
    const sub = this.subscriptions.get(channel);
    if (sub) {
      sub.unsubscribe();
      this.subscriptions.delete(channel);
    }
  }

  disconnect() {
    this.subscriptions.forEach((sub) => sub.unsubscribe());
    this.subscriptions.clear();
    this.centrifuge?.disconnect();
    this.centrifuge = null;
  }

  onEvent(handler: EventHandler) {
    this.handlers.push(handler);
    return () => {
      this.handlers = this.handlers.filter((h) => h !== handler);
    };
  }
}

export const centrifugo = new CentrifugoClient();

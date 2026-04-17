import { Centrifuge } from 'centrifuge';
import type { CentrifugoEvent } from '../types';

export type EventHandler = (event: CentrifugoEvent) => void;

export class CentrifugoClient {
  private centrifuge: Centrifuge | null = null;
  private handlers: EventHandler[] = [];
  private subscriptions: Map<string, any> = new Map();
  private pendingSubscriptions: Array<{ channel: string; token: string }> = [];

  connect(wsUrl: string, token: string) {
    console.log('[centrifugo] connect called', wsUrl);
    this.centrifuge = new Centrifuge(wsUrl, {
      token,
    });

    this.centrifuge.on('connecting', () => {
      console.log('[centrifugo] connecting');
    });

    this.centrifuge.on('connected', () => {
      console.log('[centrifugo] connected');
    });

    this.centrifuge.on('disconnected', (ctx) => {
      console.log('[centrifugo] disconnected', ctx);
    });

    this.centrifuge.on('error', (ctx) => {
      console.error('[centrifugo] error', ctx);
    });

    this.centrifuge.connect();

    // Process any subscriptions requested before connect()
    const pending = this.pendingSubscriptions;
    this.pendingSubscriptions = [];
    pending.forEach(({ channel, token }) => this.subscribe(channel, token));
  }

  subscribe(channel: string, token: string) {
    console.log('[centrifugo] subscribe', channel);
    if (!this.centrifuge) {
      console.warn('[centrifugo] no centrifuge instance, queuing subscribe');
      this.pendingSubscriptions.push({ channel, token });
      return;
    }
    if (this.subscriptions.has(channel)) {
      console.log('[centrifugo] already subscribed to', channel);
      return;
    }

    const sub = this.centrifuge.newSubscription(channel, { token });

    sub.on('subscribing', () => console.log('[centrifugo] subscribing to', channel));
    sub.on('subscribed', () => console.log('[centrifugo] subscribed to', channel));
    sub.on('unsubscribed', (ctx) => console.log('[centrifugo] unsubscribed from', channel, ctx));
    sub.on('publication', (ctx) => {
      const data = ctx.data as CentrifugoEvent;
      console.log('[centrifugo] publication on', channel, data);
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
    console.log('[centrifugo] disconnect');
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

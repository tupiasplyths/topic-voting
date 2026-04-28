import { useEffect, useRef, useState, useCallback } from 'react';

type WSStatus = 'connecting' | 'open' | 'closed' | 'error';

interface UseWebSocketReturn {
  status: WSStatus;
  send: (data: string) => void;
  close: () => void;
}

export function useWebSocket<T>(
  url: string,
  onMessage: (data: T) => void,
): UseWebSocketReturn {
  const [status, setStatus] = useState<WSStatus>('closed');
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reconnectDelayRef = useRef(1000);
  const onMessageRef = useRef(onMessage);
  const urlRef = useRef(url);
  const generationRef = useRef(0);

  onMessageRef.current = onMessage;
  urlRef.current = url;

  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN || wsRef.current?.readyState === WebSocket.CONNECTING) return;

    const gen = ++generationRef.current;

    setStatus('connecting');
    const ws = new WebSocket(urlRef.current);
    wsRef.current = ws;

    ws.onopen = () => {
      if (gen !== generationRef.current) return;
      setStatus('open');
      reconnectDelayRef.current = 1000;
    };

    ws.onmessage = (event) => {
      if (gen !== generationRef.current) return;
      try {
        const data = JSON.parse(event.data) as T;
        onMessageRef.current(data);
      } catch {
      }
    };

    ws.onclose = () => {
      if (gen !== generationRef.current) return;
      setStatus('closed');
      wsRef.current = null;
      reconnectTimeoutRef.current = setTimeout(() => {
        if (gen !== generationRef.current) return;
        const delay = reconnectDelayRef.current;
        reconnectDelayRef.current = Math.min(delay * 2, 30000);
        connect();
      }, reconnectDelayRef.current);
    };

    ws.onerror = () => {
      if (gen !== generationRef.current) return;
      setStatus('error');
    };
  }, []);

  const send = useCallback((data: string) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(data);
    }
  }, []);

  const close = useCallback(() => {
    generationRef.current++;
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    wsRef.current?.close();
    wsRef.current = null;
    setStatus('closed');
  }, []);

  useEffect(() => {
    if (!url) {
      close();
      return;
    }
    connect();
    return () => {
      close();
    };
  }, [connect, close, url]);

  return { status, send, close };
}

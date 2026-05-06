import { useState, useEffect, useRef, useCallback } from 'react';
import { Gem, Play, Square, Zap, Trash2, Send, DollarSign } from 'lucide-react';
import type { ChatMessage, ChatSpeed } from '../types/chat';
import {
  generateMessage,
  generateDonationMessage,
  getRandomUsername,
  getUsernameColor,
  getSpeedRangeMs,
  pickRandom,
  USERNAMES,
} from '../utils/mockChat';
import { getLabels } from '../api/client';

interface MockChatProps {
  topicId: string | null;
  topicTitle?: string;
}

export default function MockChat({ topicId, topicTitle }: MockChatProps) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [isRunning, setIsRunning] = useState(false);
  const [speed, setSpeed] = useState<ChatSpeed>('normal');
  const [labels, setLabels] = useState<string[]>([]);
  const [msgCount, setMsgCount] = useState(0);
  const [wsStatus, setWsStatus] = useState<'connected' | 'disconnected' | 'connecting'>('disconnected');

  const [inputText, setInputText] = useState('');
  const [isDonating, setIsDonating] = useState(false);
  const [donationAmount, setDonationAmount] = useState('');
  const [donationCurrency, setDonationCurrency] = useState('USD');

  const wsRef = useRef<WebSocket | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const isRunningRef = useRef(isRunning);
  const speedRef = useRef(speed);
  const topicIdRef = useRef(topicId);
  const labelsRef = useRef(labels);
  const reconnectDelayRef = useRef(1000);

  isRunningRef.current = isRunning;
  speedRef.current = speed;
  topicIdRef.current = topicId;
  labelsRef.current = labels;

  useEffect(() => {
    if (!topicId) return;
    getLabels(topicId).then(setLabels).catch(() => {});
  }, [topicId]);

  useEffect(() => {
    if (!topicId) return;
    const interval = setInterval(() => {
      getLabels(topicId).then(setLabels).catch(() => {});
    }, 5000);
    return () => clearInterval(interval);
  }, [topicId]);

  const addSystemMessage = (text: string) => {
    setMessages((prev) => {
      const next = [
        ...prev,
        {
          id: `sys-${Date.now()}`,
          username: '',
          message: text,
          color: '',
          is_donation: false,
          bits_amount: 0,
          donation_amount: 0,
          donation_currency: '',
          timestamp: Date.now(),
        },
      ];
      return next.length > 200 ? next.slice(next.length - 200) : next;
    });
  };

  const connectWs = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return;
    if (!topicId) return;
    setWsStatus('connecting');
    const apiUrl = import.meta.env.VITE_API_URL || 'http://localhost:8585';
    const wsUrl = apiUrl.replace('http', 'ws') + `/ws/chat?topic_id=${topicId}`;
    const ws = new WebSocket(wsUrl);
    ws.onopen = () => {
      setWsStatus('connected');
      reconnectDelayRef.current = 1000;
      addSystemMessage('Connected to chat');
    };
    ws.onclose = () => {
      setWsStatus('disconnected');
      wsRef.current = null;
      const delay = reconnectDelayRef.current;
      reconnectDelayRef.current = Math.min(delay * 2, 30000);
      setTimeout(() => {
        connectWs();
      }, delay);
    };
    ws.onerror = () => {
      setWsStatus('disconnected');
      wsRef.current = null;
    };
    wsRef.current = ws;
  }, [topicId]);

  useEffect(() => {
    if (!topicId) {
      wsRef.current?.close();
      wsRef.current = null;
      setWsStatus('disconnected');
      return;
    }
    connectWs();
    return () => {
      wsRef.current?.close();
      wsRef.current = null;
    };
  }, [topicId, connectWs]);

  const sendWsMessage = (chatMsg: ChatMessage) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(
        JSON.stringify({
          type: 'chat_message',
          data: {
            username: chatMsg.username,
            message: chatMsg.message,
            is_donation: chatMsg.is_donation,
            bits_amount: chatMsg.bits_amount,
            donation_amount: chatMsg.donation_amount,
            donation_currency: chatMsg.donation_currency,
          },
        }),
      );
    }
  };

  const addMessage = (chatMsg: ChatMessage) => {
    setMessages((prev) => {
      const next = [...prev, chatMsg];
      return next.length > 200 ? next.slice(next.length - 200) : next;
    });
    setMsgCount((c) => c + 1);
    sendWsMessage(chatMsg);
  };

  const scheduleNext = useCallback(() => {
    if (!isRunningRef.current || !topicIdRef.current) return;

    const [min, max] = getSpeedRangeMs(speedRef.current);
    const delay = min + Math.random() * (max - min);

    timerRef.current = setTimeout(() => {
      if (!isRunningRef.current || !topicIdRef.current) return;

      const isDonation = Math.random() < 0.1;
      const username = getRandomUsername();
      const color = getUsernameColor(username);
      const currentLabels = labelsRef.current;

      let msg: string;
      let bits = 0;
      let item = '';
      let donation_amount = 0;
      let donation_currency = '';

      if (isDonation) {
        const result = generateDonationMessage(currentLabels, topicTitle || '');
        msg = result.message;
        bits = result.bits;
        item = result.item;
        donation_amount = result.donation_amount;
        donation_currency = result.donation_currency;
      } else {
        const result = generateMessage(currentLabels, topicTitle || '');
        msg = result.message;
        item = result.item;
      }

      const chatMsg: ChatMessage = {
        id: crypto.randomUUID(),
        username,
        message: msg,
        color,
        is_donation: isDonation,
        bits_amount: bits,
        donation_amount,
        donation_currency,
        timestamp: Date.now(),
        status: 'pending',
        classified_label: item,
      };

      addMessage(chatMsg);
      scheduleNext();
    }, delay);
  }, []);

  const handleManualSend = () => {
    if (!inputText.trim() || !topicId) return;

    const username = pickRandom(USERNAMES);
    const color = getUsernameColor(username);
    const currentLabels = labelsRef.current;
    const item = currentLabels.length > 0 ? pickRandom(currentLabels) : '';

    const isDonation = isDonating && donationAmount.trim() !== '';
    const amount = isDonation ? Math.max(0, parseFloat(donationAmount) || 0) : 0;

    const chatMsg: ChatMessage = {
      id: crypto.randomUUID(),
      username,
      message: inputText.trim(),
      color,
      is_donation: isDonation,
      bits_amount: 0,
      donation_amount: amount,
      donation_currency: isDonation ? donationCurrency : '',
      timestamp: Date.now(),
      status: 'pending',
      classified_label: item,
    };

    addMessage(chatMsg);
    setInputText('');
    setIsDonating(false);
    setDonationAmount('');
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleManualSend();
    }
  };

  const startSimulation = () => {
    if (!topicId) return;
    setIsRunning(true);
    addSystemMessage('Simulation started');
  };

  const stopSimulation = () => {
    setIsRunning(false);
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    addSystemMessage('Simulation stopped');
  };

  const clearChat = () => {
    setMessages([]);
    setMsgCount(0);
    addSystemMessage('Chat cleared');
  };

  useEffect(() => {
    if (!isRunning) return;
    scheduleNext();
    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }
    };
  }, [isRunning, speed, scheduleNext]);

  useEffect(() => {
    return () => {
      wsRef.current?.close();
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, []);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  if (!topicId) {
    return (
      <div className="flex items-center justify-center h-64 bg-gray-950 rounded-lg border border-gray-800">
        <p className="text-gray-400">
          No active topic. Create a topic on the Host page first.
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-[600px] bg-gray-950 rounded-lg border border-gray-800">
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-800">
        <div className="flex items-center gap-2">
          <button
            onClick={isRunning ? stopSimulation : startSimulation}
            className={`flex items-center gap-2 px-3 py-1.5 rounded text-sm font-medium transition-colors ${
              isRunning
                ? 'bg-red-600 hover:bg-red-700 text-white'
                : 'bg-green-600 hover:bg-green-700 text-white'
            }`}
          >
            {isRunning ? (
              <>
                <Square size={14} fill="currentColor" />
                Stop
              </>
            ) : (
              <>
                <Play size={14} fill="currentColor" />
                Start
              </>
            )}
          </button>
          <div className="flex items-center gap-1 bg-gray-800 rounded p-0.5">
            {(['slow', 'normal', 'fast'] as ChatSpeed[]).map((s) => (
              <button
                key={s}
                onClick={() => setSpeed(s)}
                className={`flex items-center gap-1 px-2 py-1 rounded text-xs font-medium transition-colors ${
                  speed === s
                    ? 'bg-indigo-600 text-white'
                    : 'text-gray-400 hover:text-white'
                }`}
              >
                <Zap size={12} />
                {s.charAt(0).toUpperCase() + s.slice(1)}
              </button>
            ))}
          </div>
          <button
            onClick={clearChat}
            className="flex items-center gap-1 px-2 py-1.5 rounded text-xs font-medium text-gray-400 hover:text-white hover:bg-gray-800 transition-colors"
            title="Clear chat"
          >
            <Trash2 size={12} />
            Clear
          </button>
        </div>
        <div className="text-xs text-gray-500">
          {msgCount} messages sent
          {isRunning && (
            <span className={`ml-2 inline-block w-2 h-2 rounded-full ${
              wsStatus === 'connected' ? 'bg-green-500' :
              wsStatus === 'connecting' ? 'bg-yellow-500' : 'bg-red-500'
            }`} />
          )}
          {!isRunning && messages.length > 0 && (
            <span className="ml-2 text-gray-600">paused</span>
          )}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto px-3 py-2 space-y-1">
        {messages.map((msg) => {
          if (!msg.username) {
            return (
              <div
                key={msg.id}
                className="text-center text-xs text-gray-500 py-1"
              >
                {msg.message}
              </div>
            );
          }

          return (
            <div
              key={msg.id}
              className={`text-sm px-2 py-0.5 rounded ${
                msg.is_donation
                  ? 'bg-purple-900/30 border border-purple-800/50'
                  : ''
              }`}
            >
              <span style={{ color: msg.color }} className="font-medium">
                {msg.username}
              </span>
              {msg.is_donation && (
                <span className="inline-flex items-center gap-0.5 ml-1 text-purple-400">
                  {msg.bits_amount > 0 && (
                    <>
                      <Gem size={12} />
                      <span className="text-xs">x{msg.bits_amount}</span>
                    </>
                  )}
                  {msg.donation_amount > 0 && (
                    <span className="text-purple-300 ml-1">
                      ${msg.donation_amount.toFixed(2)} {msg.donation_currency}
                    </span>
                  )}
                </span>
              )}
              <span className="text-gray-300 ml-1">{msg.message}</span>
            </div>
          );
        })}
        <div ref={messagesEndRef} />
      </div>

      <div className="border-t border-gray-800 px-3 py-2 space-y-2">
        <div className="flex items-center gap-2">
          <input
            type="text"
            value={inputText}
            onChange={(e) => setInputText(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Type a message..."
            className="flex-1 bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          />
          <button
            onClick={() => setIsDonating(!isDonating)}
            className={`flex items-center gap-1 px-2 py-1.5 rounded text-xs font-medium transition-colors ${
              isDonating
                ? 'bg-purple-600 text-white'
                : 'text-gray-400 hover:text-white hover:bg-gray-800'
            }`}
          >
            <DollarSign size={14} />
            Donate
          </button>
          <button
            onClick={handleManualSend}
            disabled={!inputText.trim() || wsStatus !== 'connected'}
            className="flex items-center gap-1 px-3 py-1.5 rounded text-sm font-medium bg-indigo-600 text-white hover:bg-indigo-500 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          >
            <Send size={14} />
          </button>
        </div>
        {isDonating && (
          <div className="flex items-center gap-2">
            <input
              type="number"
              value={donationAmount}
              onChange={(e) => setDonationAmount(e.target.value)}
              placeholder="Amount"
              min="0"
              step="0.01"
              className="w-24 bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
            <select
              value={donationCurrency}
              onChange={(e) => setDonationCurrency(e.target.value)}
              className="bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm text-white focus:outline-none focus:ring-1 focus:ring-indigo-500"
            >
              <option value="USD">USD</option>
              <option value="CAD">CAD</option>
              <option value="EUR">EUR</option>
              <option value="GBP">GBP</option>
            </select>
          </div>
        )}
      </div>
    </div>
  );
}

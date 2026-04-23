import { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useWebSocket } from '../hooks/useWebSocket';
import { getLeaderboard } from '../api/client';
import VoteBarChart from '../components/VoteBarChart';
import type { Leaderboard, WSMessage } from '../types';

export default function DisplayPage() {
  const [searchParams] = useSearchParams();
  const topicId = searchParams.get('topic_id');
  const maxEntries = parseInt(searchParams.get('max_entries') || '10', 10);
  const bg = searchParams.get('bg') || 'dark';

  const [entries, setEntries] = useState<Leaderboard['entries']>([]);
  const [topic, setTopic] = useState('');

  useEffect(() => {
    if (!topicId) return;
    let cancelled = false;
    getLeaderboard(topicId)
      .then((lb) => {
        if (!cancelled) {
          setEntries(lb.entries);
          setTopic(lb.topic);
        }
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [topicId]);

  const handleMessage = (msg: WSMessage<Leaderboard>) => {
    if (msg.type === 'leaderboard_update' && msg.data) {
      setEntries(msg.data.entries);
      setTopic(msg.data.topic);
    }
  };

  const wsUrl = topicId ? `/ws/dashboard?topic_id=${topicId}` : null;
  useWebSocket<WSMessage<Leaderboard>>(wsUrl || '', handleMessage);

  if (!topicId) {
    return (
      <div className="min-h-screen bg-gray-900 flex items-center justify-center">
        <p className="text-gray-400 text-lg">No topic selected</p>
      </div>
    );
  }

  return (
    <div
      className={`min-h-screen ${
        bg === 'transparent' ? 'bg-transparent' : 'bg-gray-900'
      } p-6 overflow-hidden`}
    >
      {topic && (
        <h1 className="text-3xl font-bold text-white mb-6 text-center">
          {topic}
        </h1>
      )}
      <VoteBarChart entries={entries} maxEntries={maxEntries} />
    </div>
  );
}

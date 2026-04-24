import { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useWebSocket } from '../hooks/useWebSocket';
import { getLeaderboard } from '../api/client';
import DisplayOverlay from '../components/DisplayOverlay';
import type { Leaderboard, WSMessage } from '../types';

export default function DisplayPage() {
  const [searchParams] = useSearchParams();
  const topicId = searchParams.get('topic_id');
  const maxEntries = parseInt(searchParams.get('max_entries') || '10', 10);
  const bg = searchParams.get('bg') || 'dark';
  const showTitle = searchParams.get('show_title') !== 'false';

  const [entries, setEntries] = useState<Leaderboard['entries']>([]);
  const [topic, setTopic] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!topicId) return;
    let cancelled = false;
    setLoading(true);
    getLeaderboard(topicId)
      .then((lb) => {
        if (!cancelled) {
          setEntries(lb.entries);
          setTopic(lb.topic);
          setLoading(false);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });
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

  const wsUrl = topicId ? `/ws/dashboard?topic_id=${topicId}` : '';
  useWebSocket<WSMessage<Leaderboard>>(wsUrl, handleMessage);

  if (!topicId) {
    return (
      <div className="min-h-screen bg-gray-900 flex items-center justify-center">
        <p className="text-gray-400 text-lg">No topic selected</p>
      </div>
    );
  }

  if (loading) {
    return (
      <div
        className={`min-h-screen ${
          bg === 'transparent' ? 'bg-transparent' : 'bg-gray-900'
        } flex items-center justify-center`}
      >
        <div className="text-center">
          <div className="w-16 h-16 border-4 border-indigo-500 border-t-transparent rounded-full animate-spin mx-auto mb-4" />
          <p className="text-gray-400 text-lg">Loading...</p>
        </div>
      </div>
    );
  }

  return (
    <div
      className={`min-h-screen ${
        bg === 'transparent' ? 'bg-transparent' : 'bg-gray-900'
      } p-6 overflow-hidden`}
    >
      {topic && showTitle && (
        <h1 className="text-3xl font-bold text-white mb-6 text-center">
          {topic}
        </h1>
      )}
      <DisplayOverlay entries={entries} maxEntries={maxEntries} />
    </div>
  );
}

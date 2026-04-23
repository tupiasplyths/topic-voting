import { useState, useEffect } from 'react';
import { useWebSocket } from '../hooks/useWebSocket';
import { getLeaderboard } from '../api/client';
import type { Leaderboard, LeaderboardEntry, WSMessage } from '../types';

interface Props {
  topicId: string;
}

export default function Leaderboard({ topicId }: Props) {
  const [entries, setEntries] = useState<LeaderboardEntry[]>([]);
  const [topic, setTopic] = useState('');
  const [updatedAt, setUpdatedAt] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    getLeaderboard(topicId)
      .then((lb) => {
        if (!cancelled) {
          setEntries(lb.entries);
          setTopic(lb.topic);
          setUpdatedAt(lb.updated_at);
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
      setUpdatedAt(msg.data.updated_at);
    }
  };

  const wsUrl = `/ws/dashboard?topic_id=${topicId}`;
  const { status } = useWebSocket<WSMessage<Leaderboard>>(wsUrl, handleMessage);

  const maxWeight = Math.max(...entries.map((e) => e.total_weight), 1);

  const formattedTime = updatedAt
    ? new Date(updatedAt).toLocaleTimeString()
    : null;

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-white">
          {topic || 'Leaderboard'}
        </h2>
        <div className="flex items-center gap-3">
          {formattedTime && (
            <span className="text-xs text-gray-500">Updated {formattedTime}</span>
          )}
          <span
            className={`w-2 h-2 rounded-full ${
              status === 'open' ? 'bg-green-400' : 'bg-gray-600'
            }`}
          />
        </div>
      </div>

      {entries.length === 0 ? (
        <p className="text-gray-500 text-sm py-8 text-center">
          No votes yet. Waiting for votes...
        </p>
      ) : (
        <ul className="space-y-2">
          {entries.map((entry, i) => {
            const pct = (entry.total_weight / maxWeight) * 100;
            return (
              <li key={entry.label} className="space-y-1">
                <div className="flex items-center justify-between text-sm">
                  <span className="font-medium text-white">
                    {i + 1}. {entry.label}
                  </span>
                  <span className="text-gray-400 text-xs">
                    {entry.total_weight} pts · {entry.vote_count} votes
                  </span>
                </div>
                <div className="h-3 bg-gray-700 rounded-full overflow-hidden">
                  <div
                    className="h-full bg-indigo-500 rounded-full transition-all duration-500 ease-out"
                    style={{ width: `${pct}%` }}
                  />
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}

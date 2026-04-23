import { useState } from 'react';
import TopicManager from '../components/TopicManager';
import Leaderboard from '../components/Leaderboard';
import VoteBarChart from '../components/VoteBarChart';
import ConnectionStatus from '../components/ConnectionStatus';
import { useWebSocket } from '../hooks/useWebSocket';
import type { Leaderboard as LeaderboardType, WSMessage } from '../types';
import type { Topic } from '../types';

export default function HostPage() {
  const [activeTopic, setActiveTopic] = useState<Topic | null>(null);
  const [lbEntries, setLbEntries] = useState<LeaderboardType['entries']>([]);

  const handleMessage = (msg: WSMessage<LeaderboardType>) => {
    if (msg.type === 'leaderboard_update' && msg.data) {
      setLbEntries(msg.data.entries);
    }
  };

  const wsUrl = activeTopic
    ? `/ws/dashboard?topic_id=${activeTopic.id}`
    : null;

  const { status } = useWebSocket<WSMessage<LeaderboardType>>(
    wsUrl || '',
    handleMessage,
  );

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-bold text-white">Host Dashboard</h1>
        <ConnectionStatus wsStatus={wsUrl ? status : 'closed'} />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-1">
          <TopicManager onActiveTopicChange={setActiveTopic} />
        </div>

        <div className="lg:col-span-2 space-y-6">
          {activeTopic ? (
            <>
              <Leaderboard topicId={activeTopic.id} />
              <div className="bg-gray-800 rounded-lg p-4">
                <h3 className="text-sm font-medium text-gray-400 mb-3">
                  Chart View
                </h3>
                <VoteBarChart entries={lbEntries} />
              </div>
            </>
          ) : (
            <div className="bg-gray-800 rounded-lg p-8 text-center">
              <p className="text-gray-400">
                Create a topic to start the voting session.
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

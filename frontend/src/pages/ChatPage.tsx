import { useState, useEffect } from 'react';
import type { Topic } from '../types';
import { getActiveTopic } from '../api/client';
import MockChat from '../components/MockChat';

export default function ChatPage() {
  const [activeTopic, setActiveTopic] = useState<Topic | null>(null);

  useEffect(() => {
    getActiveTopic()
      .then(setActiveTopic)
      .catch(() => {});
  }, []);

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl font-bold text-white">Mock Twitch Chat</h1>
        {activeTopic && (
          <span className="text-sm text-gray-400">
            Topic: <span className="text-indigo-400">{activeTopic.title}</span>
          </span>
        )}
      </div>
      <MockChat
        topicId={activeTopic?.id ?? null}
        topicTitle={activeTopic?.title}
      />
    </div>
  );
}

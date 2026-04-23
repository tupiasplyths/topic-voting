import { useState, useEffect, useCallback } from 'react';
import { Plus, X } from 'lucide-react';
import { getTopics, getActiveTopic, createTopic, closeTopic } from '../api/client';
import type { Topic, CreateTopicRequest } from '../types';

interface Props {
  onActiveTopicChange: (topic: Topic | null) => void;
}

export default function TopicManager({ onActiveTopicChange }: Props) {
  const [topics, setTopics] = useState<Topic[]>([]);
  const [activeTopic, setActiveTopic] = useState<Topic | null>(null);
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [threshold, setThreshold] = useState(0.5);
  const [setActive, setSetActive] = useState(true);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const [t, active] = await Promise.all([getTopics(), getActiveTopic()]);
      setTopics(t);
      setActiveTopic(active);
      onActiveTopicChange(active);
    } catch {
      setError('Failed to load topics');
    }
  }, [onActiveTopicChange]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim()) return;

    setLoading(true);
    setError(null);
    try {
      const req: CreateTopicRequest = {
        title: title.trim(),
        description: description.trim(),
        classifier_threshold: threshold,
        set_active: setActive,
      };
      await createTopic(req);
      setTitle('');
      setDescription('');
      setThreshold(0.5);
      setSetActive(true);
      await refresh();
    } catch {
      setError('Failed to create topic');
    } finally {
      setLoading(false);
    }
  };

  const handleClose = async (id: string) => {
    try {
      await closeTopic(id);
      await refresh();
    } catch {
      setError('Failed to close topic');
    }
  };

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold text-white">Topics</h2>

      {error && (
        <div className="bg-red-900/50 border border-red-700 text-red-300 px-3 py-2 rounded text-sm">
          {error}
        </div>
      )}

      {activeTopic && (
        <div className="bg-indigo-900/50 border border-indigo-700 rounded-lg p-3">
          <div className="flex items-center justify-between">
            <div>
              <span className="text-xs text-indigo-300 uppercase tracking-wide">Active</span>
              <p className="font-medium text-white">{activeTopic.title}</p>
              {activeTopic.description && (
                <p className="text-sm text-gray-400">{activeTopic.description}</p>
              )}
            </div>
            <button
              onClick={() => handleClose(activeTopic.id)}
              className="p-1.5 rounded hover:bg-red-800 text-gray-400 hover:text-red-300 transition-colors"
              title="Close topic"
            >
              <X size={16} />
            </button>
          </div>
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-3 bg-gray-800 rounded-lg p-4">
        <div>
          <label className="block text-sm font-medium text-gray-300 mb-1">
            Title
          </label>
          <input
            type="text"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="e.g. Best Food"
            className="w-full bg-gray-700 border border-gray-600 rounded px-3 py-2 text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-indigo-500"
            required
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-300 mb-1">
            Description (optional)
          </label>
          <input
            type="text"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="What are we voting on?"
            className="w-full bg-gray-700 border border-gray-600 rounded px-3 py-2 text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-indigo-500"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-300 mb-1">
            Confidence Threshold: {threshold.toFixed(1)}
          </label>
          <input
            type="range"
            min="0.1"
            max="0.9"
            step="0.1"
            value={threshold}
            onChange={(e) => setThreshold(parseFloat(e.target.value))}
            className="w-full accent-indigo-500"
          />
        </div>

        <label className="flex items-center gap-2 text-sm text-gray-300">
          <input
            type="checkbox"
            checked={setActive}
            onChange={(e) => setSetActive(e.target.checked)}
            className="accent-indigo-500"
          />
          Set as active topic
        </label>

        <button
          type="submit"
          disabled={loading}
          className="w-full flex items-center justify-center gap-2 bg-indigo-600 hover:bg-indigo-700 disabled:bg-indigo-800 disabled:text-gray-400 text-white text-sm font-medium py-2 rounded transition-colors"
        >
          <Plus size={16} />
          {loading ? 'Creating...' : 'Create Topic'}
        </button>
      </form>

      {topics.length > 0 && (
        <div>
          <h3 className="text-sm font-medium text-gray-400 mb-2">All Topics</h3>
          <ul className="space-y-1">
            {topics.map((t) => (
              <li
                key={t.id}
                className="flex items-center justify-between text-sm px-3 py-2 rounded bg-gray-800"
              >
                <span className={t.is_active ? 'text-white font-medium' : 'text-gray-400'}>
                  {t.title}
                </span>
                <span
                  className={`text-xs px-2 py-0.5 rounded ${
                    t.is_active
                      ? 'bg-green-900 text-green-300'
                      : 'bg-gray-700 text-gray-500'
                  }`}
                >
                  {t.is_active ? 'Active' : 'Closed'}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

import { useState, useEffect, useCallback } from 'react';
import { GitMerge, X } from 'lucide-react';
import { useWebSocket } from '../hooks/useWebSocket';
import { getLeaderboard, mergeLabels } from '../api/client';
import { useToast } from '../components/Toast';
import type { Leaderboard, LeaderboardEntry, WSMessage } from '../types';

interface Props {
  topicId: string;
}

export default function Leaderboard({ topicId }: Props) {
  const [entries, setEntries] = useState<LeaderboardEntry[]>([]);
  const [topic, setTopic] = useState('');
  const [updatedAt, setUpdatedAt] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const [mergeMode, setMergeMode] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [targetLabel, setTargetLabel] = useState<string>('');
  const [customTarget, setCustomTarget] = useState('');
  const [useCustomTarget, setUseCustomTarget] = useState(false);
  const [merging, setMerging] = useState(false);

  const { toast } = useToast();

  const toggleSelect = useCallback((label: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(label)) {
        next.delete(label);
      } else {
        next.add(label);
      }
      return next;
    });
    setTargetLabel('');
    setUseCustomTarget(false);
    setCustomTarget('');
  }, []);

  const handleMerge = useCallback(async () => {
    if (selected.size < 2) return;

    const labels = Array.from(selected);
    const target = useCustomTarget
      ? customTarget.trim()
      : targetLabel || labels[0];
    if (!target) return;

    const sources = useCustomTarget
      ? labels
      : labels.filter((l) => l !== target);
    if (sources.length === 0) return;

    setMerging(true);
    try {
      const result = await mergeLabels({
        topic_id: topicId,
        source_labels: sources,
        target_label: target,
      });
      toast(
        `Merged ${result.merged_labels.length} label(s) into "${target}" (${result.votes_affected} votes)`,
        'success',
      );
      setSelected(new Set());
      setTargetLabel('');
      setCustomTarget('');
      setUseCustomTarget(false);
    } catch {
      toast('Failed to merge labels', 'error');
    } finally {
      setMerging(false);
    }
  }, [selected, targetLabel, customTarget, useCustomTarget, topicId, toast]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    getLeaderboard(topicId)
      .then((lb) => {
        if (!cancelled) {
          setEntries(lb.entries);
          setTopic(lb.topic);
          setUpdatedAt(lb.updated_at);
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
      setUpdatedAt(msg.data.updated_at);
    }
  };

  const wsUrl = `/ws/dashboard?topic_id=${topicId}`;
  const { status } = useWebSocket<WSMessage<Leaderboard>>(wsUrl, handleMessage);

  const maxWeight = Math.max(...entries.map((e) => e.total_weight), 1);

  const formattedTime = updatedAt
    ? new Date(updatedAt).toLocaleTimeString()
    : null;

  const exitMergeMode = () => {
    setMergeMode(false);
    setSelected(new Set());
    setTargetLabel('');
    setCustomTarget('');
    setUseCustomTarget(false);
  };

  const sortedEntries = [...entries].sort(
    (a, b) => b.total_weight - a.total_weight,
  );

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-white">
          {topic || 'Leaderboard'}
        </h2>
        <div className="flex items-center gap-3">
          {formattedTime && (
            <span className="text-xs text-gray-500">
              Updated {formattedTime}
            </span>
          )}
          <span
            className={`w-2 h-2 rounded-full ${
              status === 'open' ? 'bg-green-400' : 'bg-gray-600'
            }`}
          />
          {!mergeMode && entries.length >= 2 && (
            <button
              onClick={() => setMergeMode(true)}
              className="flex items-center gap-1 text-xs text-gray-400 hover:text-indigo-400 transition-colors px-2 py-1 rounded border border-gray-700 hover:border-indigo-500"
              title="Merge labels"
            >
              <GitMerge size={14} />
              Merge
            </button>
          )}
          {mergeMode && (
            <button
              onClick={exitMergeMode}
              className="flex items-center gap-1 text-xs text-gray-400 hover:text-red-400 transition-colors px-2 py-1 rounded border border-gray-700 hover:border-red-500"
            >
              <X size={14} />
              Cancel
            </button>
          )}
        </div>
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-8">
          <div className="w-8 h-8 border-2 border-indigo-500 border-t-transparent rounded-full animate-spin" />
        </div>
      ) : entries.length === 0 ? (
        <div className="text-center py-8">
          <div className="inline-block w-12 h-12 border-4 border-indigo-500 border-t-transparent rounded-full animate-spin mb-4" />
          <p className="text-gray-500 text-sm">Waiting for votes...</p>
        </div>
      ) : (
        <ul className="space-y-2">
          {sortedEntries.map((entry, i) => {
            const pct = (entry.total_weight / maxWeight) * 100;
            const isSelected = selected.has(entry.label);
            return (
              <li
                key={entry.label}
                className={`space-y-1 rounded px-1 py-0.5 transition-colors ${
                  mergeMode
                    ? isSelected
                      ? 'bg-indigo-900/40 ring-1 ring-indigo-500'
                      : 'hover:bg-gray-800/50 cursor-pointer'
                    : ''
                }`}
                onClick={mergeMode ? () => toggleSelect(entry.label) : undefined}
              >
                <div className="flex items-center gap-2">
                  {mergeMode && (
                    <input
                      type="checkbox"
                      checked={isSelected}
                      onChange={() => toggleSelect(entry.label)}
                      className="h-3.5 w-3.5 rounded border-gray-600 bg-gray-800 text-indigo-500 focus:ring-indigo-500 focus:ring-offset-0 cursor-pointer"
                      onClick={(e) => e.stopPropagation()}
                    />
                  )}
                  <div className="flex-1 space-y-1">
                    <div className="flex items-center justify-between text-sm">
                      <span className="font-medium text-white">
                        {i + 1}. {entry.label}
                      </span>
                      <span className="text-gray-400 text-xs">
                        {entry.total_weight} pts &middot; {entry.vote_count}{' '}
                        votes
                      </span>
                    </div>
                    <div className="h-3 bg-gray-700 rounded-full overflow-hidden">
                      <div
                        className="h-full bg-indigo-500 rounded-full transition-all duration-500 ease-out"
                        style={{ width: `${pct}%` }}
                      />
                    </div>
                  </div>
                </div>
              </li>
            );
          })}
        </ul>
      )}

      {mergeMode && (
        <div className="flex flex-col gap-3 pt-3 border-t border-gray-700">
          {selected.size >= 2 && (
            <div className="flex items-center gap-2">
              <span className="text-xs text-gray-400">Merge into:</span>
              <div className="flex items-center gap-2">
                <select
                  value={useCustomTarget ? '__custom__' : targetLabel}
                  onChange={(e) => {
                    if (e.target.value === '__custom__') {
                      setUseCustomTarget(true);
                    } else {
                      setUseCustomTarget(false);
                      setTargetLabel(e.target.value);
                    }
                  }}
                  className="text-xs bg-gray-800 border border-gray-600 rounded px-2 py-1 text-white focus:ring-1 focus:ring-indigo-500 focus:outline-none"
                >
                  <option value="" disabled>
                    Select target...
                  </option>
                  {Array.from(selected).map((label) => (
                    <option key={label} value={label}>
                      {label}
                    </option>
                  ))}
                  <option value="__custom__">Custom...</option>
                </select>
                {useCustomTarget && (
                  <input
                    type="text"
                    value={customTarget}
                    onChange={(e) => setCustomTarget(e.target.value)}
                    placeholder="Enter label name"
                    className="text-xs bg-gray-800 border border-gray-600 rounded px-2 py-1 text-white placeholder-gray-500 focus:ring-1 focus:ring-indigo-500 focus:outline-none w-32"
                  />
                )}
              </div>
            </div>
          )}
          <div className="flex items-center gap-3">
            <div className="flex-1 text-xs text-gray-400">
              {selected.size === 0 && 'Select labels to merge'}
              {selected.size === 1 && 'Select at least one more label'}
              {selected.size >= 2 && !useCustomTarget && !targetLabel && (
                <span>Choose a target label above</span>
              )}
              {selected.size >= 2 &&
                (useCustomTarget || targetLabel) &&
                !(!useCustomTarget && !targetLabel) && (
                  <span>
                    Will merge{' '}
                    <span className="text-indigo-400 font-medium">
                      {selected.size - 1}
                    </span>{' '}
                    label(s) into{' '}
                    <span className="text-indigo-400 font-medium">
                      {useCustomTarget
                        ? customTarget || '...'
                        : targetLabel}
                    </span>
                  </span>
                )}
            </div>
            <button
              disabled={
                selected.size < 2 ||
                merging ||
                (!useCustomTarget && !targetLabel) ||
                (useCustomTarget && !customTarget.trim())
              }
              onClick={handleMerge}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded bg-indigo-600 text-white hover:bg-indigo-500 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              <GitMerge size={14} />
              {merging ? 'Merging...' : 'Merge'}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

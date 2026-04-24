import { useRef, useEffect, useState } from 'react';
import { Crown } from 'lucide-react';
import type { LeaderboardEntry } from '../types';

interface Props {
  entries: LeaderboardEntry[];
  maxEntries: number;
}

const ITEM_HEIGHT = 56;

export default function DisplayOverlay({ entries, maxEntries }: Props) {
  const [animatedPoints, setAnimatedPoints] = useState<Record<string, number>>({});
  const animationFrameRef = useRef<number | null>(null);

  const slicedEntries = entries.slice(0, maxEntries);
  const maxWeight = Math.max(...slicedEntries.map((e) => e.total_weight), 1);

  const prevLabelsSet = useRef<Set<string>>(new Set());

  const isNewEntry = (label: string) => !prevLabelsSet.current.has(label);

  useEffect(() => {
    prevLabelsSet.current = new Set(slicedEntries.map((e) => e.label));
  }, [slicedEntries]);

  useEffect(() => {
    if (animationFrameRef.current) {
      cancelAnimationFrame(animationFrameRef.current);
    }

    const startPoints = { ...animatedPoints };
    const targetPoints: Record<string, number> = {};
    slicedEntries.forEach((e) => {
      targetPoints[e.label] = e.total_weight;
    });

    const startTime = performance.now();
    const duration = 500;

    const animate = (now: number) => {
      const elapsed = now - startTime;
      const progress = Math.min(elapsed / duration, 1);
      const eased = 1 - Math.pow(1 - progress, 3);

      const next: Record<string, number> = {};
      for (const label of new Set([...Object.keys(startPoints), ...Object.keys(targetPoints)])) {
        const start = startPoints[label] ?? 0;
        const target = targetPoints[label] ?? 0;
        next[label] = Math.round(start + (target - start) * eased);
      }

      setAnimatedPoints(next);

      if (progress < 1) {
        animationFrameRef.current = requestAnimationFrame(animate);
      }
    };

    animationFrameRef.current = requestAnimationFrame(animate);

    return () => {
      if (animationFrameRef.current) {
        cancelAnimationFrame(animationFrameRef.current);
      }
    };
  }, [slicedEntries]);

  return (
    <div className="relative w-full overflow-hidden" style={{ minHeight: maxEntries * ITEM_HEIGHT }}>
      {slicedEntries.map((entry, index) => {
        const pct = (entry.total_weight / maxWeight) * 100;
        const points = animatedPoints[entry.label] ?? 0;
        const isRank1 = index === 0;
        const isNew = isNewEntry(entry.label);
        const hasDonations = entry.vote_count !== entry.total_weight;

        return (
          <div
            key={entry.label}
            className={`absolute left-0 right-0 flex items-center gap-3 px-4 ${
              isNew ? 'animate-fadeSlideIn' : ''
            }`}
            style={{
              transform: `translateY(${index * ITEM_HEIGHT}px)`,
              height: ITEM_HEIGHT,
              transition: 'transform 400ms ease-out',
              willChange: 'transform',
            }}
          >
            <div className="flex items-center gap-2 w-10 shrink-0">
              {isRank1 ? (
                <Crown
                  size={20}
                  className="text-yellow-400 animate-pulseCrown shrink-0"
                />
              ) : (
                <span
                  className={`text-sm font-bold w-6 h-6 flex items-center justify-center rounded ${
                    index === 1
                      ? 'bg-gray-300 text-gray-800'
                      : index === 2
                        ? 'bg-amber-600 text-white'
                        : 'bg-gray-700 text-gray-300'
                  }`}
                >
                  {index + 1}
                </span>
              )}
            </div>

            <div className="flex-1 min-w-0">
              <div className="flex items-center justify-between mb-1">
                <div className="flex items-center gap-2 min-w-0">
                  <span className="text-white font-medium truncate">
                    {entry.label}
                  </span>
                  {isNew && (
                    <span className="text-xs bg-green-600 text-white px-1.5 py-0.5 rounded shrink-0">
                      NEW
                    </span>
                  )}
                </div>
                <span className="text-gray-300 text-sm shrink-0 ml-2">
                  {points} pts
                </span>
              </div>

              <div className="h-4 bg-gray-800 rounded-full overflow-hidden">
                <div
                  className={`h-full rounded-full transition-all duration-500 ease-out ${
                    hasDonations
                      ? 'bg-gradient-to-r from-indigo-500 to-purple-500'
                      : 'bg-indigo-500'
                  }`}
                  style={{ width: `${pct}%` }}
                />
              </div>
            </div>
          </div>
        );
      })}

      {slicedEntries.length === 0 && (
        <div className="absolute inset-0 flex items-center justify-center">
          <div className="text-center">
            <div className="w-16 h-16 border-4 border-indigo-500 border-t-transparent rounded-full animate-spin mx-auto mb-4" />
            <p className="text-gray-400 text-lg">Waiting for votes...</p>
          </div>
        </div>
      )}
    </div>
  );
}

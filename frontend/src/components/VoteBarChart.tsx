import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  Cell,
} from 'recharts';
import type { LeaderboardEntry } from '../types';

interface Props {
  entries: LeaderboardEntry[];
  maxEntries?: number;
  animated?: boolean;
}

const COLORS = [
  '#6366f1',
  '#8b5cf6',
  '#a78bfa',
  '#c4b5fd',
  '#818cf8',
  '#7c3aed',
  '#6d28d9',
  '#5b21b6',
  '#4c1d95',
  '#7e22ce',
];

export default function VoteBarChart({
  entries,
  maxEntries = 10,
  animated = true,
}: Props) {
  const data = entries
    .slice(0, maxEntries)
    .map((e, i) => ({
      name: e.label,
      value: e.total_weight,
      votes: e.vote_count,
      fill: COLORS[i % COLORS.length],
    }))
    .reverse();

  if (data.length === 0) return null;

  return (
    <ResponsiveContainer width="100%" height={Math.max(data.length * 48, 200)}>
      <BarChart
        layout="vertical"
        data={data}
        margin={{ top: 5, right: 30, left: 20, bottom: 5 }}
      >
        <XAxis type="number" hide />
        <YAxis
          type="category"
          dataKey="name"
          tick={{ fill: '#e5e7eb', fontSize: 14 }}
          width={120}
          axisLine={false}
          tickLine={false}
        />
        <Tooltip
          contentStyle={{
            backgroundColor: '#1f2937',
            border: '1px solid #374151',
            borderRadius: '8px',
            color: '#e5e7eb',
          }}
          formatter={(value: number) => [`${value} pts`, 'Weight'] as [string, string]}
        />
        <Bar
          dataKey="value"
          radius={[0, 4, 4, 0]}
          animationDuration={animated ? 500 : 0}
        >
          {data.map((entry, index) => (
            <Cell key={`cell-${index}`} fill={entry.fill} />
          ))}
        </Bar>
      </BarChart>
    </ResponsiveContainer>
  );
}

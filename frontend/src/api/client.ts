import axios from 'axios';
import type { Topic, CreateTopicRequest, Leaderboard } from '../types';

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || 'http://localhost:8585',
  timeout: 5000,
});

export async function getTopics(): Promise<Topic[]> {
  const { data } = await api.get<Topic[]>('/api/topics');
  return data;
}

export async function getActiveTopic(): Promise<Topic | null> {
  try {
    const { data } = await api.get<Topic>('/api/topics/active');
    return data;
  } catch (err) {
    if (axios.isAxiosError(err) && err.response?.status === 404) {
      return null;
    }
    throw err;
  }
}

export async function createTopic(req: CreateTopicRequest): Promise<Topic> {
  const { data } = await api.post<Topic>('/api/topics', req);
  return data;
}

export async function closeTopic(id: string): Promise<Topic> {
  const { data } = await api.post<Topic>(`/api/topics/${id}/close`);
  return data;
}

export async function getLeaderboard(
  topicId: string,
  limit?: number,
): Promise<Leaderboard> {
  const params: Record<string, string> = { topic_id: topicId };
  if (limit !== undefined) {
    params.limit = String(limit);
  }
  const { data } = await api.get<Leaderboard>('/api/votes/leaderboard', {
    params,
  });
  return data;
}

export async function getLabels(topicId: string): Promise<string[]> {
  const { data } = await api.get<{ labels: string[] }>(
    '/api/votes/labels',
    { params: { topic_id: topicId } },
  );
  return data.labels;
}

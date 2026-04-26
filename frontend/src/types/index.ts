export interface Topic {
  id: string;
  title: string;
  description: string;
  is_active: boolean;
  classifier_threshold: number;
  created_at: string;
  closed_at?: string;
}

export interface CreateTopicRequest {
  title: string;
  description?: string;
  classifier_threshold?: number;
  set_active: boolean;
}

export interface LeaderboardEntry {
  label: string;
  total_weight: number;
  vote_count: number;
  last_vote_at: string;
}

export interface Leaderboard {
  topic_id: string;
  topic: string;
  entries: LeaderboardEntry[];
  updated_at: string;
}

export interface WSMessage<T = unknown> {
  type: string;
  data: T;
}

export interface MergeLabelsRequest {
  topic_id: string;
  source_labels: string[];
  target_label: string;
}

export interface MergeLabelsResponse {
  topic_id: string;
  merged_labels: string[];
  target_label: string;
  votes_affected: number;
}

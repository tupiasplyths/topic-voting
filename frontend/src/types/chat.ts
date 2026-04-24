export interface ChatMessage {
  id: string;
  username: string;
  message: string;
  color: string;
  is_donation: boolean;
  bits_amount: number;
  timestamp: number;
  status?: 'pending' | 'classified';
  classified_label?: string;
}

export type ChatSpeed = 'slow' | 'normal' | 'fast';

export interface ChatConfig {
  speed: ChatSpeed;
  donationRate: number;
}

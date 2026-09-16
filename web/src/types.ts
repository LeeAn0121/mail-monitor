export interface WebEvent {
  when: string;
  type: string;
  glyph: string;
  from: string;
  to: string;
  subject: string;
  result: string;
  fromIp: string;
  toIp: string;
}

export interface RankEntry {
  addr: string;
  count: number;
}

export interface Snapshot {
  events: WebEvent[];
  counts: Record<string, number>;
  senderRanking: RankEntry[];
  receiverRanking: RankEntry[];
  alertActive: boolean;
}

export interface HistoryResponse {
  events: WebEvent[];
  error?: string;
}

export const EVENT_TYPES = ["LOGIN", "RECV", "SENT", "FWD", "BOUNCE", "REJECT"] as const;
export type EventTypeName = (typeof EVENT_TYPES)[number];

export interface WebEvent {
  when: string;
  type: string;
  glyph: string;
  from: string;
  to: string;
  fromDisplay: string;
  toDisplay: string;
  origTo: string;
  subject: string;
  result: string;
  resultDetail: string;
  fromIp: string;
  toIp: string;
  raw: string;
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

export interface VersionInfo {
  version: string;
  releaseUrl: string;
  releasesUrl: string;
}

export interface BlockedSender {
  email: string;
  addedAt: string;
}

export interface HistoryResponse {
  events: WebEvent[];
  error?: string;
}

export const EVENT_TYPES = ["LOGIN", "RECV", "SENT", "FWD", "BOUNCE", "REJECT"] as const;
export type EventTypeName = (typeof EVENT_TYPES)[number];

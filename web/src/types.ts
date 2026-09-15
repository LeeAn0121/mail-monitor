export interface WebEvent {
  when: string;
  type: string;
  glyph: string;
  text: string;
  from: string;
  to: string;
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

export const EVENT_TYPES = ["LOGIN", "RECV", "SENT", "FWD", "BOUNCE", "REJECT"] as const;

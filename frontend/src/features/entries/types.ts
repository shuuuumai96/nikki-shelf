export type MoodKey = "happy" | "calm" | "tired" | "sad" | "excited";

export type EntryImage = {
  id: number;
  entryId: number;
  url: string;
  fileName: string;
  size: number;
  mimeType: string;
  createdAt: string;
};

export type Entry = {
  id: number;
  entryDate: string;
  title: string;
  body: string;
  mood: MoodKey;
  tags: string[];
  images: EntryImage[];
  version: number;
  createdAt: string;
  updatedAt: string;
};

export type EntryDateLookup = {
  entry: Entry | null;
  date: string;
  exists: boolean;
};

export type EntryInput = {
  entryDate: string;
  title: string;
  body: string;
  mood: MoodKey;
  tags: string[];
};

export type SaveStatus =
  | "idle"
  | "dirty"
  | "saving"
  | "saved"
  | "failed"
  | "conflict";

export type EntryFilter = {
  query?: string;
  tag?: string;
  mood?: string;
  from?: string;
  to?: string;
  per_page?: string;
  cursor?: string;
};

export type EntryPage = {
  items: Entry[];
  nextCursor: string;
  hasMore: boolean;
};

export type EntrySearchFilter = {
  q?: string;
  from?: string;
  to?: string;
  mood?: string;
  tag?: string;
  hasImage?: string;
  limit?: string;
  offset?: string;
};

export type EntrySearchResult = {
  id: number;
  entryDate: string;
  title: string;
  preview: string;
  mood: string;
  tags: string[];
  hasImage: boolean;
  imageCount: number;
  updatedAt: string;
};

export type EntrySearchResponse = {
  results: EntrySearchResult[];
};

export type Stats = {
  totalEntries: number;
  currentStreak: number;
  moodCounts: Record<MoodKey, number>;
  lastEntryDate: string;
};

import { defineStore } from "pinia";
import { ApiError } from "../../shared/api/client";
import { i18n } from "../../shared/i18n";
import { todayISO } from "../../shared/utils/date";
import {
  createEntry,
  deleteEntry,
  deleteImage,
  getEntry,
  getEntryByDate,
  getStats,
  listEntries,
  listTags,
  updateEntry,
  uploadImage as uploadEntryImage,
  type UploadImageRequest,
  uploadImages,
} from "./api";
import type {
  Entry,
  EntryDateLookup,
  EntryFilter,
  EntryInput,
  SaveStatus,
  Stats,
} from "./types";

type EntryState = {
  entries: Entry[];
  entriesNextCursor: string;
  entriesHasMore: boolean;
  entriesFilter: EntryFilter;
  activeEntry: Entry | null;
  activeDate: string;
  tags: string[];
  stats: Stats | null;
  loading: boolean;
  saving: boolean;
  saveStatus: SaveStatus;
  saveError: string;
  autosaveInFlight: boolean;
  pendingAutosave: boolean;
  latestAutosaveInput: EntryInput | null;
  autosaveRevision: number;
  savedRevision: number;
  imageEntryPromise: Promise<Entry> | null;
  error: string;
};

export const useEntryStore = defineStore("entries", {
  state: (): EntryState => ({
    entries: [],
    entriesNextCursor: "",
    entriesHasMore: false,
    entriesFilter: {},
    activeEntry: null,
    activeDate: todayISO(),
    tags: [],
    stats: null,
    loading: false,
    saving: false,
    saveStatus: "idle",
    saveError: "",
    autosaveInFlight: false,
    pendingAutosave: false,
    latestAutosaveInput: null,
    autosaveRevision: 0,
    savedRevision: 0,
    imageEntryPromise: null,
    error: "",
  }),
  actions: {
    async bootstrap() {
      await Promise.all([
        this.loadEntries(),
        this.loadTags(),
        this.loadStats(),
        this.loadEntryByDate(this.activeDate),
      ]);
    },
    async loadEntries(filter: EntryFilter = {}) {
      this.loading = true;
      this.error = "";
      try {
        const page = await listEntries({ ...filter, per_page: "50" });
        this.entries = page.items;
        this.entriesNextCursor = page.nextCursor;
        this.entriesHasMore = page.hasMore;
        this.entriesFilter = { ...filter };
      } catch (error) {
        this.error = errorMessage(error);
      } finally {
        this.loading = false;
      }
    },
    async loadMoreEntries() {
      if (!this.entriesHasMore || !this.entriesNextCursor || this.loading) {
        return;
      }

      this.loading = true;
      this.error = "";
      try {
        const page = await listEntries({
          ...this.entriesFilter,
          per_page: "50",
          cursor: this.entriesNextCursor,
        });
        const existingIds = new Set(this.entries.map((entry) => entry.id));
        this.entries = [
          ...this.entries,
          ...page.items.filter((entry) => !existingIds.has(entry.id)),
        ];
        this.entriesNextCursor = page.nextCursor;
        this.entriesHasMore = page.hasMore;
      } catch (error) {
        this.error = errorMessage(error);
      } finally {
        this.loading = false;
      }
    },
    async loadEntryByDate(date: string): Promise<EntryDateLookup | null> {
      this.error = "";
      try {
        const lookup = await getEntryByDate(date);
        this.activeEntry = lookup.entry;
        this.activeDate = lookup.entry?.entryDate ?? lookup.date;
        this.clearAutosaveState();
        return lookup;
      } catch (error) {
        this.error = errorMessage(error);
        return null;
      }
    },
    async save(input: EntryInput, files: File[]) {
      this.saving = true;
      this.error = "";
      try {
        const saved = this.activeEntry
          ? await updateEntry(
              this.activeEntry.id,
              input,
              this.activeEntry.version,
            )
          : await createEntry(input);

        if (files.length > 0) {
          await uploadImages(saved.id, files);
        }

        this.activeEntry = await getEntry(saved.id);
        this.activeDate = this.activeEntry.entryDate;
        this.saveStatus = "saved";
        this.saveError = "";
        await this.refreshEntrySummaries();
      } catch (error) {
        this.error = errorMessage(error);
        this.saveStatus = isConflict(error) ? "conflict" : "failed";
        this.saveError =
          this.saveStatus === "conflict" ? conflictMessage(error) : this.error;
      } finally {
        this.saving = false;
      }
    },
    uploadImage(
      input: EntryInput,
      file: File,
      onProgress: (progress: number) => void,
    ): UploadImageRequest {
      this.error = "";
      let uploadRequest: UploadImageRequest | null = null;
      let aborted = false;

      const promise = (async () => {
        const targetDate = input.entryDate;
        const startingEntryID = this.activeEntry?.id ?? null;
        const entry = await this.ensureImageEntry(input);
        if (aborted) {
          throw new ApiError(0, i18n.global.t("entries.uploadAborted"));
        }

        uploadRequest = uploadEntryImage(entry.id, file, onProgress);
        const image = await uploadRequest.promise;
        await this.refreshAfterImageUpload(
          entry.id,
          targetDate,
          startingEntryID,
        );
        return image;
      })();

      return {
        promise,
        abort: () => {
          aborted = true;
          uploadRequest?.abort();
        },
      };
    },
    async autosave(input: EntryInput) {
      if (!this.isCurrentAutosaveTarget(input)) {
        return;
      }

      this.latestAutosaveInput = cloneInput(input);
      this.autosaveRevision += 1;
      this.pendingAutosave = true;
      this.saveError = "";

      if (this.saveStatus !== "saving") {
        this.saveStatus = "dirty";
      }

      if (this.autosaveInFlight) {
        return;
      }

      await this.runAutosaveQueue();
    },
    async runAutosaveQueue() {
      if (this.autosaveInFlight) {
        return;
      }

      this.autosaveInFlight = true;
      try {
        while (this.pendingAutosave && this.latestAutosaveInput) {
          this.pendingAutosave = false;
          const revision = this.autosaveRevision;
          const input = cloneInput(this.latestAutosaveInput);

          if (!this.isCurrentAutosaveTarget(input)) {
            continue;
          }

          this.saveStatus = "saving";
          this.saveError = "";

          try {
            const saved = this.activeEntry
              ? await updateEntry(
                  this.activeEntry.id,
                  input,
                  this.activeEntry.version,
                )
              : await createEntry(input);

            if (!this.isCurrentAutosaveTarget(input)) {
              continue;
            }

            if (this.autosaveRevision === revision && !this.pendingAutosave) {
              this.activeEntry = await getEntry(saved.id);
              this.activeDate = this.activeEntry.entryDate;
              await this.refreshEntrySummaries();
              this.savedRevision = revision;
              this.saveStatus = "saved";
            } else if (!this.activeEntry) {
              this.activeEntry = {
                ...saved,
                ...cloneInput(this.latestAutosaveInput),
                id: saved.id,
                images: saved.images,
                version: saved.version,
                createdAt: saved.createdAt,
                updatedAt: saved.updatedAt,
              };
            }
          } catch (error) {
            this.error = errorMessage(error);
            this.saveStatus = isConflict(error) ? "conflict" : "failed";
            this.saveError =
              this.saveStatus === "conflict"
                ? conflictMessage(error)
                : this.error;
            this.pendingAutosave = false;
            break;
          }
        }
      } finally {
        this.autosaveInFlight = false;
        if (this.pendingAutosave && this.saveStatus !== "failed") {
          await this.runAutosaveQueue();
        }
      }
    },
    async removeActive() {
      if (!this.activeEntry) {
        return;
      }

      this.saving = true;
      this.error = "";
      try {
        await deleteEntry(this.activeEntry.id);
        this.activeEntry = null;
        await this.refreshEntrySummaries();
      } catch (error) {
        this.error = errorMessage(error);
      } finally {
        this.saving = false;
      }
    },
    async removeImage(imageId: number) {
      if (!this.activeEntry) {
        return;
      }

      this.error = "";
      try {
        await deleteImage(imageId);
        this.activeEntry = await getEntry(this.activeEntry.id);
        await this.loadEntries();
      } catch (error) {
        this.error = errorMessage(error);
      }
    },
    async reloadActive() {
      if (!this.activeEntry) {
        return;
      }

      this.error = "";
      this.saveError = "";
      this.activeEntry = await getEntry(this.activeEntry.id);
      this.activeDate = this.activeEntry.entryDate;
      this.saveStatus = "saved";
    },
    async ensureImageEntry(input: EntryInput): Promise<Entry> {
      if (this.activeEntry) {
        return this.activeEntry;
      }

      if (this.imageEntryPromise) {
        return this.imageEntryPromise;
      }

      this.imageEntryPromise = this.createImageEntry(input);
      try {
        return await this.imageEntryPromise;
      } finally {
        this.imageEntryPromise = null;
      }
    },
    async createImageEntry(input: EntryInput): Promise<Entry> {
      await this.waitForAutosaveIdle();
      if (this.activeEntry) {
        return this.activeEntry;
      }

      try {
        this.activeEntry = await createEntry(cloneInput(input));
      } catch (error) {
        if (!isConflict(error)) {
          throw error;
        }
        const lookup = await getEntryByDate(input.entryDate);
        if (!lookup.entry) {
          throw error;
        }
        this.activeEntry = lookup.entry;
      }

      this.activeDate = this.activeEntry.entryDate;
      await this.refreshEntrySummaries();
      return this.activeEntry;
    },
    async refreshAfterImageUpload(
      entryID: number,
      targetDate: string,
      startingEntryID: number | null,
    ) {
      if (this.isCurrentImageTarget(entryID, targetDate, startingEntryID)) {
        this.activeEntry = await getEntry(entryID);
        this.activeDate = this.activeEntry.entryDate;
        await this.refreshEntrySummaries();
        return;
      }

      await this.refreshEntrySummaries();
    },
    isCurrentImageTarget(
      entryID: number,
      targetDate: string,
      startingEntryID: number | null,
    ) {
      if (this.activeEntry) {
        return (
          this.activeEntry.id === entryID && this.activeDate === targetDate
        );
      }

      return startingEntryID === null && this.activeDate === targetDate;
    },
    isCurrentAutosaveTarget(input: EntryInput) {
      return this.activeDate === input.entryDate;
    },
    clearAutosaveState() {
      this.pendingAutosave = false;
      this.latestAutosaveInput = null;
      this.saveError = "";
      this.saveStatus = "idle";
    },
    async waitForAutosaveIdle() {
      while (this.autosaveInFlight) {
        await new Promise((resolve) => window.setTimeout(resolve, 50));
      }
    },
    async loadStats() {
      this.stats = await getStats();
    },
    async loadTags() {
      this.tags = await listTags();
    },
    async refreshEntrySummaries() {
      await Promise.all([
        this.loadEntries(),
        this.loadTags(),
        this.loadStats(),
      ]);
    },
    clear() {
      this.entries = [];
      this.entriesNextCursor = "";
      this.entriesHasMore = false;
      this.entriesFilter = {};
      this.activeEntry = null;
      this.activeDate = todayISO();
      this.tags = [];
      this.stats = null;
      this.loading = false;
      this.saving = false;
      this.saveStatus = "idle";
      this.saveError = "";
      this.autosaveInFlight = false;
      this.pendingAutosave = false;
      this.latestAutosaveInput = null;
      this.autosaveRevision = 0;
      this.savedRevision = 0;
      this.imageEntryPromise = null;
      this.error = "";
    },
  },
});

function cloneInput(input: EntryInput): EntryInput {
  return { ...input, tags: [...input.tags] };
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return i18n.global.t("errors.generic");
}

function isConflict(error: unknown): boolean {
  return error instanceof ApiError && error.status === 409;
}

function conflictMessage(error: unknown): string {
  if (error instanceof ApiError && error.kind === "entries.stale_version") {
    return i18n.global.t("entries.conflictStaleVersion");
  }

  if (error instanceof ApiError && error.kind === "entries.date_exists") {
    return i18n.global.t("entries.conflictDateExists");
  }

  return errorMessage(error);
}

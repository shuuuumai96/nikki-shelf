import { computed, ref, type Ref } from "vue";
import { localizedErrorMessage } from "../../../shared/api/client";
import {
  IMAGE_UPLOAD_MAX_BYTES,
  SUPPORTED_IMAGE_TYPES,
  type UploadImageRequest,
} from "../api";
import type { Entry, EntryImage, EntryInput } from "../types";

export type UploadStatus = "preparing" | "uploading" | "failed";

export type UploadImageItem = {
  id: string;
  file: File;
  signature: string;
  previewUrl: string;
  objectUrl: string;
  status: UploadStatus;
  progress: number;
  error: string;
  persisted: EntryImage | null;
  started: boolean;
  active: boolean;
  upload: UploadImageRequest | null;
};

type Translate = (key: string) => string;

type UploadImage = (payload: {
  input: EntryInput;
  file: File;
  onProgress: (progress: number) => void;
}) => UploadImageRequest;

export function useEntryImageUploads(options: {
  deleteImage: (imageId: number) => void;
  entry: Readonly<Ref<Entry | null>>;
  form: EntryInput;
  maxImages?: number;
  t: Translate;
  uploadImage: UploadImage;
}) {
  const uploadImages = ref<UploadImageItem[]>([]);
  const maxImages = options.maxImages ?? 3;

  const imageSlotsLeft = computed(() =>
    Math.max(
      0,
      maxImages -
        (options.entry.value?.images.length || 0) -
        uploadImages.value.length,
    ),
  );
  const persistedImages = computed(() => options.entry.value?.images || []);

  function queueFiles(files: File[]) {
    const available = imageSlotsLeft.value;
    if (available === 0) {
      return;
    }

    const signatures = new Set(
      uploadImages.value.map((image) => image.signature),
    );
    const selected = files
      .filter(isSupportedImage)
      .filter((file) => {
        const signature = fileSignature(file);
        if (signatures.has(signature)) {
          return false;
        }
        signatures.add(signature);
        return true;
      })
      .slice(0, available);

    const items = selected.map((file) => createUploadItem(file));
    uploadImages.value = [...uploadImages.value, ...items];
    const uploadableItems: UploadImageItem[] = [];
    items.forEach((item) => {
      if (item.file.size > IMAGE_UPLOAD_MAX_BYTES) {
        item.status = "failed";
        item.error = options.t("images.maxSize");
        return;
      }
      uploadableItems.push(item);
    });

    uploadableItems.forEach((item) => {
      void startUpload(item);
    });
  }

  function retryUpload(item: UploadImageItem) {
    if (item.status !== "failed") {
      return;
    }

    void startUpload(item);
  }

  function removeUpload(item: UploadImageItem) {
    item.active = false;
    item.upload?.abort();
    revokeObjectUrl(item);
    uploadImages.value = uploadImages.value.filter(
      (image) => image.id !== item.id,
    );
    if (item.persisted) {
      options.deleteImage(item.persisted.id);
    }
  }

  function removePersistedImage(image: EntryImage) {
    options.deleteImage(image.id);
  }

  function clearUploadImages() {
    uploadImages.value.forEach((image) => {
      image.active = false;
      image.upload?.abort();
      revokeObjectUrl(image);
    });
    uploadImages.value = [];
  }

  function hasSupportedImageFiles(files: File[]) {
    return files.some(isSupportedImage);
  }

  function createUploadItem(file: File): UploadImageItem {
    const objectUrl = URL.createObjectURL(file);
    return {
      id: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
      file,
      signature: fileSignature(file),
      previewUrl: objectUrl,
      objectUrl,
      status: "preparing",
      progress: 0,
      error: "",
      persisted: null,
      started: false,
      active: true,
      upload: null,
    };
  }

  async function startUpload(item: UploadImageItem) {
    if (item.started) {
      return;
    }

    item.started = true;
    item.active = true;
    item.status = "uploading";
    item.progress = 0;
    item.error = "";

    try {
      const upload = options.uploadImage({
        input: { ...options.form, tags: [...options.form.tags] },
        file: item.file,
        onProgress: (progress) => {
          if (!item.active) {
            return;
          }
          item.progress = progress;
        },
      });
      item.upload = upload;
      const image = await upload.promise;

      if (
        !item.active ||
        !uploadImages.value.some((current) => current.id === item.id)
      ) {
        return;
      }

      item.persisted = image;
      item.progress = 100;
      removeCompletedUpload(item);
    } catch (error) {
      if (!item.active) {
        return;
      }

      item.status = "failed";
      item.error = localizedErrorMessage(error);
      item.started = false;
    } finally {
      if (item.upload) {
        item.upload = null;
      }
    }
  }

  function removeCompletedUpload(item: UploadImageItem) {
    item.active = false;
    revokeObjectUrl(item);
    uploadImages.value = uploadImages.value.filter(
      (image) => image.id !== item.id,
    );
  }

  return {
    clearUploadImages,
    hasSupportedImageFiles,
    imageSlotsLeft,
    persistedImages,
    queueFiles,
    removePersistedImage,
    removeUpload,
    retryUpload,
    uploadImages,
  };
}

function isSupportedImage(file: File) {
  return SUPPORTED_IMAGE_TYPES.includes(file.type);
}

function fileSignature(file: File) {
  return `${file.name}:${file.size}:${file.lastModified}`;
}

function revokeObjectUrl(item: UploadImageItem) {
  if (!item.objectUrl) {
    return;
  }

  URL.revokeObjectURL(item.objectUrl);
  item.objectUrl = "";
}

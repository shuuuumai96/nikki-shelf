export type SetupStatus = {
  needsSetup: boolean;
  setupLocked: boolean;
  canCreateOwner: boolean;
  canRestoreBackup: boolean;
  requiresSetupToken: boolean;
  restoreInProgress: boolean;
};

export type SetupOwnerInput = {
  setupToken: string;
  username: string;
  password: string;
};

export type SetupRestoreFileInput = {
  setupToken: string;
  backupFile: File;
};

export type SetupRestoreInput = SetupRestoreFileInput & {
  confirmRestore: boolean;
};

export type SetupRestoreVerifyResponse = {
  valid: boolean;
  backupCreatedAt: string;
  nikkiVersion: string;
  schemaVersion: string;
  entryCount: number;
  imageCount: number;
  backupSizeBytes: number;
  warnings: string[];
};

export type SetupRestoreResponse = {
  restored: boolean;
  entryCount: number;
  imageCount: number;
};

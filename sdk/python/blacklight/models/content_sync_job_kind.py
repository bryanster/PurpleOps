from enum import Enum


class ContentSyncJobKind(str, Enum):
    BUNDLE_IMPORT = "bundle_import"
    REPROCESS = "reprocess"
    SYNC = "sync"
    V1_IMPORT = "v1_import"

    def __str__(self) -> str:
        return str(self.value)

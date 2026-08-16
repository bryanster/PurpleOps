from enum import Enum


class ContentSyncJobStatus(str, Enum):
    CANCELLED = "cancelled"
    CANCELLING = "cancelling"
    FAILED = "failed"
    INTERRUPTED = "interrupted"
    QUEUED = "queued"
    RUNNING = "running"
    SUCCEEDED = "succeeded"

    def __str__(self) -> str:
        return str(self.value)

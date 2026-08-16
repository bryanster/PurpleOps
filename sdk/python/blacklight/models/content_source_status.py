from enum import Enum


class ContentSourceStatus(str, Enum):
    ERROR = "error"
    IDLE = "idle"
    SYNCING = "syncing"

    def __str__(self) -> str:
        return str(self.value)

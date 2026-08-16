from enum import Enum


class BurndownInterval(str, Enum):
    DAILY = "daily"
    WEEKLY = "weekly"

    def __str__(self) -> str:
        return str(self.value)

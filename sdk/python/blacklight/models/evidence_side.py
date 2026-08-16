from enum import Enum


class EvidenceSide(str, Enum):
    BLUE = "blue"
    RED = "red"

    def __str__(self) -> str:
        return str(self.value)

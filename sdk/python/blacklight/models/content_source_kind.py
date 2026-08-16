from enum import Enum


class ContentSourceKind(str, Enum):
    ATOMIC = "atomic"
    ATTACK = "attack"
    CTID = "ctid"
    CUSTOM = "custom"
    SIGMA = "sigma"

    def __str__(self) -> str:
        return str(self.value)

from enum import Enum


class GuestRegisterResultPlatformRole(str, Enum):
    MEMBER = "member"

    def __str__(self) -> str:
        return str(self.value)

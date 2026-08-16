from enum import Enum


class ContentSoftwareType(str, Enum):
    MALWARE = "malware"
    TOOL = "tool"

    def __str__(self) -> str:
        return str(self.value)

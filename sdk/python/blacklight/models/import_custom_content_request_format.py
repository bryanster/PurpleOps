from enum import Enum


class ImportCustomContentRequestFormat(str, Enum):
    AUTO = "auto"
    KNOWLEDGEBASE_YAML = "knowledgebase_yaml"
    TESTCASES_JSON = "testcases_json"
    TESTCASES_YAML = "testcases_yaml"

    def __str__(self) -> str:
        return str(self.value)

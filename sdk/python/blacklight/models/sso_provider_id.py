from enum import Enum


class SSOProviderId(str, Enum):
    OIDC = "oidc"
    SAML = "saml"

    def __str__(self) -> str:
        return str(self.value)

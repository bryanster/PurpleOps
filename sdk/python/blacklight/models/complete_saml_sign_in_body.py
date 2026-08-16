from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset


T = TypeVar("T", bound="CompleteSamlSignInBody")


@_attrs_define
class CompleteSamlSignInBody:
    saml_response: str
    """ The base64-encoded `<samlp:Response>`. """
    relay_state: str | Unset = UNSET
    """ Echoed back from the authentication request, or set by the
    provider for an IdP-initiated sign-in. It is **not** trusted
    and nothing is read out of it: the SAML profile caps it at 80
    bytes, which is too small to seal, so everything this sign-in
    needs to remember is in the `bl_saml` cookie instead.
     """

    def to_dict(self) -> dict[str, Any]:
        saml_response = self.saml_response

        relay_state = self.relay_state

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "SAMLResponse": saml_response,
            }
        )
        if relay_state is not UNSET:
            field_dict["RelayState"] = relay_state

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        saml_response = d.pop("SAMLResponse")

        relay_state = d.pop("RelayState", UNSET)

        complete_saml_sign_in_body = cls(
            saml_response=saml_response,
            relay_state=relay_state,
        )

        return complete_saml_sign_in_body

from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="TOTPEnrolment")


@_attrs_define
class TOTPEnrolment:
    """A newly minted, unconfirmed authenticator secret, in the three forms a
    person might need to get it into an app. Every field carries the same
    secret; this is the only response in the API that does.

    """

    otpauth_uri: str
    """ The `otpauth://totp/...` URI, for a client that can hand a URI to an
    authenticator directly.
     """
    secret: str
    """ The base32 shared secret, for typing in by hand. It is the same
    value that is inside `otpauthUri`, spelled out so a client does not
    have to parse one to show the other.
     """
    qr_code: str
    """ The URI rendered as a PNG in a `data:` URI, ready to be the `src` of
    an `<img>`. A data URI rather than SVG markup so that showing it
    does not mean putting server-supplied markup into the page; the
    content security policy already allows `img-src data:`.
     """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        otpauth_uri = self.otpauth_uri

        secret = self.secret

        qr_code = self.qr_code

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "otpauthUri": otpauth_uri,
                "secret": secret,
                "qrCode": qr_code,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        otpauth_uri = d.pop("otpauthUri")

        secret = d.pop("secret")

        qr_code = d.pop("qrCode")

        totp_enrolment = cls(
            otpauth_uri=otpauth_uri,
            secret=secret,
            qr_code=qr_code,
        )

        totp_enrolment.additional_properties = d
        return totp_enrolment

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties

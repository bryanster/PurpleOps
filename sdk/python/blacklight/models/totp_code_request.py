from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="TOTPCodeRequest")


@_attrs_define
class TOTPCodeRequest:
    """A six-digit code from an authenticator app. The same body confirms an
    enrolment and completes a sign-in.

    """

    code: str
    """ The code as the app shows it, without the space some apps put in the
    middle. Six digits is the whole vocabulary — a value that is not six
    digits is rejected as malformed rather than counted as a guess.
     """

    def to_dict(self) -> dict[str, Any]:
        code = self.code

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "code": code,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        code = d.pop("code")

        totp_code_request = cls(
            code=code,
        )

        return totp_code_request

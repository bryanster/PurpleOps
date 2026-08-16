from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="DisableTOTPRequest")


@_attrs_define
class DisableTOTPRequest:
    """Body of `DELETE /auth/mfa/totp`. The current password is required for the
    same reason `ChangePasswordRequest` asks for it.

    """

    current_password: str

    def to_dict(self) -> dict[str, Any]:
        current_password = self.current_password

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "currentPassword": current_password,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        current_password = d.pop("currentPassword")

        disable_totp_request = cls(
            current_password=current_password,
        )

        return disable_totp_request

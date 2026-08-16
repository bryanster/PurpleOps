from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="RegenerateRecoveryCodesRequest")


@_attrs_define
class RegenerateRecoveryCodesRequest:
    """Body of `POST /auth/mfa/recovery/regenerate`. The current password is
    required for the same reason `ChangePasswordRequest` asks for it; the
    second factor is not in the body, it is the requirement that this
    session has already satisfied one.

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

        regenerate_recovery_codes_request = cls(
            current_password=current_password,
        )

        return regenerate_recovery_codes_request

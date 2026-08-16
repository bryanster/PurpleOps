from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.guest_register_result_platform_role import GuestRegisterResultPlatformRole
from uuid import UUID


T = TypeVar("T", bound="GuestRegisterResult")


@_attrs_define
class GuestRegisterResult:
    id: UUID
    email: str
    platform_role: GuestRegisterResultPlatformRole

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        email = self.email

        platform_role = self.platform_role.value

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "id": id,
                "email": email,
                "platformRole": platform_role,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        email = d.pop("email")

        platform_role = GuestRegisterResultPlatformRole(d.pop("platformRole"))

        guest_register_result = cls(
            id=id,
            email=email,
            platform_role=platform_role,
        )

        return guest_register_result

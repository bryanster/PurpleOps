from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset


T = TypeVar("T", bound="GuestRegisterRequest")


@_attrs_define
class GuestRegisterRequest:
    email: str
    """ Email address for the new account. """
    password: str
    """ Password for the new account. """
    display_name: str | Unset = UNSET
    """ Optional display name. """

    def to_dict(self) -> dict[str, Any]:
        email = self.email

        password = self.password

        display_name = self.display_name

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "email": email,
                "password": password,
            }
        )
        if display_name is not UNSET:
            field_dict["displayName"] = display_name

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        email = d.pop("email")

        password = d.pop("password")

        display_name = d.pop("displayName", UNSET)

        guest_register_request = cls(
            email=email,
            password=password,
            display_name=display_name,
        )

        return guest_register_request

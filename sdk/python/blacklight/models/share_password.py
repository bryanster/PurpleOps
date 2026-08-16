from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="SharePassword")


@_attrs_define
class SharePassword:
    password: str
    """ The share password. """

    def to_dict(self) -> dict[str, Any]:
        password = self.password

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "password": password,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        password = d.pop("password")

        share_password = cls(
            password=password,
        )

        return share_password

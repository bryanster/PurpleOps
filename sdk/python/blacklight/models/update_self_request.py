from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="UpdateSelfRequest")


@_attrs_define
class UpdateSelfRequest:
    """Body of `PATCH /users/me`. One field, and that is the design: a schema
    with no `platformRole` in it is a request that cannot ask for one, which
    is a stronger guarantee than a handler that declines to honour it
    (PLAN.md §4).

    """

    display_name: str

    def to_dict(self) -> dict[str, Any]:
        display_name = self.display_name

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "displayName": display_name,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        display_name = d.pop("displayName")

        update_self_request = cls(
            display_name=display_name,
        )

        return update_self_request

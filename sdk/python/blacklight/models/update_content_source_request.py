from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset


T = TypeVar("T", bound="UpdateContentSourceRequest")


@_attrs_define
class UpdateContentSourceRequest:
    """A patch. Every field is optional; an absent field is left alone. `kind`
    is deliberately not here.

    """

    name: str | Unset = UNSET
    url: str | Unset = UNSET
    ref: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        url = self.url

        ref = self.ref

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if name is not UNSET:
            field_dict["name"] = name
        if url is not UNSET:
            field_dict["url"] = url
        if ref is not UNSET:
            field_dict["ref"] = ref

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        name = d.pop("name", UNSET)

        url = d.pop("url", UNSET)

        ref = d.pop("ref", UNSET)

        update_content_source_request = cls(
            name=name,
            url=url,
            ref=ref,
        )

        return update_content_source_request

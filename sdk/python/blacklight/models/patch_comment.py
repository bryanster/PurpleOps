from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="PatchComment")


@_attrs_define
class PatchComment:
    body: str
    """ The replacement body. Prior body is saved as a revision. """

    def to_dict(self) -> dict[str, Any]:
        body = self.body

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "body": body,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        body = d.pop("body")

        patch_comment = cls(
            body=body,
        )

        return patch_comment

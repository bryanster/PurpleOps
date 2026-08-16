from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="CategoryBucket")


@_attrs_define
class CategoryBucket:
    category: str
    count: int

    def to_dict(self) -> dict[str, Any]:
        category = self.category

        count = self.count

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "category": category,
                "count": count,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        category = d.pop("category")

        count = d.pop("count")

        category_bucket = cls(
            category=category,
            count=count,
        )

        return category_bucket

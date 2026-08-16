from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="DistributionBucket")


@_attrs_define
class DistributionBucket:
    label: str
    count: int

    def to_dict(self) -> dict[str, Any]:
        label = self.label

        count = self.count

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "label": label,
                "count": count,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        label = d.pop("label")

        count = d.pop("count")

        distribution_bucket = cls(
            label=label,
            count=count,
        )

        return distribution_bucket

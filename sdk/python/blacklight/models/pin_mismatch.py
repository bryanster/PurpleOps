from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="PinMismatch")


@_attrs_define
class PinMismatch:
    baseline: str
    current: str

    def to_dict(self) -> dict[str, Any]:
        baseline = self.baseline

        current = self.current

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "baseline": baseline,
                "current": current,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        baseline = d.pop("baseline")

        current = d.pop("current")

        pin_mismatch = cls(
            baseline=baseline,
            current=current,
        )

        return pin_mismatch

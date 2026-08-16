from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from typing import cast
from uuid import UUID


T = TypeVar("T", bound="ReorderScenarios")


@_attrs_define
class ReorderScenarios:
    ids: list[UUID]
    """ Ordered list of scenario ids. Every scenario in the engagement must appear exactly once. Ordinals are
    reassigned 1..N to match this order. """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        ids = []
        for ids_item_data in self.ids:
            ids_item = str(ids_item_data)
            ids.append(ids_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "ids": ids,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        ids = []
        _ids = d.pop("ids")
        for ids_item_data in _ids:
            ids_item = UUID(ids_item_data)

            ids.append(ids_item)

        reorder_scenarios = cls(
            ids=ids,
        )

        reorder_scenarios.additional_properties = d
        return reorder_scenarios

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties

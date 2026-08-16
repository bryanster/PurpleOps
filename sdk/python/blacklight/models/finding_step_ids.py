from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from typing import cast
from uuid import UUID


T = TypeVar("T", bound="FindingStepIds")


@_attrs_define
class FindingStepIds:
    step_ids: list[UUID]
    """ The complete set of step ids for this finding. """

    def to_dict(self) -> dict[str, Any]:
        step_ids = []
        for step_ids_item_data in self.step_ids:
            step_ids_item = str(step_ids_item_data)
            step_ids.append(step_ids_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "stepIds": step_ids,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        step_ids = []
        _step_ids = d.pop("stepIds")
        for step_ids_item_data in _step_ids:
            step_ids_item = UUID(step_ids_item_data)

            step_ids.append(step_ids_item)

        finding_step_ids = cls(
            step_ids=step_ids,
        )

        return finding_step_ids

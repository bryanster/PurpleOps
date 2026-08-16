from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from uuid import UUID


T = TypeVar("T", bound="PresenceEntryFocus")


@_attrs_define
class PresenceEntryFocus:
    step_id: UUID | Unset = UNSET
    """ The step the user is most recently focused on. """
    execution_id: UUID | Unset = UNSET
    """ The execution the user is most recently focused on. """

    def to_dict(self) -> dict[str, Any]:
        step_id: str | Unset = UNSET
        if not isinstance(self.step_id, Unset):
            step_id = str(self.step_id)

        execution_id: str | Unset = UNSET
        if not isinstance(self.execution_id, Unset):
            execution_id = str(self.execution_id)

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if step_id is not UNSET:
            field_dict["stepId"] = step_id
        if execution_id is not UNSET:
            field_dict["executionId"] = execution_id

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        _step_id = d.pop("stepId", UNSET)
        step_id: UUID | Unset
        if isinstance(_step_id, Unset):
            step_id = UNSET
        else:
            step_id = UUID(_step_id)

        _execution_id = d.pop("executionId", UNSET)
        execution_id: UUID | Unset
        if isinstance(_execution_id, Unset):
            execution_id = UNSET
        else:
            execution_id = UUID(_execution_id)

        presence_entry_focus = cls(
            step_id=step_id,
            execution_id=execution_id,
        )

        return presence_entry_focus

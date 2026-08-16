from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast
import datetime


T = TypeVar("T", bound="SetupState")


@_attrs_define
class SetupState:
    """Whether this installation has been through the first-run wizard."""

    completed: bool
    """ Whether somebody finished the wizard. A decision, not an outcome:
    it does not mean content is installed.
     """
    completed_at: datetime.datetime | Unset = UNSET
    """ When it was finished. Absent while `completed` is false. """
    completed_by: None | str | Unset = UNSET
    """ The user who finished it, or null when nothing did — `blctl setup
    complete` on a provisioning run has no person to attribute.
     """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        completed = self.completed

        completed_at: str | Unset = UNSET
        if not isinstance(self.completed_at, Unset):
            completed_at = self.completed_at.isoformat()

        completed_by: None | str | Unset
        if isinstance(self.completed_by, Unset):
            completed_by = UNSET
        else:
            completed_by = self.completed_by

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "completed": completed,
            }
        )
        if completed_at is not UNSET:
            field_dict["completedAt"] = completed_at
        if completed_by is not UNSET:
            field_dict["completedBy"] = completed_by

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        completed = d.pop("completed")

        _completed_at = d.pop("completedAt", UNSET)
        completed_at: datetime.datetime | Unset
        if isinstance(_completed_at, Unset):
            completed_at = UNSET
        else:
            completed_at = datetime.datetime.fromisoformat(_completed_at)

        def _parse_completed_by(data: object) -> None | str | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(None | str | Unset, data)

        completed_by = _parse_completed_by(d.pop("completedBy", UNSET))

        setup_state = cls(
            completed=completed,
            completed_at=completed_at,
            completed_by=completed_by,
        )

        setup_state.additional_properties = d
        return setup_state

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
